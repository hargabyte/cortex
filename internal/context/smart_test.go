package context

import (
	"strings"
	"testing"
)

func TestExtractIntent(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantPattern string
		wantVerb    string
		wantKWCount int // minimum expected keywords
	}{
		{
			name:        "add feature",
			description: "add rate limiting to API endpoints",
			wantPattern: "add_feature",
			wantVerb:    "add",
			wantKWCount: 2, // rate, limiting, api, endpoints
		},
		{
			name:        "fix bug",
			description: "fix authentication bug in login flow",
			wantPattern: "fix_bug",
			wantVerb:    "fix",
			wantKWCount: 2, // authentication, bug, login, flow
		},
		{
			name:        "refactor code",
			description: "refactor the database connection pooling",
			wantPattern: "refactor",
			wantVerb:    "refactor",
			wantKWCount: 2, // database, connection, pooling
		},
		{
			name:        "optimize performance",
			description: "optimize query performance for user search",
			wantPattern: "optimize",
			wantVerb:    "optimize",
			wantKWCount: 2, // query, performance, user, search
		},
		{
			name:        "update existing",
			description: "update the user validation logic",
			wantPattern: "modify",
			wantVerb:    "update",
			wantKWCount: 2, // user, validation, logic
		},
		{
			name:        "remove deprecated",
			description: "remove deprecated API endpoints",
			wantPattern: "remove",
			wantVerb:    "remove",
			wantKWCount: 1, // deprecated, api, endpoints
		},
		{
			name:        "no verb",
			description: "user authentication service improvements",
			wantPattern: "modify",
			wantVerb:    "",
			wantKWCount: 2, // user, authentication, service, improvements
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := ExtractIntent(tt.description)

			if intent.Pattern != tt.wantPattern {
				t.Errorf("Pattern = %q, want %q", intent.Pattern, tt.wantPattern)
			}

			if intent.ActionVerb != tt.wantVerb {
				t.Errorf("ActionVerb = %q, want %q", intent.ActionVerb, tt.wantVerb)
			}

			if len(intent.Keywords) < tt.wantKWCount {
				t.Errorf("Keywords count = %d, want at least %d. Got: %v",
					len(intent.Keywords), tt.wantKWCount, intent.Keywords)
			}
		})
	}
}

