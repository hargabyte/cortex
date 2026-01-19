package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/metrics"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find <query>",
	Short: "Find symbols by name, concept, or importance",
	Long: `Unified entity lookup supporting name search, concept search, and ranking.

Query Modes (auto-detected):
  Single word:   Name search (prefix match across all packages)
  Multi-word:    Concept/FTS search (searches names, code, docs)
  Quoted phrase: Concept/FTS search (e.g., "auth validation")

Name Search Formats:
- Simple names: LoginUser (prefix match across all packages)
- Qualified names: auth.LoginUser (package.symbol)
- Path-qualified: auth/login.LoginUser (path/file.symbol)
- Direct IDs: sa-fn-a7f9b2-LoginUser

Ranking Flags:
  --important    Sort results by PageRank importance
  --keystones    Show only keystone entities (highly depended-on)
  --bottlenecks  Show only bottleneck entities (central to paths)

Tag Filtering:
  --tag <tag>    Filter to entities with this tag (repeatable, default: match ALL)
  --tag-any      Match ANY tag instead of ALL when multiple --tag flags used

Time Travel:
  --at <ref>     Query at specific commit/ref (commit hash, branch, tag, HEAD~N)

Change Tracking:
  --since <ref>  Show entities changed since ref (default: HEAD~1 when using --new/--changed/--removed)
  --new          Show only newly added entities
  --changed      Show only modified entities
  --removed      Show only removed entities

Density Levels:
  sparse:  Type and location only (~50-100 tokens per entity)
  medium:  Add signature and basic dependencies (~200-300 tokens)
  dense:   Full details with metrics and hashes (~400-600 tokens)
  smart:   Adaptive based on importance (keystones get dense detail)

Output Formats:
  yaml:  Human-readable YAML (default)
  json:  Machine-parseable JSON
  cgf:   Deprecated token-minimal format

Examples:
  cx find LoginUser                        # Name search: prefix match
  cx find "auth validation"                # Concept search: FTS
  cx find auth.LoginUser                   # Qualified name match
  cx find auth/login.LoginUser             # Path-qualified match
  cx find sa-fn-a7f9b2-LoginUser           # Direct entity ID lookup
  cx find --type=F Login                   # Find functions matching "Login"
  cx find --exact LoginUser                # Exact match only
  cx find --file=login.go User             # Filter by file path
  cx find --lang=go User                   # Filter by language
  cx find --important                      # Top entities by PageRank
  cx find --keystones                      # Only keystone entities
  cx find --important "database"           # FTS search, sorted by importance
  cx find LoginUser --density=sparse       # Minimal output for token budget
  cx find LoginUser --format=json          # JSON output for parsing
  cx find --tag important                  # Find all entities with 'important' tag
  cx find --tag auth --tag security        # Entities with BOTH auth AND security tags
  cx find --tag auth --tag security --tag-any  # Entities with EITHER tag
  cx find Login --tag core                 # Name search filtered by tag
  cx find LoginUser --at HEAD~5            # Find entity 5 commits ago
  cx find --keystones --at abc123          # Keystones at specific commit
  cx find --new                            # Show all new entities since HEAD~1
  cx find --changed                        # Show all modified entities since HEAD~1
  cx find --removed                        # Show all removed entities since HEAD~1
  cx find --since HEAD~5 --new             # New entities since 5 commits ago
  cx find Auth --since HEAD~10             # Auth* entities changed in last 10 commits`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFind,
}

var (
	findType        string
	findFile        string
	findLang        string
	findExact       bool
	findQualified   bool
	findLimit       int
	findImportant   bool
	findKeystones   bool
	findBottlenecks bool
	findTop         int
	findRecompute   bool
	findTags        []string
	findTagAny      bool
	findAt          string // Time travel: query at specific commit/ref
	findSince       string // Change tracking: show entities changed since ref
	findNew         bool   // Change tracking: show only new/added entities
	findChanged     bool   // Change tracking: show only modified entities
	findRemoved     bool   // Change tracking: show only removed entities
)

