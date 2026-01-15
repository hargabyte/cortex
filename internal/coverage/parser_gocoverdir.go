// Package coverage provides Go test coverage data integration for cx.
// This file implements parsing for Go 1.20+ GOCOVERDIR binary format.
package coverage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GOCOVERDIRData represents coverage data from a GOCOVERDIR directory.
// It supports both single-directory (aggregate) and per-test directory structures.
type GOCOVERDIRData struct {
	// Aggregate coverage data (merged from all test runs)
	Aggregate *CoverageData

	// Per-test coverage data (when using subdirectory structure)
	// Key is the test name (derived from subdirectory name)
	PerTest map[string]*CoverageData
}

// ParseGOCOVERDIR parses coverage data from a Go 1.20+ GOCOVERDIR directory.
// It detects whether the directory contains:
//   - Direct coverage files (covmeta.*, covcounters.*) - aggregate mode
//   - Subdirectories with coverage files - per-test mode (test name = subdir name)
//
// This uses `go tool covdata textfmt` to convert the binary format to text,
// then parses the text format using the existing parser.
func ParseGOCOVERDIR(dirPath string) (*GOCOVERDIRData, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("stat gocoverdir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dirPath)
	}

	result := &GOCOVERDIRData{
		PerTest: make(map[string]*CoverageData),
	}

	// Check for direct coverage files in the directory
	hasDirectCoverage, err := hasCoverageFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("check coverage files: %w", err)
	}

	// Check for subdirectories with coverage files (per-test mode)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var coveredSubdirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(dirPath, entry.Name())
			hasCov, err := hasCoverageFiles(subdir)
			if err != nil {
				continue // Skip problematic subdirs
			}
			if hasCov {
				coveredSubdirs = append(coveredSubdirs, entry.Name())
			}
		}
	}

	// Parse aggregate coverage if direct files exist
	if hasDirectCoverage {
		aggregate, err := parseGOCOVERDIRSingle(dirPath)
		if err != nil {
			return nil, fmt.Errorf("parse aggregate coverage: %w", err)
		}
		result.Aggregate = aggregate
	}

	// Parse per-test coverage from subdirectories
	for _, testName := range coveredSubdirs {
		subdir := filepath.Join(dirPath, testName)
		testCoverage, err := parseGOCOVERDIRSingle(subdir)
		if err != nil {
			// Log warning but continue with other tests
			fmt.Fprintf(os.Stderr, "Warning: failed to parse coverage for %s: %v\n", testName, err)
			continue
		}
		result.PerTest[testName] = testCoverage
	}

	// If no aggregate but we have per-test data, merge them for aggregate
	if result.Aggregate == nil && len(result.PerTest) > 0 {
		merged, err := mergeGOCOVERDIRSubdirs(dirPath, coveredSubdirs)
		if err != nil {
			return nil, fmt.Errorf("merge per-test coverage: %w", err)
		}
		result.Aggregate = merged
	}

	// Validate we found some coverage data
	if result.Aggregate == nil && len(result.PerTest) == 0 {
		return nil, fmt.Errorf("no coverage data found in %s", dirPath)
	}

	return result, nil
}

// parseGOCOVERDIRSingle parses coverage from a single GOCOVERDIR directory
// using `go tool covdata textfmt`.
func parseGOCOVERDIRSingle(dirPath string) (*CoverageData, error) {
	// Create temp file for text output
	tmpFile, err := os.CreateTemp("", "cx-coverage-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Run go tool covdata textfmt
	cmd := exec.Command("go", "tool", "covdata", "textfmt", "-i="+dirPath, "-o="+tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go tool covdata textfmt failed: %w\nOutput: %s", err, string(output))
	}

	// Parse the text format output using existing parser
	return ParseCoverageFile(tmpPath)
}

// mergeGOCOVERDIRSubdirs merges coverage from multiple subdirectories
// using `go tool covdata textfmt` with multiple -i directories.
func mergeGOCOVERDIRSubdirs(basePath string, subdirs []string) (*CoverageData, error) {
	if len(subdirs) == 0 {
		return nil, fmt.Errorf("no subdirectories to merge")
	}

	// Build comma-separated list of subdirectories
	var dirs []string
	for _, subdir := range subdirs {
		dirs = append(dirs, filepath.Join(basePath, subdir))
	}
	inputDirs := strings.Join(dirs, ",")

	// Create temp file for text output
	tmpFile, err := os.CreateTemp("", "cx-coverage-merged-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Run go tool covdata textfmt with merged input
	cmd := exec.Command("go", "tool", "covdata", "textfmt", "-i="+inputDirs, "-o="+tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go tool covdata textfmt merge failed: %w\nOutput: %s", err, string(output))
	}

	// Parse the merged text format output
	return ParseCoverageFile(tmpPath)
}

// hasCoverageFiles checks if a directory contains GOCOVERDIR coverage files.
// GOCOVERDIR uses covmeta.* and covcounters.* files.
func hasCoverageFiles(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "covmeta.") || strings.HasPrefix(name, "covcounters.") {
			return true, nil
		}
	}
	return false, nil
}

// IsGOCOVERDIR checks if the given path is a GOCOVERDIR directory
// (contains covmeta.* or covcounters.* files, or subdirectories that do).
func IsGOCOVERDIR(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for direct coverage files
	if has, _ := hasCoverageFiles(path); has {
		return true
	}

	// Check for subdirectories with coverage files
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(path, entry.Name())
			if has, _ := hasCoverageFiles(subdir); has {
				return true
			}
		}
	}

	return false
}

// HasPerTestAttribution checks if the GOCOVERDIR data has per-test coverage
// (i.e., was generated with one subdirectory per test).
func (d *GOCOVERDIRData) HasPerTestAttribution() bool {
	return len(d.PerTest) > 0
}

// TestNames returns the list of test names that have coverage data.
func (d *GOCOVERDIRData) TestNames() []string {
	names := make([]string, 0, len(d.PerTest))
	for name := range d.PerTest {
		names = append(names, name)
	}
	return names
}
