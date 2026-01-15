package bd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddDependency creates a dependency between entities.
// Uses: bd dep add <from> <to> --type <depType>
func (c *Client) AddDependency(fromID, toID, depType string) error {
	args := []string{"dep", "add", fromID, toID}
	if depType != "" {
		args = append(args, "--type", depType)
	}
	_, err := c.execBD(args...)
	return err
}

// AddBlockingDependency creates a blocking dependency (blocker blocks blocked).
// Uses: bd dep <blocker-id> --blocks <blocked-id>
func (c *Client) AddBlockingDependency(blockerID, blockedID string) error {
	_, err := c.execBD("dep", blockerID, "--blocks", blockedID)
	return err
}

// RemoveDependency removes a dependency between entities.
// Uses: bd dep remove <from> <to>
func (c *Client) RemoveDependency(fromID, toID string) error {
	_, err := c.execBD("dep", "remove", fromID, toID)
	return err
}

// ListDependencies gets dependencies for an entity.
// Uses: bd dep list <id> --json
func (c *Client) ListDependencies(id string) ([]Dependency, error) {
	var deps []Dependency
	if err := c.execBDJSON(&deps, "dep", "list", id); err != nil {
		return nil, err
	}
	return deps, nil
}

// GetDependencyTree gets the dependency tree visualization for an entity.
// Uses: bd dep tree <id>
func (c *Client) GetDependencyTree(id string) (string, error) {
	output, err := c.execBD("dep", "tree", id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDependents gets entities that depend on this one (entities blocked by this one).
// Uses: bd show <id> --refs --json (parses refs output)
func (c *Client) GetDependents(id string) ([]string, error) {
	// Try to get refs using --refs flag
	type RefsResult struct {
		Refs []struct {
			ID string `json:"id"`
		} `json:"refs"`
	}

	var result RefsResult
	err := c.execBDJSON(&result, "show", id, "--refs")
	if err != nil {
		// If --refs --json doesn't work, fall back to parsing dep list
		return c.getDependentsFallback(id)
	}

	var ids []string
	for _, ref := range result.Refs {
		ids = append(ids, ref.ID)
	}
	return ids, nil
}

// getDependentsFallback gets dependents by listing all deps and filtering
func (c *Client) getDependentsFallback(id string) ([]string, error) {
	deps, err := c.ListDependencies(id)
	if err != nil {
		return nil, err
	}

	// Find deps where this entity is the ToID (the blocker)
	var dependents []string
	for _, dep := range deps {
		if dep.ToID == id && dep.DepType == "blocks" {
			dependents = append(dependents, dep.FromID)
		}
	}
	return dependents, nil
}

// RelateDependency creates a soft link relationship (no blocking effect).
// Uses: bd dep relate <id1> <id2>
func (c *Client) RelateDependency(id1, id2 string) error {
	_, err := c.execBD("dep", "relate", id1, id2)
	return err
}

// UnrelateDependency removes a soft link relationship.
// Uses: bd dep unrelate <id1> <id2>
func (c *Client) UnrelateDependency(id1, id2 string) error {
	_, err := c.execBD("dep", "unrelate", id1, id2)
	return err
}

// DetectCycles checks for dependency cycles in the graph.
// Uses: bd dep cycles
func (c *Client) DetectCycles() (bool, error) {
	output, err := c.execBD("dep", "cycles")
	if err != nil {
		// Exit code might indicate cycles found
		return true, nil
	}

	// If output is empty or indicates no cycles, return false
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || strings.Contains(strings.ToLower(outputStr), "no cycle") {
		return false, nil
	}

	// Otherwise cycles were found
	return true, nil
}

// GetBlocked returns all blocked entities.
// Uses: bd blocked --json
func (c *Client) GetBlocked() ([]Entity, error) {
	var entities []Entity
	if err := c.execBDJSON(&entities, "blocked"); err != nil {
		return nil, err
	}
	return entities, nil
}

// jsonlDependency represents a dependency as stored in issues.jsonl
type jsonlDependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// jsonlIssue represents the minimal issue structure needed for dependency extraction
type jsonlIssue struct {
	ID           string            `json:"id"`
	Dependencies []jsonlDependency `json:"dependencies"`
}

// ReadAllDependenciesFromJSONL reads .beads/issues.jsonl directly and returns
// all dependency relationships in one pass. This is much faster than calling
// `bd dep list <id>` for each entity (O(1) file read vs O(N) subprocess calls).
//
// Returns a map where the key is an entity ID and the value is a slice of
// Dependency objects representing all dependencies FROM that entity.
func (c *Client) ReadAllDependenciesFromJSONL() (map[string][]Dependency, error) {
	// Find the JSONL file
	var jsonlPath string
	if c.WorkDir == "" || c.WorkDir == "." {
		jsonlPath = filepath.Join(".beads", "issues.jsonl")
	} else {
		jsonlPath = filepath.Join(c.WorkDir, ".beads", "issues.jsonl")
	}

	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open issues.jsonl: %w", err)
	}
	defer file.Close()

	// Result map: entityID -> []Dependency
	result := make(map[string][]Dependency)

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var issue jsonlIssue
		if err := json.Unmarshal(line, &issue); err != nil {
			// Skip malformed lines
			continue
		}

		// Initialize empty slice for this entity
		if _, exists := result[issue.ID]; !exists {
			result[issue.ID] = []Dependency{}
		}

		// Process dependencies
		for _, dep := range issue.Dependencies {
			// The issue_id in the dependency is the entity that HAS the dependency
			// The depends_on_id is what it depends ON
			d := Dependency{
				FromID:  dep.IssueID,
				ToID:    dep.DependsOnID,
				DepType: dep.Type,
			}
			result[dep.IssueID] = append(result[dep.IssueID], d)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading issues.jsonl: %w", err)
	}

	return result, nil
}
