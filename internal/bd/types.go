// Package bd provides a client for interacting with the beads CLI tool.
// It wraps bd commands to provide programmatic access to beads functionality.
package bd

// Entity represents a beads entity (code entity or issue)
type Entity struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	IssueType   string   `json:"issue_type"`
	Labels      []string `json:"labels"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// Dependency represents a beads dependency relationship
type Dependency struct {
	FromID  string `json:"from_id"`
	ToID    string `json:"to_id"`
	DepType string `json:"dep_type"`
}

// EntityFilter contains filters for listing entities
type EntityFilter struct {
	Type     string
	Status   string
	Priority *int
	Label    string
	Limit    int
}

// CreateOptions contains options for creating an entity
type CreateOptions struct {
	Title       string
	Type        string
	Description string
	Priority    int
	Labels      []string
	Parent      string
}

// UpdateOptions contains options for updating an entity
type UpdateOptions struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *int
}
