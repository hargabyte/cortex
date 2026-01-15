package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check database health",
	Long: `Run health checks on the .cx/cortex.db database.

Checks:
  - Database integrity (SQLite integrity_check)
  - Orphan dependencies (referencing deleted entities)
  - Stale entities (in files that no longer exist)

Examples:
  cx doctor        # Run all checks
  cx doctor --fix  # Run checks and auto-fix issues`,
	RunE: runDoctor,
}

var doctorFix bool

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Auto-fix issues found")
}

type doctorResult struct {
	passed       bool
	issueCount   int
	issueDetails []string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Open store
	st, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	db := st.DB()

	fmt.Println("# cx doctor")
	totalIssues := 0

	// Check 1: Database integrity
	fmt.Println("# Checking database integrity...")
	result := checkIntegrity(db)
	if result.passed {
		fmt.Println("#   ✓ Database integrity OK")
	} else {
		fmt.Printf("#   ✗ Database integrity check failed\n")
		for _, detail := range result.issueDetails {
			fmt.Printf("#     - %s\n", detail)
		}
		totalIssues += result.issueCount
	}

	// Check 2: Orphan dependencies
	fmt.Println("# Checking for orphan dependencies...")
	result = checkOrphanDependencies(st, db)
	if result.passed {
		fmt.Println("#   ✓ No orphan dependencies found")
	} else {
		fmt.Printf("#   ⚠ Found %d orphan dependencies\n", result.issueCount)
		for _, detail := range result.issueDetails {
			fmt.Printf("#     - %s\n", detail)
		}
		totalIssues += result.issueCount
	}

	// Check 3: Stale entities
	fmt.Println("# Checking for stale entities...")
	result = checkStaleEntities(st)
	if result.passed {
		fmt.Println("#   ✓ No stale entities found")
	} else {
		fmt.Printf("#   ⚠ Found %d stale entities\n", result.issueCount)
		for _, detail := range result.issueDetails {
			fmt.Printf("#     - %s\n", detail)
		}
		totalIssues += result.issueCount
	}

	// Summary
	fmt.Println("#")
	if totalIssues == 0 {
		fmt.Println("# Summary: All checks passed ✓")
	} else {
		fmt.Printf("# Summary: %d issue(s) found\n", totalIssues)
		if !doctorFix {
			fmt.Println("# Run with --fix to repair")
		}
	}

	return nil
}

// checkIntegrity runs SQLite's PRAGMA integrity_check
func checkIntegrity(db *sql.DB) doctorResult {
	var integrityResult string
	err := db.QueryRow("PRAGMA integrity_check").Scan(&integrityResult)
	if err != nil {
		return doctorResult{
			passed:       false,
			issueCount:   1,
			issueDetails: []string{fmt.Sprintf("integrity check error: %v", err)},
		}
	}

	if integrityResult == "ok" {
		return doctorResult{passed: true}
	}

	return doctorResult{
		passed:       false,
		issueCount:   1,
		issueDetails: []string{integrityResult},
	}
}

// checkOrphanDependencies finds dependencies referencing non-existent entities
func checkOrphanDependencies(st *store.Store, db *sql.DB) doctorResult {
	query := `
		SELECT d.from_id, d.to_id, d.dep_type
		FROM dependencies d
		LEFT JOIN entities e1 ON d.from_id = e1.id
		LEFT JOIN entities e2 ON d.to_id = e2.id
		WHERE e1.id IS NULL OR e2.id IS NULL
	`

	rows, err := db.Query(query)
	if err != nil {
		return doctorResult{
			passed:       false,
			issueCount:   1,
			issueDetails: []string{fmt.Sprintf("query error: %v", err)},
		}
	}
	defer rows.Close()

	var orphans []struct {
		fromID  string
		toID    string
		depType string
	}

	for rows.Next() {
		var fromID, toID, depType string
		if err := rows.Scan(&fromID, &toID, &depType); err != nil {
			continue
		}
		orphans = append(orphans, struct {
			fromID  string
			toID    string
			depType string
		}{fromID, toID, depType})
	}

	if len(orphans) == 0 {
		return doctorResult{passed: true}
	}

	// Collect details
	var details []string
	for _, orphan := range orphans {
		details = append(details, fmt.Sprintf("%s -[%s]-> %s", orphan.fromID, orphan.depType, orphan.toID))
	}

	// Fix if requested
	if doctorFix {
		fmt.Printf("#   Removing %d orphan dependencies...\n", len(orphans))
		for _, orphan := range orphans {
			if err := st.DeleteDependency(orphan.fromID, orphan.toID, orphan.depType); err != nil {
				fmt.Printf("#     ⚠ Failed to delete %s -[%s]-> %s: %v\n",
					orphan.fromID, orphan.depType, orphan.toID, err)
			}
		}
		fmt.Printf("#   ✓ Removed %d orphan dependencies\n", len(orphans))
		// Return passed since we fixed the issue
		return doctorResult{passed: true}
	}

	return doctorResult{
		passed:       false,
		issueCount:   len(orphans),
		issueDetails: details,
	}
}

// checkStaleEntities finds entities in files that no longer exist
func checkStaleEntities(st *store.Store) doctorResult {
	// Get all active entities
	entities, err := st.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return doctorResult{
			passed:       false,
			issueCount:   1,
			issueDetails: []string{fmt.Sprintf("query error: %v", err)},
		}
	}

	var staleEntities []*store.Entity
	fileCache := make(map[string]bool) // Cache file existence checks

	for _, e := range entities {
		// Check cache first
		exists, cached := fileCache[e.FilePath]
		if !cached {
			// Check if file exists
			_, err := os.Stat(e.FilePath)
			exists = !os.IsNotExist(err)
			fileCache[e.FilePath] = exists
		}

		if !exists {
			staleEntities = append(staleEntities, e)
		}
	}

	if len(staleEntities) == 0 {
		return doctorResult{passed: true}
	}

	// Collect details (show up to 10 examples)
	var details []string
	maxShow := 10
	if len(staleEntities) < maxShow {
		maxShow = len(staleEntities)
	}

	for i := 0; i < maxShow; i++ {
		e := staleEntities[i]
		details = append(details, fmt.Sprintf("%s (%s)", e.ID, e.FilePath))
	}

	if len(staleEntities) > maxShow {
		details = append(details, fmt.Sprintf("... and %d more", len(staleEntities)-maxShow))
	}

	// Fix if requested
	if doctorFix {
		fmt.Printf("#   Archiving %d stale entities...\n", len(staleEntities))
		for _, e := range staleEntities {
			if err := st.ArchiveEntity(e.ID); err != nil {
				fmt.Printf("#     ⚠ Failed to archive %s: %v\n", e.ID, err)
			}
		}
		fmt.Printf("#   ✓ Archived %d stale entities\n", len(staleEntities))
		// Return passed since we fixed the issue
		return doctorResult{passed: true}
	}

	return doctorResult{
		passed:       false,
		issueCount:   len(staleEntities),
		issueDetails: details,
	}
}
