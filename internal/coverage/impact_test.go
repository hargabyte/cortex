package coverage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

// setupTestStore creates a test store with sample entities and coverage data
func setupTestStoreForImpact(t *testing.T) (*store.Store, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "impact_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .cx directory
	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create .cx dir: %v", err)
	}

	// Open store
	s, err := store.Open(cxDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open store: %v", err)
	}

	cleanup := func() {
		s.Close()
		os.RemoveAll(tmpDir)
	}

	return s, cleanup
}

// createTestEntities creates sample entities for testing
func createTestEntities(t *testing.T, s *store.Store) {
	t.Helper()

	entities := []*store.Entity{
		{
			ID:         "sa-fn-test1",
			Name:       "FunctionOne",
			EntityType: "function",
			FilePath:   "internal/auth/login.go",
			LineStart:  10,
			LineEnd:    intPtr(30),
			Language:   "go",
			Status:     "active",
		},
		{
			ID:         "sa-fn-test2",
			Name:       "FunctionTwo",
			EntityType: "function",
			FilePath:   "internal/auth/login.go",
			LineStart:  35,
			LineEnd:    intPtr(50),
			Language:   "go",
			Status:     "active",
		},
		{
			ID:         "sa-fn-test3",
			Name:       "FunctionThree",
			EntityType: "function",
			FilePath:   "internal/api/handler.go",
			LineStart:  10,
			LineEnd:    intPtr(40),
			Language:   "go",
			Status:     "active",
		},
	}

	for _, e := range entities {
		if err := s.CreateEntity(e); err != nil {
			t.Fatalf("failed to create entity %s: %v", e.ID, err)
		}
	}
}

// createTestCoverage creates sample coverage data
func createTestCoverage(t *testing.T, s *store.Store) {
	t.Helper()

	coverages := []EntityCoverage{
		{
			EntityID:        "sa-fn-test1",
			CoveragePercent: 85.0,
			CoveredLines:    []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26},
			UncoveredLines:  []int{27, 28, 29, 30},
		},
		{
			EntityID:        "sa-fn-test2",
			CoveragePercent: 0.0,
			CoveredLines:    []int{},
			UncoveredLines:  []int{35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50},
		},
		{
			EntityID:        "sa-fn-test3",
			CoveragePercent: 50.0,
			CoveredLines:    []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			UncoveredLines:  []int{26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40},
		},
	}

	if err := StoreCoverage(s, coverages); err != nil {
		t.Fatalf("failed to store coverage: %v", err)
	}
}

// createTestMappings creates test-to-entity mappings
func createTestMappings(t *testing.T, s *store.Store) {
	t.Helper()

	mappings := []struct {
		testFile string
		testName string
		entityID string
	}{
		{"internal/auth/login_test.go", "TestLoginSuccess", "sa-fn-test1"},
		{"internal/auth/login_test.go", "TestLoginFailure", "sa-fn-test1"},
		{"internal/api/handler_test.go", "TestHandlerGet", "sa-fn-test3"},
	}

	for _, m := range mappings {
		_, err := s.DB().Exec(`
			INSERT INTO test_entity_map (test_file, test_name, entity_id)
			VALUES (?, ?, ?)
		`, m.testFile, m.testName, m.entityID)
		if err != nil {
			t.Fatalf("failed to create test mapping: %v", err)
		}
	}
}

// createTestMetrics creates metrics for entities
func createTestMetrics(t *testing.T, s *store.Store) {
	t.Helper()

	metrics := []struct {
		entityID  string
		pageRank  float64
		inDegree  int
		outDegree int
	}{
		{"sa-fn-test1", 0.05, 10, 3},  // Keystone
		{"sa-fn-test2", 0.001, 1, 5},  // Normal
		{"sa-fn-test3", 0.02, 6, 2},   // Bottleneck
	}

	for _, m := range metrics {
		if err := s.SaveMetrics(&store.Metrics{
			EntityID:  m.entityID,
			PageRank:  m.pageRank,
			InDegree:  m.inDegree,
			OutDegree: m.outDegree,
		}); err != nil {
			t.Fatalf("failed to store metrics: %v", err)
		}
	}
}

func intPtr(i int) *int {
	return &i
}

func TestGetCoveringTests(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMappings(t, s)

	tests, err := GetCoveringTests(s, "sa-fn-test1")
	if err != nil {
		t.Fatalf("GetCoveringTests failed: %v", err)
	}

	if len(tests) != 2 {
		t.Errorf("expected 2 covering tests, got %d", len(tests))
	}

	// Check test names
	testNames := make(map[string]bool)
	for _, test := range tests {
		testNames[test.TestName] = true
	}

	if !testNames["TestLoginSuccess"] {
		t.Error("expected TestLoginSuccess to be in covering tests")
	}
	if !testNames["TestLoginFailure"] {
		t.Error("expected TestLoginFailure to be in covering tests")
	}
}

