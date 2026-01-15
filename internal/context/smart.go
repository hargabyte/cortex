// Package context provides intent-aware context assembly for AI agents.
// The smart context system analyzes natural language task descriptions to
// find relevant code entities and assemble focused context within a token budget.
package context

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

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

// EntryPoint represents a discovered entry point for the task.
type EntryPoint struct {
	Entity     *store.Entity `yaml:"-" json:"-"`
	ID         string        `yaml:"id" json:"id"`
	Name       string        `yaml:"name" json:"name"`
	Type       string        `yaml:"type" json:"type"`
	Location   string        `yaml:"location" json:"location"`
	Relevance  float64       `yaml:"relevance" json:"relevance"`
	Reason     string        `yaml:"reason" json:"reason"`
	PageRank   float64       `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`
	IsKeystone bool          `yaml:"is_keystone,omitempty" json:"is_keystone,omitempty"`
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
	Intent       *Intent                     `yaml:"intent" json:"intent"`
	EntryPoints  []*EntryPoint               `yaml:"entry_points" json:"entry_points"`
	Relevant     []*RelevantEntity           `yaml:"relevant_entities" json:"relevant_entities"`
	Excluded     []*ExcludedEntity           `yaml:"excluded,omitempty" json:"excluded,omitempty"`
	TokensUsed   int                         `yaml:"tokens_used" json:"tokens_used"`
	TokensBudget int                         `yaml:"tokens_budget" json:"tokens_budget"`
	Warnings     []string                    `yaml:"warnings,omitempty" json:"warnings,omitempty"`
}

// SmartContextOptions configures the smart context assembly.
type SmartContextOptions struct {
	TaskDescription string  // Natural language task description
	Budget          int     // Token budget (default: 4000)
	Depth           int     // Max hops from entry points (default: 2)
	SearchLimit     int     // Max search results for entry points (default: 20)
	KeystoneBoost   float64 // Multiplier for keystone entities (default: 2.0)
}

// DefaultSmartContextOptions returns default options.
func DefaultSmartContextOptions() SmartContextOptions {
	return SmartContextOptions{
		Budget:        4000,
		Depth:         2,
		SearchLimit:   20,
		KeystoneBoost: 2.0,
	}
}

// SmartContext assembles intent-aware context for a task description.
type SmartContext struct {
	store   *store.Store
	graph   *graph.Graph
	options SmartContextOptions
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

	return &SmartContext{
		store:   s,
		graph:   g,
		options: opts,
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

	// Step 2: Find entry points using search
	entryPoints, err := sc.findEntryPoints(intent)
	if err != nil {
		return nil, err
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
		"add":        "add_feature",
		"implement":  "add_feature",
		"create":     "add_feature",
		"build":      "add_feature",
		"fix":        "fix_bug",
		"repair":     "fix_bug",
		"debug":      "fix_bug",
		"resolve":    "fix_bug",
		"update":     "modify",
		"change":     "modify",
		"modify":     "modify",
		"edit":       "modify",
		"refactor":   "refactor",
		"restructure": "refactor",
		"reorganize": "refactor",
		"clean":      "refactor",
		"optimize":   "optimize",
		"improve":    "optimize",
		"speed":      "optimize",
		"performance": "optimize",
		"remove":     "remove",
		"delete":     "remove",
		"deprecate":  "remove",
		"test":       "test",
		"verify":     "test",
		"document":   "document",
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
