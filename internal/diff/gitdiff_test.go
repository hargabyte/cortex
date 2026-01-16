package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseNameStatus(t *testing.T) {
	gd := NewGitDiff("/tmp/test")

	tests := []struct {
		name     string
		input    string
		expected []FileChange
	}{
		{
			name:  "empty input",
			input: "",
			expected: nil,
		},
		{
			name:  "single modified go file",
			input: "M\tsrc/main.go",
			expected: []FileChange{
				{Path: "src/main.go", Status: "M"},
			},
		},
		{
			name:  "added file",
			input: "A\tnew_file.go",
			expected: []FileChange{
				{Path: "new_file.go", Status: "A"},
			},
		},
		{
			name:  "deleted file",
			input: "D\told_file.go",
			expected: []FileChange{
				{Path: "old_file.go", Status: "D"},
			},
		},
		{
			name: "multiple files",
			input: `M	src/main.go
A	src/new.go
D	src/old.go`,
			expected: []FileChange{
				{Path: "src/main.go", Status: "M"},
				{Path: "src/new.go", Status: "A"},
				{Path: "src/old.go", Status: "D"},
			},
		},
		{
			name:  "renamed file",
			input: "R100\told.go\tnew.go",
			expected: []FileChange{
				{Path: "new.go", Status: "R", OldPath: "old.go"},
			},
		},
		{
			name:  "non-source file filtered out",
			input: "M\tREADME.md",
			expected: nil,
		},
		{
			name: "mixed source and non-source",
			input: `M	src/main.go
M	README.md
A	config.yaml`,
			expected: []FileChange{
				{Path: "src/main.go", Status: "M"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gd.parseNameStatus(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d changes, got %d", len(tt.expected), len(result))
			}

			for i, fc := range result {
				if fc.Path != tt.expected[i].Path {
					t.Errorf("change %d: expected path %q, got %q", i, tt.expected[i].Path, fc.Path)
				}
				if fc.Status != tt.expected[i].Status {
					t.Errorf("change %d: expected status %q, got %q", i, tt.expected[i].Status, fc.Status)
				}
				if fc.OldPath != tt.expected[i].OldPath {
					t.Errorf("change %d: expected old_path %q, got %q", i, tt.expected[i].OldPath, fc.OldPath)
				}
			}
		})
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"app.ts", true},
		{"app.tsx", true},
		{"script.js", true},
		{"script.jsx", true},
		{"module.mjs", true},
		{"common.cjs", true},
		{"Main.java", true},
		{"lib.rs", true},
		{"script.py", true},
		{"code.c", true},
		{"header.h", true},
		{"code.cpp", true},
		{"code.cc", true},
		{"code.cxx", true},
		{"header.hpp", true},
		{"header.hh", true},
		{"header.hxx", true},
		{"Program.cs", true},
		{"index.php", true},
		{"Main.kt", true},
		{"build.gradle.kts", true},
		{"app.rb", true},
		{"Rakefile.rake", true},
		{"README.md", false},
		{"config.yaml", false},
		{"package.json", false},
		{"Makefile", false},
		{".gitignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isSourceFile(tt.path)
			if result != tt.expected {
				t.Errorf("isSourceFile(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestStatusDescription(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"A", "added"},
		{"M", "modified"},
		{"D", "deleted"},
		{"R", "renamed"},
		{"C", "copied"},
		{"U", "unmerged"},
		{"X", "changed"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := StatusDescription(tt.status)
			if result != tt.expected {
				t.Errorf("StatusDescription(%q) = %q, expected %q", tt.status, result, tt.expected)
			}
		})
	}
}

// TestGitDiffIntegration tests actual git operations if we're in a git repo.
func TestGitDiffIntegration(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir, err := os.MkdirTemp("", "gitdiff-test-*")
	if err != nil {
		t.Skipf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("could not init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("could not write test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	// Modify the file
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("could not modify test file: %v", err)
	}

	// Test GetUncommittedChanges
	gd := NewGitDiff(tmpDir)
	changes, err := gd.GetUncommittedChanges()
	if err != nil {
		t.Fatalf("GetUncommittedChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].Path != "test.go" {
		t.Errorf("expected path 'test.go', got %q", changes[0].Path)
	}

	if changes[0].Status != "M" {
		t.Errorf("expected status 'M', got %q", changes[0].Status)
	}

	// Stage the changes
	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = tmpDir
	cmd.Run()

	// Test GetStagedChanges
	stagedChanges, err := gd.GetStagedChanges()
	if err != nil {
		t.Fatalf("GetStagedChanges failed: %v", err)
	}

	if len(stagedChanges) != 1 {
		t.Fatalf("expected 1 staged change, got %d", len(stagedChanges))
	}

	if stagedChanges[0].Path != "test.go" {
		t.Errorf("expected path 'test.go', got %q", stagedChanges[0].Path)
	}
}
