// Package cmd provides the diff command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show changes since last scan",
	Long: `Compare current codebase state against the last scan to identify changes.

Shows entities that have been added, modified, or removed since the last scan.
This is useful for understanding what has changed before running a new scan.

Examples:
  cx diff                    # Show all changes since last scan
  cx diff --file src/auth    # Show changes only in specific path
  cx diff --detailed         # Show hash values for modified entities`,
	RunE: runDiff,
}

var (
	diffFile     string
	diffDetailed bool
)

func init() {
	rootCmd.AddCommand(diffCmd)

	diffCmd.Flags().StringVar(&diffFile, "file", "", "Show changes for specific file/directory only")
	diffCmd.Flags().BoolVar(&diffDetailed, "detailed", false, "Show hash changes for modified entities")
}

// DiffOutput represents the diff command output
type DiffOutput struct {
	Summary  DiffSummary  `yaml:"summary" json:"summary"`
	Added    []DiffEntity `yaml:"added,omitempty" json:"added,omitempty"`
	Modified []DiffEntity `yaml:"modified,omitempty" json:"modified,omitempty"`
	Removed  []DiffEntity `yaml:"removed,omitempty" json:"removed,omitempty"`
}

// DiffSummary contains counts of changes
type DiffSummary struct {
	FilesChanged     int    `yaml:"files_changed" json:"files_changed"`
	EntitiesAdded    int    `yaml:"entities_added" json:"entities_added"`
	EntitiesModified int    `yaml:"entities_modified" json:"entities_modified"`
	EntitiesRemoved  int    `yaml:"entities_removed" json:"entities_removed"`
	LastScan         string `yaml:"last_scan,omitempty" json:"last_scan,omitempty"`
}

// DiffEntity represents a changed entity
type DiffEntity struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Location string `yaml:"location" json:"location"`
	Change   string `yaml:"change,omitempty" json:"change,omitempty"`
	OldHash  string `yaml:"old_hash,omitempty" json:"old_hash,omitempty"`
	NewHash  string `yaml:"new_hash,omitempty" json:"new_hash,omitempty"`
}

func runDiff(cmd *cobra.Command, args []string) error {
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

	// Get all file entries from last scan
	fileEntries, err := storeDB.GetAllFileEntries()
	if err != nil {
		return fmt.Errorf("failed to get file entries: %w", err)
	}

	if len(fileEntries) == 0 {
		return fmt.Errorf("no scan data found: run 'cx scan' first")
	}

	// Find the most recent scan time
	var lastScanTime time.Time
	for _, entry := range fileEntries {
		if entry.ScannedAt.After(lastScanTime) {
			lastScanTime = entry.ScannedAt
		}
	}

	// Build map of stored file hashes
	storedHashes := make(map[string]string)
	for _, entry := range fileEntries {
		// Apply file filter if specified
		if diffFile != "" {
			if entry.FilePath != diffFile && !hasPrefix(entry.FilePath, diffFile) {
				continue
			}
		}
		storedHashes[entry.FilePath] = entry.ScanHash
	}

	// Track changes
	var added []DiffEntity
	var modified []DiffEntity
	var removed []DiffEntity
	changedFiles := make(map[string]bool)

	// Check stored files for modifications and deletions
	for filePath, storedHash := range storedHashes {
		absPath := filepath.Join(projectRoot, filePath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			// File deleted
			entities, _ := storeDB.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})
			for _, e := range entities {
				removed = append(removed, DiffEntity{
					Name:     e.Name,
					Type:     e.EntityType,
					Location: formatStoreLocation(e),
					Change:   "file_deleted",
				})
			}
			changedFiles[filePath] = true
			continue
		}

		// Check if file content changed
		currentHash := extract.ComputeFileHash(content)
		if currentHash != storedHash {
			changedFiles[filePath] = true

			// Get stored entities for this file
			storedEntities, _ := storeDB.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})

			// For each stored entity, check if it's modified
			for _, e := range storedEntities {
				diffEntity := DiffEntity{
					Name:     e.Name,
					Type:     e.EntityType,
					Location: formatStoreLocation(e),
				}

				// We can't easily recompute individual entity hashes without full parsing,
				// so we just mark them as potentially modified
				diffEntity.Change = "file_modified"
				if diffDetailed {
					diffEntity.OldHash = e.SigHash
				}

				modified = append(modified, diffEntity)
			}
		}
	}

	// Check for new files (not in stored hashes but exist on disk)
	// Walk the project to find new files
	err = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip hidden directories and common excludes
			base := filepath.Base(path)
			if base == ".git" || base == ".cx" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check source files
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".js" && ext != ".py" && ext != ".rs" && ext != ".java" {
			return nil
		}

		relPath, _ := filepath.Rel(projectRoot, path)

		// Apply file filter if specified
		if diffFile != "" {
			if relPath != diffFile && !hasPrefix(relPath, diffFile) {
				return nil
			}
		}

		// Check if this file is new (not in stored hashes)
		if _, exists := storedHashes[relPath]; !exists {
			changedFiles[relPath] = true
			added = append(added, DiffEntity{
				Name:     relPath,
				Type:     "file",
				Location: relPath,
				Change:   "new_file",
			})
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	// Build output
	diffOutput := &DiffOutput{
		Summary: DiffSummary{
			FilesChanged:     len(changedFiles),
			EntitiesAdded:    len(added),
			EntitiesModified: len(modified),
			EntitiesRemoved:  len(removed),
			LastScan:         lastScanTime.Format(time.RFC3339),
		},
		Added:    added,
		Modified: modified,
		Removed:  removed,
	}

	// Format output
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return err
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), diffOutput, density)
}

// hasPrefix checks if path has the given prefix (for directory filtering)
func hasPrefix(path, prefix string) bool {
	// Ensure prefix ends with separator for directory matching
	if prefix != "" && prefix[len(prefix)-1] != filepath.Separator {
		prefix = prefix + string(filepath.Separator)
	}
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}