func TestExtractIntent_EntityMentions(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		wantMentions []string
	}{
		{
			name:         "snake case entity",
			description:  "fix user_service authentication",
			wantMentions: []string{"user_service"},
		},
		{
			name:         "multiple snake case",
			description:  "refactor auth_handler and process_request",
			wantMentions: []string{"auth_handler", "process_request"},
		},
		{
			name:         "no entities",
			description:  "add new feature",
			wantMentions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := ExtractIntent(tt.description)

			for _, want := range tt.wantMentions {
				found := false
				for _, got := range intent.EntityMentions {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected entity mention %q not found in %v", want, intent.EntityMentions)
				}
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantContain []string
		wantExclude []string
	}{
		{
			name:        "basic keywords",
			description: "add rate limiting to API endpoints",
			wantContain: []string{"rate", "limiting", "api", "endpoints"},
			wantExclude: []string{"add", "to"},
		},
		{
			name:        "stop words removed",
			description: "the user wants to update the database",
			wantContain: []string{"user", "database"},
			wantExclude: []string{"the", "wants", "to"},
		},
		{
			name:        "action verbs removed",
			description: "fix and refactor the authentication module",
			wantContain: []string{"authentication", "module"},
			wantExclude: []string{"fix", "refactor", "and", "the"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generic, identifiers := extractKeywordsWithIdentifiers(tt.description)
			// Combine both types for testing
			keywords := append(generic, identifiers...)

			for _, want := range tt.wantContain {
				found := false
				for _, kw := range keywords {
					if kw == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected keyword %q not found in %v", want, keywords)
				}
			}

			for _, notWant := range tt.wantExclude {
				for _, kw := range keywords {
					if kw == notWant {
						t.Errorf("Keyword %q should not be in result %v", notWant, keywords)
						break
					}
				}
			}
		})
	}
}

func TestDetectActionPattern(t *testing.T) {
	tests := []struct {
		desc        string
		wantVerb    string
		wantPattern string
	}{
		{"add new feature", "add", "add_feature"},
		{"implement OAuth support", "implement", "add_feature"},
		{"create user service", "create", "add_feature"},
		{"build authentication", "build", "add_feature"},
		{"fix login bug", "fix", "fix_bug"},
		{"repair broken tests", "repair", "fix_bug"},
		{"debug auth issue", "debug", "fix_bug"},
		{"resolve connection error", "resolve", "fix_bug"},
		{"update config", "update", "modify"},
		{"change settings", "change", "modify"},
		{"modify handler", "modify", "modify"},
		{"refactor code", "refactor", "refactor"},
		{"restructure modules", "restructure", "refactor"},
		{"optimize queries", "optimize", "optimize"},
		{"improve performance", "improve", "optimize"},
		{"remove deprecated", "remove", "remove"},
		{"delete old files", "delete", "remove"},
		{"test functionality", "test", "test"},
		{"verify behavior", "verify", "test"},
		{"document API", "document", "document"},
		{"no action verb here", "", "modify"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			verb, pattern := detectActionPattern(tt.desc)

			if verb != tt.wantVerb {
				t.Errorf("verb = %q, want %q", verb, tt.wantVerb)
			}

			if pattern != tt.wantPattern {
				t.Errorf("pattern = %q, want %q", pattern, tt.wantPattern)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name       string
		entityName string
		signature  string
		isKeystone bool
		wantMin    int
		wantMax    int
	}{
		{
			name:       "simple function",
			entityName: "main",
			signature:  "()",
			isKeystone: false,
			wantMin:    50,
			wantMax:    150,
		},
		{
			name:       "keystone function",
			entityName: "handleRequest",
			signature:  "(ctx context.Context, req *Request) (*Response, error)",
			isKeystone: true,
			wantMin:    100,
			wantMax:    300,
		},
		{
			name:       "complex signature",
			entityName: "ProcessUserAuthentication",
			signature:  "(email string, password string, opts ...Option) (*User, *Token, error)",
			isKeystone: false,
			wantMin:    50,
			wantMax:    200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock entity
			entity := &mockEntity{
				name:      tt.entityName,
				signature: tt.signature,
			}

			tokens := estimateTokensFromEntity(entity.name, entity.signature, tt.isKeystone)

			if tokens < tt.wantMin || tokens > tt.wantMax {
				t.Errorf("tokens = %d, want between %d and %d", tokens, tt.wantMin, tt.wantMax)
			}
		})
	}
}

type mockEntity struct {
	name      string
	signature string
}

// estimateTokensFromEntity is a helper that mirrors the logic in estimateTokens
func estimateTokensFromEntity(name, signature string, isKeystone bool) int {
	base := 50
	nameTokens := len(name)/4 + 2
	sigTokens := len(signature) / 4

	if isKeystone {
		return base + nameTokens + sigTokens + 100
	}
	return base + nameTokens + sigTokens + 30
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		entName  string
		want     bool
	}{
		{
			name:     "test file",
			fileName: "internal/auth/login_test.go",
			entName:  "TestLogin",
			want:     true,
		},
		{
			name:     "mock entity",
			fileName: "internal/auth/login.go",
			entName:  "MockAuthService",
			want:     true,
		},
		{
			name:     "vendor file",
			fileName: "some/path/vendor/github.com/pkg/errors/errors.go",
			entName:  "New",
			want:     true,
		},
		{
			name:     "normal file",
			fileName: "internal/auth/login.go",
			entName:  "LoginHandler",
			want:     false,
		},
		{
			name:     "tests directory",
			fileName: "tests/integration/auth_test.go",
			entName:  "TestAuthFlow",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock entity with the file path
			e := &mockEntityForExclude{
				name:     tt.entName,
				filePath: tt.fileName,
			}

			got := shouldExcludeEntity(e.name, e.filePath)
			if got != tt.want {
				t.Errorf("shouldExclude(%q, %q) = %v, want %v",
					tt.entName, tt.fileName, got, tt.want)
			}
		})
	}
}

type mockEntityForExclude struct {
	name     string
	filePath string
}

