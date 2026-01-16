package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// TagExport represents the export format for tags
type TagExport struct {
	Tags []ExportedTag `yaml:"tags" json:"tags"`
}

// ExportedTag represents a single exported tag
type ExportedTag struct {
	EntityID   string `yaml:"entity_id" json:"entity_id"`
	EntityName string `yaml:"entity_name,omitempty" json:"entity_name,omitempty"`
	Tag        string `yaml:"tag" json:"tag"`
	Note       string `yaml:"note,omitempty" json:"note,omitempty"`
}

// tagCmd is the main tag command
var tagCmd = &cobra.Command{
	Use:   "tag <entity> <tag...>",
	Short: "Add tags/bookmarks to entities",
	Long: `Add tags to code entities for organization, bookmarking, and filtering.

Tags are arbitrary labels that help organize and find entities. Common uses:
  - Bookmarking important code: "important", "review", "todo"
  - Categorization: "auth", "api", "core", "utils"
  - Workflow tracking: "needs-test", "needs-docs", "refactor"
  - Agent collaboration: "assigned:claude", "owner:team-a"

Examples:
  cx tag LoginUser important                # Tag single entity
  cx tag LoginUser auth security            # Add multiple tags
  cx tag sa-fn-abc123-Login review          # Tag by direct ID
  cx tag Store@internal/store core          # Tag with file hint
  cx tag LoginUser -n "Needs security audit"  # Tag with note`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTagAdd,
}

// untagCmd removes tags from entities
var untagCmd = &cobra.Command{
	Use:   "untag <entity> <tag>",
	Short: "Remove a tag from an entity",
	Long: `Remove a tag from a code entity.

Examples:
  cx untag LoginUser review          # Remove 'review' tag
  cx untag sa-fn-abc123-Login todo   # Remove by direct ID`,
	Args: cobra.ExactArgs(2),
	RunE: runTagRemove,
}

// tagsCmd lists tags for an entity or all tags
var tagsCmd = &cobra.Command{
	Use:   "tags [entity]",
	Short: "List tags for an entity or all tags",
	Long: `List tags for a specific entity, or list all tags in the database.

Examples:
  cx tags LoginUser             # List tags for entity
  cx tags                       # List all unique tags with counts
  cx tags --find auth           # Find all entities with 'auth' tag
  cx tags --find auth --find security --all  # Entities with ALL tags
  cx tags --find auth --find security --any  # Entities with ANY tag`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTagsList,
}

var (
	tagNote      string
	tagCreatedBy string
	tagFind      []string
	tagMatchAll  bool
	tagMatchAny  bool
)

func init() {
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(untagCmd)
	rootCmd.AddCommand(tagsCmd)

	tagCmd.Flags().StringVarP(&tagNote, "note", "n", "", "Add a note explaining the tag")
	tagCmd.Flags().StringVar(&tagCreatedBy, "by", "", "Who is adding the tag (default: cli)")

	tagsCmd.Flags().StringArrayVar(&tagFind, "find", nil, "Find entities with this tag (can be repeated)")
	tagsCmd.Flags().BoolVar(&tagMatchAll, "all", false, "Require ALL tags when using --find multiple times")
	tagsCmd.Flags().BoolVar(&tagMatchAny, "any", false, "Require ANY tag when using --find multiple times (default)")
}

func runTagAdd(cmd *cobra.Command, args []string) error {
	entityQuery := args[0]
	tags := args[1:]

	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// Resolve the entity
	entity, err := resolveEntityByName(entityQuery, storeDB, "")
	if err != nil {
		return err
	}

	// Add each tag
	createdBy := tagCreatedBy
	if createdBy == "" {
		createdBy = "cli"
	}

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		err := storeDB.AddTagWithNote(entity.ID, tag, createdBy, tagNote)
		if err != nil {
			return fmt.Errorf("failed to add tag %q: %w", tag, err)
		}
	}

	if len(tags) == 1 {
		fmt.Printf("Tagged %s with %q\n", entity.Name, tags[0])
	} else {
		fmt.Printf("Tagged %s with %s\n", entity.Name, strings.Join(tags, ", "))
	}

	return nil
}

func runTagRemove(cmd *cobra.Command, args []string) error {
	entityQuery := args[0]
	tag := args[1]

	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// Resolve the entity
	entity, err := resolveEntityByName(entityQuery, storeDB, "")
	if err != nil {
		return err
	}

	// Remove the tag
	err = storeDB.RemoveTag(entity.ID, tag)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("tag %q not found on entity %s", tag, entity.Name)
		}
		return fmt.Errorf("failed to remove tag: %w", err)
	}

	fmt.Printf("Removed tag %q from %s\n", tag, entity.Name)
	return nil
}

