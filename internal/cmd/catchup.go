package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// catchupCmd represents the cx catchup command
var catchupCmd = &cobra.Command{
	Use:   "catchup",
	Short: "Catch up on what changed while you were away",
	Long: `Get up to speed on codebase changes since your last session.

This is an agent-optimized command that:
  1. Shows how stale your graph is
  2. Runs an incremental scan to update the graph
  3. Shows a summary of what changed

It's designed for the "I was away, what changed?" workflow.

Output:
  before:      State before catchup (staleness info)
  scan:        Scan results (files processed, entities found)
  changes:     Summary of changes (added, modified, removed)
  suggestions: What to look at first

Examples:
  cx catchup                  # Full catchup workflow
  cx catchup --dry-run        # Show what would change without scanning
  cx catchup --since HEAD~5   # Show changes since specific commit
  cx catchup --json           # JSON output for scripts`,
	RunE: runCatchup,
}

var (
	catchupDryRun bool
	catchupSince  string
	catchupJSON   bool
)

func init() {
	rootCmd.AddCommand(catchupCmd)

	catchupCmd.Flags().BoolVar(&catchupDryRun, "dry-run", false, "Show what changed without running scan")
	catchupCmd.Flags().StringVar(&catchupSince, "since", "", "Compare against specific git ref (default: last scan commit)")
	catchupCmd.Flags().BoolVar(&catchupJSON, "json", false, "Output in JSON format")
}

// CatchupOutput represents the catchup results
type CatchupOutput struct {
	Before      *CatchupBefore  `yaml:"before" json:"before"`
	Scan        *CatchupScan    `yaml:"scan,omitempty" json:"scan,omitempty"`
	Changes     *CatchupChanges `yaml:"changes" json:"changes"`
	Suggestions []string        `yaml:"suggestions,omitempty" json:"suggestions,omitempty"`
}

// CatchupBefore shows state before catchup
type CatchupBefore struct {
	LastScanTime   string `yaml:"last_scan_time" json:"last_scan_time"`
	LastScanCommit string `yaml:"last_scan_commit" json:"last_scan_commit"`
	CurrentHead    string `yaml:"current_head" json:"current_head"`
	CommitsBehind  int    `yaml:"commits_behind" json:"commits_behind"`
	FilesChanged   int    `yaml:"files_changed" json:"files_changed"`
}

// CatchupScan shows scan results
type CatchupScan struct {
	FilesProcessed int    `yaml:"files_processed" json:"files_processed"`
	EntitiesFound  int    `yaml:"entities_found" json:"entities_found"`
	DepsFound      int    `yaml:"dependencies_found" json:"dependencies_found"`
	Duration       string `yaml:"duration" json:"duration"`
}

// CatchupChanges shows what changed
type CatchupChanges struct {
	EntitiesAdded    int                 `yaml:"entities_added" json:"entities_added"`
	EntitiesModified int                 `yaml:"entities_modified" json:"entities_modified"`
	EntitiesRemoved  int                 `yaml:"entities_removed" json:"entities_removed"`
	KeystonesChanged int                 `yaml:"keystones_changed" json:"keystones_changed"`
	ByFile           []CatchupFileChange `yaml:"by_file,omitempty" json:"by_file,omitempty"`
}

// CatchupFileChange shows changes in a specific file
type CatchupFileChange struct {
	File     string `yaml:"file" json:"file"`
	Added    int    `yaml:"added" json:"added"`
	Modified int    `yaml:"modified" json:"modified"`
	Removed  int    `yaml:"removed" json:"removed"`
}

func runCatchup(cmd *cobra.Command, args []string) error {
	// Find .cx directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx init && cx scan' first")
	}
	projectRoot := filepath.Dir(cxDir)

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Get latest scan metadata
	meta, err := storeDB.GetLatestScanMetadata()
	if err != nil {
		return fmt.Errorf("failed to get scan metadata: %w", err)
	}

	if meta == nil {
		return fmt.Errorf("no previous scans found - run 'cx scan' first")
	}

	// Build before state
	currentHead := getCurrentGitHead()
	modifiedFiles := getFilesModifiedSince(storeDB, meta.ScanTime)
	commitsBehind := 0
	if meta.GitCommit != "" && currentHead != "" {
		commitsBehind = countCommitsBetween(meta.GitCommit, currentHead)
	}

	catchupOut := &CatchupOutput{
		Before: &CatchupBefore{
			LastScanTime:   meta.ScanTime.Format(time.RFC3339),
			LastScanCommit: meta.GitCommit,
			CurrentHead:    currentHead,
			CommitsBehind:  commitsBehind,
			FilesChanged:   len(modifiedFiles),
		},
	}

	// Determine comparison ref
	compareRef := catchupSince
	if compareRef == "" && meta.GitCommit != "" {
		compareRef = meta.GitCommit
	}

	// If dry-run, just show what would change
	if catchupDryRun {
		changes, err := getCatchupChanges(storeDB, projectRoot, compareRef)
		if err != nil {
			return fmt.Errorf("failed to get changes: %w", err)
		}
		catchupOut.Changes = changes
		catchupOut.Suggestions = generateCatchupSuggestions(catchupOut)
		return outputCatchup(cmd, catchupOut)
	}

	// Run incremental scan
	scanStart := time.Now()
	scanResult, err := runIncrementalScan(cxDir)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	catchupOut.Scan = &CatchupScan{
		FilesProcessed: scanResult.filesProcessed,
		EntitiesFound:  scanResult.entitiesFound,
		DepsFound:      scanResult.depsFound,
		Duration:       time.Since(scanStart).Round(time.Millisecond).String(),
	}

	// Get changes after scan
	changes, err := getCatchupChanges(storeDB, projectRoot, compareRef)
	if err != nil {
		// Continue even if we can't get detailed changes
		changes = &CatchupChanges{}
	}
	catchupOut.Changes = changes

	// Generate suggestions
	catchupOut.Suggestions = generateCatchupSuggestions(catchupOut)

	return outputCatchup(cmd, catchupOut)
}

