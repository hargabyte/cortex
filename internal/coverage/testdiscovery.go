// Package coverage provides test discovery and coverage mapping.
// This file implements test function discovery by scanning test files and
// storing discovered test functions as entities with test metadata.
package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
)

// TestDiscovery handles scanning and discovering test functions in a codebase.
type TestDiscovery struct {
	store    *store.Store
	basePath string
}

// NewTestDiscovery creates a new test discovery instance.
func NewTestDiscovery(s *store.Store, basePath string) *TestDiscovery {
	return &TestDiscovery{
		store:    s,
		basePath: basePath,
	}
}

// DiscoveredTest represents a test function discovered during scanning.
type DiscoveredTest struct {
	// EntityID is the cx entity ID for this test
	EntityID string
	// Name is the test function name
	Name string
	// FullName is the fully qualified name (with parent describes for JS)
	FullName string
	// FilePath is the test file path (relative)
	FilePath string
	// StartLine is the starting line number
	StartLine int
	// EndLine is the ending line number
	EndLine int
	// Language is the programming language
	Language string
	// TestType is the type of test (unit, benchmark, etc.)
	TestType extract.TestType
}

// DiscoverTests scans the codebase for test files and returns discovered tests.
// It extracts test function entities from all test files.
func (td *TestDiscovery) DiscoverTests() ([]DiscoveredTest, error) {
	var tests []DiscoveredTest

	// Walk the directory tree looking for test files
	err := filepath.Walk(td.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories and non-source files
		if info.IsDir() {
			// Skip common non-source directories
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" || base == ".git" || base == ".cx" {
				return filepath.SkipDir
			}
			return nil
		}

		// Determine language from file extension
		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}

		// Check if this is a test file
		if !extract.IsTestFile(path, lang) {
			return nil
		}

		// Extract tests from this file
		fileTests, err := td.discoverTestsInFile(path, lang)
		if err != nil {
			// Log warning but continue scanning
			fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", path, err)
			return nil
		}

		tests = append(tests, fileTests...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return tests, nil
}

// discoverTestsInFile extracts test functions from a single file.
func (td *TestDiscovery) discoverTestsInFile(filePath, language string) ([]DiscoveredTest, error) {
	var tests []DiscoveredTest

	// Get relative path
	relPath := filePath
	if td.basePath != "" {
		rel, err := filepath.Rel(td.basePath, filePath)
		if err == nil {
			relPath = rel
		}
	}

	// Create parser for the language
	var lang parser.Language
	switch language {
	case "go":
		lang = parser.Go
	case "typescript":
		lang = parser.TypeScript
	case "javascript":
		lang = parser.JavaScript
	case "python":
		lang = parser.Python
	case "rust":
		lang = parser.Rust
	case "java":
		lang = parser.Java
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	p, err := parser.NewParser(lang)
	if err != nil {
		return nil, fmt.Errorf("create parser: %w", err)
	}
	defer p.Close()

	result, err := p.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}
	result.FilePath = relPath
	defer result.Close()

	// Extract based on language
	switch language {
	case "go":
		extractor := extract.NewExtractorWithBase(result, td.basePath)
		testInfos, err := extract.ExtractGoTestFunctions(extractor)
		if err != nil {
			return nil, fmt.Errorf("extract go tests: %w", err)
		}

		for _, info := range testInfos {
			tests = append(tests, DiscoveredTest{
				EntityID:  generateTestEntityID(relPath, info.Name, int(info.StartLine)),
				Name:      info.Name,
				FullName:  info.FullName,
				FilePath:  relPath,
				StartLine: int(info.StartLine),
				EndLine:   int(info.EndLine),
				Language:  language,
				TestType:  info.TestType,
			})
		}

	case "typescript", "javascript":
		testInfos, err := extract.ExtractJSTestFunctions(result, relPath, language)
		if err != nil {
			return nil, fmt.Errorf("extract js tests: %w", err)
		}

		for _, info := range testInfos {
			tests = append(tests, DiscoveredTest{
				EntityID:  generateTestEntityID(relPath, info.Name, int(info.StartLine)),
				Name:      info.Name,
				FullName:  info.FullName,
				FilePath:  relPath,
				StartLine: int(info.StartLine),
				EndLine:   int(info.EndLine),
				Language:  language,
				TestType:  info.TestType,
			})
		}

	default:
		// For other languages, use general entity extraction and filter
		var extractor interface {
			ExtractAll() ([]extract.Entity, error)
		}

		switch language {
		case "python":
			extractor = extract.NewPythonExtractorWithBase(result, td.basePath)
		case "rust":
			extractor = extract.NewRustExtractorWithBase(result, td.basePath)
		case "java":
			extractor = extract.NewJavaExtractorWithBase(result, td.basePath)
		default:
			return tests, nil
		}

		entities, err := extractor.ExtractAll()
		if err != nil {
			return nil, fmt.Errorf("extract entities: %w", err)
		}

		// Filter to only test functions
		for _, e := range entities {
			e.Language = language
			e.File = relPath
			if extract.IsTestFunction(&e) {
				info := extract.EntityToTestInfo(&e)
				if info != nil {
					tests = append(tests, DiscoveredTest{
						EntityID:  e.GenerateEntityID(),
						Name:      e.Name,
						FullName:  info.FullName,
						FilePath:  relPath,
						StartLine: int(e.StartLine),
						EndLine:   int(e.EndLine),
						Language:  language,
						TestType:  info.TestType,
					})
				}
			}
		}
	}

	return tests, nil
}

