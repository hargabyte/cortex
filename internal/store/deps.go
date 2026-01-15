package store

import "time"

// CreateDependency inserts a single dependency.
// Uses INSERT OR REPLACE to handle duplicates gracefully.
func (s *Store) CreateDependency(d *Dependency) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO dependencies (from_id, to_id, dep_type, created_at)
		VALUES (?, ?, ?, ?)`,
		d.FromID, d.ToID, d.DepType, now)
	return err
}

// CreateDependenciesBulk inserts multiple dependencies in a single transaction.
// Uses prepared statement for efficiency.
func (s *Store) CreateDependenciesBulk(deps []*Dependency) error {
	if len(deps) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO dependencies (from_id, to_id, dep_type, created_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, d := range deps {
		_, err := stmt.Exec(d.FromID, d.ToID, d.DepType, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetDependencies returns dependencies matching the filter.
// If filter.FromID is set, returns dependencies FROM that entity.
// If filter.ToID is set, returns dependencies TO that entity.
// If filter.DepType is set, filters by dependency type.
func (s *Store) GetDependencies(filter DependencyFilter) ([]*Dependency, error) {
	query := `SELECT from_id, to_id, dep_type, created_at FROM dependencies WHERE 1=1`
	args := []interface{}{}

	if filter.FromID != "" {
		query += " AND from_id = ?"
		args = append(args, filter.FromID)
	}
	if filter.ToID != "" {
		query += " AND to_id = ?"
		args = append(args, filter.ToID)
	}
	if filter.DepType != "" {
		query += " AND dep_type = ?"
		args = append(args, filter.DepType)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []*Dependency
	for rows.Next() {
		var d Dependency
		var createdAt string
		if err := rows.Scan(&d.FromID, &d.ToID, &d.DepType, &createdAt); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		deps = append(deps, &d)
	}

	return deps, rows.Err()
}

// GetDependenciesFrom returns all dependencies from a specific entity.
func (s *Store) GetDependenciesFrom(entityID string) ([]*Dependency, error) {
	return s.GetDependencies(DependencyFilter{FromID: entityID})
}

// GetDependenciesTo returns all dependencies to a specific entity (callers/users).
func (s *Store) GetDependenciesTo(entityID string) ([]*Dependency, error) {
	return s.GetDependencies(DependencyFilter{ToID: entityID})
}

// DeleteDependency removes a specific dependency.
func (s *Store) DeleteDependency(fromID, toID, depType string) error {
	_, err := s.db.Exec(`
		DELETE FROM dependencies
		WHERE from_id = ? AND to_id = ? AND dep_type = ?`,
		fromID, toID, depType)
	return err
}

// DeleteDependenciesFrom removes all dependencies from an entity.
// Used when rescanning - removes old call graph edges.
func (s *Store) DeleteDependenciesFrom(entityID string) error {
	_, err := s.db.Exec(`
		DELETE FROM dependencies
		WHERE from_id = ?`,
		entityID)
	return err
}

// DeleteDependenciesByFile removes all dependencies where from_id matches an entity in the file.
// Used when rescanning a file to clean up old edges.
func (s *Store) DeleteDependenciesByFile(filePath string) error {
	_, err := s.db.Exec(`
		DELETE FROM dependencies
		WHERE from_id IN (SELECT id FROM entities WHERE file_path = ?)`,
		filePath)
	return err
}

// GetAllDependencies returns all dependencies in the database.
// Used for building the full graph.
func (s *Store) GetAllDependencies() ([]*Dependency, error) {
	return s.GetDependencies(DependencyFilter{})
}

// CountDependencies returns the total number of dependencies.
func (s *Store) CountDependencies() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM dependencies`).Scan(&count)
	return count, err
}