func init() {
	rootCmd.AddCommand(findCmd)

	// Find-specific flags (per spec)
	findCmd.Flags().StringVar(&findType, "type", "", "Filter by entity type (F|T|M|C|E)")
	findCmd.Flags().StringVar(&findFile, "file", "", "Filter by file path")
	findCmd.Flags().StringVar(&findLang, "lang", "", "Filter by language (go, typescript, python, rust, java)")
	findCmd.Flags().BoolVar(&findExact, "exact", false, "Exact match only (default: prefix match)")
	findCmd.Flags().BoolVar(&findQualified, "qualified", false, "Show qualified names in output")
	findCmd.Flags().IntVar(&findLimit, "limit", 100, "Maximum results to return")

	// Ranking flags (from rank command)
	findCmd.Flags().BoolVar(&findImportant, "important", false, "Sort results by PageRank importance")
	findCmd.Flags().BoolVar(&findKeystones, "keystones", false, "Show only keystone entities (highly depended-on)")
	findCmd.Flags().BoolVar(&findBottlenecks, "bottlenecks", false, "Show only bottleneck entities (central to paths)")
	findCmd.Flags().IntVar(&findTop, "top", 20, "Number of results for --important/--keystones/--bottlenecks")
	findCmd.Flags().BoolVar(&findRecompute, "recompute", false, "Force recompute metrics (for --important/--keystones)")

	// Tag filtering flags
	findCmd.Flags().StringArrayVar(&findTags, "tag", nil, "Filter by tag (can be repeated, default: match ALL tags)")
	findCmd.Flags().BoolVar(&findTagAny, "tag-any", false, "Match ANY tag instead of ALL (use with --tag)")

	// Time travel flag
	findCmd.Flags().StringVar(&findAt, "at", "", "Query at specific commit/ref (e.g., HEAD~5, commit hash, branch)")

	// Change tracking flags
	findCmd.Flags().StringVar(&findSince, "since", "", "Show entities changed since ref (e.g., HEAD~5, commit hash)")
	findCmd.Flags().BoolVar(&findNew, "new", false, "Show only newly added entities (since HEAD~1 or --since ref)")
	findCmd.Flags().BoolVar(&findChanged, "changed", false, "Show only modified entities (since HEAD~1 or --since ref)")
	findCmd.Flags().BoolVar(&findRemoved, "removed", false, "Show only removed entities (since HEAD~1 or --since ref)")
}

func runFind(cmd *cobra.Command, args []string) error {
	// Get query if provided
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	// Validate --at flag if provided
	if findAt != "" && !store.IsValidRef(findAt) {
		return fmt.Errorf("invalid --at ref %q: must be commit hash, branch, tag, or HEAD~N", findAt)
	}

	// Validate --since flag if provided
	if findSince != "" && !store.IsValidRef(findSince) {
		return fmt.Errorf("invalid --since ref %q: must be commit hash, branch, tag, or HEAD~N", findSince)
	}

	// Check if change tracking is enabled
	isChangeTracking := findNew || findChanged || findRemoved || findSince != ""

	// If no query and no ranking flags and no tag filters and no change tracking, show error
	if query == "" && !findImportant && !findKeystones && !findBottlenecks && len(findTags) == 0 && !isChangeTracking {
		return fmt.Errorf("query required (use --important, --keystones, --bottlenecks, --tag, --new, --changed, --removed, or --since for results without query)")
	}

	// Load config for thresholds
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Parse format and density early (needed for change tracking)
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Handle change tracking mode (--since, --new, --changed, --removed)
	if isChangeTracking {
		return runFindWithChangeTracking(cmd, storeDB, query, format, density)
	}

	// Determine if this is a rank-only query (no text query, just --important/--keystones/--bottlenecks)
	isRankOnlyQuery := (findImportant || findKeystones || findBottlenecks) && query == ""

	// If we have ranking flags but also a query, we'll combine them
	// Determine search mode: name search (single word) vs concept/FTS search (multi-word)
	isConceptSearch := isMultiWordQuery(query) && !isRankOnlyQuery

	// Route to appropriate search mode
	if isRankOnlyQuery || findImportant || findKeystones || findBottlenecks {
		// Use ranking mode
		return runFindWithRanking(cmd, storeDB, cfg, query, isConceptSearch, format, density)
	}

	// Tag-only query (no text query, just --tag filters)
	if query == "" && len(findTags) > 0 {
		return runFindByTagsOnly(cmd, storeDB, format, density)
	}

	if isConceptSearch {
		// Use FTS/concept search
		return runFindWithFTS(cmd, storeDB, query, format, density)
	}

	// Default: name search
	return runFindByName(cmd, storeDB, query, format, density)
}

