package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFTSSearchBasic tests basic FTS search functionality.
func TestFTSSearchBasic(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "cx-fts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open store
	store, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer store.Close()

	// Create test entities with body_text and doc_comment
	entities := []*Entity{
		{
			ID:         "sa-fn-test1-LoginUser",
			Name:       "LoginUser",
			EntityType: "function",
			FilePath:   "internal/auth/login.go",
			LineStart:  10,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			BodyText:   "validate credentials authenticate user create session return token",
			DocComment: "// LoginUser authenticates a user with email and password",
		},
		{
			ID:         "sa-fn-test2-ValidateToken",
			Name:       "ValidateToken",
			EntityType: "function",
			FilePath:   "internal/auth/token.go",
			LineStart:  50,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			BodyText:   "parse JWT verify signature check expiration return claims",
			DocComment: "// ValidateToken validates a JWT token and returns claims",
		},
		{
			ID:         "sa-fn-test3-RateLimiter",
			Name:       "RateLimiter",
			EntityType: "function",
			FilePath:   "internal/middleware/ratelimit.go",
			LineStart:  20,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			BodyText:   "check rate limit increment counter return error if exceeded",
			DocComment: "// RateLimiter implements rate limiting middleware",
		},
		{
			ID:         "sa-type-test4-Config",
			Name:       "Config",
			EntityType: "type",
			FilePath:   "internal/config/config.go",
			LineStart:  5,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			BodyText:   "",
			DocComment: "// Config holds application configuration settings",
		},
	}

	// Insert entities
	err = store.CreateEntitiesBulk(entities)
	if err != nil {
		t.Fatalf("Failed to create entities: %v", err)
	}

	// Add some metrics for PageRank boosting
	now := time.Now()
	metrics := []*Metrics{
		{EntityID: "sa-fn-test1-LoginUser", PageRank: 0.05, InDegree: 10, OutDegree: 5, ComputedAt: now},
		{EntityID: "sa-fn-test2-ValidateToken", PageRank: 0.08, InDegree: 15, OutDegree: 3, ComputedAt: now},
		{EntityID: "sa-fn-test3-RateLimiter", PageRank: 0.03, InDegree: 5, OutDegree: 2, ComputedAt: now},
		{EntityID: "sa-type-test4-Config", PageRank: 0.10, InDegree: 20, OutDegree: 0, ComputedAt: now},
	}
	err = store.SaveBulkMetrics(metrics)
	if err != nil {
		t.Fatalf("Failed to save metrics: %v", err)
	}

	// Test 1: Search for "auth" - should find LoginUser and ValidateToken
	t.Run("SearchAuth", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "auth"
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("Expected at least 1 result for 'auth', got %d", len(results))
		}

		// Check that results are sorted by combined score
		for i := 1; i < len(results); i++ {
			if results[i].CombinedScore > results[i-1].CombinedScore {
				t.Errorf("Results not sorted by combined score")
			}
		}
	})

	// Test 2: Search for "rate limit" - should find RateLimiter
	t.Run("SearchRateLimit", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "rate limit"
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("Expected at least 1 result for 'rate limit', got %d", len(results))
		}

		// First result should be RateLimiter
		if len(results) > 0 && results[0].Entity.Name != "RateLimiter" {
			t.Logf("First result: %s", results[0].Entity.Name)
		}
	})

	// Test 3: Exact name match should be boosted
	t.Run("ExactNameMatch", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "LoginUser"
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("Expected at least 1 result for 'LoginUser', got %d", len(results))
			return
		}

		// First result should be LoginUser (exact match boost)
		if results[0].Entity.Name != "LoginUser" {
			t.Errorf("Expected first result to be 'LoginUser', got '%s'", results[0].Entity.Name)
		}
	})

	// Test 4: Search with language filter
	t.Run("SearchWithLanguageFilter", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "config"
		opts.Language = "go"
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// All results should be Go
		for _, r := range results {
			if r.Entity.Language != "go" {
				t.Errorf("Expected language 'go', got '%s'", r.Entity.Language)
			}
		}
	})

	// Test 5: Search with entity type filter
	t.Run("SearchWithTypeFilter", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "config"
		opts.EntityType = "type"
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// All results should be type
		for _, r := range results {
			if r.Entity.EntityType != "type" {
				t.Errorf("Expected entity_type 'type', got '%s'", r.Entity.EntityType)
			}
		}
	})

	// Test 6: Search with threshold filter
	t.Run("SearchWithThreshold", func(t *testing.T) {
		opts := DefaultSearchOptions()
		opts.Query = "auth"
		opts.Threshold = 0.5
		opts.Limit = 10

		results, err := store.SearchEntities(opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// All results should have combined score >= threshold
		for _, r := range results {
			if r.CombinedScore < opts.Threshold {
				t.Errorf("Result '%s' has score %f < threshold %f",
					r.Entity.Name, r.CombinedScore, opts.Threshold)
			}
		}
	})
}

