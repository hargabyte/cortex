// Package cmd implements the quickstart command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// quickstartCmd represents the quickstart command
var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Initialize, scan, and show project overview in one command",
	Long: `Complete setup for a new project in a single command.

Quickstart runs the full initialization sequence:
  1. cx init    - Create .cx directory and database (if not exists)
  2. cx scan    - Scan codebase and build entity graph
  3. cx rank    - Compute importance metrics (PageRank)
  4. Display project summary with top keystones

This is the recommended way to set up cx on a new project.

Examples:
  cx quickstart                    # Full setup
  cx quickstart --force            # Reinitialize existing project
  cx quickstart --with-coverage    # Also prompt for coverage import
  cx quickstart --quiet            # Minimal output`,
	RunE: runQuickstart,
}

var (
	quickstartForce        bool
	quickstartWithCoverage bool
	quickstartQuiet        bool
)

func init() {
	rootCmd.AddCommand(quickstartCmd)
	quickstartCmd.Flags().BoolVar(&quickstartForce, "force", false, "Reinitialize even if .cx already exists")
	quickstartCmd.Flags().BoolVar(&quickstartWithCoverage, "with-coverage", false, "Prompt for coverage file import")
	quickstartCmd.Flags().BoolVar(&quickstartQuiet, "quiet", false, "Minimal output (no summary)")

	// Deprecate: use scan --overview instead
	DeprecateCommand(quickstartCmd, DeprecationInfo{
		OldCommand: "cx quickstart",
		NewCommand: "cx scan",
		NewFlags:   "--overview",
	})
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cxDir := filepath.Join(cwd, ".cx")
	dbPath := filepath.Join(cxDir, "cortex.db")

	// Step 1: Initialize
	if !quickstartQuiet {
		fmt.Println("⚡ CX Quickstart")
		fmt.Println()
	}

	// Check if already initialized
	_, err = os.Stat(dbPath)
	dbExists := err == nil

	if dbExists && !quickstartForce {
		if !quickstartQuiet {
			fmt.Println("✓ Database already initialized")
		}
	} else {
		if dbExists && quickstartForce {
			if err := os.Remove(dbPath); err != nil {
				return fmt.Errorf("removing existing database: %w", err)
			}
		}

		if !quickstartQuiet {
			fmt.Print("○ Initializing database... ")
		}

		s, err := store.Open(cxDir)
		if err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}
		s.Close()

		if !quickstartQuiet {
			fmt.Println("✓")
		}
	}

	// Step 2: Scan - call the scan command
	if !quickstartQuiet {
		fmt.Print("○ Scanning codebase... ")
	}

	// Temporarily suppress output for scan
	oldQuiet := quiet
	quiet = true
	scanForce = quickstartForce
	err = runScan(cmd, []string{cwd})
	quiet = oldQuiet

	if err != nil {
		if !quickstartQuiet {
			fmt.Printf("⚠ (%v)\n", err)
		}
	} else {
		if !quickstartQuiet {
			fmt.Println("✓")
		}
	}

	// Step 3: Compute metrics - call the rank command
	if !quickstartQuiet {
		fmt.Print("○ Computing importance metrics... ")
	}

	// Call rank with recompute flag
	rankRecompute = true
	err = runRank(cmd, []string{})
	rankRecompute = false

	if err != nil {
		if !quickstartQuiet {
			fmt.Printf("⚠ (%v)\n", err)
		}
	} else {
		if !quickstartQuiet {
			fmt.Println("✓")
		}
	}

	// Open store for summary
	s, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	// Step 4: Coverage prompt
	if quickstartWithCoverage {
		if !quickstartQuiet {
			fmt.Println()
			fmt.Println("Coverage Import:")
			fmt.Println("  1. Run: go test -coverprofile=coverage.out ./...")
			fmt.Println("  2. Run: cx coverage import coverage.out")
		}
	}

	// Step 5: Summary
	if !quickstartQuiet {
		fmt.Println()
		fmt.Println("─────────────────────────────────────────")
		printQuickstartSummary(s, cwd)
	}

	return nil
}

func printQuickstartSummary(s *store.Store, cwd string) {
	// Get stats
	activeCount, _ := s.CountEntities(store.EntityFilter{Status: "active"})
	depCount, _ := s.CountDependencies()
	fileCount, _ := s.CountFileIndex()

	// Count by type
	funcCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "function"})
	methodCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "method"})
	typeCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "struct"})
	typeCount2, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "interface"})
	typeCount += typeCount2

	fmt.Printf("Project: %s\n", filepath.Base(cwd))
	fmt.Printf("Entities: %d (functions: %d, methods: %d, types: %d)\n",
		activeCount, funcCount, methodCount, typeCount)
	fmt.Printf("Dependencies: %d\n", depCount)
	fmt.Printf("Files indexed: %d\n", fileCount)
	fmt.Println()

	// Get top keystones by PageRank
	topMetrics, err := s.GetTopByPageRank(5)
	if err == nil && len(topMetrics) > 0 {
		fmt.Println("Top Keystones:")
		for i, m := range topMetrics {
			e, err := s.GetEntity(m.EntityID)
			if err != nil || e == nil {
				continue
			}
			loc := formatLocationShort(e.FilePath, e.LineStart)
			fmt.Printf("  %d. %s (%s) @ %s\n", i+1, e.Name, e.EntityType, loc)
		}
		fmt.Println()
	}

	fmt.Println("Next steps:")
	fmt.Println("  cx context --smart \"<your task>\" --budget 8000")
	fmt.Println("  cx impact <file-to-modify>")
	fmt.Println("  cx map                    # project skeleton")
}

func formatLocationShort(filePath string, line int) string {
	// Get just the filename and line
	parts := strings.Split(filePath, "/")
	if len(parts) > 2 {
		// Show last 2 path components
		filePath = strings.Join(parts[len(parts)-2:], "/")
	}
	return fmt.Sprintf("%s:%d", filePath, line)
}
