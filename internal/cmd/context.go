package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/context"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context [target]",
	Short: "Export relevant context for AI consumption",
	Long: `Assemble task-relevant context within a token budget in YAML format.

Target can be:
  - A bead ID (task context): expands linked code and dependencies
  - A file path: all entities defined in that file
  - An entity ID: single entity and its dependencies

Smart Context (--smart):
  Parse natural language task descriptions to find relevant code context.
  Uses intent extraction, keyword search, and flow tracing to assemble
  focused context within the token budget.

  cx context --smart "add rate limiting to API endpoints" --budget 8000
  cx context --smart "fix auth bug in login" --budget 6000
  cx context --smart "optimize database queries" --budget 10000

Output Structure:
  context:      Metadata (target, budget, tokens_used)
  entry_points: Main functions/classes (starting point for analysis)
  relevant:     Code entities sorted by relevance
  excluded:     Entities filtered out (tests, mocks, etc.)

Token Budget Behavior:
When context exceeds --max-tokens, entries are pruned based on --budget-mode:

  importance (default): Keep keystones/high-PR entities, drop leaves first
  distance:            Keep direct deps (1-hop), drop distant hops first

Relevance Levels:
  high:      Direct dependencies, entry points
  medium:    Transitive dependencies
  low:       Distant or weak relationships

Exclusion Rules (applied by default):
  - Test files and test functions
  - Mock objects and test doubles
  - vendor/ and third-party packages
  - Can be overridden with --include flag

Inclusion Controls:
  --include deps      Include direct dependencies
  --include callers   Include entities that call target
  --include types     Include type definitions
  --exclude tests     Exclude test code (default)
  --exclude mocks     Exclude mock code (default)

Density Effects:
  sparse:   Type, location, relevance only
  medium:   Add signatures and dependency reasons (default)
  dense:    Include actual code snippets and full analysis

Coverage Information (--with-coverage):
  When enabled, adds coverage data to each relevant entity:
  - coverage: Test coverage percentage (e.g., "45.5%")
  - coverage_warning: Warning for keystones with <50% coverage

Examples:
  cx context bd-a7c                                # Task context, default budget
  cx context src/auth/login.go                     # File context
  cx context sa-fn-a7f9b2-LoginUser                # Entity context
  cx context bd-a7c --hops=2                       # 2-hop graph expansion
  cx context bd-a7c --max-tokens=8000              # Larger token budget
  cx context bd-a7c --budget-mode=distance         # Distance-based pruning
  cx context bd-a7c --density=dense                # Full detail with code
  cx context bd-a7c --include deps,types           # Explicit inclusions
  cx context bd-a7c --exclude tests,mocks          # Explicit exclusions
  cx context bd-a7c --format=json                  # JSON output
  cx context --smart "add rate limiting to API"    # Smart context assembly
  cx context --smart "task" --with-coverage        # Include coverage data
  cx context <entity> --with-coverage              # Entity context with coverage`,
	Args: cobra.MaximumNArgs(1),
	RunE: runContext,
}

var (
	contextHops         int
	contextMaxTokens    int
	contextBudget       string
	contextDensity      string
	contextInclude      []string
	contextExclude      []string
	contextForTask      string
	contextSmart        string
	contextDepth        int
	contextWithCoverage bool
)

