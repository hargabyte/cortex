package cmd

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

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
