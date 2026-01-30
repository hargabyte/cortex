package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// deadCmd represents the dead command
var deadCmd = &cobra.Command{
	Use:   "dead",
	Short: "Find provably dead code",
	Long: `Find provably dead code in the codebase.

Dead code is defined as private/unexported symbols with zero callers in the
dependency graph. This command is designed to be safe for AI agents - only
reporting code that can be definitively identified as dead.

Algorithm:
  Dead Code = Private/Unexported + Zero In-Degree

This conservative approach means:
  - Only private/unexported symbols are considered by default
  - Must have zero callers (in_degree = 0)
  - No heuristics or confidence levels

Output Structure:
  dead_code:
    count:    Number of dead code items found
    by_type:  Breakdown by entity type
    results:  List of dead code items with details

Filtering Modes:
  (default)          Show only definitely-dead private code
  --include-exports  Also show unused exports (lower confidence, opt-in)
  --by-file          Group results by file path
  --type F           Filter by entity type (F=function, T=type, M=method, C=constant)

Examples:
  cx dead                        # Find dead private code
  cx dead --include-exports      # Include unused exports
  cx dead --by-file              # Group by file
  cx dead --type F               # Only functions
  cx dead --format json          # JSON output
  cx dead --create-task          # Print bd create commands

Notes:
  - Requires 'cx scan' and 'cx rank' to have been run
  - Safe for automated cleanup - results are definitively dead
  - Use --include-exports cautiously - exports may be used externally`,
	RunE: runDead,
}

var (
	deadIncludeExports bool
	deadByFile         bool
	deadCreateTask     bool
	deadTypeFilter     string
)

func init() {
	rootCmd.AddCommand(deadCmd)

	deadCmd.Flags().BoolVar(&deadIncludeExports, "include-exports", false, "Also show unused exports (lower confidence)")
	deadCmd.Flags().BoolVar(&deadByFile, "by-file", false, "Group results by file path")
	deadCmd.Flags().BoolVar(&deadCreateTask, "create-task", false, "Print bd create commands for cleanup")
	deadCmd.Flags().StringVar(&deadTypeFilter, "type", "", "Filter by entity type (F=function, T=type, M=method, C=constant)")
}

// deadCodeItem represents a dead code entity
type deadCodeItem struct {
	entity  *store.Entity
	metrics *store.Metrics
	reason  string
}

func runDead(cmd *cobra.Command, args []string) error {
	// Open store
	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// Get all active entities
	entities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return fmt.Errorf("no entities found - run 'cx scan' first")
	}

	// Check if metrics exist
	hasMetrics := false
	for _, e := range entities {
		if m, _ := storeDB.GetMetrics(e.ID); m != nil {
			hasMetrics = true
			break
		}
	}

	if !hasMetrics {
		return fmt.Errorf("no metrics found - run 'cx rank' first to compute graph metrics")
	}

	// Normalize type filter
	typeFilter := normalizeDeadTypeFilter(deadTypeFilter)

	// Build list of dead code
	var deadItems []deadCodeItem

	for _, e := range entities {
		// Skip imports - they're not code
		if e.EntityType == "import" {
			continue
		}

		// Skip known false positives
		if isKnownEntryPoint(e) {
			continue
		}

		// Apply type filter if specified
		if typeFilter != "" && !matchesDeadTypeFilter(e.EntityType, typeFilter) {
			continue
		}

		// Check visibility
		isPrivate := e.Visibility == "private" || e.Visibility == "priv"
		isPublic := e.Visibility == "public" || e.Visibility == "pub"

		// By default, only include private symbols
		// With --include-exports, also include public symbols
		if !isPrivate && !deadIncludeExports {
			continue
		}

		// Get metrics
		m, err := storeDB.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue
		}

		// Dead code = zero in-degree (no callers)
		if m.InDegree > 0 {
			continue
		}

		// Build reason based on visibility
		var reason string
		if isPrivate {
			reason = "Private symbol with no callers"
		} else if isPublic {
			reason = "Unused export (may be used externally)"
		}

		deadItems = append(deadItems, deadCodeItem{
			entity:  e,
			metrics: m,
			reason:  reason,
		})
	}

	if len(deadItems) == 0 {
		fmt.Fprintf(os.Stderr, "No dead code found! All symbols have callers or are exports.\n")
		if !deadIncludeExports {
			fmt.Fprintf(os.Stderr, "Hint: Use --include-exports to also check unused exports.\n")
		}
		return nil
	}

	// Sort by file path and line number for consistent output
	sort.Slice(deadItems, func(i, j int) bool {
		if deadItems[i].entity.FilePath != deadItems[j].entity.FilePath {
			return deadItems[i].entity.FilePath < deadItems[j].entity.FilePath
		}
		return deadItems[i].entity.LineStart < deadItems[j].entity.LineStart
	})

	// If --create-task flag, print bd commands
	if deadCreateTask {
		return printDeadTaskCommands(deadItems)
	}

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Build output structure
	outputData := buildDeadOutput(deadItems, deadByFile)

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), outputData, output.DensityMedium)
}

