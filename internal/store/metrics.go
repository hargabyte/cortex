package store

import (
	"database/sql"
	"fmt"
	"time"
)

// SaveMetrics stores metrics for a single entity.
// If metrics for this entity already exist, they are replaced.
func (s *Store) SaveMetrics(m *Metrics) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO metrics
		(entity_id, pagerank, in_degree, out_degree, betweenness, computed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.EntityID, m.PageRank, m.InDegree, m.OutDegree, m.Betweenness,
		m.ComputedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save metrics for %s: %w", m.EntityID, err)
	}
	return nil
}

// GetMetrics retrieves metrics for a specific entity.
// Returns sql.ErrNoRows if the entity is not found.
func (s *Store) GetMetrics(entityID string) (*Metrics, error) {
	row := s.db.QueryRow(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics WHERE entity_id = ?`, entityID)

	m, err := scanMetrics(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get metrics for %s: %w", entityID, err)
	}
	return m, nil
}

// GetAllMetrics retrieves all cached metrics.
func (s *Store) GetAllMetrics() ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics ORDER BY pagerank DESC`)
	if err != nil {
		return nil, fmt.Errorf("query all metrics: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetTopByPageRank returns the top N entities by PageRank score.
func (s *Store) GetTopByPageRank(n int) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics ORDER BY pagerank DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("query top by pagerank: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetTopByBetweenness returns the top N entities by betweenness centrality.
func (s *Store) GetTopByBetweenness(n int) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics ORDER BY betweenness DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("query top by betweenness: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetTopByInDegree returns the top N entities by in-degree (most depended upon).
func (s *Store) GetTopByInDegree(n int) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics ORDER BY in_degree DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("query top by in_degree: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetTopByOutDegree returns the top N entities by out-degree (most dependencies).
func (s *Store) GetTopByOutDegree(n int) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics ORDER BY out_degree DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("query top by out_degree: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetKeystones returns entities with PageRank >= threshold.
// These are the central, highly-connected entities in the codebase.
func (s *Store) GetKeystones(threshold float64) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics WHERE pagerank >= ? ORDER BY pagerank DESC`, threshold)
	if err != nil {
		return nil, fmt.Errorf("query keystones: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetBottlenecks returns entities with betweenness >= threshold.
// These are entities that many paths flow through, making them critical points.
func (s *Store) GetBottlenecks(threshold float64) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics WHERE betweenness >= ? ORDER BY betweenness DESC`, threshold)
	if err != nil {
		return nil, fmt.Errorf("query bottlenecks: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// GetHighlyConnected returns entities with in_degree >= threshold.
// These are entities that many other entities depend on.
func (s *Store) GetHighlyConnected(threshold int) ([]*Metrics, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, pagerank, in_degree, out_degree, betweenness, computed_at
		FROM metrics WHERE in_degree >= ? ORDER BY in_degree DESC`, threshold)
	if err != nil {
		return nil, fmt.Errorf("query highly connected: %w", err)
	}
	defer rows.Close()

	return scanMetricsRows(rows)
}

// SaveBulkMetrics saves multiple metrics efficiently using a transaction.
func (s *Store) SaveBulkMetrics(metrics []*Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO metrics
		(entity_id, pagerank, in_degree, out_degree, betweenness, computed_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, m := range metrics {
		_, err := stmt.Exec(m.EntityID, m.PageRank, m.InDegree, m.OutDegree,
			m.Betweenness, m.ComputedAt.Format(time.RFC3339))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("save metrics for %s: %w", m.EntityID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// DeleteMetrics removes metrics for a specific entity.
func (s *Store) DeleteMetrics(entityID string) error {
	_, err := s.db.Exec("DELETE FROM metrics WHERE entity_id = ?", entityID)
	if err != nil {
		return fmt.Errorf("delete metrics for %s: %w", entityID, err)
	}
	return nil
}

// ClearMetrics removes all cached metrics.
func (s *Store) ClearMetrics() error {
	_, err := s.db.Exec("DELETE FROM metrics")
	if err != nil {
		return fmt.Errorf("clear metrics: %w", err)
	}
	return nil
}

// CountMetrics returns the number of entities with cached metrics.
func (s *Store) CountMetrics() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count metrics: %w", err)
	}
	return count, nil
}

// scanMetrics scans a single row into Metrics.
func scanMetrics(row *sql.Row) (*Metrics, error) {
	var m Metrics
	var computedAt string
	err := row.Scan(&m.EntityID, &m.PageRank, &m.InDegree, &m.OutDegree,
		&m.Betweenness, &computedAt)
	if err != nil {
		return nil, err
	}

	m.ComputedAt, _ = time.Parse(time.RFC3339, computedAt)
	return &m, nil
}

// scanMetricsRows scans multiple rows into a slice of Metrics.
func scanMetricsRows(rows *sql.Rows) ([]*Metrics, error) {
	var results []*Metrics
	for rows.Next() {
		var m Metrics
		var computedAt string
		err := rows.Scan(&m.EntityID, &m.PageRank, &m.InDegree, &m.OutDegree,
			&m.Betweenness, &computedAt)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		m.ComputedAt, _ = time.Parse(time.RFC3339, computedAt)
		results = append(results, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return results, nil
}
