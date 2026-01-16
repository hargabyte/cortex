// Package cache provides SQLite-backed caching for computed metrics and file indexing.
// The cache is stored in .cx/cache.db and provides efficient storage and retrieval
// of PageRank, betweenness centrality, and other graph metrics.
package cache

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Cache manages the .cx/cache.db SQLite database for storing computed metrics
// and file scan state.
type Cache struct {
	db     *sql.DB
	dbPath string
}

// Open opens or creates the cache database at the specified .cx directory.
// It initializes the schema if the database is new.
func Open(cxDir string) (*Cache, error) {
	dbPath := filepath.Join(cxDir, "cache.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	cache := &Cache{db: db, dbPath: dbPath}

	// Initialize schema
	if err := cache.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return cache, nil
}

// Close closes the database connection.
func (c *Cache) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Clear removes all cached data from both metrics and file_index tables.
func (c *Cache) Clear() error {
	_, err := c.db.Exec("DELETE FROM metrics; DELETE FROM file_index;")
	if err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}
	return nil
}

// ClearMetrics removes all cached metrics data.
func (c *Cache) ClearMetrics() error {
	_, err := c.db.Exec("DELETE FROM metrics")
	if err != nil {
		return fmt.Errorf("clear metrics: %w", err)
	}
	return nil
}

// ClearFileIndex removes all file index data.
func (c *Cache) ClearFileIndex() error {
	_, err := c.db.Exec("DELETE FROM file_index")
	if err != nil {
		return fmt.Errorf("clear file index: %w", err)
	}
	return nil
}

// Path returns the database file path.
func (c *Cache) Path() string {
	return c.dbPath
}

// DB returns the underlying database connection for advanced operations.
func (c *Cache) DB() *sql.DB {
	return c.db
}

// Stats returns cache statistics.
type Stats struct {
	MetricsCount   int64
	FileIndexCount int64
}

// GetStats returns statistics about the cache contents.
func (c *Cache) GetStats() (*Stats, error) {
	var stats Stats

	err := c.db.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&stats.MetricsCount)
	if err != nil {
		return nil, fmt.Errorf("count metrics: %w", err)
	}

	err = c.db.QueryRow("SELECT COUNT(*) FROM file_index").Scan(&stats.FileIndexCount)
	if err != nil {
		return nil, fmt.Errorf("count file index: %w", err)
	}

	return &stats, nil
}