// StoreDiscoveredTests saves discovered test functions to the database.
// This allows querying test functions as entities.
func (td *TestDiscovery) StoreDiscoveredTests(tests []DiscoveredTest) error {
	// Store tests as entities with test_type metadata
	for _, test := range tests {
		endLine := test.EndLine
		entity := &store.Entity{
			ID:         test.EntityID,
			Name:       test.Name,
			EntityType: "function",
			Kind:       string(test.TestType), // Use Kind to store test type
			FilePath:   test.FilePath,
			LineStart:  test.StartLine,
			LineEnd:    &endLine,
			Language:   test.Language,
			Status:     "active",
			// Store full name in DocComment field as metadata
			DocComment: fmt.Sprintf("Test: %s", test.FullName),
		}

		// Use INSERT OR REPLACE to handle rescans
		err := td.store.CreateEntity(entity)
		if err != nil {
			// Try update instead
			err = td.store.UpdateEntity(entity)
			if err != nil {
				return fmt.Errorf("store test entity %s: %w", test.EntityID, err)
			}
		}
	}

	return nil
}

// GetDiscoveredTests retrieves all discovered test functions from the database.
func GetDiscoveredTests(s *store.Store, language string) ([]DiscoveredTest, error) {
	// Query entities that are test functions
	// Test functions are stored with Kind set to test type
	filter := store.EntityFilter{
		EntityType: "function",
		Status:     "active",
	}
	if language != "" {
		filter.Language = language
	}

	entities, err := s.QueryEntities(filter)
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}

	var tests []DiscoveredTest
	for _, e := range entities {
		// Check if this is a test function by examining Kind field
		testType := extract.TestType(e.Kind)
		if testType != extract.TestTypeUnit &&
			testType != extract.TestTypeBenchmark &&
			testType != extract.TestTypeExample &&
			testType != extract.TestTypeFuzz {
			continue
		}

		// Also verify it's in a test file
		if !extract.IsTestFile(e.FilePath, e.Language) {
			continue
		}

		endLine := 0
		if e.LineEnd != nil {
			endLine = *e.LineEnd
		}

		tests = append(tests, DiscoveredTest{
			EntityID:  e.ID,
			Name:      e.Name,
			FullName:  e.Name, // Could parse from DocComment
			FilePath:  e.FilePath,
			StartLine: e.LineStart,
			EndLine:   endLine,
			Language:  e.Language,
			TestType:  testType,
		})
	}

	return tests, nil
}

// MapTestToCoverage maps a discovered test to the entities it covers.
// This uses per-test coverage data from GOCOVERDIR or per-test coverage.out files.
func MapTestToCoverage(s *store.Store, testName string, coverageData *CoverageData, basePath string) ([]string, error) {
	// Create mapper for this test's coverage
	mapper := NewMapper(s, coverageData, basePath)
	entityCoverages, err := mapper.MapCoverageToEntities()
	if err != nil {
		return nil, fmt.Errorf("map coverage: %w", err)
	}

	// Return entity IDs that this test covers (has covered lines)
	var coveredEntityIDs []string
	for _, cov := range entityCoverages {
		if len(cov.CoveredLines) > 0 {
			coveredEntityIDs = append(coveredEntityIDs, cov.EntityID)
		}
	}

	return coveredEntityIDs, nil
}

// generateTestEntityID creates a unique entity ID for a test function.
func generateTestEntityID(filePath, name string, line int) string {
	// Use sa-test prefix to distinguish test entities
	pathHash := hashString(filePath)[:6]
	sanitizedName := sanitizeName(name)
	return fmt.Sprintf("sa-test-%s-%d-%s", pathHash, line, sanitizedName)
}

// hashString computes a simple hash of a string.
func hashString(s string) string {
	// Simple hash using Go's built-in hash
	var h uint64
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return fmt.Sprintf("%x", h)
}

// sanitizeName cleans a name for use in entity IDs.
func sanitizeName(name string) string {
	if len(name) > 32 {
		name = name[:32]
	}
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteByte('_')
		}
	}
	return result.String()
}

// detectLanguage determines the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	default:
		return ""
	}
}