// isMultiWordQuery checks if the query contains multiple words (indicating FTS search)
func isMultiWordQuery(query string) bool {
	if query == "" {
		return false
	}
	// Trim and check for spaces
	trimmed := strings.TrimSpace(query)
	return strings.Contains(trimmed, " ")
}

// runFindWithChangeTracking handles --since, --new, --changed, --removed flags
func runFindWithChangeTracking(cmd *cobra.Command, storeDB *store.Store, query string, format output.Format, density output.Density) error {
	// Determine the "from" ref for the diff
	fromRef := findSince
	if fromRef == "" {
		fromRef = "HEAD~1" // Default to comparing with previous commit
	}

	// Get diff results
	diffOpts := store.DiffOptions{
		FromRef: fromRef,
		ToRef:   "HEAD",
		Table:   "entities",
	}

	diffResult, err := storeDB.DoltDiff(diffOpts)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	// Collect changes based on flags
	// If no specific type flag is set, show all changes
	showAll := !findNew && !findChanged && !findRemoved

	type changeEntry struct {
		change   store.DiffChange
		diffType string
	}
	var changes []changeEntry

	if showAll || findNew {
		for _, c := range diffResult.Added {
			changes = append(changes, changeEntry{c, "added"})
		}
	}
	if showAll || findChanged {
		for _, c := range diffResult.Modified {
			changes = append(changes, changeEntry{c, "modified"})
		}
	}
	if showAll || findRemoved {
		for _, c := range diffResult.Removed {
			changes = append(changes, changeEntry{c, "removed"})
		}
	}

	// Filter by query if provided
	if query != "" {
		var filtered []changeEntry
		for _, c := range changes {
			// Check name matching
			if findExact {
				if c.change.EntityName == query {
					filtered = append(filtered, c)
				}
			} else {
				// Prefix match
				if strings.HasPrefix(strings.ToLower(c.change.EntityName), strings.ToLower(query)) ||
					strings.Contains(strings.ToLower(c.change.EntityName), strings.ToLower(query)) {
					filtered = append(filtered, c)
				}
			}
		}
		changes = filtered
	}

	// Filter by type if specified
	if findType != "" {
		entityType := mapCGFTypeToStore(findType)
		var filtered []changeEntry
		for _, c := range changes {
			if c.change.EntityType == entityType {
				filtered = append(filtered, c)
			}
		}
		changes = filtered
	}

	// Filter by file if specified
	if findFile != "" {
		var filtered []changeEntry
		for _, c := range changes {
			if strings.Contains(c.change.FilePath, findFile) {
				filtered = append(filtered, c)
			}
		}
		changes = filtered
	}

	// Apply limit
	if findLimit > 0 && len(changes) > findLimit {
		changes = changes[:findLimit]
	}

	if len(changes) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No changes found since %s\n", fromRef)
		return nil
	}

	// Build output
	// Use a custom output structure that includes diff type
	type DiffEntityOutput struct {
		Type       string `yaml:"type" json:"type"`
		Location   string `yaml:"location" json:"location"`
		DiffType   string `yaml:"diff_type" json:"diff_type"`
		EntityName string `yaml:"name,omitempty" json:"name,omitempty"`
	}

	type DiffListOutput struct {
		FromRef string                       `yaml:"from_ref" json:"from_ref"`
		ToRef   string                       `yaml:"to_ref" json:"to_ref"`
		Results map[string]*DiffEntityOutput `yaml:"results" json:"results"`
		Count   int                          `yaml:"count" json:"count"`
	}

	diffOutput := &DiffListOutput{
		FromRef: fromRef,
		ToRef:   "HEAD",
		Results: make(map[string]*DiffEntityOutput),
		Count:   0,
	}

	seen := make(map[string]bool)
	for _, c := range changes {
		// Use entity name as key, but handle duplicates
		key := c.change.EntityName
		if seen[key] {
			// Append file path for disambiguation
			key = fmt.Sprintf("%s (%s)", c.change.EntityName, c.change.FilePath)
		}
		seen[key] = true

		location := c.change.FilePath
		if c.change.LineStart > 0 {
			location = fmt.Sprintf("%s:%d", c.change.FilePath, c.change.LineStart)
		}

		diffOutput.Results[key] = &DiffEntityOutput{
			Type:       c.change.EntityType,
			Location:   location,
			DiffType:   c.diffType,
			EntityName: c.change.EntityName,
		}
		diffOutput.Count++
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), diffOutput, density)
}

