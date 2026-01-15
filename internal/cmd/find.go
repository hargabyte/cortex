package cmd

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find <name>",
	Short: "Find symbols matching a query pattern",
	Long: `Quick entity lookup by name. Supports:
- Simple names: LoginUser (prefix match across all packages)
- Qualified names: auth.LoginUser (package.symbol)
- Path-qualified: auth/login.LoginUser (path/file.symbol)
- Direct IDs: sa-fn-a7f9b2-LoginUser

Output is in YAML format by default with medium density.

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
  cx find LoginUser                        # Prefix match, medium YAML
  cx find auth.LoginUser                   # Qualified name match
  cx find auth/login.LoginUser             # Path-qualified match
  cx find sa-fn-a7f9b2-LoginUser           # Direct entity ID lookup
  cx find --type=F Login                   # Find functions matching "Login"
  cx find --exact LoginUser                # Exact match only
  cx find --file=login.go User             # Filter by file path
  cx find --lang=go User                   # Filter by language
  cx find LoginUser --density=sparse       # Minimal output for token budget
  cx find LoginUser --format=json          # JSON output for parsing
  cx find LoginUser --density=dense        # Full details with metrics`,
	Args: cobra.ExactArgs(1),
	RunE: runFind,
}

var (
	findType      string
	findFile      string
	findLang      string
	findExact     bool
	findQualified bool
	findLimit     int
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
}

func runFind(cmd *cobra.Command, args []string) error {
	query := args[0]

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

	// Build filter from flags (query all matching entities, then limit after name filtering)
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

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
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
