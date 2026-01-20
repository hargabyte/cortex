// Package context provides intent-aware context assembly for AI agents.
// The smart context system analyzes natural language task descriptions to
// find relevant code entities and assemble focused context within a token budget.
package context

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/anthropics/cx/internal/embeddings"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/store"
)

// Intent represents the extracted intent from a task description.
type Intent struct {
	// Keywords are significant terms extracted from the task
	Keywords []string `yaml:"keywords" json:"keywords"`

	// IdentifierKeywords are keywords that look like code identifiers (higher weight)
	IdentifierKeywords []string `yaml:"identifier_keywords,omitempty" json:"identifier_keywords,omitempty"`

	// Pattern is the detected task pattern (e.g., "add_feature", "fix_bug", "refactor")
	Pattern string `yaml:"pattern" json:"pattern"`

	// EntityMentions are potential entity names found in the task
	EntityMentions []string `yaml:"entity_mentions,omitempty" json:"entity_mentions,omitempty"`

	// ActionVerb is the primary action verb (add, fix, update, etc.)
	ActionVerb string `yaml:"action_verb,omitempty" json:"action_verb,omitempty"`
}

// EntryPointSource indicates how an entry point was discovered.
type EntryPointSource string

const (
	SourceExplicitMention EntryPointSource = "explicit_mention"
	SourceKeywordMatch    EntryPointSource = "keyword_match"
	SourceSemanticMatch   EntryPointSource = "semantic_match"
	SourceHybridMatch     EntryPointSource = "hybrid_match"
)

// EntryPoint represents a discovered entry point for the task.
type EntryPoint struct {
	Entity          *store.Entity    `yaml:"-" json:"-"`
	ID              string           `yaml:"id" json:"id"`
	Name            string           `yaml:"name" json:"name"`
	Type            string           `yaml:"type" json:"type"`
	Location        string           `yaml:"location" json:"location"`
	Relevance       float64          `yaml:"relevance" json:"relevance"`
	Reason          string           `yaml:"reason" json:"reason"`
	PageRank        float64          `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`
	IsKeystone      bool             `yaml:"is_keystone,omitempty" json:"is_keystone,omitempty"`
	Source          EntryPointSource `yaml:"source,omitempty" json:"source,omitempty"`
	SemanticScore   float64          `yaml:"semantic_score,omitempty" json:"semantic_score,omitempty"`
	KeywordScore    float64          `yaml:"keyword_score,omitempty" json:"keyword_score,omitempty"`
}

// RelevantEntity represents an entity relevant to the task context.
type RelevantEntity struct {
	Entity     *store.Entity `yaml:"-" json:"-"`
	ID         string        `yaml:"id" json:"id"`
	Name       string        `yaml:"name" json:"name"`
	Type       string        `yaml:"type" json:"type"`
	Location   string        `yaml:"location" json:"location"`
	Hop        int           `yaml:"hop" json:"hop"`
	Relevance  float64       `yaml:"relevance" json:"relevance"`
	Reason     string        `yaml:"reason" json:"reason"`
	PageRank   float64       `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`
	IsKeystone bool          `yaml:"is_keystone,omitempty" json:"is_keystone,omitempty"`
	Tokens     int           `yaml:"tokens" json:"tokens"`
}

// ExcludedEntity represents an entity excluded from context.
type ExcludedEntity struct {
	Name   string `yaml:"name" json:"name"`
	Reason string `yaml:"reason" json:"reason"`
}

// SmartContextResult contains the assembled smart context.
type SmartContextResult struct {
	Intent           *Intent           `yaml:"intent" json:"intent"`
	EntryPoints      []*EntryPoint     `yaml:"entry_points" json:"entry_points"`
	Relevant         []*RelevantEntity `yaml:"relevant_entities" json:"relevant_entities"`
	Excluded         []*ExcludedEntity `yaml:"excluded,omitempty" json:"excluded,omitempty"`
	TokensUsed       int               `yaml:"tokens_used" json:"tokens_used"`
	TokensBudget     int               `yaml:"tokens_budget" json:"tokens_budget"`
	Warnings         []string          `yaml:"warnings,omitempty" json:"warnings,omitempty"`
	HybridSearchUsed bool              `yaml:"hybrid_search_used,omitempty" json:"hybrid_search_used,omitempty"`
}