// runFindByName performs traditional name-based search
func runFindByName(cmd *cobra.Command, storeDB *store.Store, query string, format output.Format, density output.Density) error {
	// Build filter from flags
	filter := store.EntityFilter{
		Status: "active",
		Limit:  10000, // Get all candidates, limit applied after name filtering
	}
	if findType != "" {
		filter.EntityType = mapCGFTypeToStore(findType)
	}
	if findFile != "" {
		filter.FilePath = findFile
	}
	if findLang != "" {
		filter.Language = normalizeLanguage(findLang)
	}

	// Query entities from store (use AS OF if --at specified)
	var entities []*store.Entity
	var err error
	if findAt != "" {
		entities, err = storeDB.QueryEntitiesAt(filter, findAt)
	} else {
		entities, err = storeDB.QueryEntities(filter)
	}
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Filter by name pattern
	matches := filterByName(entities, query, findExact)

	// Filter by tags if specified
	if len(findTags) > 0 {
		matchAll := !findTagAny // default is match ALL tags
		matches = filterByTags(matches, storeDB, findTags, matchAll)
	}

	// Apply limit after filtering
	if findLimit > 0 && len(matches) > findLimit {
		matches = matches[:findLimit]
	}

	// Build ListOutput
	listOutput := &output.ListOutput{
		Results: make(map[string]*output.EntityOutput),
		Count:   0,
	}

	// Deduplicate matches by location (file:line range + name)
	seen := make(map[string]bool)
	for _, e := range matches {
		name := e.Name
		if !findQualified {
			name = extractSymbolName(e)
		}

		// Use location + name as deduplication key
		location := formatEntityLocation(e)
		key := location + ":" + name
		if seen[key] {
			continue
		}
		seen[key] = true

		// Convert store entity to EntityOutput
		entityOut := storeEntityToOutput(e, density, storeDB)
		listOutput.Results[name] = entityOut
		listOutput.Count++
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), listOutput, density)
}

// runFindByTagsOnly performs tag-only search (no query, just --tag filters)
func runFindByTagsOnly(cmd *cobra.Command, storeDB *store.Store, format output.Format, density output.Density) error {
	// Get entities matching the tag criteria
	matchAll := !findTagAny // default is match ALL tags
	entities, err := storeDB.FindByTags(findTags, matchAll)
	if err != nil {
		return fmt.Errorf("failed to find entities by tags: %w", err)
	}

	// Build ListOutput
	listOutput := &output.ListOutput{
		Results: make(map[string]*output.EntityOutput),
		Count:   0,
	}

	// Deduplicate matches by location (file:line range + name)
	seen := make(map[string]bool)
	for _, e := range entities {
		name := e.Name
		if !findQualified {
			name = extractSymbolName(e)
		}

		// Use location + name as deduplication key
		location := formatEntityLocation(e)
		key := location + ":" + name
		if seen[key] {
			continue
		}
		seen[key] = true

		// Convert store entity to EntityOutput
		entityOut := storeEntityToOutput(e, density, storeDB)
		listOutput.Results[name] = entityOut
		listOutput.Count++
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), listOutput, density)
}

