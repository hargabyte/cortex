package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// mapCmd represents the map command
var mapCmd = &cobra.Command{
	Use:   "map [path]",
	Short: "Show project structure with function signatures (skeleton view)",
	Long: `Show project structure with function signatures only (no bodies),
enabling complete project overview in ~10k tokens.

The map command displays a skeleton view of the codebase, showing:
- Function and method signatures with { ... } placeholder for bodies
- Type definitions with their fields
- Constants and variables
- Doc comments preserved

This is useful for:
- Getting a quick overview of a codebase
- Providing context to AI agents without overwhelming token budgets
- Understanding the public API of a package

Filters:
  --filter F     Functions only
  --filter T     Types only
  --filter M     Methods only
  --filter C     Constants only
  --lang go      Filter by language

Output Formats:
  --format text  Human-readable Go-like code (default)
  --format yaml  YAML format
  --format json  JSON format

Examples:
  cx map                          # Show all entities
  cx map internal/store           # Show entities in a specific directory
  cx map --filter F               # Show only functions
  cx map --filter T               # Show only types
  cx map --lang go                # Filter by language
  cx map --format yaml            # YAML output`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMap,
}

var (
	mapFilter string
	mapLang   string
	mapDepth  int
)

func init() {
	rootCmd.AddCommand(mapCmd)

	mapCmd.Flags().StringVar(&mapFilter, "filter", "", "Filter by entity type (F=function, T=type, M=method, C=constant)")
	mapCmd.Flags().StringVar(&mapLang, "lang", "", "Filter by language (go, typescript, python, rust, java)")
	mapCmd.Flags().IntVar(&mapDepth, "depth", 0, "How deep to expand nested types (0 = no limit)")
}

func runMap(cmd *cobra.Command, args []string) error {
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

	// Build filter from flags
	filter := store.EntityFilter{
		Status: "active",
		Limit:  100000, // Allow large codebases
	}

	// Apply path filter if provided
	if len(args) > 0 {
		filter.FilePath = args[0]
	}

	// Apply type filter
	if mapFilter != "" {
		filter.EntityType = mapTypeFilter(mapFilter)
	}

	// Apply language filter
	if mapLang != "" {
		filter.Language = normalizeLanguage(mapLang)
	}

	// Query entities from store
	entities, err := storeDB.QueryEntities(filter)
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Handle "text" format specially for map command
	formatLower := strings.ToLower(outputFormat)
	if formatLower == "text" {
		return outputMapText(cmd, entities)
	}

	// Parse format for yaml/json
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	return outputMapStructured(cmd, entities, format)
}

// mapTypeFilter converts short type filter to entity type
func mapTypeFilter(f string) string {
	switch strings.ToUpper(f) {
	case "F":
		return "function"
	case "T":
		return "type"
	case "M":
		return "method"
	case "C":
		return "constant"
	case "V":
		return "variable"
	case "I":
		return "import"
	default:
		return ""
	}
}

// outputMapText outputs entities in Go-like text format grouped by file
func outputMapText(cmd *cobra.Command, entities []*store.Entity) error {
	// Group entities by file
	byFile := make(map[string][]*store.Entity)
	for _, e := range entities {
		byFile[e.FilePath] = append(byFile[e.FilePath], e)
	}

	// Sort file paths
	var filePaths []string
	for fp := range byFile {
		filePaths = append(filePaths, fp)
	}
	sort.Strings(filePaths)

	w := cmd.OutOrStdout()

	for i, filePath := range filePaths {
		if i > 0 {
			fmt.Fprintln(w)
		}

		// Print file header
		fmt.Fprintf(w, "// %s\n", filePath)

		fileEntities := byFile[filePath]

		// Sort entities by line number
		sort.Slice(fileEntities, func(i, j int) bool {
			return fileEntities[i].LineStart < fileEntities[j].LineStart
		})

		// Print each entity's skeleton
		for _, e := range fileEntities {
			skeleton := e.Skeleton
			if skeleton == "" {
				// Generate skeleton on the fly if not stored
				skeleton = generateSkeletonFromEntity(e)
			}

			if skeleton != "" {
				// Print with proper indentation
				lines := strings.Split(skeleton, "\n")
				for _, line := range lines {
					fmt.Fprintln(w, line)
				}
				fmt.Fprintln(w)
			}
		}
	}

	return nil
}

