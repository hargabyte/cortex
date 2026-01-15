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

Output Modes:
  (default)    Concise context (~500 tokens)
  --full       Extended context with map skeleton (~2000 tokens)
  --export     Output default template for customization

Examples:
  cx prime              # Output context for current project
  cx prime --full       # Include project skeleton and keystones
  cx prime --export     # Output default content for customization`,
	RunE: runPrime,
}

var (
	primeFull   bool
	primeExport bool
)

func init() {
	rootCmd.AddCommand(primeCmd)
	primeCmd.Flags().BoolVar(&primeFull, "full", false, "Extended output with keystones and map")
	primeCmd.Flags().BoolVar(&primeExport, "export", false, "Output default content (ignores PRIME.md override)")

	// Deprecate prime command - now part of context
	DeprecateCommand(primeCmd, DeprecationInfo{
		OldCommand: "cx prime",
		NewCommand: "cx context",
	})
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
	fmt.Print(generatePrimeContent(dbInfo, primeFull))
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

func generatePrimeContent(stats primeDBStats, full bool) string {
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

		// Add keystones if --full and we have entities
		if full && stats.active > 0 {
			content += getKeystonesSection(stats.path)
		}
	} else {
		content += `- **Database**: Not initialized
- Run ` + "`cx quickstart`" + ` to enable code graph (or ` + "`cx init && cx scan`" + `)
`
	}

	content += `
## Essential Commands

` + "```bash" + `
# Start of session
cx prime                                    # This context

# Before ANY coding task
cx context --smart "<task>" --budget 8000   # Focused context

# Before modifying code
cx impact <file>                            # Blast radius

# Project overview
cx map                                      # Skeleton (~10k tokens)
cx rank --keystones                         # Critical entities
` + "```" + `

## Discovery & Analysis

| Command | Purpose |
|---------|---------|
| ` + "`cx find <name>`" + ` | Name search (--type=F/T/M, --exact) |
| ` + "`cx search \"query\"`" + ` | Concept search (--top N) |
| ` + "`cx show <name>`" + ` | Entity details (--density dense) |
| ` + "`cx near <name>`" + ` | Neighborhood (--depth N) |
| ` + "`cx graph <name>`" + ` | Dependencies (--hops N) |
| ` + "`cx impact <file>`" + ` | Blast radius (--depth N) |
| ` + "`cx diff`" + ` | Changes since scan |

## Quality & Testing

| Command | Purpose |
|---------|---------|
| ` + "`cx coverage import`" + ` | Import coverage.out |
| ` + "`cx gaps --keystones-only`" + ` | Undertested critical code |
| ` + "`cx test-impact --diff`" + ` | Smart test selection |
| ` + "`cx verify --strict`" + ` | Check drift (CI) |

## Quick Patterns

` + "```bash" + `
# Understand codebase
cx map && cx rank --keystones --top 10

# Before refactoring
cx impact <file> && cx gaps --keystones-only

# Smart testing
cx test-impact --diff --output-command
` + "```" + `

## Notes
- Supports: Go, TypeScript, JavaScript, Java, Rust, Python
- Run ` + "`cx scan`" + ` after major code changes
- Run ` + "`cx help-agents`" + ` for full agent reference
`

	return content
}

// getKeystonesSection returns a markdown section with top keystones
func getKeystonesSection(dbPath string) string {
	cxDir := filepath.Dir(dbPath)
	s, err := store.Open(cxDir)
	if err != nil {
		return ""
	}
	defer s.Close()

	// Get top 5 by PageRank
	topMetrics, err := s.GetTopByPageRank(5)
	if err != nil || len(topMetrics) == 0 {
		return ""
	}

	content := "\n**Top Keystones:**\n"
	for _, m := range topMetrics {
		// Fetch entity details
		e, err := s.GetEntity(m.EntityID)
		if err != nil || e == nil {
			continue
		}

		// Shorten the file path - keep last 2 components
		shortPath := e.FilePath
		parts := splitPath(shortPath)
		if len(parts) > 2 {
			shortPath = parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
		content += fmt.Sprintf("- `%s` (%s) @ %s:%d\n", e.Name, e.EntityType, shortPath, e.LineStart)
	}

	return content
}

// splitPath splits a path into components (works cross-platform)
func splitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == path {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}