func init() {
	rootCmd.AddCommand(contextCmd)

	// Context-specific flags matching the spec
	contextCmd.Flags().IntVar(&contextHops, "hops", 1, "Graph expansion depth")
	contextCmd.Flags().IntVar(&contextMaxTokens, "max-tokens", 4000, "Token budget")
	contextCmd.Flags().IntVar(&contextMaxTokens, "budget", 4000, "Token budget (alias for --max-tokens)")
	contextCmd.Flags().StringVar(&contextBudget, "budget-mode", "importance", "Budget mode (importance|distance)")
	contextCmd.Flags().StringVar(&contextDensity, "density", "medium", "Detail level (sparse|medium|dense)")
	contextCmd.Flags().StringSliceVar(&contextInclude, "include", nil, "What to expand (deps,callers,types)")
	contextCmd.Flags().StringSliceVar(&contextExclude, "exclude", nil, "What to skip (tests,mocks)")
	contextCmd.Flags().StringVar(&contextForTask, "for-task", "", "Bead/task ID to get context for (requires beads integration)")

	// Smart context flags
	contextCmd.Flags().StringVar(&contextSmart, "smart", "", "Natural language task description for intent-aware context assembly")
	contextCmd.Flags().IntVar(&contextDepth, "depth", 2, "Max hops from entry points for --smart mode")

	// Coverage flag
	contextCmd.Flags().BoolVar(&contextWithCoverage, "with-coverage", false, "Include test coverage data for each entity")
}

// contextEntry represents an entity in the context graph with metadata
type contextEntry struct {
	entity     *store.Entity
	beadInfo   *integration.BeadInfo // For task entries
	hop        int
	importance float64
	pageRank   float64
	inDegree   int
	isTask     bool
	isType     bool
	isLinked   bool
	tokens     int
	warnings   []string
}

