// Package cmd implements the prime command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// primeCmd represents the prime command
var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output essential Cortex workflow context for AI agents",
	Long: `Output essential Cortex workflow context in AI-optimized markdown format.

Designed for Claude Code hooks (SessionStart, PreCompact) to provide
agents with quick reference for cx commands after context compaction.

Examples:
  cx prime              # Output context for current project
  cx prime --full       # Force full output (ignore token limits)
  cx prime --export     # Output default content for customization`,
	RunE: runPrime,
}

var (
	primeFull   bool
	primeExport bool
)

func init() {
	rootCmd.AddCommand(primeCmd)
	primeCmd.Flags().BoolVar(&primeFull, "full", false, "Force full output")
	primeCmd.Flags().BoolVar(&primeExport, "export", false, "Output default content (ignores PRIME.md override)")
}

func runPrime(cmd *cobra.Command, args []string) error {
	// Check for custom PRIME.md override (unless --export)
	if !primeExport {
		cwd, _ := os.Getwd()
		customPath := filepath.Join(cwd, ".cx", "PRIME.md")
		if content, err := os.ReadFile(customPath); err == nil {
			fmt.Print(string(content))
			return nil
		}
	}

	// Get database info if available
	dbInfo := getPrimeDBInfo()

	// Output the prime context
	fmt.Print(generatePrimeContent(dbInfo))
	return nil
}

type primeDBStats struct {
	exists       bool
	path         string
	entities     int
	active       int
	archived     int
	dependencies int
	files        int
}

func getPrimeDBInfo() primeDBStats {
	cwd, err := os.Getwd()
	if err != nil {
		return primeDBStats{}
	}

	cxDir := filepath.Join(cwd, ".cx")

	// Check if .cx directory exists (don't auto-create)
	if _, err := os.Stat(cxDir); os.IsNotExist(err) {
		return primeDBStats{exists: false}
	}

	dbPath := filepath.Join(cxDir, "cortex.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return primeDBStats{exists: false}
	}

	s, err := store.Open(cxDir)
	if err != nil {
		return primeDBStats{exists: true, path: dbPath}
	}
	defer s.Close()

	stats := primeDBStats{
		exists: true,
		path:   dbPath,
	}

	// Get entity counts using store methods
	stats.active, _ = s.CountEntities(store.EntityFilter{Status: "active"})
	stats.archived, _ = s.CountEntities(store.EntityFilter{Status: "archived"})
	stats.entities = stats.active + stats.archived
	stats.dependencies, _ = s.CountDependencies()
	stats.files, _ = s.CountFileIndex()

	return stats
}

func generatePrimeContent(stats primeDBStats) string {
	content := `# Cortex (cx) Workflow Context

> **Context Recovery**: Run ` + "`cx prime`" + ` after compaction, clear, or new session

## Status
`

	if stats.exists {
		content += fmt.Sprintf(`- **Database**: Initialized
- **Entities**: %d active, %d archived
- **Dependencies**: %d tracked
- **Files indexed**: %d
`, stats.active, stats.archived, stats.dependencies, stats.files)
	} else {
		content += `- **Database**: Not initialized
- Run ` + "`cx init && cx scan`" + ` to enable code graph
`
	}

	content += `
## Core Commands

### Discovery
- ` + "`cx find <pattern>`" + ` - Search entities by name
- ` + "`cx show <name>`" + ` - Entity details + dependencies
- ` + "`cx near <name>`" + ` - Explore neighborhood (callers, callees)
- ` + "`cx rank --keystones`" + ` - Find critical entities (high PageRank)

### Analysis
- ` + "`cx impact <file>`" + ` - Blast radius: what breaks if this changes?
- ` + "`cx graph <name> --hops 2`" + ` - Dependency graph visualization
- ` + "`cx context --smart \"<task>\"`" + ` - Get focused context for a task

### Maintenance
- ` + "`cx scan`" + ` - Rescan codebase after changes
- ` + "`cx doctor`" + ` - Health check
- ` + "`cx db info`" + ` - Database statistics

## Usage Patterns

**Before starting any coding task:**
` + "```bash" + `
cx context --smart "add rate limiting to API" --budget 8000
` + "```" + `

**Before modifying a file:**
` + "```bash" + `
cx impact src/auth/login.go
` + "```" + `

**Understanding unfamiliar code:**
` + "```bash" + `
cx find UserService
cx show UserService
cx near UserService --depth 2
` + "```" + `

## Notes
- cx uses SQLite + tree-sitter for code graph
- Run ` + "`cx scan`" + ` after major code changes
- Supports Go (TypeScript, Python, Rust, Java coming)
- Use ` + "`cx doctor`" + ` if something seems wrong
`

	return content
}
