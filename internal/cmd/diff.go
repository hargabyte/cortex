package cmd

import (
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff [from-ref] [to-ref]",
	Short: "Show entity changes between commits",
	Long: `Compare entity graphs between two Dolt commits or refs.

Shows added, modified, and removed entities between two points in the
codebase history. Uses Dolt's DOLT_DIFF table function to compute changes.

Arguments:
  from-ref    Starting commit/branch/tag (default: HEAD~1)
  to-ref      Ending commit/branch/tag (default: WORKING for uncommitted changes)

Ref Formats:
  HEAD~N      N commits before HEAD
  HEAD        Latest commit
  WORKING     Uncommitted working changes
  <commit>    Specific commit hash
  <branch>    Branch name
  <tag>       Tag name

Filters:
  --table     Diff a specific table (entities, dependencies)
  --entity    Filter to entities matching name pattern
  --summary   Show only summary counts, not full list

Examples:
  cx diff                           # Show uncommitted changes
  cx diff HEAD~1                    # Changes since previous commit
  cx diff HEAD~5 HEAD               # Changes over last 5 commits
  cx diff main                      # Changes since main branch
  cx diff --entity LoginUser        # Filter to entities matching "LoginUser"
  cx diff --summary                 # Just show counts
  cx diff --format json             # JSON output for parsing`,
	Args: cobra.MaximumNArgs(2),
	RunE: runDiff,
}

var (
	diffTable   string
	diffEntity  string
	diffSummary bool
)

func init() {
	rootCmd.AddCommand(diffCmd)

	diffCmd.Flags().StringVar(&diffTable, "table", "entities", "Table to diff (entities, dependencies)")
	diffCmd.Flags().StringVar(&diffEntity, "entity", "", "Filter to entities matching name pattern")
	diffCmd.Flags().BoolVar(&diffSummary, "summary", false, "Show only summary counts")
}

func runDiff(cmd *cobra.Command, args []string) error {
	// Parse refs from args
	fromRef := "HEAD~1"
	toRef := "WORKING"
	if len(args) >= 1 {
		fromRef = args[0]
	}
	if len(args) >= 2 {
		toRef = args[1]
	}

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

	// If summary only, use the quick summary function
	if diffSummary {
		added, modified, removed, err := st.DoltDiffSummary(fromRef, toRef)
		if err != nil {
			return fmt.Errorf("diff summary: %w", err)
		}

		summary := DiffSummary{
			EntitiesAdded:    added,
			EntitiesModified: modified,
			EntitiesRemoved:  removed,
		}

		return outputDiffSummary(cmd, summary, fromRef, toRef, format)
	}

	// Full diff
	opts := store.DiffOptions{
		FromRef:    fromRef,
		ToRef:      toRef,
		Table:      diffTable,
		EntityName: diffEntity,
	}

	result, err := st.DoltDiff(opts)
	if err != nil {
		return fmt.Errorf("dolt diff: %w", err)
	}

	return outputDiffResult(cmd, result, format)
}

// outputDiffSummary outputs just the summary counts
func outputDiffSummary(cmd *cobra.Command, summary DiffSummary, fromRef, toRef string, format output.Format) error {
	data := struct {
		FromRef string      `yaml:"from_ref" json:"from_ref"`
		ToRef   string      `yaml:"to_ref" json:"to_ref"`
		Summary DiffSummary `yaml:"summary" json:"summary"`
	}{
		FromRef: fromRef,
		ToRef:   toRef,
		Summary: summary,
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("get formatter: %w", err)
	}
	return formatter.FormatToWriter(cmd.OutOrStdout(), data, output.DensityMedium)
}

// outputDiffResult outputs the full diff result
func outputDiffResult(cmd *cobra.Command, result *store.DiffResult, format output.Format) error {
	// Convert to output format
	out := DiffOutput{
		Summary: DiffSummary{
			EntitiesAdded:    len(result.Added),
			EntitiesModified: len(result.Modified),
			EntitiesRemoved:  len(result.Removed),
		},
	}

	// Convert changes to output format
	for _, c := range result.Added {
		out.Added = append(out.Added, diffChangeToEntity(c))
	}
	for _, c := range result.Modified {
		out.Modified = append(out.Modified, diffChangeToEntity(c))
	}
	for _, c := range result.Removed {
		out.Removed = append(out.Removed, diffChangeToEntity(c))
	}

	// Wrap in result struct with refs
	data := struct {
		FromRef string     `yaml:"from_ref" json:"from_ref"`
		ToRef   string     `yaml:"to_ref" json:"to_ref"`
		Diff    DiffOutput `yaml:"diff" json:"diff"`
	}{
		FromRef: result.FromRef,
		ToRef:   result.ToRef,
		Diff:    out,
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("get formatter: %w", err)
	}
	return formatter.FormatToWriter(cmd.OutOrStdout(), data, output.DensityMedium)
}

// diffChangeToEntity converts a store.DiffChange to a DiffEntity for output
func diffChangeToEntity(c store.DiffChange) DiffEntity {
	location := c.FilePath
	if c.LineStart > 0 {
		location = fmt.Sprintf("%s:%d", c.FilePath, c.LineStart)
	}

	entity := DiffEntity{
		Name:     c.EntityName,
		Type:     c.EntityType,
		Location: location,
		Change:   c.DiffType,
	}

	if c.OldSigHash != nil {
		entity.OldHash = *c.OldSigHash
	}
	if c.NewSigHash != nil {
		entity.NewHash = *c.NewSigHash
	}

	return entity
}