// outputMapStructured outputs entities in YAML or JSON format
func outputMapStructured(cmd *cobra.Command, entities []*store.Entity, format output.Format) error {
	// Build output structure
	mapOutput := &MapOutput{
		Files: make(map[string]*FileMap),
		Count: len(entities),
	}

	for _, e := range entities {
		if _, ok := mapOutput.Files[e.FilePath]; !ok {
			mapOutput.Files[e.FilePath] = &FileMap{
				Entities: make(map[string]*MapEntity),
			}
		}

		skeleton := e.Skeleton
		if skeleton == "" {
			skeleton = generateSkeletonFromEntity(e)
		}

		mapOutput.Files[e.FilePath].Entities[e.Name] = &MapEntity{
			Type:       e.EntityType,
			Location:   formatEntityLocation(e),
			Skeleton:   skeleton,
			DocComment: e.DocComment,
			Visibility: inferVisibility(e.Name),
		}
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	density, _ := output.ParseDensity(outputDensity)
	return formatter.FormatToWriter(cmd.OutOrStdout(), mapOutput, density)
}

// MapOutput represents the output structure for cx map
type MapOutput struct {
	Files map[string]*FileMap `yaml:"files" json:"files"`
	Count int                 `yaml:"count" json:"count"`
}

// FileMap represents entities in a single file
type FileMap struct {
	Entities map[string]*MapEntity `yaml:"entities" json:"entities"`
}

// MapEntity represents a single entity in the map output
type MapEntity struct {
	Type       string `yaml:"type" json:"type"`
	Location   string `yaml:"location" json:"location"`
	Skeleton   string `yaml:"skeleton" json:"skeleton"`
	DocComment string `yaml:"doc_comment,omitempty" json:"doc_comment,omitempty"`
	Visibility string `yaml:"visibility,omitempty" json:"visibility,omitempty"`
}

// generateSkeletonFromEntity generates a skeleton from a store.Entity
// This is a fallback for entities that don't have a pre-computed skeleton
func generateSkeletonFromEntity(e *store.Entity) string {
	var sb strings.Builder

	// Add doc comment if present
	if e.DocComment != "" {
		sb.WriteString(e.DocComment)
		sb.WriteByte('\n')
	}

	switch e.EntityType {
	case "function":
		sb.WriteString("func ")
		sb.WriteString(e.Name)
		if e.Signature != "" {
			sb.WriteString(formatSignatureForSkeleton(e.Signature))
		} else {
			sb.WriteString("()")
		}
		sb.WriteString(" { ... }")

	case "method":
		sb.WriteString("func ")
		if e.Receiver != "" {
			sb.WriteByte('(')
			receiverName := "r"
			typeName := strings.TrimPrefix(e.Receiver, "*")
			if len(typeName) > 0 {
				receiverName = strings.ToLower(string(typeName[0]))
			}
			sb.WriteString(receiverName)
			sb.WriteByte(' ')
			sb.WriteString(e.Receiver)
			sb.WriteString(") ")
		}
		sb.WriteString(e.Name)
		if e.Signature != "" {
			sb.WriteString(formatSignatureForSkeleton(e.Signature))
		} else {
			sb.WriteString("()")
		}
		sb.WriteString(" { ... }")

	case "type":
		sb.WriteString("type ")
		sb.WriteString(e.Name)
		sb.WriteByte(' ')
		if e.Kind == "struct" {
			sb.WriteString("struct { ... }")
		} else if e.Kind == "interface" {
			sb.WriteString("interface { ... }")
		} else {
			sb.WriteString(e.Kind)
		}

	case "constant":
		sb.WriteString("const ")
		sb.WriteString(e.Name)

	case "variable":
		sb.WriteString("var ")
		sb.WriteString(e.Name)

	case "import":
		sb.WriteString("import ")
		sb.WriteString(e.Name)

	default:
		sb.WriteString(e.Name)
	}

	return sb.String()
}

// formatSignatureForSkeleton converts the stored signature format to Go syntax
// Stored format: (name: type, name: type) -> (return1, return2)
// Go format: (name type, name type) (return1, return2)
func formatSignatureForSkeleton(sig string) string {
	if sig == "" {
		return "()"
	}

	// Replace ": " with " " for parameter types
	result := strings.ReplaceAll(sig, ": ", " ")

	// Replace " -> " with " " for return types
	result = strings.ReplaceAll(result, " -> ", " ")

	return result
}
