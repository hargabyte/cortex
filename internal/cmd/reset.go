package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the database to a clean state",
	Long: `Reinitialize the .cx database for a fresh start.

By default, creates a backup before resetting. Use --no-backup to skip.

Modes:
  cx reset              # Full reset with backup
  cx reset --scan-only  # Clear file index only (fastest)
  cx reset --hard       # Delete database file entirely

Examples:
  cx reset                    # Safe reset with backup
  cx reset --scan-only        # Clear scan state, keep entities
  cx reset --hard --force     # Delete everything, no backup`,
	RunE: runReset,
}

var (
	resetForce    bool // Skip confirmation
	resetHard     bool // Delete database file entirely
	resetScanOnly bool // Only clear file index
	resetNoBackup bool // Skip backup
	resetDryRun   bool // Show what would happen
)

func init() {
	rootCmd.AddCommand(resetCmd)

	resetCmd.Flags().BoolVar(&resetForce, "force", false, "Skip confirmation prompt")
	resetCmd.Flags().BoolVar(&resetHard, "hard", false, "Delete database file entirely (requires --force)")
	resetCmd.Flags().BoolVar(&resetScanOnly, "scan-only", false, "Only clear file index (fastest, keeps entities)")
	resetCmd.Flags().BoolVar(&resetNoBackup, "no-backup", false, "Skip backup before reset")
	resetCmd.Flags().BoolVar(&resetDryRun, "dry-run", false, "Show what would be done without making changes")
}

func runReset(cmd *cobra.Command, args []string) error {
	// Find .cx directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	// Dolt repo lives in .cx/cortex/ directory
	dbPath := filepath.Join(cxDir, "cortex")

	// Check if database exists (Dolt uses a directory, not a single file)
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) || !info.IsDir() {
		return fmt.Errorf("database not found at %s", dbPath)
	}

	// Open store to get stats
	st, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	// Get current stats
	activeCount, _ := st.CountEntities(store.EntityFilter{Status: "active"})
	archivedCount, _ := st.CountEntities(store.EntityFilter{Status: "archived"})
	depCount, _ := st.CountDependencies()
	fileCount, _ := st.CountFileIndex()
	metricsCount, _ := st.CountMetrics()

	// Get database size (calculate total size of Dolt directory)
	dbSize := getDirSize(dbPath)

	// Display what will be reset
	fmt.Println("# cx reset")
	fmt.Println()
	fmt.Printf("Database: %s (%s)\n", dbPath, formatBytes(dbSize))
	fmt.Printf("Entities: %d active, %d archived\n", activeCount, archivedCount)
	fmt.Printf("Dependencies: %d\n", depCount)
	fmt.Printf("Files indexed: %d\n", fileCount)
	fmt.Printf("Metrics entries: %d\n", metricsCount)
	fmt.Println()

	// Describe the operation
	if resetScanOnly {
		fmt.Println("Mode: --scan-only (clear file index only)")
		fmt.Printf("Will clear: %d file index entries\n", fileCount)
		fmt.Println("Will keep: entities, dependencies, metrics")
	} else if resetHard {
		fmt.Println("Mode: --hard (delete entire database)")
		fmt.Printf("Will delete: %s\n", dbPath)
	} else {
		fmt.Println("Mode: full reset")
		fmt.Println("Will clear: all entities, dependencies, metrics, file index")
	}
	fmt.Println()

	// Dry-run mode - stop here
	if resetDryRun {
		fmt.Println("[dry-run] No changes made")
		st.Close()
		return nil
	}

	// Require --force for --hard mode
	if resetHard && !resetForce {
		st.Close()
		return fmt.Errorf("--hard requires --force flag to confirm deletion")
	}

	// Confirm unless --force
	if !resetForce {
		fmt.Print("Continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			st.Close()
			fmt.Println("Reset cancelled")
			return nil
		}
	}

	// Backup unless --no-backup (skip for --scan-only)
	if !resetNoBackup && !resetScanOnly {
		backupPath := filepath.Join(cxDir, fmt.Sprintf("backup-%s.jsonl", time.Now().Format("20060102-150405")))
		fmt.Printf("Creating backup: %s\n", backupPath)

		f, err := os.Create(backupPath)
		if err != nil {
			st.Close()
			return fmt.Errorf("failed to create backup file: %w", err)
		}

		// Export entities
		entities, _ := st.QueryEntities(store.EntityFilter{})
		for _, e := range entities {
			fmt.Fprintf(f, `{"type":"entity","data":{"id":"%s","name":"%s","file_path":"%s"}}`+"\n",
				e.ID, e.Name, e.FilePath)
		}

		// Export dependencies
		deps, _ := st.GetAllDependencies()
		for _, d := range deps {
			fmt.Fprintf(f, `{"type":"dependency","data":{"from_id":"%s","to_id":"%s","dep_type":"%s"}}`+"\n",
				d.FromID, d.ToID, d.DepType)
		}

		f.Close()
		fmt.Printf("Backup created: %d entities, %d dependencies\n", len(entities), len(deps))
	}

	// Execute reset based on mode
	if resetHard {
		// Close store before deleting directory
		st.Close()

		fmt.Println("Deleting database directory...")
		if err := os.RemoveAll(dbPath); err != nil {
			return fmt.Errorf("failed to delete database: %w", err)
		}

		fmt.Println("Database deleted successfully")
		fmt.Println()
		fmt.Println("Run 'cx scan' to rebuild the code graph")
		return nil
	}

	if resetScanOnly {
		fmt.Println("Clearing file index...")
		if err := st.ClearFileIndex(); err != nil {
			st.Close()
			return fmt.Errorf("failed to clear file index: %w", err)
		}

		st.Close()
		fmt.Println("File index cleared successfully")
		fmt.Println()
		fmt.Println("Run 'cx scan' to rescan the codebase")
		return nil
	}

	// Full reset: clear everything
	fmt.Println("Clearing metrics...")
	if err := st.ClearMetrics(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear metrics: %v\n", err)
	}

	fmt.Println("Clearing file index...")
	if err := st.ClearFileIndex(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear file index: %v\n", err)
	}

	fmt.Println("Deleting dependencies...")
	db := st.DB()
	if _, err := db.Exec("DELETE FROM dependencies"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete dependencies: %v\n", err)
	}

	fmt.Println("Deleting entities...")
	if _, err := db.Exec("DELETE FROM entities"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete entities: %v\n", err)
	}

	fmt.Println("Deleting links...")
	if _, err := db.Exec("DELETE FROM entity_links"); err != nil {
		// Table might not exist in older databases
	}

	fmt.Println("Vacuuming database...")
	if _, err := db.Exec("VACUUM"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: vacuum failed: %v\n", err)
	}

	st.Close()

	// Get new database size
	newSize := getDirSize(dbPath)

	fmt.Println()
	fmt.Println("Database reset successfully")
	fmt.Printf("Size: %s -> %s\n", formatBytes(dbSize), formatBytes(newSize))
	fmt.Println()
	fmt.Println("Run 'cx scan' to rebuild the code graph")

	return nil
}

// getDirSize calculates the total size of a directory and its contents.
func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
