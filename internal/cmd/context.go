package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/context"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/diff"
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

Modes:
  cx context                      Session recovery (workflow context)
  cx context --smart "<task>"     Intent-aware context assembly
  cx context --diff               Context for uncommitted changes
  cx context --staged             Context for staged changes only
  cx context <target>             Entity/file/bead context

Target can be:
  - A bead ID (task context): expands linked code and dependencies
  - A file path: all entities defined in that file
  - An entity ID: single entity and its dependencies

Session Recovery (no args):
  When called without arguments or --smart, outputs essential workflow context
  for AI agents. Designed for context recovery after compaction/clear/new session.

  cx context              # Concise context (~500 tokens)
  cx context --full       # Extended with keystones and map (~2000 tokens)

Smart Context (--smart):
  Parse natural language task descriptions to find relevant code context.
  Uses intent extraction, keyword search, and flow tracing to assemble
  focused context within the token budget.

  cx context --smart "add rate limiting to API endpoints" --budget 8000
  cx context --smart "fix auth bug in login" --budget 6000
  cx context --smart "optimize database queries" --budget 10000

Diff-Based Context (--diff, --staged, --commit-range):
  Get context focused on code changes. Analyzes git diff to identify:
  - Modified entities (signature vs body changes)
  - Added and removed entities
  - Callers affected by the changes

  cx context --diff                  # All uncommitted changes
  cx context --staged                # Only staged changes
  cx context --commit-range HEAD~3   # Changes in last 3 commits
  cx context --commit-range main..   # Changes since branching from main

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
  cx context                                       # Session recovery
  cx context --full                                # Extended session recovery
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
  cx context <entity> --with-coverage              # Entity context with coverage
  cx context --diff                                # Context for uncommitted changes
  cx context --staged                              # Context for staged changes only
  cx context --commit-range HEAD~5                 # Context for recent commits`,
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
	contextFull         bool   // For session recovery mode (--full)
	contextDiff         bool   // For diff-based context (uncommitted changes)
	contextStaged       bool   // For staged-only diff context
	contextCommitRange  string // For commit range diff context (e.g., HEAD~3)
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

	// Session recovery flags
	contextCmd.Flags().BoolVar(&contextFull, "full", false, "Extended session recovery with keystones and map")

	// Diff-based context flags
	contextCmd.Flags().BoolVar(&contextDiff, "diff", false, "Context for uncommitted changes (staged + unstaged)")
	contextCmd.Flags().BoolVar(&contextStaged, "staged", false, "Context for staged changes only")
	contextCmd.Flags().StringVar(&contextCommitRange, "commit-range", "", "Context for commit range (e.g., HEAD~3, main..feature)")
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

	// Handle diff-based context modes
	if contextDiff || contextStaged || contextCommitRange != "" {
		return runDiffContext(cmd)
	}

	// Handle no-arg session recovery mode (was: cx prime)
	if len(args) == 0 {
		return runSessionRecovery(cmd)
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
	Intent       *SmartContextIntent               `yaml:"intent" json:"intent"`
	EntryPoints  map[string]*output.EntryPoint     `yaml:"entry_points" json:"entry_points"`
	Relevant     map[string]*output.RelevantEntity `yaml:"relevant_entities" json:"relevant_entities"`
	Excluded     map[string]string                 `yaml:"excluded,omitempty" json:"excluded,omitempty"`
	TokensUsed   int                               `yaml:"tokens_used" json:"tokens_used"`
	TokensBudget int                               `yaml:"tokens_budget" json:"tokens_budget"`
	Warnings     []string                          `yaml:"warnings,omitempty" json:"warnings,omitempty"`
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

// runSessionRecovery outputs essential Cortex workflow context for AI agents.
// This is the no-arg behavior of `cx context` (previously `cx prime`).
func runSessionRecovery(cmd *cobra.Command) error {
	// Check for custom PRIME.md override
	cwd, _ := os.Getwd()
	customPath := filepath.Join(cwd, ".cx", "PRIME.md")
	if content, err := os.ReadFile(customPath); err == nil {
		fmt.Fprint(cmd.OutOrStdout(), string(content))
		return nil
	}

	// Get database info if available
	dbInfo := getSessionRecoveryDBInfo()

	// Output the session recovery context
	fmt.Fprint(cmd.OutOrStdout(), generateSessionRecoveryContent(dbInfo, contextFull))
	return nil
}

// sessionRecoveryDBStats holds database statistics for session recovery output
type sessionRecoveryDBStats struct {
	exists       bool
	path         string
	entities     int
	active       int
	archived     int
	dependencies int
	files        int
}

func getSessionRecoveryDBInfo() sessionRecoveryDBStats {
	cwd, err := os.Getwd()
	if err != nil {
		return sessionRecoveryDBStats{}
	}

	cxDir := filepath.Join(cwd, ".cx")

	// Check if .cx directory exists (don't auto-create)
	if _, err := os.Stat(cxDir); os.IsNotExist(err) {
		return sessionRecoveryDBStats{exists: false}
	}

	dbPath := filepath.Join(cxDir, "cortex.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return sessionRecoveryDBStats{exists: false}
	}

	s, err := store.Open(cxDir)
	if err != nil {
		return sessionRecoveryDBStats{exists: true, path: dbPath}
	}
	defer s.Close()

	stats := sessionRecoveryDBStats{
		exists: true,
		path:   dbPath,
	}

	// Get entity counts using store methods
	stats.active, _ = s.CountEntities(store.EntityFilter{Status: "active"})
	stats.archived, _ = s.CountEntities(store.EntityFilter{Status: "archived"})
	stats.entities = stats.active + stats.archived
	stats.dependencies, _ = s.CountDependencies()
	stats.files, _ = s.CountFileIndex()

	return stats
}

func generateSessionRecoveryContent(stats sessionRecoveryDBStats, full bool) string {
	content := `# Cortex (cx) Workflow Context

> **Context Recovery**: Run ` + "`cx context`" + ` after compaction, clear, or new session

## Status
`

	if stats.exists {
		content += fmt.Sprintf(`- **Database**: Initialized
- **Entities**: %d active, %d archived
- **Dependencies**: %d tracked
- **Files indexed**: %d
`, stats.active, stats.archived, stats.dependencies, stats.files)

		// Add keystones if --full and we have entities
		if full && stats.active > 0 {
			content += getSessionRecoveryKeystonesSection(stats.path)
		}
	} else {
		content += `- **Database**: Not initialized
- Run ` + "`cx quickstart`" + ` to enable code graph (or ` + "`cx init && cx scan`" + `)
`
	}

	content += `
## Essential Commands

` + "```bash" + `
# Start of session
cx                                          # This context (or cx context)

# Before ANY coding task
cx context --smart "<task>" --budget 8000   # Focused context

# Before modifying code
cx safe <file>                              # Full safety assessment
cx safe <file> --quick                      # Just blast radius

# Project overview
cx map                                      # Skeleton (~10k tokens)
cx find --keystones                         # Critical entities
` + "```" + `

## Discovery & Analysis

| Command | Purpose |
|---------|---------|
| ` + "`cx find <name>`" + ` | Name search (--type=F/T/M, --exact) |
| ` + "`cx find \"query\"`" + ` | Concept search (multi-word = FTS) |
| ` + "`cx find --keystones`" + ` | Find critical entities |
| ` + "`cx show <name>`" + ` | Entity details (--density dense) |
| ` + "`cx show <name> --related`" + ` | Neighborhood (calls, callers) |
| ` + "`cx show <name> --graph`" + ` | Dependencies (--hops N) |

## Safety & Testing

| Command | Purpose |
|---------|---------|
| ` + "`cx safe <file>`" + ` | Full safety check (impact + coverage + drift) |
| ` + "`cx safe <file> --quick`" + ` | Just blast radius |
| ` + "`cx safe --coverage`" + ` | Coverage gaps |
| ` + "`cx safe --drift`" + ` | Check for staleness |
| ` + "`cx test --diff --run`" + ` | Smart test selection |
| ` + "`cx coverage import`" + ` | Import coverage.out |

## Quick Patterns

` + "```bash" + `
# Understand codebase
cx map && cx find --keystones --top 10

# Before refactoring
cx safe <file>

# Smart testing
cx test --diff --run

# Find critical untested code
cx safe --coverage --keystones-only
` + "```" + `

## Notes
- Supports: Go, TypeScript, JavaScript, Java, Rust, Python
- Run ` + "`cx scan`" + ` after major code changes
- Run ` + "`cx help-agents`" + ` for full command reference
`

	return content
}

// getSessionRecoveryKeystonesSection returns a markdown section with top keystones
func getSessionRecoveryKeystonesSection(dbPath string) string {
	cxDir := filepath.Dir(dbPath)
	s, err := store.Open(cxDir)
	if err != nil {
		return ""
	}
	defer s.Close()

	// Get top 5 by PageRank
	topMetrics, err := s.GetTopByPageRank(5)
	if err != nil || len(topMetrics) == 0 {
		return ""
	}

	content := "\n**Top Keystones:**\n"
	for _, m := range topMetrics {
		// Fetch entity details
		e, err := s.GetEntity(m.EntityID)
		if err != nil || e == nil {
			continue
		}

		// Shorten the file path - keep last 2 components
		shortPath := e.FilePath
		parts := sessionRecoverySplitPath(shortPath)
		if len(parts) > 2 {
			shortPath = parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
		content += fmt.Sprintf("- `%s` (%s) @ %s:%d\n", e.Name, e.EntityType, shortPath, e.LineStart)
	}

	return content
}

// sessionRecoverySplitPath splits a path into components (works cross-platform)
func sessionRecoverySplitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == path {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

// runDiffContext handles diff-based context assembly.
// This provides context focused on uncommitted or staged changes.
func runDiffContext(cmd *cobra.Command) error {
	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Find project root and open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}
	projectRoot := filepath.Dir(cxDir)

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Build graph for caller tracing
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Load config
	cfg, err := config.Load(projectRoot)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Build diff context options
	opts := diff.DefaultDiffContextOptions()
	opts.Budget = contextMaxTokens
	opts.Depth = contextDepth
	opts.Staged = contextStaged
	opts.CommitRange = contextCommitRange

	// Create diff context assembler
	dc := diff.NewDiffContext(storeDB, g, projectRoot, cfg, opts)

	// Assemble context
	result, err := dc.Assemble()
	if err != nil {
		return fmt.Errorf("diff context assembly failed: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), result, density)
}