func runContext(cmd *cobra.Command, args []string) error {
	// Handle --smart mode (intent-aware context assembly)
	if contextSmart != "" {
		return runSmartContext(cmd, contextSmart)
	}

	// Standard context mode requires a target
	if len(args) == 0 {
		return fmt.Errorf("target argument required (or use --smart for intent-aware context)")
	}
	target := args[0]

	// If --for-task is provided, use it as the target
	if contextForTask != "" {
		target = contextForTask
		// Verify beads is available for --for-task
		if !integration.BeadsAvailable() {
			return fmt.Errorf("--for-task requires beads integration (bd CLI and .beads/ directory)")
		}
	}

	// Validate budget mode
	if contextBudget != "importance" && contextBudget != "distance" {
		return fmt.Errorf("invalid budget-mode %q: must be importance or distance", contextBudget)
	}

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
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

	// Build graph for expansion
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Determine target type and get root entities
	entries := []*contextEntry{}
	var taskEntry *contextEntry

	if isBeadID(target) && integration.BeadsAvailable() {
		// Task context - get from beads integration
		beadInfo, err := integration.GetBead(target)
		if err != nil {
			return fmt.Errorf("task not found: %s", target)
		}

		// Create a pseudo-entry for the task
		taskEntry = &contextEntry{
			beadInfo: beadInfo,
			hop:      0,
			isTask:   true,
		}
		entries = append(entries, taskEntry)

		// Find linked code entities from task description
		linkedIDs := extractLinkedEntitiesFromDesc(beadInfo.Description)
		for _, id := range linkedIDs {
			entity, err := storeDB.GetEntity(id)
			if err == nil && entity != nil {
				entries = append(entries, &contextEntry{
					entity:   entity,
					hop:      0,
					isLinked: true,
				})
			}
		}
	} else if isFilePathTarget(target) {
		// File context
		entities, err := storeDB.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err != nil {
			return fmt.Errorf("failed to query entities: %w", err)
		}
		for _, e := range entities {
			entries = append(entries, &contextEntry{
				entity: e,
				hop:    0,
			})
		}
	} else {
		// Entity context
		entity, err := storeDB.GetEntity(target)
		if err != nil {
			return fmt.Errorf("entity not found: %s", target)
		}
		entries = append(entries, &contextEntry{
			entity: entity,
			hop:    0,
		})
	}

	// Expand graph using graph package
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.entity != nil {
			seen[e.entity.ID] = true
		}
	}

	for hop := 0; hop < contextHops; hop++ {
		newEntries := []*contextEntry{}

		for _, entry := range entries {
			if entry.hop != hop || entry.isTask || entry.entity == nil {
				continue
			}

			// Get successors (outgoing deps)
			if shouldIncludeExpansion("deps", contextInclude) {
				successors := g.Successors(entry.entity.ID)
				for _, targetID := range successors {
					if !seen[targetID] && !shouldExcludeEntity(targetID, contextExclude) {
						depEntity, err := storeDB.GetEntity(targetID)
						if err == nil && depEntity != nil {
							seen[targetID] = true

							// Check if it's a type reference
							deps, _ := storeDB.GetDependencies(store.DependencyFilter{
								FromID: entry.entity.ID,
								ToID:   targetID,
							})
							isType := false
							for _, d := range deps {
								if d.DepType == "uses_type" {
									isType = true
									break
								}
							}

							newEntries = append(newEntries, &contextEntry{
								entity: depEntity,
								hop:    hop + 1,
								isType: isType,
							})
						}
					}
				}
			}

			// Get predecessors (callers)
			if shouldIncludeExpansion("callers", contextInclude) {
				predecessors := g.Predecessors(entry.entity.ID)
				for _, sourceID := range predecessors {
					if !seen[sourceID] && !shouldExcludeEntity(sourceID, contextExclude) {
						srcEntity, err := storeDB.GetEntity(sourceID)
						if err == nil && srcEntity != nil {
							seen[sourceID] = true
							newEntries = append(newEntries, &contextEntry{
								entity: srcEntity,
								hop:    hop + 1,
							})
						}
					}
				}
			}
		}

		entries = append(entries, newEntries...)
	}

	// Get metrics for all entities
	keystones := []*contextEntry{}
	for _, entry := range entries {
		if entry.entity != nil && !entry.isTask {
			m, err := storeDB.GetMetrics(entry.entity.ID)
			if err == nil && m != nil {
				entry.pageRank = m.PageRank
				entry.inDegree = m.InDegree
				entry.importance = computeImportanceScore(m.PageRank, m.InDegree)

				if m.PageRank >= 0.30 {
					keystones = append(keystones, entry)
				}
			}
		}
		entry.tokens = estimateTokensStore(entry, density)
	}

	// Apply token budget
	var budgetWarnings []string
	entries, budgetWarnings = applyTokenBudget(entries, contextMaxTokens, contextBudget, density)

	// Calculate total tokens used
	tokensUsed := 0
	for _, e := range entries {
		tokensUsed += e.tokens
	}

	// Build ContextOutput for YAML/JSON output
	contextOut := &output.ContextOutput{
		Context: &output.ContextMetadata{
			Target:     target,
			Budget:     contextMaxTokens,
			TokensUsed: tokensUsed,
		},
		EntryPoints: make(map[string]*output.EntryPoint),
		Relevant:    make(map[string]*output.RelevantEntity),
		Excluded:    make(map[string]string),
	}

	// Add entry points (hop 0 relevant entries or task-linked entities)
	relevantEntries := filterEntries(entries, func(e *contextEntry) bool {
		return e.hop == 0 && !e.isTask && (e.isLinked || taskEntry != nil)
	})
	for _, entry := range relevantEntries {
		if entry.entity != nil {
			location := formatStoreLocation(entry.entity)
			contextOut.EntryPoints[entry.entity.Name] = &output.EntryPoint{
				Type:     mapStoreEntityTypeToString(entry.entity.EntityType),
				Location: location,
			}
		}
	}

	// Add relevant entities (all non-excluded entries)
	for _, entry := range entries {
		if entry.entity != nil && !entry.isTask {
			location := formatStoreLocation(entry.entity)
			relevance := "medium"
			if entry.importance >= 0.5 {
				relevance = "high"
			} else if entry.importance < 0.1 {
				relevance = "low"
			}

			reason := fmt.Sprintf("Hop %d from target", entry.hop)
			if entry.isLinked {
				reason = "Linked from task description"
			} else if entry.isType {
				reason = "Type reference"
			}

			relEntity := &output.RelevantEntity{
				Type:      mapStoreEntityTypeToString(entry.entity.EntityType),
				Location:  location,
				Relevance: relevance,
				Reason:    reason,
			}

			// Add coverage data if --with-coverage flag is set
			if contextWithCoverage {
				addCoverageToRelevantEntity(relEntity, entry.entity.ID, entry.pageRank, storeDB)
			}

			contextOut.Relevant[entry.entity.Name] = relEntity
		}
	}

	// Add excluded entities from budget warnings
	for _, warning := range budgetWarnings {
		// Parse exclusion reason from warning if available
		if strings.Contains(warning, "Dropped") {
			contextOut.Excluded["[budget-pruned]"] = warning
		}
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), contextOut, density)
}