func runTagsList(cmd *cobra.Command, args []string) error {
	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// If --find is specified, find entities by tags
	if len(tagFind) > 0 {
		return runTagsFind(cmd, storeDB, tagFind)
	}

	// If entity is specified, list its tags
	if len(args) > 0 {
		return runTagsForEntity(cmd, storeDB, args[0])
	}

	// Otherwise, list all unique tags
	return runTagsListAll(cmd, storeDB)
}

func runTagsForEntity(cmd *cobra.Command, storeDB *store.Store, entityQuery string) error {
	// Resolve the entity
	entity, err := resolveEntityByName(entityQuery, storeDB, "")
	if err != nil {
		return err
	}

	// Get tags for the entity
	tags, err := storeDB.GetTags(entity.ID)
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Printf("No tags for %s\n", entity.Name)
		return nil
	}

	// Build output
	type tagDetailOutput struct {
		Tag       string `yaml:"tag" json:"tag"`
		CreatedBy string `yaml:"created_by,omitempty" json:"created_by,omitempty"`
		Note      string `yaml:"note,omitempty" json:"note,omitempty"`
	}

	type entityTagsOutput struct {
		Entity   string            `yaml:"entity" json:"entity"`
		EntityID string            `yaml:"entity_id" json:"entity_id"`
		Location string            `yaml:"location" json:"location"`
		Tags     []tagDetailOutput `yaml:"tags" json:"tags"`
		Count    int               `yaml:"count" json:"count"`
	}

	tagDetails := make([]tagDetailOutput, len(tags))
	for i, t := range tags {
		tagDetails[i] = tagDetailOutput{
			Tag:       t.Tag,
			CreatedBy: t.CreatedBy,
			Note:      t.Note,
		}
	}

	out := entityTagsOutput{
		Entity:   entity.Name,
		EntityID: entity.ID,
		Location: formatEntityLocation(entity),
		Tags:     tagDetails,
		Count:    len(tags),
	}

	// Get formatter
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), out, output.DensityMedium)
}

func runTagsListAll(cmd *cobra.Command, storeDB *store.Store) error {
	tagCounts, err := storeDB.ListAllTags()
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tagCounts) == 0 {
		fmt.Println("No tags in database")
		return nil
	}

	// Sort by count descending, then alphabetically
	type tagCountOutput struct {
		Tag   string `yaml:"tag" json:"tag"`
		Count int    `yaml:"count" json:"count"`
	}

	var sorted []tagCountOutput
	for tag, count := range tagCounts {
		sorted = append(sorted, tagCountOutput{Tag: tag, Count: count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Tag < sorted[j].Tag
	})

	type allTagsOutput struct {
		Tags  []tagCountOutput `yaml:"tags" json:"tags"`
		Total int              `yaml:"total" json:"total"`
	}

	out := allTagsOutput{
		Tags:  sorted,
		Total: len(sorted),
	}

	// Get formatter
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), out, output.DensityMedium)
}

func runTagsFind(cmd *cobra.Command, storeDB *store.Store, tags []string) error {
	// Determine match mode (default is ANY)
	matchAll := tagMatchAll
	if !tagMatchAll && !tagMatchAny {
		matchAll = false // Default to ANY
	}

	entities, err := storeDB.FindByTags(tags, matchAll)
	if err != nil {
		return fmt.Errorf("failed to find entities: %w", err)
	}

	if len(entities) == 0 {
		if matchAll {
			fmt.Printf("No entities found with ALL tags: %s\n", strings.Join(tags, ", "))
		} else {
			fmt.Printf("No entities found with ANY tag: %s\n", strings.Join(tags, ", "))
		}
		return nil
	}

	// Build output
	type entityResult struct {
		ID       string `yaml:"id" json:"id"`
		Name     string `yaml:"name" json:"name"`
		Type     string `yaml:"type" json:"type"`
		Location string `yaml:"location" json:"location"`
	}

	type findOutput struct {
		Query    []string       `yaml:"query" json:"query"`
		MatchAll bool           `yaml:"match_all" json:"match_all"`
		Entities []entityResult `yaml:"entities" json:"entities"`
		Count    int            `yaml:"count" json:"count"`
	}

	results := make([]entityResult, len(entities))
	for i, e := range entities {
		results[i] = entityResult{
			ID:       e.ID,
			Name:     e.Name,
			Type:     e.EntityType,
			Location: formatEntityLocation(e),
		}
	}

	out := findOutput{
		Query:    tags,
		MatchAll: matchAll,
		Entities: results,
		Count:    len(results),
	}

	// Get formatter
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), out, output.DensityMedium)
}

// tagsExportCmd exports all tags to a YAML file
var tagsExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export all tags to a file",
	Long: `Export all entity tags to a YAML file for backup or sharing.

If no file is specified, outputs to stdout. Default export location
when using -o without a path is .cx/tags.yaml.

Examples:
  cx tags export                    # Export to stdout
  cx tags export tags.yaml          # Export to file
  cx tags export -o                 # Export to .cx/tags.yaml (default)
  cx tags export -o backup.yaml     # Export to specific file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTagsExport,
}

// tagsImportCmd imports tags from a YAML file
var tagsImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import tags from a file",
	Long: `Import entity tags from a YAML file.

