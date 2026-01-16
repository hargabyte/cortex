// Package diff provides git diff parsing and analysis for context assembly.
package diff

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileChange represents a changed file from git diff.
type FileChange struct {
	// Path is the relative file path.
	Path string `yaml:"path" json:"path"`
	// Status is the change type: A (added), M (modified), D (deleted), R (renamed).
	Status string `yaml:"status" json:"status"`
	// OldPath is set for renamed files.
	OldPath string `yaml:"old_path,omitempty" json:"old_path,omitempty"`
}

// DiffOptions configures git diff parsing.
type DiffOptions struct {
	// Staged only includes staged changes (git diff --cached).
	Staged bool
	// CommitRange specifies a commit range (e.g., "HEAD~3", "main..feature").
	CommitRange string
	// Path filters diff to specific path.
	Path string
}

// GitDiff parses git diff output and returns changed files.
type GitDiff struct {
	projectRoot string
}

// NewGitDiff creates a new GitDiff parser.
func NewGitDiff(projectRoot string) *GitDiff {
	return &GitDiff{
		projectRoot: projectRoot,
	}
}

// GetChangedFiles returns files changed according to the diff options.
func (gd *GitDiff) GetChangedFiles(opts DiffOptions) ([]FileChange, error) {
	args := []string{"diff", "--name-status"}

	if opts.Staged {
		args = append(args, "--cached")
	} else if opts.CommitRange != "" {
		args = append(args, opts.CommitRange)
	}

	// Add path filter if specified
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = gd.projectRoot
	out, err := cmd.Output()
	if err != nil {
		// If no commits yet, try without range
		if opts.CommitRange != "" {
			args = []string{"diff", "--name-status", "--cached"}
			if opts.Path != "" {
				args = append(args, "--", opts.Path)
			}
			cmd = exec.Command("git", args...)
			cmd.Dir = gd.projectRoot
			out, err = cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("git diff failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	}

	return gd.parseNameStatus(string(out))
}

// GetUncommittedChanges returns all uncommitted changes (staged + unstaged).
func (gd *GitDiff) GetUncommittedChanges() ([]FileChange, error) {
	// Get unstaged changes
	unstagedArgs := []string{"diff", "--name-status"}
	cmd := exec.Command("git", unstagedArgs...)
	cmd.Dir = gd.projectRoot
	unstagedOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	// Get staged changes
	stagedArgs := []string{"diff", "--name-status", "--cached"}
	cmd = exec.Command("git", stagedArgs...)
	cmd.Dir = gd.projectRoot
	stagedOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w", err)
	}

	// Parse both
	unstaged, err := gd.parseNameStatus(string(unstagedOut))
	if err != nil {
		return nil, err
	}

	staged, err := gd.parseNameStatus(string(stagedOut))
	if err != nil {
		return nil, err
	}

	// Merge, preferring staged status
	seen := make(map[string]bool)
	var result []FileChange

	for _, fc := range staged {
		seen[fc.Path] = true
		result = append(result, fc)
	}

	for _, fc := range unstaged {
		if !seen[fc.Path] {
			result = append(result, fc)
		}
	}

	return result, nil
}

// GetStagedChanges returns only staged changes.
func (gd *GitDiff) GetStagedChanges() ([]FileChange, error) {
	return gd.GetChangedFiles(DiffOptions{Staged: true})
}

// GetCommitRangeChanges returns changes in a commit range.
func (gd *GitDiff) GetCommitRangeChanges(commitRange string) ([]FileChange, error) {
	return gd.GetChangedFiles(DiffOptions{CommitRange: commitRange})
}

// parseNameStatus parses git diff --name-status output.
func (gd *GitDiff) parseNameStatus(output string) ([]FileChange, error) {
	var changes []FileChange

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[1]

		fc := FileChange{
			Path:   path,
			Status: string(status[0]), // First character is the status
		}

		// Handle renamed files (R100 old new)
		if strings.HasPrefix(status, "R") && len(parts) >= 3 {
			fc.OldPath = parts[1]
			fc.Path = parts[2]
		}

		// Only include source files
		if isSourceFile(fc.Path) {
			changes = append(changes, fc)
		}
	}

	return changes, nil
}

// isSourceFile checks if a file is a supported source file.
func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".java", ".rs", ".py", ".c", ".h", ".cpp", ".cc", ".cxx",
		".hpp", ".hh", ".hxx", ".cs", ".php", ".kt", ".kts", ".rb", ".rake":
		return true
	default:
		return false
	}
}

// StatusDescription returns a human-readable description of a status code.
func StatusDescription(status string) string {
	switch status {
	case "A":
		return "added"
	case "M":
		return "modified"
	case "D":
		return "deleted"
	case "R":
		return "renamed"
	case "C":
		return "copied"
	case "U":
		return "unmerged"
	default:
		return "changed"
	}
}
