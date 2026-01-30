package cmd

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show Dolt commit history with entity statistics",
	Long: `Display the commit history of the Cortex code graph database.

Shows commits from the dolt_log system table with statistics about
entities and dependencies at each commit point.

Each entry includes:
  - Commit hash (short form)
  - Date and time
  - Commit message
  - Entity and dependency counts (with --stats)

Arguments:
  None

Flags:
  --limit N      Number of commits to show (default: 10)
  --stats        Include entity/dependency counts per commit
  --format       Output format: yaml|json (default: yaml)

Examples:
  cx history                    # Show last 10 commits
  cx history --limit 20         # Show last 20 commits
  cx history --stats            # Include entity counts
  cx history --format json      # JSON output for parsing`,
	RunE: runHistory,
}

var (
	historyLimit int
	historyStats bool
)

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.Flags().IntVar(&historyLimit, "limit", 10, "Number of commits to show")
	historyCmd.Flags().BoolVar(&historyStats, "stats", false, "Include entity/dependency counts per commit")
}

// HistoryEntry represents a single commit in the history output
type HistoryEntry struct {
	Commit    string        `yaml:"commit" json:"commit"`
	Date      string        `yaml:"date" json:"date"`
	Message   string        `yaml:"message" json:"message"`
	Committer string        `yaml:"committer,omitempty" json:"committer,omitempty"`
	Stats     *HistoryStats `yaml:"stats,omitempty" json:"stats,omitempty"`
}

// HistoryStats contains entity/dependency counts at a commit
type HistoryStats struct {
	Entities     int `yaml:"entities" json:"entities"`
	Dependencies int `yaml:"dependencies" json:"dependencies"`
}

// HistoryOutput is the full output structure
type HistoryOutput struct {
	Commits []HistoryEntry `yaml:"commits" json:"commits"`
	Total   int            `yaml:"total" json:"total"`
}

func runHistory(cmd *cobra.Command, args []string) error {
	// Find config directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	// Open store
	st, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Get commit history
	entries, err := st.DoltLog(historyLimit)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found. Run 'cx scan' to create the initial commit.")
		return nil
	}

	// Build output
	historyOut := HistoryOutput{
		Commits: make([]HistoryEntry, 0, len(entries)),
		Total:   len(entries),
	}

	for _, entry := range entries {
		he := HistoryEntry{
			Commit:    shortenHash(entry.CommitHash),
			Date:      entry.Date,
			Message:   strings.TrimSpace(entry.Message),
			Committer: entry.Committer,
		}

		// Add stats if requested
		if historyStats {
			stats, err := st.DoltLogStats(entry.CommitHash)
			if err == nil {
				he.Stats = &HistoryStats{
					Entities:     stats.Entities,
					Dependencies: stats.Dependencies,
				}
			}
		}

		historyOut.Commits = append(historyOut.Commits, he)
	}

	// Output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), historyOut, output.DensityMedium)
}

// shortenHash returns first 7 characters of a commit hash
func shortenHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
