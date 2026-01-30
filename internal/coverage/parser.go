// Package coverage provides Go test coverage data integration for cx.
// It parses coverage.out files and maps coverage data to code entities.
package coverage

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CoverageBlock represents a single block of coverage data from a coverage.out file.
// Format: file:startLine.startCol,endLine.endCol numStmt count
type CoverageBlock struct {
	FilePath  string // Relative file path from the coverage.out file
	StartLine int    // Starting line number
	StartCol  int    // Starting column number
	EndLine   int    // Ending line number
	EndCol    int    // Ending column number
	NumStmt   int    // Number of statements in this block
	Count     int    // Execution count (0 = not covered, >0 = covered)
}

// IsCovered returns true if this block was executed at least once.
func (b *CoverageBlock) IsCovered() bool {
	return b.Count > 0
}

// CoverageData represents parsed coverage data from a coverage.out file.
type CoverageData struct {
	Mode   string          // Coverage mode: set, count, or atomic
	Blocks []CoverageBlock // All coverage blocks
}

// ParseCoverageFile parses a Go coverage.out file and returns structured coverage data.
// The coverage.out format is:
//
//	mode: set|count|atomic
//	path/to/file.go:startLine.startCol,endLine.endCol numStmt count
//	...
func ParseCoverageFile(path string) (*CoverageData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open coverage file: %w", err)
	}
	defer file.Close()

	data := &CoverageData{
		Blocks: make([]CoverageBlock, 0),
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// First line should be "mode: <mode>"
		if lineNum == 1 {
			if !strings.HasPrefix(line, "mode: ") {
				return nil, fmt.Errorf("invalid coverage file: first line should be 'mode: <mode>'")
			}
			data.Mode = strings.TrimPrefix(line, "mode: ")
			continue
		}

		// Parse coverage block
		block, err := parseCoverageBlock(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		data.Blocks = append(data.Blocks, block)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coverage file: %w", err)
	}

	return data, nil
}

// parseCoverageBlock parses a single coverage block line.
// Format: path/to/file.go:startLine.startCol,endLine.endCol numStmt count
func parseCoverageBlock(line string) (CoverageBlock, error) {
	block := CoverageBlock{}

	// Split by colon to separate file path and coverage data
	colonIdx := strings.LastIndex(line, ":")
	if colonIdx == -1 {
		return block, fmt.Errorf("invalid format: missing colon separator")
	}

	// Extract file path (handle Windows paths with drive letters)
	filePath := line[:colonIdx]
	remainder := line[colonIdx+1:]

	// Split remainder by space to get position and counts
	parts := strings.Fields(remainder)
	if len(parts) != 3 {
		return block, fmt.Errorf("invalid format: expected 3 fields after colon, got %d", len(parts))
	}

	positionStr := parts[0]
	numStmtStr := parts[1]
	countStr := parts[2]

	// Parse position: startLine.startCol,endLine.endCol
	commaIdx := strings.Index(positionStr, ",")
	if commaIdx == -1 {
		return block, fmt.Errorf("invalid position format: missing comma")
	}

	startPos := positionStr[:commaIdx]
	endPos := positionStr[commaIdx+1:]

	// Parse start position
	startParts := strings.Split(startPos, ".")
	if len(startParts) != 2 {
		return block, fmt.Errorf("invalid start position format")
	}
	startLine, err := strconv.Atoi(startParts[0])
	if err != nil {
		return block, fmt.Errorf("invalid start line: %w", err)
	}
	startCol, err := strconv.Atoi(startParts[1])
	if err != nil {
		return block, fmt.Errorf("invalid start column: %w", err)
	}

	// Parse end position
	endParts := strings.Split(endPos, ".")
	if len(endParts) != 2 {
		return block, fmt.Errorf("invalid end position format")
	}
	endLine, err := strconv.Atoi(endParts[0])
	if err != nil {
		return block, fmt.Errorf("invalid end line: %w", err)
	}
	endCol, err := strconv.Atoi(endParts[1])
	if err != nil {
		return block, fmt.Errorf("invalid end column: %w", err)
	}

	// Parse counts
	numStmt, err := strconv.Atoi(numStmtStr)
	if err != nil {
		return block, fmt.Errorf("invalid statement count: %w", err)
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return block, fmt.Errorf("invalid execution count: %w", err)
	}

	block.FilePath = filePath
	block.StartLine = startLine
	block.StartCol = startCol
	block.EndLine = endLine
	block.EndCol = endCol
	block.NumStmt = numStmt
	block.Count = count

	return block, nil
}

// GetFilesWithCoverage returns a map of file paths to their coverage blocks.
func (d *CoverageData) GetFilesWithCoverage() map[string][]CoverageBlock {
	fileBlocks := make(map[string][]CoverageBlock)

	for _, block := range d.Blocks {
		fileBlocks[block.FilePath] = append(fileBlocks[block.FilePath], block)
	}

	return fileBlocks
}

// GetCoverageForFile returns all coverage blocks for a specific file.
func (d *CoverageData) GetCoverageForFile(filePath string) []CoverageBlock {
	var blocks []CoverageBlock

	for _, block := range d.Blocks {
		if block.FilePath == filePath {
			blocks = append(blocks, block)
		}
	}

	return blocks
}