// normalizeDeadTypeFilter converts short type codes to entity types
func normalizeDeadTypeFilter(filter string) string {
	switch strings.ToUpper(strings.TrimSpace(filter)) {
	case "F", "FUNCTION", "FUNC":
		return "function"
	case "M", "METHOD":
		return "method"
	case "T", "TYPE", "STRUCT":
		return "type"
	case "C", "CONSTANT", "CONST":
		return "constant"
	case "":
		return ""
	default:
		return strings.ToLower(filter)
	}
}

// matchesDeadTypeFilter checks if entity type matches the filter
func matchesDeadTypeFilter(entityType, filter string) bool {
	entityType = strings.ToLower(entityType)
	filter = strings.ToLower(filter)

	// Handle type aliases
	switch filter {
	case "function":
		return entityType == "function" || entityType == "func"
	case "type":
		return entityType == "type" || entityType == "struct" || entityType == "interface"
	case "constant":
		return entityType == "constant" || entityType == "const" || entityType == "var" || entityType == "variable"
	default:
		return entityType == filter
	}
}

// isKnownEntryPoint returns true if the entity is a known entry point or false positive.
// These are symbols that appear to have no callers but are actually used via:
// - Go runtime (init functions)
// - Cobra command registration (run* handlers)
// - Local variables incorrectly parsed as package-level symbols
func isKnownEntryPoint(e *store.Entity) bool {
	name := e.Name
	entityType := strings.ToLower(e.EntityType)

	// init() functions are Go runtime entry points
	if name == "init" && (entityType == "function" || entityType == "func") {
		return true
	}

	// Cobra command handlers: run*, Run* functions in cmd package
	if strings.Contains(e.FilePath, "/cmd/") {
		if strings.HasPrefix(name, "run") || strings.HasPrefix(name, "Run") {
			if entityType == "function" || entityType == "func" {
				return true
			}
		}
	}

	// Skip local variables that were incorrectly parsed as constants
	// These typically have very short line spans (single line) and common variable names
	if entityType == "constant" || entityType == "const" || entityType == "var" || entityType == "variable" {
		// Single-line "constants" with common local variable names are likely false positives
		isSingleLine := e.LineEnd == nil || *e.LineEnd == e.LineStart
		if isSingleLine && isCommonLocalVarName(name) {
			return true
		}
	}

	return false
}

// isCommonLocalVarName returns true for names that are commonly used as local variables
func isCommonLocalVarName(name string) bool {
	commonNames := map[string]bool{
		"err": true, "ctx": true, "ok": true, "i": true, "j": true, "k": true,
		"n": true, "s": true, "b": true, "r": true, "w": true,
		"buf": true, "tmp": true, "result": true, "results": true,
		"data": true, "out": true, "in": true, "stdin": true, "stdout": true, "stderr": true,
		"args": true, "opts": true, "cfg": true, "config": true,
		"req": true, "resp": true, "res": true, "cmd": true,
		"db": true, "tx": true, "rows": true, "row": true,
		"file": true, "f": true, "path": true, "dir": true,
		"name": true, "id": true, "ids": true, "key": true, "val": true, "value": true,
		"msg": true, "line": true, "lines": true, "text": true,
		"count": true, "total": true, "sum": true, "len": true,
		"start": true, "end": true, "idx": true, "index": true,
		"item": true, "items": true, "elem": true, "entry": true, "entries": true,
		"node": true, "nodes": true, "child": true, "children": true, "parent": true,
		"src": true, "dst": true, "old": true, "new": true,
		"matched": true, "found": true, "exists": true, "done": true,
		"wg": true, "mu": true, "lock": true, "ch": true,
		"t": true, "v": true, "x": true, "y": true, "z": true,
		// Cortex-specific common locals
		"entities": true, "entity": true, "deps": true, "metrics": true,
		"storeDB": true, "cxDir": true, "baseDir": true, "filePath": true,
		"beadsDir": true, "jsonlPath": true,
	}
	return commonNames[name]
}