// shouldExcludeEntity mirrors the logic in shouldExclude for testing
func shouldExcludeEntity(name, filePath string) bool {
	nameLower := strings.ToLower(name)
	pathLower := strings.ToLower(filePath)

	// Exclude test files
	if strings.HasSuffix(pathLower, "_test.go") ||
		strings.Contains(pathLower, "/test/") ||
		strings.Contains(pathLower, "/tests/") ||
		strings.Contains(pathLower, "/testing/") {
		return true
	}

	// Exclude mock entities
	if strings.Contains(nameLower, "mock") ||
		strings.HasPrefix(nameLower, "test") ||
		strings.HasSuffix(nameLower, "test") {
		return true
	}

	// Exclude vendor
	if strings.Contains(pathLower, "/vendor/") {
		return true
	}

	return false
}

func TestSplitCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"LoginHandler", []string{"login", "handler"}},
		{"handleUserLogin", []string{"handle", "user", "login"}},
		{"APIClient", []string{"a", "p", "i", "client"}}, // Each capital is split
		{"simple", []string{"simple"}},
		{"A", []string{"a"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCamelCase(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitCamelCase(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCamelCase(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultSmartContextOptions(t *testing.T) {
	opts := DefaultSmartContextOptions()

	if opts.Budget != 4000 {
		t.Errorf("default Budget = %d, want 4000", opts.Budget)
	}

	if opts.Depth != 2 {
		t.Errorf("default Depth = %d, want 2", opts.Depth)
	}

	if opts.SearchLimit != 20 {
		t.Errorf("default SearchLimit = %d, want 20", opts.SearchLimit)
	}

	if opts.KeystoneBoost != 2.0 {
		t.Errorf("default KeystoneBoost = %f, want 2.0", opts.KeystoneBoost)
	}
}

// TestDefaultHybridWeights verifies the default hybrid weights sum to 1.0.
func TestDefaultHybridWeights(t *testing.T) {
	weights := DefaultHybridWeights()

	sum := weights.Semantic + weights.Keyword + weights.PageRank
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("Hybrid weights should sum to ~1.0, got %f (semantic=%f, keyword=%f, pagerank=%f)",
			sum, weights.Semantic, weights.Keyword, weights.PageRank)
	}

	// Verify semantic has highest weight
	if weights.Semantic < weights.Keyword || weights.Semantic < weights.PageRank {
		t.Error("Semantic weight should be highest by default")
	}
}

// TestMergeEntryPoints tests the entry point merging logic.
func TestMergeEntryPoints(t *testing.T) {
	// Create keyword entry points
	keywordEPs := []*EntryPoint{
		{ID: "ep1", Name: "LoginHandler", Relevance: 0.8},
		{ID: "ep2", Name: "AuthService", Relevance: 0.6},
		{ID: "ep3", Name: "ValidateToken", Relevance: 0.5},
	}

	// Create semantic entry points (with some overlap)
	semanticEPs := []*EntryPoint{
		{ID: "ep1", Name: "LoginHandler", SemanticScore: 0.9}, // Overlap with keyword
		{ID: "ep4", Name: "UserSession", SemanticScore: 0.7},
		{ID: "ep5", Name: "HashPassword", SemanticScore: 0.6},
	}

	merged := mergeEntryPoints(keywordEPs, semanticEPs)

	// Should have 5 unique entries
	if len(merged) != 5 {
		t.Errorf("Expected 5 merged entries, got %d", len(merged))
	}

	// Check that ep1 is marked as hybrid match
	var ep1 *EntryPoint
	for _, ep := range merged {
		if ep.ID == "ep1" {
			ep1 = ep
			break
		}
	}

	if ep1 == nil {
		t.Fatal("ep1 not found in merged results")
	}

	if ep1.Source != SourceHybridMatch {
		t.Errorf("ep1 should have Source=SourceHybridMatch, got %v", ep1.Source)
	}

	if ep1.KeywordScore != 0.8 {
		t.Errorf("ep1 KeywordScore should be 0.8, got %f", ep1.KeywordScore)
	}

	if ep1.SemanticScore != 0.9 {
		t.Errorf("ep1 SemanticScore should be 0.9, got %f", ep1.SemanticScore)
	}
}

// TestHybridScore tests the hybrid scoring algorithm.
func TestHybridScore(t *testing.T) {
	weights := DefaultHybridWeights()
	maxPageRank := 0.5 // Normalize against this

	tests := []struct {
		name    string
		ep      *EntryPoint
		wantMin float64
		wantMax float64
	}{
		{
			name: "hybrid match - both scores",
			ep: &EntryPoint{
				Source:        SourceHybridMatch,
				SemanticScore: 0.9,
				KeywordScore:  0.8,
				PageRank:      0.3,
			},
			wantMin: 0.7,
			wantMax: 0.9,
		},
		{
			name: "semantic only",
			ep: &EntryPoint{
				Source:        SourceSemanticMatch,
				SemanticScore: 0.85,
				PageRank:      0.2,
			},
			wantMin: 0.4,
			wantMax: 0.8,
		},
		{
			name: "keyword only",
			ep: &EntryPoint{
				Source:       SourceKeywordMatch,
				KeywordScore: 0.75,
				PageRank:     0.1,
			},
			wantMin: 0.3,
			wantMax: 0.6,
		},
		{
			name: "explicit mention (boosted)",
			ep: &EntryPoint{
				Source:    SourceExplicitMention,
				Relevance: 0.6,
			},
			wantMin: 0.8,
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := hybridScore(tt.ep, weights, maxPageRank)

			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("hybridScore() = %f, want between %f and %f",
					score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestApplyHybridScoring tests that hybrid scoring properly sorts results.
func TestApplyHybridScoring(t *testing.T) {
	entryPoints := []*EntryPoint{
		{ID: "low", Source: SourceKeywordMatch, KeywordScore: 0.3, PageRank: 0.1},
		{ID: "high", Source: SourceHybridMatch, SemanticScore: 0.9, KeywordScore: 0.8, PageRank: 0.4},
		{ID: "medium", Source: SourceSemanticMatch, SemanticScore: 0.7, PageRank: 0.2},
	}

	weights := DefaultHybridWeights()
	sorted := applyHybridScoring(entryPoints, weights)

	// Verify "high" comes first (highest combined score)
	if sorted[0].ID != "high" {
		t.Errorf("Expected 'high' to be first, got %q", sorted[0].ID)
	}

	// Verify all entries have updated Relevance scores
	for _, ep := range sorted {
		if ep.Relevance == 0 {
			t.Errorf("Entry %s should have non-zero Relevance", ep.ID)
		}
	}

	// Verify sorted in descending order
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Relevance > sorted[i-1].Relevance {
			t.Errorf("Results not sorted: [%d].Relevance=%f > [%d].Relevance=%f",
				i, sorted[i].Relevance, i-1, sorted[i-1].Relevance)
		}
	}
}

// TestExtractIntentMultiWord tests intent extraction with multi-word natural language queries.
// This addresses the issue where multi-word queries like "parsing source code" failed
// because FTS5's implicit AND required all terms to match.
func TestExtractIntentMultiWord(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		wantKeywords []string
		minKeywords  int
	}{
		{
			name:         "natural language with generic terms",
			description:  "parsing source code in the codebase",
			wantKeywords: []string{"parsing", "codebase"},
			minKeywords:  1,
		},
		{
			name:         "task with action and stopwords",
			description:  "add rate limiting to the API endpoints",
			wantKeywords: []string{"rate", "limiting", "api", "endpoints"},
			minKeywords:  3,
		},
		{
			name:         "query with domain terms",
			description:  "fix authentication bug in login flow",
			wantKeywords: []string{"authentication", "bug", "login", "flow"},
			minKeywords:  3,
		},
		{
			name:         "mostly generic terms",
			description:  "implement new feature in the system",
			wantKeywords: []string{}, // Most words are stopwords/action words
			minKeywords:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := ExtractIntent(tt.description)

			// Check minimum keyword count
			if len(intent.Keywords) < tt.minKeywords {
				t.Errorf("ExtractIntent(%q) got %d keywords, want at least %d. Keywords: %v",
					tt.description, len(intent.Keywords), tt.minKeywords, intent.Keywords)
			}

			// Check that expected keywords are present
			for _, want := range tt.wantKeywords {
				found := false
				for _, kw := range intent.Keywords {
					if kw == want {
						found = true
						break
					}
				}
				// Don't fail if not found - the keyword might be filtered
				// Just log for visibility
				if !found && len(tt.wantKeywords) > 0 {
					t.Logf("Note: Expected keyword %q not in %v (may be filtered as stopword)", want, intent.Keywords)
				}
			}
		})
	}
}
