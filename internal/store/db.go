// Package store provides Dolt-backed persistence for cortex state and metadata.
// The store is located at .cx/cortex/ (a Dolt repository) and provides efficient
// storage with version control capabilities including history, diff, and time-travel.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/dolthub/driver"
)

// Store manages the .cx/cortex/ Dolt database for storing application state
// and metadata with version control capabilities.
type Store struct {
	db     *sql.DB
	dbPath string // Path to the Dolt repo directory (.cx/cortex/)
}

// Open opens or creates the store database at the specified .cx directory.
// It auto-creates the directory if it doesn't exist and initializes the schema
// if the database is new. The Dolt database is stored in .cx/cortex/.
func Open(cxDir string) (*Store, error) {
	// Create .cx directory if it doesn't exist
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		return nil, fmt.Errorf("create .cx directory: %w", err)
	}

	// Dolt repo lives in .cx/cortex/
	dbPath := filepath.Join(cxDir, "cortex")

	// Create the Dolt repo directory if needed
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("create dolt directory: %w", err)
	}

	// First, connect without specifying database to create it if needed
	initDSN := fmt.Sprintf("file://%s?commitname=Cortex&commitemail=cx@local", dbPath)
	initDB, err := sql.Open("dolt", initDSN)
	if err != nil {
		return nil, fmt.Errorf("open dolt for init: %w", err)
	}

	// Create database if it doesn't exist
	_, err = initDB.Exec("CREATE DATABASE IF NOT EXISTS cortex")
	if err != nil {
		initDB.Close()
		return nil, fmt.Errorf("create database: %w", err)
	}
	initDB.Close()

	// Now connect to the specific database
	dsn := fmt.Sprintf("file://%s?commitname=Cortex&commitemail=cx@local&database=cortex", dbPath)
	db, err := sql.Open("dolt", dsn)
	if err != nil {
		return nil, fmt.Errorf("open dolt db: %w", err)
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