// isBeadID checks if a string looks like a bead ID
func isBeadID(s string) bool {
	// Bead IDs typically have formats like:
	// - bd-abc123
	// - Project-abc123
	// - sa-fn-hash-Name
	if strings.HasPrefix(s, "bd-") {
		return true
	}
	if strings.HasPrefix(s, "sa-") {
		return true
	}
	// Check for project-style IDs (contains dash, not a path)
	if strings.Contains(s, "-") && !strings.Contains(s, "/") && !strings.Contains(s, ".") {
		return true
	}
	return false
}

// isFilePathTarget checks if a string looks like a file path (for context command)
func isFilePathTarget(s string) bool {
	// Check for common file path patterns
	if strings.Contains(s, "/") {
		return true
	}
	// Check for file extension
	ext := filepath.Ext(s)
	fileExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true,
	}
	return fileExts[ext]
}

// extractLinkedEntitiesFromDesc extracts entity IDs from task description
func extractLinkedEntitiesFromDesc(desc string) []string {
	var ids []string

	// Look for patterns like:
	// sa-fn-hash-Name
	// sa-type-hash-TypeName

	lines := strings.Split(desc, "\n")
	for _, line := range lines {
		// Look for direct entity ID references
		words := strings.Fields(line)
		for _, word := range words {
			word = strings.Trim(word, ".,;:()[]{}\"'")
			if strings.HasPrefix(word, "sa-") {
				ids = append(ids, word)
			}
		}
	}

	return ids
}

// shouldIncludeExpansion checks if an expansion type should be included
func shouldIncludeExpansion(what string, include []string) bool {
	// Default: include deps, exclude callers unless specified
	if len(include) == 0 {
		return what == "deps" || what == "types"
	}
	for _, inc := range include {
		if strings.EqualFold(inc, what) {
			return true
		}
	}
	return false
}

// shouldExcludeEntity checks if an entity should be excluded
func shouldExcludeEntity(id string, exclude []string) bool {
	idLower := strings.ToLower(id)
	for _, exc := range exclude {
		exc = strings.ToLower(exc)
		switch exc {
		case "tests":
			if strings.Contains(idLower, "test") || strings.Contains(idLower, "_test") {
				return true
			}
		case "mocks":
			if strings.Contains(idLower, "mock") {
				return true
			}
		default:
			if strings.Contains(idLower, exc) {
				return true
			}
		}
	}
	return false
}

// computeImportanceScore computes a combined importance score
func computeImportanceScore(pageRank float64, inDegree int) float64 {
	// Combine PageRank and in-degree for importance
	// PageRank is weighted more heavily as it captures transitive importance
	degreeScore := float64(inDegree) / 100.0 // Normalize
	if degreeScore > 1.0 {
		degreeScore = 1.0
	}
	return pageRank*0.7 + degreeScore*0.3
}

