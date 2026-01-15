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

// IsPerTestCoverageDir checks if a directory contains per-test coverage.out files.
// This is an alternative to GOCOVERDIR for per-test attribution using traditional
// coverage.out files. Directory structure:
//
//	.coverage/
//	  TestFoo.out
//	  TestBar.out
//	  TestBaz_SubTest.out
func IsPerTestCoverageDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	// Check for *.out files that look like test names
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".out") && strings.HasPrefix(name, "Test") {
			return true
		}
	}

	return false
}

// ParsePerTestCoverageDir parses a directory of per-test coverage.out files.
// Each file should be named TestName.out and contains coverage data for that test.
// Returns GOCOVERDIRData for compatibility with existing code.
func ParsePerTestCoverageDir(dirPath string) (*GOCOVERDIRData, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dirPath)
	}

	result := &GOCOVERDIRData{
		PerTest: make(map[string]*CoverageData),
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var allCoverageFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".out") {
			filePath := filepath.Join(dirPath, name)
			allCoverageFiles = append(allCoverageFiles, filePath)

			// Extract test name from filename
			testName := strings.TrimSuffix(name, ".out")

			// Parse coverage file
			coverageData, err := ParseCoverageFile(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", name, err)
				continue
			}

			result.PerTest[testName] = coverageData
		}
	}

	if len(result.PerTest) == 0 {
		return nil, fmt.Errorf("no coverage.out files found in %s", dirPath)
	}

	// Merge all coverage files for aggregate data
	if len(allCoverageFiles) > 0 {
		aggregate, err := MergeCoverageFiles(allCoverageFiles)
		if err != nil {
			return nil, fmt.Errorf("merge coverage files: %w", err)
		}
		result.Aggregate = aggregate
	}

	return result, nil
}

// MergeCoverageFiles merges multiple coverage.out files into one.
// Uses go tool cover -merge if available (Go 1.20+), otherwise manual merge.
func MergeCoverageFiles(files []string) (*CoverageData, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files to merge")
	}

	if len(files) == 1 {
		return ParseCoverageFile(files[0])
	}

	// Try go tool cover -merge (Go 1.20+)
	tmpFile, err := os.CreateTemp("", "cx-coverage-merged-*.out")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Build merge command
	args := append([]string{"tool", "cover", "-merge"}, files...)
	args = append(args, "-o", tmpPath)

	cmd := exec.Command("go", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to manual merge if go tool cover -merge isn't available
		return manualMergeCoverageFiles(files)
	}

	if len(output) > 0 {
		fmt.Fprintf(os.Stderr, "go tool cover -merge: %s\n", string(output))
	}

	return ParseCoverageFile(tmpPath)
}

// manualMergeCoverageFiles merges coverage files without using go tool.
// It combines blocks and takes the max count for overlapping blocks.
func manualMergeCoverageFiles(files []string) (*CoverageData, error) {
	merged := &CoverageData{
		Blocks: make([]CoverageBlock, 0),
	}

	// Map to track unique blocks: "file:startLine.startCol,endLine.endCol" -> block
	blockMap := make(map[string]CoverageBlock)

	for _, file := range files {
		data, err := ParseCoverageFile(file)
		if err != nil {
			continue
		}

		if merged.Mode == "" {
			merged.Mode = data.Mode
		}

		for _, block := range data.Blocks {
			key := fmt.Sprintf("%s:%d.%d,%d.%d",
				block.FilePath, block.StartLine, block.StartCol, block.EndLine, block.EndCol)

			if existing, ok := blockMap[key]; ok {
				// Take max count (if any test covered it, it's covered)
				if block.Count > existing.Count {
					blockMap[key] = block
				}
			} else {
				blockMap[key] = block
			}
		}
	}

	for _, block := range blockMap {
		merged.Blocks = append(merged.Blocks, block)
	}

	return merged, nil
}