type scanResult struct {
	filesProcessed int
	entitiesFound  int
	depsFound      int
}

func runIncrementalScan(cxDir string) (*scanResult, error) {
	// Run cx scan command
	cmd := exec.Command("cx", "scan")
	cmd.Dir = filepath.Dir(cxDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("scan error: %w\n%s", err, string(output))
	}

	// Parse output for stats (simple parsing)
	result := &scanResult{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "files") && strings.Contains(line, "scanned") {
			fmt.Sscanf(line, "%d files", &result.filesProcessed)
		}
		if strings.Contains(line, "entities") {
			fmt.Sscanf(line, "%d entities", &result.entitiesFound)
		}
	}

	return result, nil
}

func getCatchupChanges(storeDB *store.Store, projectRoot string, compareRef string) (*CatchupChanges, error) {
	changes := &CatchupChanges{}

	// Use Dolt diff if we have a compare ref
	if compareRef != "" {
		added, modified, removed, err := storeDB.DoltDiffSummary(compareRef, "HEAD")
		if err == nil {
			changes.EntitiesAdded = added
			changes.EntitiesModified = modified
			changes.EntitiesRemoved = removed
		}
	}

	// Get keystones that changed
	keystonesChanged, err := countKeystonesChanged(storeDB, compareRef)
	if err == nil {
		changes.KeystonesChanged = keystonesChanged
	}

	// Get changes by file (top 10)
	byFile, err := getChangesByFile(storeDB, compareRef)
	if err == nil && len(byFile) > 0 {
		if len(byFile) > 10 {
			byFile = byFile[:10]
		}
		changes.ByFile = byFile
	}

	return changes, nil
}

func countKeystonesChanged(storeDB *store.Store, compareRef string) (int, error) {
	if compareRef == "" {
		return 0, nil
	}

	// Get diff result and count keystones among modified entities
	diffResult, err := storeDB.DoltDiff(store.DiffOptions{
		FromRef: compareRef,
		ToRef:   "HEAD",
		Table:   "entities",
	})
	if err != nil {
		return 0, err
	}

	// Count keystones among changed entities
	keystoneCount := 0
	for _, change := range diffResult.Modified {
		m, err := storeDB.GetMetrics(change.EntityID)
		if err == nil && m != nil && m.PageRank >= 0.001 {
			keystoneCount++
		}
	}
	for _, change := range diffResult.Added {
		m, err := storeDB.GetMetrics(change.EntityID)
		if err == nil && m != nil && m.PageRank >= 0.001 {
			keystoneCount++
		}
	}

	return keystoneCount, nil
}

func getChangesByFile(storeDB *store.Store, compareRef string) ([]CatchupFileChange, error) {
	if compareRef == "" {
		return nil, nil
	}

	// Get diff result
	diffResult, err := storeDB.DoltDiff(store.DiffOptions{
		FromRef: compareRef,
		ToRef:   "HEAD",
		Table:   "entities",
	})
	if err != nil {
		return nil, err
	}

	// Aggregate by file
	fileChanges := make(map[string]*CatchupFileChange)

	for _, change := range diffResult.Added {
		fc := getOrCreateFileChange(fileChanges, change.FilePath)
		fc.Added++
	}
	for _, change := range diffResult.Modified {
		fc := getOrCreateFileChange(fileChanges, change.FilePath)
		fc.Modified++
	}
	for _, change := range diffResult.Removed {
		fc := getOrCreateFileChange(fileChanges, change.FilePath)
		fc.Removed++
	}

	// Convert to slice and sort by total changes
	var changes []CatchupFileChange
	for _, fc := range fileChanges {
		changes = append(changes, *fc)
	}

	// Sort by total changes descending
	for i := 0; i < len(changes)-1; i++ {
		for j := i + 1; j < len(changes); j++ {
			totalI := changes[i].Added + changes[i].Modified + changes[i].Removed
			totalJ := changes[j].Added + changes[j].Modified + changes[j].Removed
			if totalJ > totalI {
				changes[i], changes[j] = changes[j], changes[i]
			}
		}
	}

	return changes, nil
}

func getOrCreateFileChange(m map[string]*CatchupFileChange, path string) *CatchupFileChange {
	if fc, ok := m[path]; ok {
		return fc
	}
	fc := &CatchupFileChange{File: path}
	m[path] = fc
	return fc
}

func generateCatchupSuggestions(out *CatchupOutput) []string {
	var suggestions []string

	if out.Before.CommitsBehind > 10 {
		suggestions = append(suggestions, fmt.Sprintf("Review %d commits since last scan - consider reviewing git log", out.Before.CommitsBehind))
	}

	if out.Changes != nil {
		if out.Changes.KeystonesChanged > 0 {
			suggestions = append(suggestions, fmt.Sprintf("Review %d keystone changes with 'cx find --keystones'", out.Changes.KeystonesChanged))
		}

		if out.Changes.EntitiesAdded > 20 {
			suggestions = append(suggestions, "Many new entities added - run 'cx map' for project overview")
		}

		if out.Changes.EntitiesRemoved > 10 {
			suggestions = append(suggestions, "Several entities removed - check for dead code with 'cx dead'")
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Graph is up to date - ready to work")
	}

	return suggestions
}

func outputCatchup(cmd *cobra.Command, catchupOut *CatchupOutput) error {
	var format output.Format
	if catchupJSON {
		format = output.FormatJSON
	} else {
		format = output.FormatYAML
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), catchupOut, output.DensityMedium)
}