// runFindWithFTS performs full-text/concept search (migrated from search command)
func runFindWithFTS(cmd *cobra.Command, storeDB *store.Store, query string, format output.Format, density output.Density) error {
	// Build search options
	opts := store.DefaultSearchOptions()
	opts.Query = query
	opts.Limit = findLimit
	if findLang != "" {
		opts.Language = normalizeLanguage(findLang)
	}
	if findType != "" {
		opts.EntityType = mapCGFTypeToStore(findType)
	}

	// Execute search
	results, err := storeDB.SearchEntities(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Filter by tags if specified
	if len(findTags) > 0 {
		matchAll := !findTagAny // default is match ALL tags
		// Extract entities from results for tag filtering
		entities := make([]*store.Entity, len(results))
		for i, r := range results {
			entities[i] = r.Entity
		}
		filteredEntities := filterByTags(entities, storeDB, findTags, matchAll)

		// Rebuild results with only filtered entities
		filteredIDs := make(map[string]bool)
		for _, e := range filteredEntities {
			filteredIDs[e.ID] = true
		}
		var filteredResults []*store.SearchResult
		for _, r := range results {
			if filteredIDs[r.Entity.ID] {
				filteredResults = append(filteredResults, r)
			}
		}
		results = filteredResults
	}

	if len(results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No results found for: %s\n", query)
		return nil
	}

	// Build output based on format
	searchOutput := buildSearchOutput(results, density, storeDB, query)

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), searchOutput, density)
}