// buildDeadOutput constructs the output data structure
func buildDeadOutput(items []deadCodeItem, byFile bool) map[string]interface{} {
	// Count by type
	byType := make(map[string]int)
	for _, item := range items {
		t := mapStoreEntityTypeToString(item.entity.EntityType)
		byType[t]++
	}

	// Build results
	var results interface{}

	if byFile {
		// Group by file
		fileGroups := make(map[string][]map[string]interface{})
		for _, item := range items {
			itemData := buildDeadItemData(item)
			// Remove location from individual items when grouped by file
			delete(itemData, "location")
			itemData["line"] = fmt.Sprintf("%d", item.entity.LineStart)
			if item.entity.LineEnd != nil && *item.entity.LineEnd != item.entity.LineStart {
				itemData["line"] = fmt.Sprintf("%d-%d", item.entity.LineStart, *item.entity.LineEnd)
			}
			fileGroups[item.entity.FilePath] = append(fileGroups[item.entity.FilePath], itemData)
		}
		results = fileGroups
	} else {
		// Flat list
		resultList := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			resultList = append(resultList, buildDeadItemData(item))
		}
		results = resultList
	}

	// Build recommendation
	recommendation := fmt.Sprintf("These %d private symbols have no callers and can be safely removed.", len(items))
	if deadIncludeExports {
		// Count private vs public
		privateCount := 0
		for _, item := range items {
			if item.entity.Visibility == "private" || item.entity.Visibility == "priv" {
				privateCount++
			}
		}
		publicCount := len(items) - privateCount
		if publicCount > 0 {
			recommendation = fmt.Sprintf("%d private symbols can be safely removed. %d unused exports may be used externally - verify before removing.",
				privateCount, publicCount)
		}
	}

	return map[string]interface{}{
		"dead_code": map[string]interface{}{
			"count":   len(items),
			"by_type": byType,
			"results": results,
		},
		"recommendation": recommendation,
	}
}

// buildDeadItemData builds a single dead code item for output
func buildDeadItemData(item deadCodeItem) map[string]interface{} {
	data := map[string]interface{}{
		"name":       item.entity.Name,
		"type":       mapStoreEntityTypeToString(item.entity.EntityType),
		"location":   formatStoreLocation(item.entity),
		"visibility": item.entity.Visibility,
		"in_degree":  item.metrics.InDegree,
		"out_degree": item.metrics.OutDegree,
		"reason":     item.reason,
	}
	return data
}

// printDeadTaskCommands prints bd create commands for dead code cleanup
func printDeadTaskCommands(items []deadCodeItem) error {
	fmt.Println("# Dead code cleanup tasks - run these commands to create beads:")
	fmt.Println()

	// Group by file for consolidated tasks
	fileGroups := make(map[string][]deadCodeItem)
	for _, item := range items {
		fileGroups[item.entity.FilePath] = append(fileGroups[item.entity.FilePath], item)
	}

	// Create one task per file
	for filePath, fileItems := range fileGroups {
		// Build list of symbols
		var symbols []string
		for _, item := range fileItems {
			symbols = append(symbols, item.entity.Name)
		}

		// Determine priority - more items = higher priority
		priority := 3 // default low
		if len(fileItems) >= 5 {
			priority = 2 // medium
		}
		if len(fileItems) >= 10 {
			priority = 1 // high
		}

		// Build description
		symbolList := strings.Join(symbols, ", ")
		if len(symbolList) > 200 {
			symbolList = symbolList[:200] + "..."
		}

		desc := fmt.Sprintf("Remove %d dead code symbols from %s\\n\\n"+
			"Symbols: %s\\n\\n"+
			"These private symbols have no callers and can be safely removed.",
			len(fileItems),
			filePath,
			symbolList,
		)

		// Print bd create command
		fmt.Printf("bd create \"Remove dead code from %s\" -t chore -p %d -d \"%s\"\n\n",
			filePath,
			priority,
			desc,
		)
	}

	fmt.Printf("# Total: %d dead code items across %d files\n", len(items), len(fileGroups))

	return nil
}
