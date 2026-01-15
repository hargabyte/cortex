// Package store provides SQLite-backed persistence for cortex state and metadata.
// The store is located at .cx/cortex.db and provides efficient storage and retrieval
// of application state and configuration data.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store manages the .cx/cortex.db SQLite database for storing application state
// and metadata.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open opens or creates the store database at the specified .cx directory.
// It auto-creates the directory if it doesn't exist and initializes the schema
// if the database is new.
func Open(cxDir string) (*Store, error) {
	// Create .cx directory if it doesn't exist
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		return nil, fmt.Errorf("create .cx directory: %w", err)
	}

	dbPath := filepath.Join(cxDir, "cortex.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store db: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	store := &Store{db: db, dbPath: dbPath}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// OpenDefault opens the store in the default .cx directory in the current working directory.
func OpenDefault() (*Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cxDir := filepath.Join(cwd, ".cx")
	return Open(cxDir)
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying database connection for advanced operations.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.dbPath
}