// SmartContextOptions configures the smart context assembly.
type SmartContextOptions struct {
	TaskDescription string  // Natural language task description
	Budget          int     // Token budget (default: 4000)
	Depth           int     // Max hops from entry points (default: 2)
	SearchLimit     int     // Max search results for entry points (default: 20)
	KeystoneBoost   float64 // Multiplier for keystone entities (default: 2.0)
	TagBoost        float64 // Multiplier for tagged entities (default: 1.5)
	DisableSemantic bool    // Disable semantic search even if embeddings exist
}

// HybridWeights configures the hybrid scoring algorithm.
type HybridWeights struct {
	Semantic float64 // Weight for semantic similarity (default: 0.5)
	Keyword  float64 // Weight for keyword match score (default: 0.3)
	PageRank float64 // Weight for PageRank importance (default: 0.2)
}

// DefaultHybridWeights returns the default hybrid scoring weights.
func DefaultHybridWeights() HybridWeights {
	return HybridWeights{
		Semantic: 0.5,
		Keyword:  0.3,
		PageRank: 0.2,
	}
}

// boostTags defines tags that increase relevance in smart context
var boostTags = map[string]float64{
	"important":   1.8, // User-marked as important
	"keystone":    1.8, // User-marked as keystone
	"core":        1.5, // Core functionality
	"critical":    1.5, // Critical code
	"entry-point": 1.4, // Entry point
	"api":         1.3, // API surface
	"public":      1.2, // Public interface
}

// DefaultSmartContextOptions returns default options.
func DefaultSmartContextOptions() SmartContextOptions {
	return SmartContextOptions{
		Budget:        4000,
		Depth:         2,
		SearchLimit:   20,
		KeystoneBoost: 2.0,
		TagBoost:      1.5,
	}
}

// SmartContext assembles intent-aware context for a task description.
type SmartContext struct {
	store         *store.Store
	graph         *graph.Graph
	options       SmartContextOptions
	embedder      embeddings.Embedder
	hasEmbeddings bool
}

// NewSmartContext creates a new smart context assembler.
func NewSmartContext(s *store.Store, g *graph.Graph, opts SmartContextOptions) *SmartContext {
	if opts.Budget <= 0 {
		opts.Budget = 4000
	}
	if opts.Depth <= 0 {
		opts.Depth = 2
	}
	if opts.SearchLimit <= 0 {
		opts.SearchLimit = 20
	}
	if opts.KeystoneBoost <= 0 {
		opts.KeystoneBoost = 2.0
	}
	if opts.TagBoost <= 0 {
		opts.TagBoost = 1.5
	}

	sc := &SmartContext{
		store:   s,
		graph:   g,
		options: opts,
	}

	// Check if embeddings are available (unless disabled)
	if !opts.DisableSemantic {
		sc.hasEmbeddings = sc.checkEmbeddingsAvailable()
	}

	return sc
}

// checkEmbeddingsAvailable returns true if embeddings exist in the store.
func (sc *SmartContext) checkEmbeddingsAvailable() bool {
	count, err := sc.store.EmbeddingCount()
	if err != nil {
		return false
	}
	return count > 0
}

// initEmbedder initializes the embedder lazily when needed.
func (sc *SmartContext) initEmbedder() error {
	if sc.embedder != nil {
		return nil
	}
	emb, err := embeddings.NewLocalEmbedder()
	if err != nil {
		return err
	}
	sc.embedder = emb
	return nil
}

// Close releases resources held by the SmartContext.
func (sc *SmartContext) Close() {
	if sc.embedder != nil {
		sc.embedder.Close()
		sc.embedder = nil
	}
}