// estimateTokensStore estimates the token count for an entry at given density
func estimateTokensStore(entry *contextEntry, density output.Density) int {
	if entry == nil {
		return 0
	}

	// Base token estimates by density
	base := 10 // Minimal overhead

	// For task entries
	if entry.isTask && entry.beadInfo != nil {
		titleTokens := len(strings.Fields(entry.beadInfo.Title)) + 2
		descTokens := len(strings.Fields(entry.beadInfo.Description))
		return base + titleTokens + descTokens + 20
	}

	// For code entities
	if entry.entity == nil {
		return 0
	}

	// Add tokens based on content
	titleTokens := len(strings.Fields(entry.entity.Name)) + 2

	switch {
	case density == output.DensitySparse:
		// Just marker, location, name
		return base + titleTokens + 5
	case density == output.DensityMedium || density == output.DensitySmart:
		// Add signature info
		sigTokens := len(strings.Fields(entry.entity.Signature)) + 5
		return base + titleTokens + sigTokens + 10
	case density == output.DensityDense:
		// Full signature, metrics, edges
		sigTokens := len(strings.Fields(entry.entity.Signature))
		metricsTokens := 15 // importance, pr, deps, betw
		return base + titleTokens + sigTokens + metricsTokens + 20
	}

	return base + titleTokens
}

// applyTokenBudget trims entries to fit within budget
func applyTokenBudget(entries []*contextEntry, maxTokens int, mode string, density output.Density) ([]*contextEntry, []string) {
	var warnings []string

	// Calculate total tokens
	total := 0
	for _, e := range entries {
		total += e.tokens
	}

	if total <= maxTokens {
		return entries, warnings // Fits within budget
	}

	warnings = append(warnings, fmt.Sprintf("Budget exceeded: %d tokens > %d max, pruning in %s mode",
		total, maxTokens, mode))

	// Sort based on mode
	sortedEntries := make([]*contextEntry, len(entries))
	copy(sortedEntries, entries)

	if mode == "importance" {
		// Sort by importance (high first), then by hop (close first)
		sort.Slice(sortedEntries, func(i, j int) bool {
			// Tasks always first
			if sortedEntries[i].isTask {
				return true
			}
			if sortedEntries[j].isTask {
				return false
			}
			// Then by importance
			if sortedEntries[i].importance != sortedEntries[j].importance {
				return sortedEntries[i].importance > sortedEntries[j].importance
			}
			// Then by hop distance
			return sortedEntries[i].hop < sortedEntries[j].hop
		})
	} else {
		// Sort by distance (close first), then importance
		sort.Slice(sortedEntries, func(i, j int) bool {
			// Tasks always first
			if sortedEntries[i].isTask {
				return true
			}
			if sortedEntries[j].isTask {
				return false
			}
			// Then by hop distance
			if sortedEntries[i].hop != sortedEntries[j].hop {
				return sortedEntries[i].hop < sortedEntries[j].hop
			}
			// Then by importance
			return sortedEntries[i].importance > sortedEntries[j].importance
		})
	}

	// Keep entries until budget exceeded
	result := []*contextEntry{}
	used := 0
	dropped := 0

	for _, e := range sortedEntries {
		if used+e.tokens <= maxTokens || e.isTask {
			result = append(result, e)
			used += e.tokens
		} else {
			dropped++
		}
	}

	if dropped > 0 {
		warnings = append(warnings, fmt.Sprintf("Dropped %d entries to fit budget", dropped))
	}

	return result, warnings
}

// filterEntries filters entries by a predicate
func filterEntries(entries []*contextEntry, pred func(*contextEntry) bool) []*contextEntry {
	var result []*contextEntry
	for _, e := range entries {
		if pred(e) {
			result = append(result, e)
		}
	}
	return result
}

// runSmartContext handles the --smart flag for intent-aware context assembly.
func runSmartContext(cmd *cobra.Command, taskDescription string) error {
	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
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

	// Build graph for expansion
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Create smart context assembler
	opts := context.DefaultSmartContextOptions()
	opts.TaskDescription = taskDescription
	opts.Budget = contextMaxTokens
	opts.Depth = contextDepth

	sc := context.NewSmartContext(storeDB, g, opts)

	// Assemble context
	result, err := sc.Assemble()
	if err != nil {
		return fmt.Errorf("smart context assembly failed: %w", err)
	}

	// Convert to SmartContextOutput for YAML/JSON output
	smartOut := buildSmartContextOutput(result, density, storeDB)

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), smartOut, density)
}

