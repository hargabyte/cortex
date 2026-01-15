package coverage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsGOCOVERDIR_EmptyDir(t *testing.T) {
	// Create empty temp directory
	tmpDir, err := os.MkdirTemp("", "gocoverdir-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if IsGOCOVERDIR(tmpDir) {
		t.Error("empty directory should not be detected as GOCOVERDIR")
	}
}

func TestIsGOCOVERDIR_WithCoverageFiles(t *testing.T) {
	// Create temp directory with mock coverage files
	tmpDir, err := os.MkdirTemp("", "gocoverdir-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock covmeta file
	covmetaPath := filepath.Join(tmpDir, "covmeta.abc123")
	if err := os.WriteFile(covmetaPath, []byte("mock"), 0644); err != nil {
		t.Fatalf("create covmeta: %v", err)
	}

	if !IsGOCOVERDIR(tmpDir) {
		t.Error("directory with covmeta.* should be detected as GOCOVERDIR")
	}
}

func TestIsGOCOVERDIR_WithSubdirs(t *testing.T) {
	// Create temp directory with subdirectories containing coverage files
	tmpDir, err := os.MkdirTemp("", "gocoverdir-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectory with coverage file
	subDir := filepath.Join(tmpDir, "TestFoo")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	covmetaPath := filepath.Join(subDir, "covcounters.abc123")
	if err := os.WriteFile(covmetaPath, []byte("mock"), 0644); err != nil {
		t.Fatalf("create covcounters: %v", err)
	}

	if !IsGOCOVERDIR(tmpDir) {
		t.Error("directory with subdirectories containing coverage files should be detected as GOCOVERDIR")
	}
}

func TestIsGOCOVERDIR_RegularFile(t *testing.T) {
	// Create a regular file (not a directory)
	tmpFile, err := os.CreateTemp("", "coverage-*.out")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if IsGOCOVERDIR(tmpFile.Name()) {
		t.Error("regular file should not be detected as GOCOVERDIR")
	}
}

func TestIsGOCOVERDIR_NonExistent(t *testing.T) {
	if IsGOCOVERDIR("/nonexistent/path/that/does/not/exist") {
		t.Error("non-existent path should not be detected as GOCOVERDIR")
	}
}

func TestHasCoverageFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected bool
	}{
		{
			name:     "empty directory",
			files:    []string{},
			expected: false,
		},
		{
			name:     "covmeta file",
			files:    []string{"covmeta.abc123"},
			expected: true,
		},
		{
			name:     "covcounters file",
			files:    []string{"covcounters.abc123.456.789"},
			expected: true,
		},
		{
			name:     "both files",
			files:    []string{"covmeta.abc", "covcounters.abc.1.2"},
			expected: true,
		},
		{
			name:     "unrelated files",
			files:    []string{"README.md", "coverage.out", "test.go"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "hasCoverageFiles-*")
			if err != nil {
				t.Fatalf("create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create test files
			for _, file := range tt.files {
				path := filepath.Join(tmpDir, file)
				if err := os.WriteFile(path, []byte("mock"), 0644); err != nil {
					t.Fatalf("create file %s: %v", file, err)
				}
			}

			result, err := hasCoverageFiles(tmpDir)
			if err != nil {
				t.Fatalf("hasCoverageFiles error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("hasCoverageFiles() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGOCOVERDIRData_HasPerTestAttribution(t *testing.T) {
	tests := []struct {
		name     string
		data     *GOCOVERDIRData
		expected bool
	}{
		{
			name: "empty PerTest",
			data: &GOCOVERDIRData{
				Aggregate: &CoverageData{},
				PerTest:   make(map[string]*CoverageData),
			},
			expected: false,
		},
		{
			name: "with PerTest data",
			data: &GOCOVERDIRData{
				Aggregate: &CoverageData{},
				PerTest: map[string]*CoverageData{
					"TestFoo": {},
					"TestBar": {},
				},
			},
			expected: true,
		},
		{
			name: "only aggregate",
			data: &GOCOVERDIRData{
				Aggregate: &CoverageData{},
				PerTest:   nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle nil PerTest
			if tt.data.PerTest == nil {
				tt.data.PerTest = make(map[string]*CoverageData)
			}
			result := tt.data.HasPerTestAttribution()
			if result != tt.expected {
				t.Errorf("HasPerTestAttribution() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGOCOVERDIRData_TestNames(t *testing.T) {
	data := &GOCOVERDIRData{
		PerTest: map[string]*CoverageData{
			"TestAlpha": {},
			"TestBeta":  {},
			"TestGamma": {},
		},
	}

	names := data.TestNames()
	if len(names) != 3 {
		t.Errorf("TestNames() returned %d names, want 3", len(names))
	}

	// Check all names are present (order not guaranteed)
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, expected := range []string{"TestAlpha", "TestBeta", "TestGamma"} {
		if !nameSet[expected] {
			t.Errorf("TestNames() missing %s", expected)
		}
	}
}

// TestParseGOCOVERDIR_Integration tests parsing real GOCOVERDIR data
// This test is skipped if go tool covdata is not available
func TestParseGOCOVERDIR_Integration(t *testing.T) {
	// Check if go tool covdata is available
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	// Create temp directory for coverage data
	tmpDir, err := os.MkdirTemp("", "gocoverdir-integration-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate real coverage data by running this package's tests
	// with GOCOVERDIR
	cmd := exec.Command("go", "test", "-cover", "-covermode=atomic",
		"-test.gocoverdir="+tmpDir, "-run=TestHasCoverageFiles", ".")
	cmd.Dir = filepath.Dir(tmpDir) // Run from parent dir
	// Set working directory to this package
	wd, _ := os.Getwd()
	cmd.Dir = wd

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to generate coverage data: %v\nOutput: %s", err, string(output))
	}

	// Now parse the generated GOCOVERDIR
	data, err := ParseGOCOVERDIR(tmpDir)
	if err != nil {
		t.Fatalf("ParseGOCOVERDIR failed: %v", err)
	}

	// Validate we got aggregate coverage
	if data.Aggregate == nil {
		t.Error("expected Aggregate coverage data")
	}

	if data.Aggregate != nil && len(data.Aggregate.Blocks) == 0 {
		t.Error("expected non-empty coverage blocks")
	}
}

// TestParseGOCOVERDIR_PerTest tests per-test subdirectory structure
func TestParseGOCOVERDIR_PerTest(t *testing.T) {
	// Check if go tool covdata is available
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	// Create temp directory for per-test coverage data
	tmpDir, err := os.MkdirTemp("", "gocoverdir-pertest-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Get current working directory
	wd, _ := os.Getwd()

	// Run two different tests to separate subdirectories
	tests := []string{"TestHasCoverageFiles", "TestIsGOCOVERDIR_EmptyDir"}
	for _, testName := range tests {
		testDir := filepath.Join(tmpDir, testName)
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("create test dir: %v", err)
		}

		cmd := exec.Command("go", "test", "-cover", "-covermode=atomic",
			"-test.gocoverdir="+testDir, "-run=^"+testName+"$", ".")
		cmd.Dir = wd

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Skipf("failed to generate coverage for %s: %v\nOutput: %s", testName, err, string(output))
		}
	}

	// Parse the per-test GOCOVERDIR
	data, err := ParseGOCOVERDIR(tmpDir)
	if err != nil {
		t.Fatalf("ParseGOCOVERDIR failed: %v", err)
	}

	// Validate per-test attribution
	if !data.HasPerTestAttribution() {
		t.Error("expected per-test attribution")
	}

	if len(data.PerTest) != 2 {
		t.Errorf("expected 2 tests, got %d", len(data.PerTest))
	}

	// Check test names
	for _, testName := range tests {
		if _, ok := data.PerTest[testName]; !ok {
			t.Errorf("missing test %s in PerTest", testName)
		}
	}

	// Aggregate should be merged from per-test data
	if data.Aggregate == nil {
		t.Error("expected merged Aggregate coverage")
	}
}

func TestDeriveTestFile(t *testing.T) {
	tests := []struct {
		testName string
		expected string
	}{
		{"TestFoo", "TestFoo_test.go"},
		{"TestParseGOCOVERDIR", "TestParseGOCOVERDIR_test.go"},
		{"TestHTTPHandler_Success", "TestHTTPHandler_Success_test.go"},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := deriveTestFile(tt.testName)
			if result != tt.expected {
				t.Errorf("deriveTestFile(%q) = %q, want %q", tt.testName, result, tt.expected)
			}
		})
	}
}