// Assemble builds the smart context for the configured task.
func (sc *SmartContext) Assemble() (*SmartContextResult, error) {
	result := &SmartContextResult{
		TokensBudget: sc.options.Budget,
	}

	// Step 1: Extract intent from task description
	intent := ExtractIntent(sc.options.TaskDescription)
	result.Intent = intent

	// Step 2: Find entry points using hybrid search (semantic + keyword)
	var entryPoints []*EntryPoint
	var err error

	if sc.hasEmbeddings {
		// Hybrid search: combine semantic and keyword results
		keywordEPs, keywordErr := sc.findEntryPoints(intent)
		if keywordErr != nil {
			return nil, keywordErr
		}

		semanticEPs, semanticErr := sc.findSemanticEntryPoints(sc.options.TaskDescription)
		if semanticErr != nil {
			// Semantic search failed - fall back to keyword only
			result.Warnings = append(result.Warnings,
				"Semantic search failed, using keyword search only: "+semanticErr.Error())
			entryPoints = keywordEPs
		} else {
			// Merge and apply hybrid scoring
			entryPoints = mergeEntryPoints(keywordEPs, semanticEPs)
			entryPoints = applyHybridScoring(entryPoints, DefaultHybridWeights())
			result.HybridSearchUsed = true
		}
	} else {
		// No embeddings - use keyword search only
		entryPoints, err = sc.findEntryPoints(intent)
		if err != nil {
			return nil, err
		}
	}

	// Limit entry points after merging
	maxEntryPoints := 10
	if len(entryPoints) > maxEntryPoints {
		entryPoints = entryPoints[:maxEntryPoints]
	}

	result.EntryPoints = entryPoints

	if len(entryPoints) == 0 {
		result.Warnings = append(result.Warnings,
			"No entry points found for task description. Try more specific keywords.")
		return result, nil
	}

	// Step 3: Trace flow from entry points
	relevantEntities, excluded := sc.traceFlow(entryPoints, intent)
	result.Relevant = relevantEntities
	result.Excluded = excluded

	// Calculate total tokens used
	for _, e := range relevantEntities {
		result.TokensUsed += e.Tokens
	}

	return result, nil
}

// ExtractIntent parses a task description and extracts the intent.
func ExtractIntent(description string) *Intent {
	intent := &Intent{}

	// Extract action verb and pattern from original description
	intent.ActionVerb, intent.Pattern = detectActionPattern(strings.ToLower(strings.TrimSpace(description)))

	// Extract entity mentions (CamelCase or snake_case patterns) BEFORE lowercasing
	intent.EntityMentions = extractEntityMentions(description)

	// Normalize the description for keyword extraction
	normalizedDesc := strings.ToLower(strings.TrimSpace(description))

	// Extract keywords, separating identifier-like from generic
	intent.Keywords, intent.IdentifierKeywords = extractKeywordsWithIdentifiers(normalizedDesc)

	return intent
}

// detectActionPattern identifies the action verb and task pattern.
func detectActionPattern(desc string) (verb string, pattern string) {
	// Common action verbs and their patterns
	patterns := map[string]string{
		"add":         "add_feature",
		"implement":   "add_feature",
		"create":      "add_feature",
		"build":       "add_feature",
		"fix":         "fix_bug",
		"repair":      "fix_bug",
		"debug":       "fix_bug",
		"resolve":     "fix_bug",
		"update":      "modify",
		"change":      "modify",
		"modify":      "modify",
		"edit":        "modify",
		"refactor":    "refactor",
		"restructure": "refactor",
		"reorganize":  "refactor",
		"clean":       "refactor",
		"optimize":    "optimize",
		"improve":     "optimize",
		"speed":       "optimize",
		"performance": "optimize",
		"remove":      "remove",
		"delete":      "remove",
		"deprecate":   "remove",
		"test":        "test",
		"verify":      "test",
		"document":    "document",
	}

	words := strings.Fields(desc)
	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,;:!?\"'")
		if pattern, ok := patterns[word]; ok {
			return word, pattern
		}
	}

	// Default to modification pattern if no verb detected
	return "", "modify"
}