// runFindWithRanking performs ranked search (migrated from rank command)
func runFindWithRanking(cmd *cobra.Command, storeDB *store.Store, cfg *config.Config, query string, isConceptSearch bool, format output.Format, density output.Density) error {
	// Get all active entities (use AS OF if --at specified)
	filter := store.EntityFilter{Status: "active"}
	var entities []*store.Entity
	var err error
	if findAt != "" {
		entities, err = storeDB.QueryEntitiesAt(filter, findAt)
	} else {
		entities, err = storeDB.QueryEntities(filter)
	}
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return fmt.Errorf("no entities found - run `cx scan` first")
	}

	// Check if recompute needed
	needRecompute := findRecompute
	if !needRecompute {
		for _, e := range entities {
			if m, _ := storeDB.GetMetrics(e.ID); m == nil {
				needRecompute = true
				break
			}
		}
	}

	if needRecompute {
		fmt.Fprintf(os.Stderr, "Computing metrics for %d entities...\n", len(entities))

		// Build graph from store
		g, err := graph.BuildFromStore(storeDB)
		if err != nil {
			return fmt.Errorf("failed to build graph: %w", err)
		}

		// Create adjacency map
		adjacency := g.Edges

		// Compute PageRank
		prConfig := metrics.PageRankConfig{
			Damping:       cfg.Metrics.PageRankDamping,
			MaxIterations: cfg.Metrics.PageRankIterations,
			Tolerance:     0.0001,
		}
		pagerank := metrics.ComputePageRank(adjacency, prConfig)

		// Compute betweenness
		betweenness := metrics.ComputeBetweenness(adjacency)

		// Compute degrees
		inDegree, outDegree := metrics.ComputeInOutDegree(adjacency)

		// Save to store
		bulkMetrics := make([]*store.Metrics, 0, len(entities))
		for _, e := range entities {
			m := &store.Metrics{
				EntityID:    e.ID,
				PageRank:    pagerank[e.ID],
				Betweenness: betweenness[e.ID],
				InDegree:    inDegree[e.ID],
				OutDegree:   outDegree[e.ID],
				ComputedAt:  time.Now(),
			}
			bulkMetrics = append(bulkMetrics, m)
		}

		if err := storeDB.SaveBulkMetrics(bulkMetrics); err != nil {
			return fmt.Errorf("failed to save metrics: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Metrics computed and saved.\n")
	}

	// If we have a query, filter entities first
	var candidateEntities []*store.Entity
	if query != "" && isConceptSearch {
		// Use FTS to filter
		opts := store.DefaultSearchOptions()
		opts.Query = query
		opts.Limit = 1000 // Get more candidates for ranking
		if findLang != "" {
			opts.Language = normalizeLanguage(findLang)
		}
		if findType != "" {
			opts.EntityType = mapCGFTypeToStore(findType)
		}

		results, err := storeDB.SearchEntities(opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		for _, r := range results {
			candidateEntities = append(candidateEntities, r.Entity)
		}
	} else if query != "" {
		// Use name filter
		nameFilter := store.EntityFilter{
			Status: "active",
			Limit:  10000,
		}
		if findType != "" {
			nameFilter.EntityType = mapCGFTypeToStore(findType)
		}
		if findLang != "" {
			nameFilter.Language = normalizeLanguage(findLang)
		}

		var allEntities []*store.Entity
		if findAt != "" {
			allEntities, err = storeDB.QueryEntitiesAt(nameFilter, findAt)
		} else {
			allEntities, err = storeDB.QueryEntities(nameFilter)
		}
		if err != nil {
			return fmt.Errorf("failed to query entities: %w", err)
		}

		candidateEntities = filterByName(allEntities, query, findExact)
	} else {
		// No query, use all entities
		candidateEntities = entities
	}

	// Filter by tags if specified
	if len(findTags) > 0 {
		matchAll := !findTagAny // default is match ALL tags
		candidateEntities = filterByTags(candidateEntities, storeDB, findTags, matchAll)
	}

	// Build ranked list
	type rankedEntity struct {
		entity      *store.Entity
		pagerank    float64
		betweenness float64
		inDegree    int
		outDegree   int
	}

	ranked := make([]rankedEntity, 0, len(candidateEntities))
	for _, e := range candidateEntities {
		m, err := storeDB.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue
		}

		ranked = append(ranked, rankedEntity{
			entity:      e,
			pagerank:    m.PageRank,
			betweenness: m.Betweenness,
			inDegree:    m.InDegree,
			outDegree:   m.OutDegree,
		})
	}

	// Sort by appropriate metric
	// Note: We sort first and take top-N, rather than filtering by threshold.
	// This ensures --keystones/--bottlenecks always return the top N most
	// important entities, even if none meet the strict threshold (which can
	// happen in large codebases where PageRank is distributed thinly).
	// The threshold is used only for labeling importance in output.
	if findBottlenecks {
		// Sort by betweenness descending
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].betweenness > ranked[j].betweenness
		})
	} else if findKeystones || findImportant {
		// Sort by PageRank descending (for --keystones and --important)
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].pagerank > ranked[j].pagerank
		})
	} else {
		// Default: sort by PageRank descending
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].pagerank > ranked[j].pagerank
		})
	}

	// Apply top-N limit
	limit := findTop
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	// Build ListOutput with ranked entities
	listOutput := &output.ListOutput{
		Results: make(map[string]*output.EntityOutput),
		Count:   0,
	}

	for _, r := range ranked {
		name := r.entity.Name

		// Determine importance classification
		importance := "normal"
		if r.pagerank >= cfg.Metrics.KeystoneThreshold {
			importance = "keystone"
		} else if r.betweenness >= cfg.Metrics.BottleneckThreshold {
			importance = "bottleneck"
		} else if r.inDegree == 0 {
			importance = "leaf"
		}

		// Build EntityOutput
		entityOut := &output.EntityOutput{
			Type:     mapStoreEntityTypeToString(r.entity.EntityType),
			Location: formatStoreLocation(r.entity),
		}

		// Add metrics (always included for ranking)
		entityOut.Metrics = &output.Metrics{
			PageRank:    r.pagerank,
			InDegree:    r.inDegree,
			OutDegree:   r.outDegree,
			Importance:  importance,
			Betweenness: r.betweenness,
		}

		// Add signature if density includes it
		if density.IncludesSignature() && r.entity.Signature != "" {
			entityOut.Signature = r.entity.Signature
		}

		// Add visibility
		if density.IncludesSignature() {
			entityOut.Visibility = inferVisibility(r.entity.Name)
		}

		listOutput.Results[name] = entityOut
		listOutput.Count++
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), listOutput, density)
}

// filterByName filters entities by name pattern using shared utils
func filterByName(entities []*store.Entity, query string, exact bool) []*store.Entity {
	var matches []*store.Entity

	for _, e := range entities {
		// Check name matching using shared functions from utils.go
		if exact {
			if matchesQueryExact(e, query) {
				matches = append(matches, e)
			}
		} else {
			if matchesQueryExact(e, query) || matchesQueryPrefix(e, query) {
				matches = append(matches, e)
			}
		}
	}

	return matches
}

