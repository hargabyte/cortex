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
  cx find LoginUser --format=json          # JSON output for parsing`,
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
}

func runFind(cmd *cobra.Command, args []string) error {
	// Get query if provided
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	// If no query and no ranking flags, show error
	if query == "" && !findImportant && !findKeystones && !findBottlenecks {
		return fmt.Errorf("query required (use --important, --keystones, or --bottlenecks for ranked results without query)")
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

	// Determine if this is a rank-only query (no text query, just --important/--keystones/--bottlenecks)
	isRankOnlyQuery := (findImportant || findKeystones || findBottlenecks) && query == ""

	// If we have ranking flags but also a query, we'll combine them
	// Determine search mode: name search (single word) vs concept/FTS search (multi-word)
	isConceptSearch := isMultiWordQuery(query) && !isRankOnlyQuery

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Route to appropriate search mode
	if isRankOnlyQuery || findImportant || findKeystones || findBottlenecks {
		// Use ranking mode
		return runFindWithRanking(cmd, storeDB, cfg, query, isConceptSearch, format, density)
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

	// Query entities from store
	entities, err := storeDB.QueryEntities(filter)
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Filter by name pattern
	matches := filterByName(entities, query, findExact)

	// Apply limit after name filtering
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
	// Get all active entities
	entities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active"})
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
		filter := store.EntityFilter{
			Status: "active",
			Limit:  10000,
		}
		if findType != "" {
			filter.EntityType = mapCGFTypeToStore(findType)
		}
		if findLang != "" {
			filter.Language = normalizeLanguage(findLang)
		}

		allEntities, err := storeDB.QueryEntities(filter)
		if err != nil {
			return fmt.Errorf("failed to query entities: %w", err)
		}

		candidateEntities = filterByName(allEntities, query, findExact)
	} else {
		// No query, use all entities
		candidateEntities = entities
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

	// Filter by importance category if requested
	if findKeystones {
		filtered := []rankedEntity{}
		for _, r := range ranked {
			if r.pagerank >= cfg.Metrics.KeystoneThreshold {
				filtered = append(filtered, r)
			}
		}
		ranked = filtered
	} else if findBottlenecks {
		filtered := []rankedEntity{}
		for _, r := range ranked {
			if r.betweenness >= cfg.Metrics.BottleneckThreshold {
				filtered = append(filtered, r)
			}
		}
		ranked = filtered
	}

	// Sort by appropriate metric
	if findBottlenecks {
		// Sort by betweenness descending
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].betweenness > ranked[j].betweenness
		})
	} else {
		// Sort by PageRank descending (default and keystones)
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