// SmartContextOutput represents the output structure for smart context.
type SmartContextOutput struct {
	Intent       *SmartContextIntent            `yaml:"intent" json:"intent"`
	EntryPoints  map[string]*output.EntryPoint  `yaml:"entry_points" json:"entry_points"`
	Relevant     map[string]*output.RelevantEntity `yaml:"relevant_entities" json:"relevant_entities"`
	Excluded     map[string]string              `yaml:"excluded,omitempty" json:"excluded,omitempty"`
	TokensUsed   int                            `yaml:"tokens_used" json:"tokens_used"`
	TokensBudget int                            `yaml:"tokens_budget" json:"tokens_budget"`
	Warnings     []string                       `yaml:"warnings,omitempty" json:"warnings,omitempty"`
}

// SmartContextIntent represents the extracted intent for output.
type SmartContextIntent struct {
	Keywords       []string `yaml:"keywords" json:"keywords"`
	Pattern        string   `yaml:"pattern" json:"pattern"`
	EntityMentions []string `yaml:"entity_mentions,omitempty" json:"entity_mentions,omitempty"`
}

// buildSmartContextOutput converts SmartContextResult to output format.
func buildSmartContextOutput(result *context.SmartContextResult, density output.Density, storeDB *store.Store) *SmartContextOutput {
	out := &SmartContextOutput{
		EntryPoints:  make(map[string]*output.EntryPoint),
		Relevant:     make(map[string]*output.RelevantEntity),
		Excluded:     make(map[string]string),
		TokensUsed:   result.TokensUsed,
		TokensBudget: result.TokensBudget,
		Warnings:     result.Warnings,
	}

	// Convert intent
	if result.Intent != nil {
		out.Intent = &SmartContextIntent{
			Keywords:       result.Intent.Keywords,
			Pattern:        result.Intent.Pattern,
			EntityMentions: result.Intent.EntityMentions,
		}
	}

	// Convert entry points
	for _, ep := range result.EntryPoints {
		note := ep.Reason
		if ep.IsKeystone {
			note = "[keystone] " + note
		}
		out.EntryPoints[ep.Name] = &output.EntryPoint{
			Type:     ep.Type,
			Location: ep.Location,
			Note:     note,
		}
	}

	// Convert relevant entities
	for _, re := range result.Relevant {
		relevance := "medium"
		if re.Relevance >= 1.0 {
			relevance = "high"
		} else if re.Relevance < 0.3 {
			relevance = "low"
		}

		reason := re.Reason
		if re.IsKeystone {
			reason = "[keystone] " + reason
		}

		relEntity := &output.RelevantEntity{
			Type:      re.Type,
			Location:  re.Location,
			Relevance: relevance,
			Reason:    reason,
		}

		// Add coverage data if --with-coverage flag is set
		if contextWithCoverage {
			addCoverageToRelevantEntity(relEntity, re.ID, re.PageRank, storeDB)
		}

		out.Relevant[re.Name] = relEntity
	}

	// Convert excluded entities
	for _, ex := range result.Excluded {
		out.Excluded[ex.Name] = ex.Reason
	}

	return out
}

// addCoverageToRelevantEntity adds coverage data to a RelevantEntity.
// It retrieves coverage information from the database and sets the Coverage
// and CoverageWarning fields on the entity.
func addCoverageToRelevantEntity(relEntity *output.RelevantEntity, entityID string, pageRank float64, storeDB *store.Store) {
	cov, err := coverage.GetEntityCoverage(storeDB, entityID)
	if err != nil {
		// No coverage data available for this entity
		return
	}

	// Set coverage percentage
	relEntity.Coverage = fmt.Sprintf("%.1f%%", cov.CoveragePercent)

	// Check if this is a keystone with low coverage (< 50%)
	// Keystones are entities with PageRank >= 0.15 or high importance
	isKeystone := pageRank >= 0.15
	if isKeystone && cov.CoveragePercent < 50.0 {
		relEntity.CoverageWarning = "Keystone below 50% - add tests before modifying"
	}
}