// extractKeywordsWithIdentifiers extracts keywords from the description,
// separating identifier-like keywords (higher weight) from generic keywords.
// Identifier-like keywords contain uppercase letters, underscores, or look like code.
func extractKeywordsWithIdentifiers(desc string) (generic []string, identifiers []string) {
	// Stop words to exclude
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "to": true, "for": true,
		"in": true, "on": true, "at": true, "by": true, "with": true,
		"from": true, "of": true, "is": true, "are": true, "was": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"must": true, "and": true, "or": true, "but": true, "if": true,
		"then": true, "so": true, "that": true, "this": true, "these": true,
		"those": true, "it": true, "its": true, "i": true, "me": true,
		"my": true, "we": true, "our": true, "you": true, "your": true,
		"he": true, "she": true, "they": true, "them": true, "their": true,
		"what": true, "which": true, "who": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "every": true,
		"some": true, "any": true, "no": true, "not": true, "only": true,
		"more": true, "most": true, "other": true, "into": true, "through": true,
		"need": true, "needs": true, "want": true, "wants": true, "like": true,
	}

	// Action verbs already captured
	actionWords := map[string]bool{
		"add": true, "implement": true, "create": true, "build": true,
		"fix": true, "repair": true, "debug": true, "resolve": true,
		"update": true, "change": true, "modify": true, "edit": true,
		"refactor": true, "restructure": true, "reorganize": true, "clean": true,
		"optimize": true, "improve": true, "speed": true,
		"remove": true, "delete": true, "deprecate": true,
		"test": true, "verify": true, "document": true,
	}

	words := strings.Fields(desc)
	genericSet := make(map[string]bool)
	identifierSet := make(map[string]bool)

	for _, word := range words {
		// Clean punctuation
		cleanWord := strings.Trim(word, ".,;:!?\"'()[]{}/<>")
		lowerWord := strings.ToLower(cleanWord)

		// Skip short words, stop words, and action words
		if len(lowerWord) < 3 || stopWords[lowerWord] || actionWords[lowerWord] {
			continue
		}

		// Check if word looks like an identifier
		if looksLikeIdentifier(cleanWord) {
			if !identifierSet[lowerWord] {
				identifierSet[lowerWord] = true
				identifiers = append(identifiers, lowerWord)
			}
		} else {
			if !genericSet[lowerWord] {
				genericSet[lowerWord] = true
				generic = append(generic, lowerWord)
			}
		}
	}

	return generic, identifiers
}

// looksLikeIdentifier returns true if a word looks like a code identifier.
// This includes words with uppercase letters, underscores, or mixed case.
func looksLikeIdentifier(word string) bool {
	hasUpper := false
	hasUnderscore := strings.Contains(word, "_")

	for _, r := range word {
		if unicode.IsUpper(r) {
			hasUpper = true
			break
		}
	}

	// Identifier-like if it has uppercase or underscores
	return hasUpper || hasUnderscore
}

// extractEntityMentions finds potential code entity names in the description.
func extractEntityMentions(desc string) []string {
	var mentions []string
	seen := make(map[string]bool)

	// Pattern for CamelCase identifiers
	camelCase := regexp.MustCompile(`[A-Z][a-z]+(?:[A-Z][a-z]+)+`)

	// Pattern for snake_case identifiers
	snakeCase := regexp.MustCompile(`[a-z]+(?:_[a-z]+)+`)

	// Pattern for code-like identifiers (e.g., handleLogin, processAPI)
	codeLike := regexp.MustCompile(`[a-z]+[A-Z][a-zA-Z]*`)

	// Find all matches
	for _, pattern := range []*regexp.Regexp{camelCase, snakeCase, codeLike} {
		matches := pattern.FindAllString(desc, -1)
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				mentions = append(mentions, match)
			}
		}
	}

	return mentions
}

