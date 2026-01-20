package cmd

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// blameCmd represents the blame command
var blameCmd = &cobra.Command{
	Use:   "blame <entity>",
	Short: "Show commit history for an entity",
	Long: `Display the commit history for a specific code entity.

Shows which commits modified the entity over time, including:
  - Commit hash and date
  - Who made the change
  - What changed (signature, location, body)
  - Change type (added, modified, current)

Uses Dolt's dolt_history_entities table to track changes across commits.

Arguments:
  <entity>  Entity name, qualified name, or direct ID

Flags:
  --limit N    Number of commits to show (default: 20)
  --deps       Include dependency changes (calls added/removed)
  --format     Output format: yaml|json (default: yaml)

Examples:
  cx blame LoginUser                    # Show history for LoginUser
  cx blame store.Store                  # Qualified name lookup
  cx blame sa-fn-a7f9b2-LoginUser       # Direct ID lookup
  cx blame LoginUser --limit 50         # Show more history
  cx blame LoginUser --deps             # Include dependency changes
  cx blame LoginUser --format json      # JSON output for parsing`,
	Args: cobra.ExactArgs(1),
	RunE: runBlame,
}

var (
	blameLimit int
	blameDeps  bool
)

func init() {
	rootCmd.AddCommand(blameCmd)

	blameCmd.Flags().IntVar(&blameLimit, "limit", 20, "Number of commits to show")
	blameCmd.Flags().BoolVar(&blameDeps, "deps", false, "Include dependency changes")
}

// BlameEntry represents a single commit in the blame output
type BlameEntry struct {
	Commit     string  `yaml:"commit" json:"commit"`
	Date       string  `yaml:"date" json:"date"`
	Committer  string  `yaml:"committer,omitempty" json:"committer,omitempty"`
	ChangeType string  `yaml:"change_type" json:"change_type"`
	Location   string  `yaml:"location" json:"location"`
	Signature  *string `yaml:"signature,omitempty" json:"signature,omitempty"`
	SigHash    *string `yaml:"sig_hash,omitempty" json:"sig_hash,omitempty"`
	BodyHash   *string `yaml:"body_hash,omitempty" json:"body_hash,omitempty"`
}

// BlameDepsEntry represents a dependency change in the blame output
type BlameDepsEntry struct {
	Commit     string `yaml:"commit" json:"commit"`
	Date       string `yaml:"date" json:"date"`
	Committer  string `yaml:"committer,omitempty" json:"committer,omitempty"`
	DepType    string `yaml:"dep_type" json:"dep_type"`
	FromID     string `yaml:"from_id" json:"from_id"`
	ToID       string `yaml:"to_id" json:"to_id"`
	ChangeType string `yaml:"change_type,omitempty" json:"change_type,omitempty"`
}

// BlameOutput is the full output structure
type BlameOutput struct {
	Entity       string           `yaml:"entity" json:"entity"`
	EntityID     string           `yaml:"entity_id" json:"entity_id"`
	EntityType   string           `yaml:"entity_type" json:"entity_type"`
	Commits      []BlameEntry     `yaml:"commits" json:"commits"`
	TotalCommits int              `yaml:"total_commits" json:"total_commits"`
	Dependencies []BlameDepsEntry `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

func runBlame(cmd *cobra.Command, args []string) error {
	query := args[0]

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

	// Resolve entity by name or ID
	entity, err := resolveEntityByName(query, st, "")
	if err != nil {
		return err
	}

	// Get entity history
	history, err := st.EntityHistory(store.EntityHistoryOptions{
		EntityID: entity.ID,
		Limit:    blameLimit,
	})
	if err != nil {
		return fmt.Errorf("get entity history: %w", err)
	}

	if len(history) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No history found for entity %q\n", entity.Name)
		return nil
	}

	// Build output
	blameOut := BlameOutput{
		Entity:       entity.Name,
		EntityID:     entity.ID,
		EntityType:   entity.EntityType,
		Commits:      make([]BlameEntry, 0, len(history)),
		TotalCommits: len(history),
	}

	for _, h := range history {
		// Build location string
		location := h.FilePath
		if h.LineEnd != nil {
			location = fmt.Sprintf("%s:%d-%d", h.FilePath, h.LineStart, *h.LineEnd)
		} else {
			location = fmt.Sprintf("%s:%d", h.FilePath, h.LineStart)
		}

		entry := BlameEntry{
			Commit:     shortenHash(h.CommitHash),
			Date:       formatCommitDate(h.CommitDate),
			Committer:  h.Committer,
			ChangeType: h.ChangeType,
			Location:   location,
			Signature:  h.Signature,
			SigHash:    h.SigHash,
			BodyHash:   h.BodyHash,
		}
		blameOut.Commits = append(blameOut.Commits, entry)
	}

	// Include dependency history if requested
	if blameDeps {
		deps, err := st.DependencyHistory(store.DependencyHistoryOptions{
			EntityID: entity.ID,
			Limit:    blameLimit,
		})
		if err != nil {
			// Non-fatal - just skip deps
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not get dependency history: %v\n", err)
		} else {
			blameOut.Dependencies = make([]BlameDepsEntry, 0, len(deps))
			for _, d := range deps {
				entry := BlameDepsEntry{
					Commit:    shortenHash(d.CommitHash),
					Date:      formatCommitDate(d.CommitDate),
					Committer: d.Committer,
					DepType:   d.DepType,
					FromID:    d.FromID,
					ToID:      d.ToID,
				}
				blameOut.Dependencies = append(blameOut.Dependencies, entry)
			}
		}
	}

	// Output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), blameOut, output.DensityMedium)
}

// formatCommitDate formats a Dolt commit date for display.
// Input format: "2026-01-19 23:19:51.851 +0000 UTC"
// Output format: "2026-01-19 23:19:51"
func formatCommitDate(date string) string {
	// Try to extract just the date and time portion
	parts := strings.Split(date, " ")
	if len(parts) >= 2 {
		return parts[0] + " " + strings.Split(parts[1], ".")[0]
	}
	return date
}
