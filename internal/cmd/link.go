package cmd

import (
	"database/sql"
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// linkCmd represents the link command
var linkCmd = &cobra.Command{
	Use:   "link <entity-id> [external-id]",
	Short: "Manage links to external systems",
	Long: `Create, list, and remove links between code entities and external systems.

Links connect code entities to beads (tasks), GitHub issues, Jira tickets, etc.
This enables bidirectional navigation between code and project management tools.

Examples:
  cx link sa-fn-abc123-Login bd-task-456           # Link entity to bead
  cx link sa-fn-abc123-Login issue-789 --system github
  cx link --list sa-fn-abc123-Login               # List entity's links
  cx link --remove sa-fn-abc123-Login bd-task-456 # Remove link`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runLink,
}

var (
	linkList   bool
	linkRemove bool
	linkSystem string
	linkType   string
)

func init() {
	rootCmd.AddCommand(linkCmd)
	linkCmd.Flags().BoolVar(&linkList, "list", false, "List links for entity")
	linkCmd.Flags().BoolVar(&linkRemove, "remove", false, "Remove a link")
	linkCmd.Flags().StringVar(&linkSystem, "system", "beads", "External system (beads, github, jira)")
	linkCmd.Flags().StringVar(&linkType, "type", "related", "Link type (related, implements, fixes, discovered-from)")
}

func runLink(cmd *cobra.Command, args []string) error {
	entityID := args[0]

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

	// Verify entity exists
	_, err = storeDB.GetEntity(entityID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("entity not found: %s", entityID)
		}
		return fmt.Errorf("failed to get entity: %w", err)
	}

	// Validate system
	if !isValidSystem(linkSystem) {
		return fmt.Errorf("invalid system: %s (must be one of: beads, github, jira)", linkSystem)
	}

	// Validate link type
	if !isValidLinkType(linkType) {
		return fmt.Errorf("invalid link type: %s (must be one of: related, implements, fixes, discovered-from)", linkType)
	}

	// Handle list operation
	if linkList {
		return handleLinkList(storeDB, entityID)
	}

	// Handle remove operation
	if linkRemove {
		if len(args) < 2 {
			return fmt.Errorf("remove operation requires entity-id and external-id")
		}
		externalID := args[1]
		return handleLinkRemove(storeDB, entityID, externalID)
	}

	// Handle create operation (default)
	if len(args) < 2 {
		return fmt.Errorf("create operation requires entity-id and external-id")
	}
	externalID := args[1]
	return handleLinkCreate(storeDB, entityID, externalID)
}

func handleLinkCreate(storeDB *store.Store, entityID, externalID string) error {
	link := &store.EntityLink{
		EntityID:       entityID,
		ExternalSystem: linkSystem,
		ExternalID:     externalID,
		LinkType:       linkType,
	}

	err := storeDB.CreateLink(link)
	if err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	fmt.Printf("Linked %s -> %s:%s (%s)\n", entityID, linkSystem, externalID, linkType)
	return nil
}

func handleLinkList(storeDB *store.Store, entityID string) error {
	links, err := storeDB.GetLinks(entityID)
	if err != nil {
		return fmt.Errorf("failed to get links: %w", err)
	}

	if len(links) == 0 {
		fmt.Printf("No links found for %s\n", entityID)
		return nil
	}

	fmt.Printf("Links for %s:\n", entityID)
	for _, link := range links {
		formattedTime := link.CreatedAt.Format("2006-01-02")
		fmt.Printf("  %s:%s (%s) - %s\n", link.ExternalSystem, link.ExternalID, link.LinkType, formattedTime)
	}
	return nil
}

func handleLinkRemove(storeDB *store.Store, entityID, externalID string) error {
	// First check if link exists with default system
	existingLinks, err := storeDB.GetLinks(entityID)
	if err != nil {
		return fmt.Errorf("failed to get links: %w", err)
	}

	// Find matching link
	var foundLink *store.EntityLink
	for _, link := range existingLinks {
		if link.ExternalID == externalID && link.ExternalSystem == linkSystem {
			foundLink = link
			break
		}
	}

	if foundLink == nil {
		return fmt.Errorf("link not found: %s -> %s:%s", entityID, linkSystem, externalID)
	}

	err = storeDB.DeleteLink(entityID, linkSystem, externalID)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	fmt.Printf("Removed link %s -> %s:%s\n", entityID, linkSystem, externalID)
	return nil
}

func isValidSystem(system string) bool {
	validSystems := map[string]bool{
		"beads":  true,
		"github": true,
		"jira":   true,
	}
	return validSystems[system]
}

func isValidLinkType(linkType string) bool {
	validTypes := map[string]bool{
		"related":         true,
		"implements":      true,
		"fixes":           true,
		"discovered-from": true,
	}
	return validTypes[linkType]
}