// findEntryPoints discovers relevant entry points using search.
func (sc *SmartContext) findEntryPoints(intent *Intent) ([]*EntryPoint, error) {
	var entryPoints []*EntryPoint
	seenIDs := make(map[string]bool)

	// First, search for explicitly mentioned entities
	for _, mention := range intent.EntityMentions {
		results, err := sc.store.SearchEntities(store.SearchOptions{
			Query: mention,
			Limit: 5,
		})
		if err != nil {
			continue
		}

		for _, r := range results {
			if seenIDs[r.Entity.ID] {
				continue
			}
			seenIDs[r.Entity.ID] = true

			ep := &EntryPoint{
				Entity:    r.Entity,
				ID:        r.Entity.ID,
				Name:      r.Entity.Name,
				Type:      r.Entity.EntityType,
				Location:  formatLocation(r.Entity),
				Relevance: r.CombinedScore * 1.5, // Boost explicit mentions
				Reason:    "Explicitly mentioned in task",
				PageRank:  r.PageRank,
			}

			if r.PageRank >= 0.15 {
				ep.IsKeystone = true
			}

			entryPoints = append(entryPoints, ep)
		}
	}

	// Search using keywords
	if len(intent.Keywords) > 0 {
		query := strings.Join(intent.Keywords, " ")
		results, err := sc.store.SearchEntities(store.SearchOptions{
			Query: query,
			Limit: sc.options.SearchLimit,
		})
		if err == nil {
			for _, r := range results {
				if seenIDs[r.Entity.ID] {
					continue
				}
				seenIDs[r.Entity.ID] = true

				ep := &EntryPoint{
					Entity:    r.Entity,
					ID:        r.Entity.ID,
					Name:      r.Entity.Name,
					Type:      r.Entity.EntityType,
					Location:  formatLocation(r.Entity),
					Relevance: r.CombinedScore,
					Reason:    "Keyword match: " + query,
					PageRank:  r.PageRank,
				}

				if r.PageRank >= 0.15 {
					ep.IsKeystone = true
				}

				entryPoints = append(entryPoints, ep)
			}
		}

		// Fallback: if combined search returned few results, search individual keywords
		// and merge results. This handles cases where terms don't co-occur but are
		// individually relevant.
		minResultsThreshold := 3
		if len(entryPoints) < minResultsThreshold && len(intent.Keywords) > 1 {
			for _, kw := range intent.Keywords {
				// Skip short keywords that are likely noise
				if len(kw) < 3 {
					continue
				}
				kwResults, kwErr := sc.store.SearchEntities(store.SearchOptions{
					Query: kw,
					Limit: 5, // Limit per-keyword results
				})
				if kwErr != nil {
					continue
				}
				for _, r := range kwResults {
					if seenIDs[r.Entity.ID] {
						continue
					}
					seenIDs[r.Entity.ID] = true

					ep := &EntryPoint{
						Entity:    r.Entity,
						ID:        r.Entity.ID,
						Name:      r.Entity.Name,
						Type:      r.Entity.EntityType,
						Location:  formatLocation(r.Entity),
						Relevance: r.CombinedScore * 0.8, // Slightly lower score for fallback results
						Reason:    "Keyword match: " + kw,
						PageRank:  r.PageRank,
					}

					if r.PageRank >= 0.15 {
						ep.IsKeystone = true
					}

					entryPoints = append(entryPoints, ep)
				}
			}
		}
	}

	// Sort by relevance
	sort.Slice(entryPoints, func(i, j int) bool {
		// Keystones first, then by relevance
		if entryPoints[i].IsKeystone != entryPoints[j].IsKeystone {
			return entryPoints[i].IsKeystone
		}
		return entryPoints[i].Relevance > entryPoints[j].Relevance
	})

	// Limit entry points
	maxEntryPoints := 10
	if len(entryPoints) > maxEntryPoints {
		entryPoints = entryPoints[:maxEntryPoints]
	}

	return entryPoints, nil
}

// mergeEntryPoints combines keyword and semantic entry points, deduplicating by entity ID.
// For entities found by both methods, it combines their scores.
func mergeEntryPoints(keywordEPs, semanticEPs []*EntryPoint) []*EntryPoint {
	merged := make(map[string]*EntryPoint)

	// Add keyword entry points first
	for _, ep := range keywordEPs {
		epCopy := *ep
		epCopy.Source = SourceKeywordMatch
		epCopy.KeywordScore = ep.Relevance
		merged[ep.ID] = &epCopy
	}

	// Merge semantic entry points
	for _, ep := range semanticEPs {
		if existing, ok := merged[ep.ID]; ok {
			// Entity found by both - combine scores
			existing.Source = SourceHybridMatch
			existing.SemanticScore = ep.SemanticScore
			existing.Reason = "Hybrid match: keyword + semantic"
		} else {
			// New semantic-only entry point
			epCopy := *ep
			merged[ep.ID] = &epCopy
		}
	}

	// Convert map to slice
	result := make([]*EntryPoint, 0, len(merged))
	for _, ep := range merged {
		result = append(result, ep)
	}

	return result
}

