package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Commands for managing the .cx/cortex.db database.`,
}

var dbInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show database statistics",
	Long:  `Display statistics about the database including entity counts, dependencies, file index, and database size.`,
	RunE:  runDbInfo,
}

var dbCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact the database",
	Long:  `Run VACUUM on the database to reclaim unused space and optionally remove archived entities.`,
	RunE:  runDbCompact,
}

var dbExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export database to JSONL",
	Long:  `Export all entities and dependencies to JSONL format (JSON Lines). Outputs to stdout by default or to a file if --output is specified.`,
	RunE:  runDbExport,
}

// dbStatusCmd is a top-level alias for "db info" - more intuitive for AI agents
var dbStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show database status (alias for 'db info')",
	Long:    `Display database statistics including entity counts, dependencies, and file index. This is an alias for 'cx db info'.`,
	RunE:    runDbInfo,
}

// Flags
var (
	compactRemoveArchived bool
	compactDryRun         bool
	exportOutput          string
)

// dbDoctorCmd is an alias for the top-level doctor command
var dbDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check database health (alias for 'cx doctor')",
	Long: `Run health checks on the .cx/cortex.db database.

This is an alias for 'cx doctor'. See 'cx doctor --help' for full documentation.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(dbStatusCmd) // Top-level status alias
	dbCmd.AddCommand(dbInfoCmd)
	dbCmd.AddCommand(dbCompactCmd)
	dbCmd.AddCommand(dbExportCmd)
	dbCmd.AddCommand(dbDoctorCmd)

	dbCompactCmd.Flags().BoolVar(&compactRemoveArchived, "remove-archived", false, "Remove archived entities before compacting")
	dbCompactCmd.Flags().BoolVar(&compactDryRun, "dry-run", false, "Show what would be done without making changes")
	dbExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	// db doctor shares flags with top-level doctor command
	dbDoctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Auto-fix issues found")
	dbDoctorCmd.Flags().BoolVar(&doctorDeep, "deep", false, "Run deep checks including archived entity ratio")
	dbDoctorCmd.Flags().BoolVar(&doctorYes, "yes", false, "Auto-confirm fixes without prompting")

	// Deprecate top-level status command
	DeprecateCommand(dbStatusCmd, DeprecationInfo{
		OldCommand: "cx status",
		NewCommand: "cx db info",
	})
}

func runDbInfo(cmd *cobra.Command, args []string) error {
	s, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	// Get database path
	dbPath := s.Path()

	// Get file size
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("stat database file: %w", err)
	}
	dbSize := fileInfo.Size()

	// Count entities
	totalEntities, err := s.CountEntities(store.EntityFilter{})
	if err != nil {
		return fmt.Errorf("count total entities: %w", err)
	}

	activeEntities, err := s.CountEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("count active entities: %w", err)
	}

	archivedEntities, err := s.CountEntities(store.EntityFilter{Status: "archived"})
	if err != nil {
		return fmt.Errorf("count archived entities: %w", err)
	}

	// Count dependencies
	depCount, err := s.CountDependencies()
	if err != nil {
		return fmt.Errorf("count dependencies: %w", err)
	}

	// Count files
	fileCount, err := s.CountFileIndex()
	if err != nil {
		return fmt.Errorf("count file index: %w", err)
	}

	// Count external links
	linkCount, err := s.CountLinks()
	if err != nil {
		return fmt.Errorf("count links: %w", err)
	}

	// Format file size
	sizeStr := formatBytes(dbSize)

	// Print statistics
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Size: %s\n", sizeStr)
	fmt.Println()
	fmt.Printf("Entities:     %d (active: %d, archived: %d)\n", totalEntities, activeEntities, archivedEntities)
	fmt.Printf("Dependencies: %d\n", depCount)
	fmt.Printf("Files indexed: %d\n", fileCount)
	fmt.Printf("External links: %d\n", linkCount)

	return nil
}

func runDbCompact(cmd *cobra.Command, args []string) error {
	s, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if compactDryRun {
		fmt.Println("[dry-run] Would perform the following operations:")
	}

	// Remove archived entities if requested
	if compactRemoveArchived {
		// Get archived entities
		archivedEntities, err := s.QueryEntities(store.EntityFilter{Status: "archived"})
		if err != nil {
			return fmt.Errorf("query archived entities: %w", err)
		}

		if compactDryRun {
			fmt.Printf("[dry-run] Would remove %d archived entities\n", len(archivedEntities))
			for i, entity := range archivedEntities {
				if i >= 10 {
					fmt.Printf("[dry-run]   ... and %d more\n", len(archivedEntities)-10)
					break
				}
				fmt.Printf("[dry-run]   - %s (%s)\n", entity.Name, entity.ID)
			}
		} else {
			fmt.Println("Removing archived entities...")
			// Delete each archived entity
			deletedCount := 0
			for _, entity := range archivedEntities {
				if err := s.DeleteEntity(entity.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to delete entity %s: %v\n", entity.ID, err)
				} else {
					deletedCount++
				}
			}
			fmt.Printf("Removed %d archived entities\n", deletedCount)
		}
	}

	// Get size before compacting
	dbPath := s.Path()
	beforeInfo, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("stat database before compact: %w", err)
	}
	sizeBefore := beforeInfo.Size()

	if compactDryRun {
		fmt.Printf("[dry-run] Would run VACUUM on database (%s)\n", formatBytes(sizeBefore))
		fmt.Println("[dry-run] No changes made")
		return nil
	}

	// Run VACUUM
	fmt.Println("Compacting database...")
	db := s.DB()
	if _, err := db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum database: %w", err)
	}

	// Get size after compacting
	afterInfo, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("stat database after compact: %w", err)
	}
	sizeAfter := afterInfo.Size()

	// Calculate space saved
	spaceSaved := sizeBefore - sizeAfter
	percentSaved := 0.0
	if sizeBefore > 0 {
		percentSaved = (float64(spaceSaved) / float64(sizeBefore)) * 100
	}

	fmt.Printf("Database compacted successfully\n")
	fmt.Printf("Size before: %s\n", formatBytes(sizeBefore))
	fmt.Printf("Size after:  %s\n", formatBytes(sizeAfter))
	fmt.Printf("Space saved: %s (%.1f%%)\n", formatBytes(spaceSaved), percentSaved)

	return nil
}

func runDbExport(cmd *cobra.Command, args []string) error {
	s, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	// Determine output writer
	var out *os.File
	if exportOutput == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(exportOutput)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	encoder := json.NewEncoder(out)

	// Export entities
	entities, err := s.QueryEntities(store.EntityFilter{})
	if err != nil {
		return fmt.Errorf("query entities: %w", err)
	}

	for _, entity := range entities {
		line := map[string]interface{}{
			"type": "entity",
			"data": entity,
		}
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("encode entity: %w", err)
		}
	}

	// Export dependencies
	deps, err := s.GetAllDependencies()
	if err != nil {
		return fmt.Errorf("get dependencies: %w", err)
	}

	for _, dep := range deps {
		line := map[string]interface{}{
			"type": "dependency",
			"data": dep,
		}
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("encode dependency: %w", err)
		}
	}

	// If writing to file, print success message
	if exportOutput != "" {
		fmt.Fprintf(os.Stderr, "Exported %d entities and %d dependencies to %s\n",
			len(entities), len(deps), exportOutput)
	}

	return nil
}

// formatBytes formats a byte count into a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