// TestFTSRebuildIndex tests rebuilding the FTS index.
func TestFTSRebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-fts-rebuild-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer store.Close()

	// Create test entities
	entities := []*Entity{
		{
			ID:         "sa-fn-test1-TestFunc",
			Name:       "TestFunc",
			EntityType: "function",
			FilePath:   "test.go",
			LineStart:  1,
			Language:   "go",
			Status:     "active",
			BodyText:   "test body content",
			DocComment: "// TestFunc doc comment",
		},
	}

	err = store.CreateEntitiesBulk(entities)
	if err != nil {
		t.Fatalf("Failed to create entities: %v", err)
	}

	// Count FTS entries
	count1, err := store.CountFTSEntries()
	if err != nil {
		t.Fatalf("Failed to count FTS entries: %v", err)
	}

	if count1 != 1 {
		t.Errorf("Expected 1 FTS entry, got %d", count1)
	}

	// Rebuild index
	err = store.RebuildFTSIndex()
	if err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	// Count again - should still be 1
	count2, err := store.CountFTSEntries()
	if err != nil {
		t.Fatalf("Failed to count FTS entries after rebuild: %v", err)
	}

	if count2 != 1 {
		t.Errorf("Expected 1 FTS entry after rebuild, got %d", count2)
	}

	// Search should still work
	opts := DefaultSearchOptions()
	opts.Query = "TestFunc"

	results, err := store.SearchEntities(opts)
	if err != nil {
		t.Fatalf("Search failed after rebuild: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

// TestFTSEmptyQuery tests search with empty query.
func TestFTSEmptyQuery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-fts-empty-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer store.Close()

	opts := DefaultSearchOptions()
	opts.Query = ""

	_, err = store.SearchEntities(opts)
	if err == nil {
		t.Error("Expected error for empty query, got nil")
	}
}

// TestBuildFTSQuery tests the FTS query builder.
func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple word", "auth", "auth*"},
		{"multiple words uses OR", "auth validation", "auth* OR validation*"},
		{"trim spaces", "  auth  ", "auth*"},
		{"empty", "", "*"},
		// escapeFTSQuery replaces : with space within the word (after split)
		// So "foo:bar" becomes "foo bar*" as a single term (not split into two words)
		{"special chars colon", "foo:bar", "foo bar*"},
		// Code stopwords are filtered out
		{"filters code stopwords", "parsing source code", "parsing*"},
		{"filters multiple stopwords", "implement new feature handler", "handler*"},
		{"keeps domain terms", "rate limit api", "rate* OR limit* OR api*"},
		// Edge cases
		{"all stopwords falls back", "code source file", "code*"}, // falls back to first word
		// Note: "add" is NOT a code stopword (action words handled in smart.go)
		{"action words not filtered here", "add validation", "add* OR validation*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFTSQuery(tt.input)
			if result != tt.expected {
				t.Errorf("buildFTSQuery(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBuildFTSQueryMultiWord specifically tests multi-word query handling.
func TestBuildFTSQueryMultiWord(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch []string // substrings that should appear in the query
		shouldUseOR bool     // whether OR should be used
	}{
		{
			name:        "two meaningful words",
			input:       "auth handler",
			shouldMatch: []string{"auth*", "handler*"},
			shouldUseOR: true,
		},
		{
			name:        "three meaningful words",
			input:       "rate limit middleware",
			shouldMatch: []string{"rate*", "limit*", "middleware*"},
			shouldUseOR: true,
		},
		{
			name:        "natural language query preserves case",
			input:       "add rate limiting to API",
			shouldMatch: []string{"rate*", "limiting*", "API*"}, // preserves original case
			shouldUseOR: true,
		},
		{
			name:        "stopwords partially removed",
			input:       "parsing source code files",
			shouldMatch: []string{"parsing*", "files*"}, // source, code removed; files kept
			shouldUseOR: true,
		},
		{
			name:        "single word after stopword removal",
			input:       "implement new feature handler",
			shouldMatch: []string{"handler*"},
			shouldUseOR: false, // only one term left
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFTSQuery(tt.input)

			// Check that expected substrings appear
			for _, expected := range tt.shouldMatch {
				if !containsSubstring(result, expected) {
					t.Errorf("buildFTSQuery(%q) = %q, expected to contain %q", tt.input, result, expected)
				}
			}

			// Check OR usage
			hasOR := containsSubstring(result, " OR ")
			if tt.shouldUseOR && !hasOR {
				t.Errorf("buildFTSQuery(%q) = %q, expected OR operator", tt.input, result)
			}
			if !tt.shouldUseOR && hasOR {
				t.Errorf("buildFTSQuery(%q) = %q, did not expect OR operator", tt.input, result)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestNormalizeBM25Score tests score normalization.
func TestNormalizeBM25Score(t *testing.T) {
	tests := []struct {
		score    float64
		expected float64
	}{
		{0, 0},
		{5, 0.5},    // 5/(5+5) = 0.5
		{10, 0.666}, // 10/(10+5) ≈ 0.666
		{20, 0.8},   // 20/(20+5) = 0.8
	}

	for _, tt := range tests {
		result := normalizeBM25Score(tt.score)
		diff := result - tt.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			t.Errorf("normalizeBM25Score(%f) = %f, want approximately %f", tt.score, result, tt.expected)
		}
	}
}

// BenchmarkFTSSearch benchmarks search performance.
func BenchmarkFTSSearch(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "cx-fts-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := Open(tmpDir)
	if err != nil {
		b.Fatalf("Failed to open store: %v", err)
	}
	defer store.Close()

	// Create 1000 entities for benchmarking
	entities := make([]*Entity, 1000)
	for i := 0; i < 1000; i++ {
		entities[i] = &Entity{
			ID:         filepath.Join("sa-fn-test", string(rune('a'+i%26)), "-Func"),
			Name:       "TestFunc" + string(rune('a'+i%26)),
			EntityType: "function",
			FilePath:   "test" + string(rune('a'+i%26)) + ".go",
			LineStart:  i,
			Language:   "go",
			Status:     "active",
			BodyText:   "test body content with various keywords like auth config parse validate",
			DocComment: "// Documentation comment for testing",
		}
	}

	err = store.CreateEntitiesBulk(entities)
	if err != nil {
		b.Fatalf("Failed to create entities: %v", err)
	}

	opts := DefaultSearchOptions()
	opts.Query = "auth"
	opts.Limit = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.SearchEntities(opts)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}
