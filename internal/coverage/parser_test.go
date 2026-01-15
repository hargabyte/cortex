package coverage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverageFile(t *testing.T) {
	// Create a temporary coverage file
	tmpDir := t.TempDir()
	coverageFile := filepath.Join(tmpDir, "coverage.out")

	content := `mode: set
github.com/user/project/internal/auth/login.go:45.1,67.2 5 1
github.com/user/project/internal/auth/login.go:70.1,89.2 3 0
github.com/user/project/pkg/utils/helper.go:10.15,12.2 1 1
`

	if err := os.WriteFile(coverageFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Parse the file
	data, err := ParseCoverageFile(coverageFile)
	if err != nil {
		t.Fatalf("ParseCoverageFile failed: %v", err)
	}

	// Verify mode
	if data.Mode != "set" {
		t.Errorf("expected mode 'set', got '%s'", data.Mode)
	}

	// Verify number of blocks
	if len(data.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(data.Blocks))
	}

	// Verify first block
	block := data.Blocks[0]
	if block.FilePath != "github.com/user/project/internal/auth/login.go" {
		t.Errorf("expected file 'github.com/user/project/internal/auth/login.go', got '%s'", block.FilePath)
	}
	if block.StartLine != 45 {
		t.Errorf("expected start line 45, got %d", block.StartLine)
	}
	if block.StartCol != 1 {
		t.Errorf("expected start col 1, got %d", block.StartCol)
	}
	if block.EndLine != 67 {
		t.Errorf("expected end line 67, got %d", block.EndLine)
	}
	if block.EndCol != 2 {
		t.Errorf("expected end col 2, got %d", block.EndCol)
	}
	if block.NumStmt != 5 {
		t.Errorf("expected 5 statements, got %d", block.NumStmt)
	}
	if block.Count != 1 {
		t.Errorf("expected count 1, got %d", block.Count)
	}
	if !block.IsCovered() {
		t.Error("block should be covered")
	}

	// Verify second block (not covered)
	block2 := data.Blocks[1]
	if block2.Count != 0 {
		t.Errorf("expected count 0, got %d", block2.Count)
	}
	if block2.IsCovered() {
		t.Error("block should not be covered")
	}
}

func TestGetFilesWithCoverage(t *testing.T) {
	data := &CoverageData{
		Mode: "set",
		Blocks: []CoverageBlock{
			{FilePath: "file1.go", StartLine: 1, EndLine: 10, Count: 1},
			{FilePath: "file1.go", StartLine: 20, EndLine: 30, Count: 0},
			{FilePath: "file2.go", StartLine: 5, EndLine: 15, Count: 1},
		},
	}

	fileBlocks := data.GetFilesWithCoverage()

	if len(fileBlocks) != 2 {
		t.Fatalf("expected 2 files, got %d", len(fileBlocks))
	}

	if len(fileBlocks["file1.go"]) != 2 {
		t.Errorf("expected 2 blocks for file1.go, got %d", len(fileBlocks["file1.go"]))
	}

	if len(fileBlocks["file2.go"]) != 1 {
		t.Errorf("expected 1 block for file2.go, got %d", len(fileBlocks["file2.go"]))
	}
}

func TestGetCoverageForFile(t *testing.T) {
	data := &CoverageData{
		Mode: "set",
		Blocks: []CoverageBlock{
			{FilePath: "file1.go", StartLine: 1, EndLine: 10, Count: 1},
			{FilePath: "file1.go", StartLine: 20, EndLine: 30, Count: 0},
			{FilePath: "file2.go", StartLine: 5, EndLine: 15, Count: 1},
		},
	}

	blocks := data.GetCoverageForFile("file1.go")
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks for file1.go, got %d", len(blocks))
	}

	blocks = data.GetCoverageForFile("file2.go")
	if len(blocks) != 1 {
		t.Errorf("expected 1 block for file2.go, got %d", len(blocks))
	}

	blocks = data.GetCoverageForFile("nonexistent.go")
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for nonexistent.go, got %d", len(blocks))
	}
}

func TestParseCoverageBlock(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantError bool
		expected  CoverageBlock
	}{
		{
			name:      "valid block",
			line:      "github.com/user/project/file.go:10.5,20.10 3 1",
			wantError: false,
			expected: CoverageBlock{
				FilePath:  "github.com/user/project/file.go",
				StartLine: 10,
				StartCol:  5,
				EndLine:   20,
				EndCol:    10,
				NumStmt:   3,
				Count:     1,
			},
		},
		{
			name:      "invalid format - missing colon",
			line:      "file.go 10.5,20.10 3 1",
			wantError: true,
		},
		{
			name:      "invalid format - missing comma",
			line:      "file.go:10.5 20.10 3 1",
			wantError: true,
		},
		{
			name:      "invalid format - wrong field count",
			line:      "file.go:10.5,20.10 3",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := parseCoverageBlock(tt.line)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if block.FilePath != tt.expected.FilePath {
				t.Errorf("FilePath: expected %s, got %s", tt.expected.FilePath, block.FilePath)
			}
			if block.StartLine != tt.expected.StartLine {
				t.Errorf("StartLine: expected %d, got %d", tt.expected.StartLine, block.StartLine)
			}
			if block.EndLine != tt.expected.EndLine {
				t.Errorf("EndLine: expected %d, got %d", tt.expected.EndLine, block.EndLine)
			}
			if block.NumStmt != tt.expected.NumStmt {
				t.Errorf("NumStmt: expected %d, got %d", tt.expected.NumStmt, block.NumStmt)
			}
			if block.Count != tt.expected.Count {
				t.Errorf("Count: expected %d, got %d", tt.expected.Count, block.Count)
			}
		})
	}
}
