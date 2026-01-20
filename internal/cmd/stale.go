package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// staleCmd represents the cx stale command
var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "Check if the code graph is stale",
	Long: `Check if the code graph needs to be refreshed.

Compares the last scan time against:
  1. Current time (with configurable threshold)
  2. Git HEAD commit (has HEAD changed since scan?)
  3. File modifications (how many files changed since scan?)

Output:
  status: stale|fresh
  last_scan: timestamp of last scan
  git_status: whether HEAD matches scan commit
  files_changed: number of files modified since scan

Examples:
  cx stale                    # Check staleness with default 1h threshold
  cx stale --threshold 30m    # Use 30 minute threshold
  cx stale --threshold 0      # Only check git/file changes, ignore time
  cx stale --json             # JSON output for scripts`,
	RunE: runStale,
}

var (
	staleThreshold string
	staleJSON      bool
)

func init() {
	rootCmd.AddCommand(staleCmd)

	staleCmd.Flags().StringVar(&staleThreshold, "threshold", "1h", "Time threshold for staleness (e.g., 30m, 1h, 24h, 0 to disable)")
	staleCmd.Flags().BoolVar(&staleJSON, "json", false, "Output in JSON format")
}

// StaleOutput represents the staleness check results
type StaleOutput struct {
	Status       string        `yaml:"status" json:"status"`               // "stale" or "fresh"
	LastScan     *ScanInfo     `yaml:"last_scan" json:"last_scan"`         // Info about last scan
	GitStatus    *GitStatus    `yaml:"git_status" json:"git_status"`       // Git HEAD comparison
	FilesChanged *FilesChanged `yaml:"files_changed" json:"files_changed"` // Files modified since scan
	Reasons      []string      `yaml:"reasons,omitempty" json:"reasons,omitempty"`
}

// ScanInfo contains information about the last scan
type ScanInfo struct {
	Time          string `yaml:"time" json:"time"`
	Ago           string `yaml:"ago" json:"ago"`
	GitCommit     string `yaml:"git_commit,omitempty" json:"git_commit,omitempty"`
	GitBranch     string `yaml:"git_branch,omitempty" json:"git_branch,omitempty"`
	FilesScanned  int    `yaml:"files_scanned" json:"files_scanned"`
	EntitiesFound int    `yaml:"entities_found" json:"entities_found"`
}

// GitStatus contains git comparison info
type GitStatus struct {
	HeadChanged  bool   `yaml:"head_changed" json:"head_changed"`
	ScanCommit   string `yaml:"scan_commit" json:"scan_commit"`
	CurrentHead  string `yaml:"current_head" json:"current_head"`
	CommitsBehind int   `yaml:"commits_behind,omitempty" json:"commits_behind,omitempty"`
}

// FilesChanged contains file modification info
type FilesChanged struct {
	Count    int      `yaml:"count" json:"count"`
	Modified []string `yaml:"modified,omitempty" json:"modified,omitempty"`
}

func runStale(cmd *cobra.Command, args []string) error {
	// Find .cx directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx init && cx scan' first")
	}

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
		// No scans yet
		staleOut := &StaleOutput{
			Status:  "stale",
			Reasons: []string{"No scans found - run 'cx scan' first"},
		}
		return outputStale(cmd, staleOut)
	}

	// Parse threshold
	var threshold time.Duration
	if staleThreshold != "0" {
		threshold, err = time.ParseDuration(staleThreshold)
		if err != nil {
			return fmt.Errorf("invalid threshold: %w", err)
		}
	}

	// Build output
	staleOut := &StaleOutput{
		Status: "fresh",
		LastScan: &ScanInfo{
			Time:          meta.ScanTime.Format(time.RFC3339),
			Ago:           formatDurationAgo(time.Since(meta.ScanTime)),
			GitCommit:     meta.GitCommit,
			GitBranch:     meta.GitBranch,
			FilesScanned:  meta.FilesScanned,
			EntitiesFound: meta.EntitiesFound,
		},
	}

	// Check 1: Time threshold
	if threshold > 0 && time.Since(meta.ScanTime) > threshold {
		staleOut.Status = "stale"
		staleOut.Reasons = append(staleOut.Reasons,
			fmt.Sprintf("Last scan was %s ago (threshold: %s)", formatDurationAgo(time.Since(meta.ScanTime)), threshold))
	}

	// Check 2: Git HEAD
	currentHead := getCurrentGitHead()
	if currentHead != "" {
		staleOut.GitStatus = &GitStatus{
			ScanCommit:  meta.GitCommit,
			CurrentHead: currentHead,
			HeadChanged: meta.GitCommit != "" && currentHead != meta.GitCommit,
		}

		if staleOut.GitStatus.HeadChanged {
			staleOut.Status = "stale"
			staleOut.Reasons = append(staleOut.Reasons,
				fmt.Sprintf("Git HEAD changed: %s -> %s", shortenCommit(meta.GitCommit), shortenCommit(currentHead)))

			// Count commits behind
			if meta.GitCommit != "" {
				commitsBehind := countCommitsBetween(meta.GitCommit, currentHead)
				staleOut.GitStatus.CommitsBehind = commitsBehind
			}
		}
	}

	// Check 3: Files modified since scan
	modifiedFiles := getFilesModifiedSince(storeDB, meta.ScanTime)
	staleOut.FilesChanged = &FilesChanged{
		Count: len(modifiedFiles),
	}
	if len(modifiedFiles) > 0 {
		staleOut.Status = "stale"
		staleOut.Reasons = append(staleOut.Reasons,
			fmt.Sprintf("%d files modified since last scan", len(modifiedFiles)))

		// Include up to 10 modified files in output
		if len(modifiedFiles) <= 10 {
			staleOut.FilesChanged.Modified = modifiedFiles
		} else {
			staleOut.FilesChanged.Modified = modifiedFiles[:10]
		}
	}

	return outputStale(cmd, staleOut)
}

func outputStale(cmd *cobra.Command, staleOut *StaleOutput) error {
	var format output.Format
	if staleJSON {
		format = output.FormatJSON
	} else {
		format = output.FormatYAML
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), staleOut, output.DensityMedium)
}

// getCurrentGitHead returns the current git HEAD commit hash
func getCurrentGitHead() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// countCommitsBetween counts commits between two refs
func countCommitsBetween(from, to string) int {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", from, to))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count
}

// getFilesModifiedSince returns files that have been modified since the given time
func getFilesModifiedSince(storeDB *store.Store, since time.Time) []string {
	files, err := storeDB.GetAllFileEntries()
	if err != nil {
		return nil
	}

	var modified []string
	for _, f := range files {
		info, err := os.Stat(f.FilePath)
		if err != nil {
			continue // File might be deleted
		}

		if info.ModTime().After(since) {
			modified = append(modified, f.FilePath)
		}
	}

	return modified
}

// formatDurationAgo formats a duration as a human-readable "ago" string
func formatDurationAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}

// shortenCommit returns a shortened commit hash
func shortenCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
