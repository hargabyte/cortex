package cmd

import (
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search for code by concept, not just symbol name",
	Long: `Search for code entities using full-text search (FTS5).

Find code by concept, not just symbol name. The search looks through:
- Entity names (functions, types, constants, etc.)
- Function/method body text
- Documentation comments
- File paths

Results are ranked by relevance using a combination of:
- FTS5 BM25 text matching score
- PageRank importance score (if metrics are computed)
- Exact name match boosting

Search Modes:
  keyword (default): Multi-word prefix search. Each word is matched with prefix matching.

Examples:
  cx search "auth logic"           # Find authentication-related code
  cx search "rate limit"           # Find rate limiting code
  cx search LoginUser              # Exact name match (boosted)
  cx search "TODO fix"             # Find TODOs in comments/code
  cx search --mode=keyword "JWT"   # Pure keyword search
  cx search --lang go "config"     # Filter by language
  cx search --type function "parse"  # Filter by entity type
  cx search --top 20 "database"    # Get more results

Output Formats:
  yaml:  Human-readable YAML (default)
  json:  Machine-parseable JSON

Density Levels:
  sparse:  Type and location only
  medium:  Add signature and basic dependencies (default)
  dense:   Full details with metrics and scores`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

var (
	searchMode      string
	searchTop       int
	searchThreshold float64
	searchLang      string
	searchType      string
)

func init() {
	rootCmd.AddCommand(searchCmd)

	// Search-specific flags
	searchCmd.Flags().StringVar(&searchMode, "mode", "keyword", "Search mode: keyword (default)")
	searchCmd.Flags().IntVar(&searchTop, "top", 10, "Number of results to return")
	searchCmd.Flags().Float64Var(&searchThreshold, "threshold", 0.0, "Minimum relevance score (0.0-1.0)")
	searchCmd.Flags().StringVar(&searchLang, "lang", "", "Filter by language (go, typescript, python, rust, java)")
	searchCmd.Flags().StringVar(&searchType, "type", "", "Filter by entity type (function, type, constant, etc.)")
}

func runSearch(cmd *cobra.Command, args []string) error {
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

	// Build search options
	opts := store.DefaultSearchOptions()
	opts.Query = query
	opts.Limit = searchTop
	opts.Threshold = searchThreshold
	if searchLang != "" {
		opts.Language = normalizeLanguage(searchLang)
	}
	if searchType != "" {
		opts.EntityType = mapSearchTypeToStore(searchType)
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

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
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

// SearchOutput represents the search results for output formatting.
type SearchOutput struct {
	Query   string                         `json:"query" yaml:"query"`
	Results map[string]*output.EntityOutput `json:"results" yaml:"results"`
	Count   int                            `json:"count" yaml:"count"`
	Scores  map[string]*SearchScore        `json:"scores,omitempty" yaml:"scores,omitempty"`
}

// SearchScore represents the relevance scores for an entity.
type SearchScore struct {
	FTSScore      float64 `json:"fts_score" yaml:"fts_score"`
	PageRank      float64 `json:"pagerank" yaml:"pagerank"`
	CombinedScore float64 `json:"combined_score" yaml:"combined_score"`
}

// MarshalYAML implements custom YAML marshaling for SearchOutput.
func (s *SearchOutput) MarshalYAML() (interface{}, error) {
	// Return a map that will be marshaled naturally
	m := make(map[string]interface{})
	m["query"] = s.Query
	m["results"] = s.Results
	m["count"] = s.Count
	if len(s.Scores) > 0 {
		m["scores"] = s.Scores
	}
	return m, nil
}

// buildSearchOutput converts search results to output format.
func buildSearchOutput(results []*store.SearchResult, density output.Density, storeDB *store.Store, query string) *SearchOutput {
	searchOutput := &SearchOutput{
		Query:   query,
		Results: make(map[string]*output.EntityOutput),
		Scores:  make(map[string]*SearchScore),
	}

	for _, r := range results {
		e := r.Entity
		name := e.Name

		// Convert to EntityOutput
		entityOut := &output.EntityOutput{
			Type:     mapStoreEntityTypeToString(e.EntityType),
			Location: formatStoreLocation(e),
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
			entityOut.Metrics = &output.Metrics{
				PageRank:  r.PageRank,
				InDegree:  0, // Would need to look up
				OutDegree: 0, // Would need to look up
			}

			// Add search-specific scores
			searchOutput.Scores[name] = &SearchScore{
				FTSScore:      r.FTSScore,
				PageRank:      r.PageRank,
				CombinedScore: r.CombinedScore,
			}
		}

		searchOutput.Results[name] = entityOut
		searchOutput.Count++
	}

	return searchOutput
}

// mapSearchTypeToStore converts search type flag to store entity type.
func mapSearchTypeToStore(t string) string {
	switch t {
	case "function", "func", "fn", "F":
		return "function"
	case "method", "M":
		return "method"
	case "type", "struct", "class", "T":
		return "type"
	case "interface":
		return "interface"
	case "constant", "const", "C":
		return "constant"
	case "variable", "var", "V":
		return "variable"
	case "import", "I":
		return "import"
	default:
		return t
	}
}