By default, existing tags are skipped. Use --overwrite to replace
existing tags with imported values.

Examples:
  cx tags import tags.yaml               # Import, skip existing
  cx tags import tags.yaml --overwrite   # Import, overwrite existing
  cx tags import tags.yaml --dry-run     # Show what would be imported`,
	Args: cobra.ExactArgs(1),
	RunE: runTagsImport,
}

var (
	tagsExportOutput    string
	tagsImportOverwrite bool
	tagsImportDryRun    bool
)

func init() {
	// Add export and import as subcommands of tags
	tagsCmd.AddCommand(tagsExportCmd)
	tagsCmd.AddCommand(tagsImportCmd)

	tagsExportCmd.Flags().StringVarP(&tagsExportOutput, "output", "o", "", "Output file (default: stdout, or .cx/tags.yaml with -o flag)")
	tagsImportCmd.Flags().BoolVar(&tagsImportOverwrite, "overwrite", false, "Overwrite existing tags")
	tagsImportCmd.Flags().BoolVar(&tagsImportDryRun, "dry-run", false, "Show what would be imported without making changes")
}

func runTagsExport(cmd *cobra.Command, args []string) error {
	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// Get all tags with entity names
	tags, err := storeDB.GetAllTagsWithEntity()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Fprintln(os.Stderr, "No tags to export")
		return nil
	}

	// Build export structure
	export := TagExport{
		Tags: make([]ExportedTag, len(tags)),
	}
	for i, t := range tags {
		export.Tags[i] = ExportedTag{
			EntityID:   t.EntityID,
			EntityName: t.EntityName,
			Tag:        t.Tag,
			Note:       t.Note,
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&export)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	// Determine output destination
	var outputPath string
	if len(args) > 0 {
		outputPath = args[0]
	} else if tagsExportOutput != "" {
		outputPath = tagsExportOutput
	} else if cmd.Flags().Changed("output") {
		// -o flag was passed without value, use default
		cxDir, err := config.FindConfigDir(".")
		if err != nil {
			return fmt.Errorf("cx not initialized: run 'cx scan' first")
		}
		outputPath = filepath.Join(cxDir, "tags.yaml")
	}

	if outputPath == "" {
		// Output to stdout
		fmt.Print(string(data))
		return nil
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported %d tags to %s\n", len(tags), outputPath)
	return nil
}

func runTagsImport(cmd *cobra.Command, args []string) error {
	importFile := args[0]

	// Read import file
	data, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse YAML
	var importData TagExport
	if err := yaml.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(importData.Tags) == 0 {
		fmt.Println("No tags to import")
		return nil
	}

	if tagsImportDryRun {
		fmt.Printf("[dry-run] Would import %d tags:\n", len(importData.Tags))
		for _, t := range importData.Tags {
			action := "add"
			fmt.Printf("  %s: %s on %s (%s)\n", action, t.Tag, t.EntityName, t.EntityID)
		}
		return nil
	}

	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	imported := 0
	skipped := 0
	overwritten := 0
	errors := 0

	for _, t := range importData.Tags {
		// Check if tag already exists
		existingTags, err := storeDB.GetTags(t.EntityID)
		if err != nil {
			errors++
			fmt.Fprintf(os.Stderr, "Warning: failed to check existing tags for %s: %v\n", t.EntityID, err)
			continue
		}

		tagExists := false
		for _, et := range existingTags {
			if et.Tag == t.Tag {
				tagExists = true
				break
			}
		}

		if tagExists {
			if tagsImportOverwrite {
				// Remove existing and re-add
				if err := storeDB.RemoveTag(t.EntityID, t.Tag); err != nil {
					errors++
					fmt.Fprintf(os.Stderr, "Warning: failed to remove existing tag %s on %s: %v\n", t.Tag, t.EntityID, err)
					continue
				}
				if err := storeDB.AddTagWithNote(t.EntityID, t.Tag, "import", t.Note); err != nil {
					errors++
					fmt.Fprintf(os.Stderr, "Warning: failed to add tag %s on %s: %v\n", t.Tag, t.EntityID, err)
					continue
				}
				overwritten++
			} else {
				skipped++
			}
		} else {
			if err := storeDB.AddTagWithNote(t.EntityID, t.Tag, "import", t.Note); err != nil {
				errors++
				fmt.Fprintf(os.Stderr, "Warning: failed to add tag %s on %s: %v\n", t.Tag, t.EntityID, err)
				continue
			}
			imported++
		}
	}

	fmt.Printf("Import complete: %d imported, %d skipped, %d overwritten", imported, skipped, overwritten)
	if errors > 0 {
		fmt.Printf(", %d errors", errors)
	}
	fmt.Println()

	return nil
}
