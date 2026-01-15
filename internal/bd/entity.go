package bd

import (
	"fmt"
	"strings"
)

// CreateEntity creates a new code entity bead.
// Uses: bd create "title" --type <type> --description "compact-format"
func (c *Client) CreateEntity(opts CreateOptions) (string, error) {
	args := []string{"create", opts.Title}

	if opts.Type != "" {
		args = append(args, "-t", opts.Type)
	}

	if opts.Priority >= 0 && opts.Priority <= 4 {
		args = append(args, "-p", fmt.Sprintf("%d", opts.Priority))
	}

	if opts.Description != "" {
		args = append(args, "-d", opts.Description)
	}

	if opts.Parent != "" {
		args = append(args, "--parent", opts.Parent)
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	// Execute and parse the output for the created ID
	output, err := c.execBD(args...)
	if err != nil {
		return "", err
	}

	// Parse the output to extract the ID
	// bd create typically outputs something like "Created issue: bd-abc123"
	outputStr := string(output)
	id := extractCreatedID(outputStr)
	if id == "" {
		return "", fmt.Errorf("could not parse created entity ID from output: %s", outputStr)
	}

	return id, nil
}

// extractCreatedID parses the bd create output to find the entity ID
func extractCreatedID(output string) string {
	// Look for patterns like "Created issue: bd-abc123" or just the ID
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for bd- prefix which indicates an ID
		if idx := strings.Index(line, "bd-"); idx != -1 {
			// Extract the ID (find the end of the ID)
			idStart := idx
			idEnd := idStart + 3 // "bd-"
			for idEnd < len(line) {
				ch := line[idEnd]
				if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
					idEnd++
				} else {
					break
				}
			}
			return line[idStart:idEnd]
		}
		// Also check for project-prefixed IDs like "Superhero-AI-abc123"
		if strings.Contains(line, "-") {
			parts := strings.Fields(line)
			for _, part := range parts {
				// Look for word containing a hyphen followed by alphanumeric
				if strings.Contains(part, "-") && len(part) > 3 {
					// Could be a project-prefixed ID
					cleaned := strings.TrimRight(part, ".,;:")
					if isLikelyBeadID(cleaned) {
						return cleaned
					}
				}
			}
		}
	}
	return ""
}

// isLikelyBeadID checks if a string looks like a bead ID
func isLikelyBeadID(s string) bool {
	// Bead IDs are typically: prefix-shortcode (e.g., bd-abc123, Superhero-AI-hi5)
	parts := strings.Split(s, "-")
	if len(parts) < 2 {
		return false
	}
	// Last part should be alphanumeric
	lastPart := parts[len(parts)-1]
	for _, ch := range lastPart {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '.') {
			return false
		}
	}
	return len(lastPart) >= 2
}

// UpdateEntity updates an existing entity.
// Uses: bd update <id> --description "new-desc" --status <status> etc.
func (c *Client) UpdateEntity(id string, opts UpdateOptions) error {
	args := []string{"update", id}

	if opts.Title != nil {
		args = append(args, "--title", *opts.Title)
	}

	if opts.Description != nil {
		args = append(args, "-d", *opts.Description)
	}

	if opts.Status != nil {
		args = append(args, "--status", *opts.Status)
	}

	if opts.Priority != nil {
		args = append(args, "-p", fmt.Sprintf("%d", *opts.Priority))
	}

	_, err := c.execBD(args...)
	return err
}

// GetEntity retrieves a single entity by ID.
// Uses: bd show <id> --json
// Returns error if entity not found.
func (c *Client) GetEntity(id string) (*Entity, error) {
	// bd show --json returns an array, even for a single entity
	var entities []Entity
	if err := c.execBDJSON(&entities, "show", id); err != nil {
		return nil, err
	}
	// Check if we got any results
	if len(entities) == 0 {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	// Verify the returned entity has the expected ID (bd show can return empty results)
	if entities[0].ID == "" {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	return &entities[0], nil
}

// ListEntities lists entities with filters.
// Uses: bd list --type <type> --status <status> --json
func (c *Client) ListEntities(filter EntityFilter) ([]Entity, error) {
	args := []string{"list"}

	if filter.Type != "" {
		args = append(args, "--type", filter.Type)
	}

	if filter.Status != "" {
		args = append(args, "--status", filter.Status)
	}

	if filter.Priority != nil {
		args = append(args, "-p", fmt.Sprintf("%d", *filter.Priority))
	}

	if filter.Label != "" {
		args = append(args, "--label", filter.Label)
	}

	// Limit: -1 means get all (--limit 0), 0 means use bd default, >0 means that limit
	if filter.Limit < 0 {
		args = append(args, "--limit", "0") // 0 = no limit in bd
	} else if filter.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", filter.Limit))
	}

	var entities []Entity
	if err := c.execBDJSON(&entities, args...); err != nil {
		return nil, err
	}
	return entities, nil
}

// ArchiveEntity marks entity as archived (soft delete).
// Uses: bd update <id> --status archived
func (c *Client) ArchiveEntity(id string) error {
	_, err := c.execBD("update", id, "--status", "archived")
	return err
}

// CloseEntity closes an entity with an optional reason.
// Uses: bd close <id> --reason "reason"
func (c *Client) CloseEntity(id, reason string) error {
	args := []string{"close", id}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	_, err := c.execBD(args...)
	return err
}

// ClaimEntity atomically claims an entity (sets assignee and in_progress).
// Uses: bd update <id> --claim
func (c *Client) ClaimEntity(id string) error {
	_, err := c.execBD("update", id, "--claim")
	return err
}

// AddLabel adds a label to an entity.
// Uses: bd label add <id> "label"
func (c *Client) AddLabel(id, label string) error {
	_, err := c.execBD("label", "add", id, label)
	return err
}

// RemoveLabel removes a label from an entity.
// Uses: bd label remove <id> "label"
func (c *Client) RemoveLabel(id, label string) error {
	_, err := c.execBD("label", "remove", id, label)
	return err
}

// ListLabels lists labels for an entity.
// Uses: bd label list <id>
func (c *Client) ListLabels(id string) ([]string, error) {
	output, err := c.execBD("label", "list", id)
	if err != nil {
		return nil, err
	}

	// Parse output - typically one label per line
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var labels []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			labels = append(labels, line)
		}
	}
	return labels, nil
}

// Search searches for entities matching a query.
// Uses: bd search "query" --json
func (c *Client) Search(query string) ([]Entity, error) {
	var entities []Entity
	if err := c.execBDJSON(&entities, "search", query); err != nil {
		return nil, err
	}
	return entities, nil
}