// hybridScore calculates the final relevance score using hybrid weighting.
// Formula: semantic_weight * semantic_score + keyword_weight * keyword_score + pagerank_weight * normalized_pagerank
func hybridScore(ep *EntryPoint, weights HybridWeights, maxPageRank float64) float64 {
	// Normalize PageRank to 0-1 range
	normalizedPR := 0.0
	if maxPageRank > 0 {
		normalizedPR = ep.PageRank / maxPageRank
	}

	// Calculate weighted score based on source
	switch ep.Source {
	case SourceHybridMatch:
		// Both semantic and keyword scores available - use full formula
		return weights.Semantic*ep.SemanticScore +
			weights.Keyword*ep.KeywordScore +
			weights.PageRank*normalizedPR

	case SourceSemanticMatch:
		// Only semantic score - weight it higher since keyword is missing
		adjustedSemanticWeight := weights.Semantic + weights.Keyword*0.5
		return adjustedSemanticWeight*ep.SemanticScore +
			weights.PageRank*normalizedPR

	case SourceKeywordMatch:
		// Only keyword score - weight it higher since semantic is missing
		adjustedKeywordWeight := weights.Keyword + weights.Semantic*0.5
		return adjustedKeywordWeight*ep.KeywordScore +
			weights.PageRank*normalizedPR

	case SourceExplicitMention:
		// Explicit mentions get boosted score
		return ep.Relevance * 1.5

	default:
		return ep.Relevance
	}
}

// applyHybridScoring applies hybrid scoring to entry points and sorts by final score.
func applyHybridScoring(entryPoints []*EntryPoint, weights HybridWeights) []*EntryPoint {
	if len(entryPoints) == 0 {
		return entryPoints
	}

	// Find max PageRank for normalization
	maxPageRank := 0.0
	for _, ep := range entryPoints {
		if ep.PageRank > maxPageRank {
			maxPageRank = ep.PageRank
		}
	}

	// Calculate hybrid scores
	for _, ep := range entryPoints {
		ep.Relevance = hybridScore(ep, weights, maxPageRank)
	}

	// Sort by final relevance score, with keystones having priority
	sort.Slice(entryPoints, func(i, j int) bool {
		if entryPoints[i].IsKeystone != entryPoints[j].IsKeystone {
			return entryPoints[i].IsKeystone
		}
		return entryPoints[i].Relevance > entryPoints[j].Relevance
	})

	return entryPoints
}

// findSemanticEntryPoints discovers entry points using embedding-based semantic search.
func (sc *SmartContext) findSemanticEntryPoints(taskDescription string) ([]*EntryPoint, error) {
	// Initialize embedder if needed
	if err := sc.initEmbedder(); err != nil {
		return nil, err
	}

	// Embed the task description
	ctx := context.Background()
	queryVec, err := sc.embedder.Embed(ctx, taskDescription)
	if err != nil {
		return nil, err
	}

	// Find similar entities (get more than we need for merging)
	limit := sc.options.SearchLimit * 2
	if limit < 40 {
		limit = 40
	}
	results, err := sc.store.FindSimilar(queryVec, limit)
	if err != nil {
		return nil, err
	}

	var entryPoints []*EntryPoint
	for _, r := range results {
		// Get entity details
		entity, err := sc.store.GetEntity(r.EntityID)
		if err != nil || entity == nil {
			continue
		}

		// Get metrics for PageRank
		metrics, _ := sc.store.GetMetrics(r.EntityID)
		pageRank := 0.0
		isKeystone := false
		if metrics != nil {
			pageRank = metrics.PageRank
			isKeystone = metrics.PageRank >= 0.15 || metrics.InDegree >= 10
		}

		ep := &EntryPoint{
			Entity:        entity,
			ID:            entity.ID,
			Name:          entity.Name,
			Type:          entity.EntityType,
			Location:      formatLocation(entity),
			Relevance:     r.Similarity, // Will be recalculated in hybrid scoring
			Reason:        "Semantic similarity to task",
			PageRank:      pageRank,
			IsKeystone:    isKeystone,
			Source:        SourceSemanticMatch,
			SemanticScore: r.Similarity,
		}

		entryPoints = append(entryPoints, ep)
	}

	return entryPoints, nil
}