func TestGetCoveringTests_NoTests(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMappings(t, s)

	// sa-fn-test2 has no test mappings
	tests, err := GetCoveringTests(s, "sa-fn-test2")
	if err != nil {
		t.Fatalf("GetCoveringTests failed: %v", err)
	}

	if len(tests) != 0 {
		t.Errorf("expected 0 covering tests, got %d", len(tests))
	}
}

func TestGetUncoveredEntities(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMetrics(t, s)

	cfg := config.DefaultConfig()

	uncovered, err := GetUncoveredEntities(s, cfg)
	if err != nil {
		t.Fatalf("GetUncoveredEntities failed: %v", err)
	}

	if uncovered.TotalUncovered != 1 {
		t.Errorf("expected 1 uncovered entity, got %d", uncovered.TotalUncovered)
	}

	// Check that FunctionTwo is uncovered
	found := false
	for _, entities := range uncovered.Files {
		for _, e := range entities {
			if e.Name == "FunctionTwo" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("expected FunctionTwo to be in uncovered entities")
	}
}

func TestGenerateCoverageRecommendations(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMetrics(t, s)

	cfg := config.DefaultConfig()

	recommendations, err := GenerateCoverageRecommendations(s, cfg, 10)
	if err != nil {
		t.Fatalf("GenerateCoverageRecommendations failed: %v", err)
	}

	// Should have recommendations for undertested entities
	if len(recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// First recommendation should be the keystone with lower coverage
	// or the uncovered entity
	if len(recommendations) > 0 {
		first := recommendations[0]
		// The uncovered entity (sa-fn-test2) or the keystone with 85% coverage
		// might be first depending on scoring
		if first.CurrentCoverage > 75 {
			t.Errorf("expected recommendation for entity with coverage below threshold, got %.0f%%", first.CurrentCoverage)
		}
	}
}

func TestGenerateCoverageRecommendations_WithLimit(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMetrics(t, s)

	cfg := config.DefaultConfig()

	// Request only 1 recommendation
	recommendations, err := GenerateCoverageRecommendations(s, cfg, 1)
	if err != nil {
		t.Fatalf("GenerateCoverageRecommendations failed: %v", err)
	}

	if len(recommendations) > 1 {
		t.Errorf("expected at most 1 recommendation, got %d", len(recommendations))
	}
}

func TestAnalyzeImpact_File(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMappings(t, s)
	createTestMetrics(t, s)

	cfg := config.DefaultConfig()

	analysis, err := AnalyzeImpact(s, "internal/auth/login.go", cfg)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if analysis.TargetType != "file" {
		t.Errorf("expected target type 'file', got '%s'", analysis.TargetType)
	}

	if analysis.Summary.TotalEntities != 2 {
		t.Errorf("expected 2 entities, got %d", analysis.Summary.TotalEntities)
	}

	// Check coverage summary
	if analysis.Summary.CoveredEntities != 1 {
		t.Errorf("expected 1 covered entity, got %d", analysis.Summary.CoveredEntities)
	}

	if analysis.Summary.UncoveredEntities != 1 {
		t.Errorf("expected 1 uncovered entity, got %d", analysis.Summary.UncoveredEntities)
	}
}

func TestAnalyzeImpact_Entity(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)
	createTestCoverage(t, s)
	createTestMappings(t, s)
	createTestMetrics(t, s)

	cfg := config.DefaultConfig()

	analysis, err := AnalyzeImpact(s, "sa-fn-test1", cfg)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if analysis.TargetType != "entity" {
		t.Errorf("expected target type 'entity', got '%s'", analysis.TargetType)
	}

	if analysis.Summary.TotalEntities != 1 {
		t.Errorf("expected 1 entity, got %d", analysis.Summary.TotalEntities)
	}

	// Check that covering tests are included
	if len(analysis.Entities) > 0 {
		entity := analysis.Entities[0]
		if len(entity.CoveringTests) != 2 {
			t.Errorf("expected 2 covering tests, got %d", len(entity.CoveringTests))
		}
	}
}

func TestAnalyzeImpact_NotFound(t *testing.T) {
	s, cleanup := setupTestStoreForImpact(t)
	defer cleanup()

	createTestEntities(t, s)

	cfg := config.DefaultConfig()

	_, err := AnalyzeImpact(s, "nonexistent/file.go", cfg)
	if err == nil {
		t.Error("expected error for nonexistent target")
	}
}

func TestEntityRecommendations(t *testing.T) {
	tests := []struct {
		name           string
		coverage       float64
		importance     string
		testCount      int
		expectNonEmpty bool
	}{
		{"uncovered keystone", 0, "keystone", 0, true},
		{"uncovered bottleneck", 0, "bottleneck", 0, true},
		{"uncovered normal", 0, "normal", 0, true},
		{"partially covered keystone", 40, "keystone", 2, true},
		{"well covered", 85, "normal", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := generateEntityRecommendation(tt.coverage, tt.importance, tt.testCount)
			if tt.expectNonEmpty && rec == "" {
				t.Error("expected non-empty recommendation")
			}
			if !tt.expectNonEmpty && rec != "" {
				t.Errorf("expected empty recommendation, got: %s", rec)
			}
		})
	}
}

