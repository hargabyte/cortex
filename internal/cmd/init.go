// Package cmd implements the init command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .cx directory and database",
	Long: `Initialize the .cx directory and cortex.db database in the current directory.

This creates the necessary structure for cx to track code entities and their
relationships. The database stores entities, dependencies, metrics, and links.

Examples:
  cx init          # Initialize in current directory
  cx init --force  # Reinitialize (overwrites existing database)`,
	RunE: runInit,
}

var initForce bool

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initForce, "force", false, "Reinitialize even if .cx already exists")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cxDir := filepath.Join(cwd, ".cx")
	dbPath := filepath.Join(cxDir, "cortex.db")

	// Check if database already exists
	_, err = os.Stat(dbPath)
	if err == nil {
		// Database exists
		if !initForce {
			// Not forcing, so report status and exit cleanly
			relPath, _ := filepath.Rel(cwd, cxDir)
			fmt.Printf("Already initialized at %s\n", relPath)
			return nil
		}
		// Force flag set, remove old database to reinitialize
		if err := os.Remove(dbPath); err != nil {
			return fmt.Errorf("removing existing database: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Some other error occurred checking the file
		return fmt.Errorf("checking database path: %w", err)
	}

	// Open store to create .cx directory and initialize schema
	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer storeDB.Close()

	// Print success message with database path
	relPath, _ := filepath.Rel(cwd, cxDir)
	fmt.Printf("Initialized cx database at %s\n", relPath)

	return nil
}