// traceFlow traces dependencies from entry points within the token budget.
func (sc *SmartContext) traceFlow(entryPoints []*EntryPoint, intent *Intent) ([]*RelevantEntity, []*ExcludedEntity) {
	var relevant []*RelevantEntity
	var excluded []*ExcludedEntity
	seenIDs := make(map[string]bool)
	tokensUsed := 0

	// Add entry points as relevant entities (hop 0)
	for _, ep := range entryPoints {
		if seenIDs[ep.ID] {
			continue
		}

		tokens := estimateTokens(ep.Entity, ep.IsKeystone)
		if tokensUsed+tokens > sc.options.Budget {
			excluded = append(excluded, &ExcludedEntity{
				Name:   ep.Name,
				Reason: "Over budget",
			})
			continue
		}

		seenIDs[ep.ID] = true
		tokensUsed += tokens

		relevant = append(relevant, &RelevantEntity{
			Entity:     ep.Entity,
			ID:         ep.ID,
			Name:       ep.Name,
			Type:       ep.Type,
			Location:   ep.Location,
			Hop:        0,
			Relevance:  ep.Relevance,
			Reason:     ep.Reason,
			PageRank:   ep.PageRank,
			IsKeystone: ep.IsKeystone,
			Tokens:     tokens,
		})
	}

	// BFS expansion from entry points
	type queueItem struct {
		entityID string
		hop      int
		fromID   string
	}

	queue := []queueItem{}

	// Initialize queue with successors and predecessors of entry points
	for _, ep := range entryPoints {
		if ep.Entity == nil {
			continue
		}

		// Get successors (what this entity calls)
		successors := sc.graph.Successors(ep.ID)
		for _, succID := range successors {
			if !seenIDs[succID] {
				queue = append(queue, queueItem{succID, 1, ep.ID})
			}
		}

		// Get predecessors (what calls this entity) - less priority
		predecessors := sc.graph.Predecessors(ep.ID)
		for _, predID := range predecessors {
			if !seenIDs[predID] {
				queue = append(queue, queueItem{predID, 1, ep.ID})
			}
		}
	}

	// Process queue
	for len(queue) > 0 && tokensUsed < sc.options.Budget {
		item := queue[0]
		queue = queue[1:]

		if seenIDs[item.entityID] {
			continue
		}

		if item.hop > sc.options.Depth {
			continue
		}

		// Get entity
		entity, err := sc.store.GetEntity(item.entityID)
		if err != nil || entity == nil {
			continue
		}

		// Skip test files and mock entities
		if shouldExclude(entity) {
			excluded = append(excluded, &ExcludedEntity{
				Name:   entity.Name,
				Reason: "Test/mock code",
			})
			continue
		}

		// Get metrics for the entity
		metrics, _ := sc.store.GetMetrics(item.entityID)
		pageRank := 0.0
		isKeystone := false
		if metrics != nil {
			pageRank = metrics.PageRank
			isKeystone = metrics.PageRank >= 0.15 || metrics.InDegree >= 10
		}

		tokens := estimateTokens(entity, isKeystone)

		if tokensUsed+tokens > sc.options.Budget {
			excluded = append(excluded, &ExcludedEntity{
				Name:   entity.Name,
				Reason: "Over budget",
			})
			continue
		}

		seenIDs[item.entityID] = true
		tokensUsed += tokens

		// Calculate relevance based on distance and importance
		relevance := 1.0 / float64(item.hop+1)
		if isKeystone {
			relevance *= sc.options.KeystoneBoost
		}

		// Boost entities based on their tags
		entityTags, _ := sc.store.GetTags(item.entityID)
		for _, t := range entityTags {
			tagLower := strings.ToLower(t.Tag)
			if boost, hasBoost := boostTags[tagLower]; hasBoost {
				relevance *= boost
				break // Apply only the highest boost (tags are sorted, so first match wins)
			}
		}

		// Boost entities that match identifier-like keywords (strong boost for names)
		entityNameLower := strings.ToLower(entity.Name)
		for _, kw := range intent.IdentifierKeywords {
			if strings.EqualFold(entity.Name, kw) {
				// Exact name match - very strong boost
				relevance *= 2.5
				break
			} else if strings.Contains(entityNameLower, kw) {
				// Partial name match - strong boost
				relevance *= 1.8
				break
			}
		}

		// Boost entities that match generic keywords (name only, no body text)
		for _, kw := range intent.Keywords {
			if strings.Contains(entityNameLower, kw) {
				relevance *= 1.2
				break
			}
		}

		reason := "Flow trace from " + item.fromID
		if item.hop == 1 {
			// Check if it's a caller or callee
			successors := sc.graph.Successors(item.fromID)
			for _, s := range successors {
				if s == item.entityID {
					reason = "Called by entry point"
					break
				}
			}
			predecessors := sc.graph.Predecessors(item.fromID)
			for _, p := range predecessors {
				if p == item.entityID {
					reason = "Calls entry point"
					break
				}
			}
		}

		relevant = append(relevant, &RelevantEntity{
			Entity:     entity,
			ID:         entity.ID,
			Name:       entity.Name,
			Type:       entity.EntityType,
			Location:   formatLocation(entity),
			Hop:        item.hop,
			Relevance:  relevance,
			Reason:     reason,
			PageRank:   pageRank,
			IsKeystone: isKeystone,
			Tokens:     tokens,
		})

		// Add neighbors to queue for next hop
		if item.hop < sc.options.Depth {
			for _, succID := range sc.graph.Successors(item.entityID) {
				if !seenIDs[succID] {
					queue = append(queue, queueItem{succID, item.hop + 1, item.entityID})
				}
			}
		}
	}

	// Sort relevant entities: entry points first, then by relevance
	sort.Slice(relevant, func(i, j int) bool {
		// Entry points (hop 0) first
		if relevant[i].Hop != relevant[j].Hop {
			return relevant[i].Hop < relevant[j].Hop
		}
		// Then by keystone status
		if relevant[i].IsKeystone != relevant[j].IsKeystone {
			return relevant[i].IsKeystone
		}
		// Then by relevance
		return relevant[i].Relevance > relevant[j].Relevance
	})

	return relevant, excluded
}