// filterByTags filters entities to only include those with specified tags
// If matchAll is true, entity must have ALL tags. If false, entity must have ANY tag.
func filterByTags(entities []*store.Entity, storeDB *store.Store, tags []string, matchAll bool) []*store.Entity {
	if len(tags) == 0 {
		return entities
	}

	// Get entities that match the tag criteria
	taggedEntities, err := storeDB.FindByTags(tags, matchAll)
	if err != nil {
		// On error, return original list (fail open)
		return entities
	}

	// Build a set of tagged entity IDs for fast lookup
	taggedIDs := make(map[string]bool)
	for _, e := range taggedEntities {
		taggedIDs[e.ID] = true
	}

	// Filter the input entities to only those in the tagged set
	var filtered []*store.Entity
	for _, e := range entities {
		if taggedIDs[e.ID] {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// mapCGFTypeToStore converts CGF type marker to store entity type
func mapCGFTypeToStore(t string) string {
	switch strings.ToUpper(t) {
	case "F":
		return "function"
	case "T":
		return "type"
	case "M":
		return "module"
	case "C":
		return "constant"
	case "E":
		return "enum"
	default:
		return ""
	}
}

// normalizeLanguage converts language input to canonical form
func normalizeLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "go", "golang":
		return "go"
	case "ts", "typescript":
		return "typescript"
	case "py", "python":
		return "python"
	case "rs", "rust":
		return "rust"
	case "java":
		return "java"
	default:
		return strings.ToLower(lang)
	}
}

// mapEntityTypeToCGF converts store entity type to CGF type marker
func mapEntityTypeToCGF(t string) output.CGFEntityType {
	switch strings.ToLower(t) {
	case "function", "func", "method":
		return output.CGFFunction
	case "type", "struct", "class", "interface":
		return output.CGFType
	case "module", "package", "dir":
		return output.CGFModule
	case "constant", "const", "var", "variable":
		return output.CGFConstant
	case "enum", "enumeration":
		return output.CGFEnum
	default:
		// Default to function for unknown types
		return output.CGFFunction
	}
}

// formatEntityLocation moved to utils.go

// storeEntityToOutput converts a store.Entity to output.EntityOutput
func storeEntityToOutput(e *store.Entity, density output.Density, storeDB *store.Store) *output.EntityOutput {
	entityOut := &output.EntityOutput{
		Type:     mapStoreEntityTypeToString(e.EntityType),
		Location: formatEntityLocation(e),
	}

	// Add signature for medium/dense
	if density.IncludesSignature() && e.Signature != "" {
		entityOut.Signature = e.Signature
	}

	// Add visibility
	if density.IncludesSignature() {
		entityOut.Visibility = inferVisibility(e.Name)
	}

	// Add dependencies for medium/dense
	if density.IncludesEdges() {
		deps := &output.Dependencies{}

		// Get outgoing calls
		depsOut, err := storeDB.GetDependenciesFrom(e.ID)
		if err == nil {
			for _, d := range depsOut {
				if d.DepType == "calls" {
					deps.Calls = append(deps.Calls, d.ToID)
				}
			}
		}

		// Get incoming calls
		depsIn, err := storeDB.GetDependenciesTo(e.ID)
		if err == nil {
			for _, d := range depsIn {
				if d.DepType == "calls" {
					deps.CalledBy = append(deps.CalledBy, output.CalledByEntry{
						Name: d.FromID,
					})
				}
			}
		}

		if len(deps.Calls) > 0 || len(deps.CalledBy) > 0 {
			entityOut.Dependencies = deps
		}
	}

	// Add metrics for dense
	if density.IncludesMetrics() {
		metrics, err := storeDB.GetMetrics(e.ID)
		if err == nil && metrics != nil {
			entityOut.Metrics = &output.Metrics{
				PageRank:  metrics.PageRank,
				InDegree:  metrics.InDegree,
				OutDegree: metrics.OutDegree,
			}
		}
	}

	// Add hashes for dense
	if density.IncludesHashes() && (e.SigHash != "" || e.BodyHash != "") {
		entityOut.Hashes = &output.Hashes{
			Signature: e.SigHash,
			Body:      e.BodyHash,
		}
	}

	return entityOut
}

// Utility functions moved to utils.go