// formatLocation formats an entity location string.
func formatLocation(e *store.Entity) string {
	if e.LineEnd != nil && *e.LineEnd != e.LineStart {
		return e.FilePath + ":" + itoa(e.LineStart) + "-" + itoa(*e.LineEnd)
	}
	return e.FilePath + ":" + itoa(e.LineStart)
}

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// estimateTokens estimates the token count for an entity.
func estimateTokens(e *store.Entity, isKeystone bool) int {
	if e == nil {
		return 0
	}

	// Base tokens for metadata
	base := 50

	// Name tokens
	nameTokens := len(strings.Fields(e.Name)) + 2

	// Signature tokens
	sigTokens := len(strings.Fields(e.Signature))

	// Keystones get more detail
	if isKeystone {
		// Include more context for important entities
		return base + nameTokens + sigTokens + 100
	}

	return base + nameTokens + sigTokens + 30
}

// shouldExclude checks if an entity should be excluded from context.
func shouldExclude(e *store.Entity) bool {
	nameLower := strings.ToLower(e.Name)
	pathLower := strings.ToLower(e.FilePath)

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

// splitCamelCase splits a CamelCase string into words.
func splitCamelCase(s string) []string {
	var words []string
	var current []rune

	for _, r := range s {
		if unicode.IsUpper(r) && len(current) > 0 {
			words = append(words, string(current))
			current = []rune{unicode.ToLower(r)}
		} else {
			current = append(current, unicode.ToLower(r))
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}
