package store

import "time"

// Entity represents a code entity (function, type, constant, etc.)
type Entity struct {
	ID         string    `json:"id"`          // sa-fn-a7f9b2-LoginUser
	Name       string    `json:"name"`
	EntityType string    `json:"entity_type"` // function, type, constant, enum, var, import
	Kind       string    `json:"kind"`        // struct, interface, alias (for types)
	FilePath   string    `json:"file_path"`
	LineStart  int       `json:"line_start"`
	LineEnd    *int      `json:"line_end,omitempty"`
	Signature  string    `json:"signature,omitempty"`  // (email:str,pass:str)->(*User,err)
	SigHash    string    `json:"sig_hash,omitempty"`   // 8-char hash
	BodyHash   string    `json:"body_hash,omitempty"`  // 8-char hash
	Receiver   string    `json:"receiver,omitempty"`   // *Server (for methods)
	Visibility string    `json:"visibility"`           // pub, priv
	Fields     string    `json:"fields,omitempty"`     // JSON for type fields
	Language   string    `json:"language"`             // go, typescript, python, rust, java
	Status     string    `json:"status"`               // active, archived
	BodyText   string    `json:"body_text,omitempty"`   // Function body for FTS search
	DocComment string    `json:"doc_comment,omitempty"` // Doc comment for FTS search
	Skeleton   string    `json:"skeleton,omitempty"`    // signature + doc comment + { ... } placeholder
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Dependency represents a relationship between entities (calls, uses_type, etc.)
type Dependency struct {
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	DepType   string    `json:"dep_type"` // calls, uses_type, implements, extends, imports
	CreatedAt time.Time `json:"created_at"`
}

// Metrics represents computed graph metrics for an entity
type Metrics struct {
	EntityID    string    `json:"entity_id"`
	PageRank    float64   `json:"pagerank"`
	InDegree    int       `json:"in_degree"`
	OutDegree   int       `json:"out_degree"`
	Betweenness float64   `json:"betweenness"`
	ComputedAt  time.Time `json:"computed_at"`
}

// FileIndex represents the scan state of a file
type FileIndex struct {
	FilePath  string    `json:"file_path"`
	ScanHash  string    `json:"scan_hash"`
	ScannedAt time.Time `json:"scanned_at"`
}

// EntityLink represents a link between a code entity and an external system
type EntityLink struct {
	EntityID       string    `json:"entity_id"`
	ExternalSystem string    `json:"external_system"` // beads, github, jira
	ExternalID     string    `json:"external_id"`     // bd-abc123, issue-456
	LinkType       string    `json:"link_type"`       // related, implements, fixes, discovered-from
	CreatedAt      time.Time `json:"created_at"`
}

// EntityTag represents a tag/bookmark on a code entity
type EntityTag struct {
	EntityID  string    `json:"entity_id"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"` // who created the tag (user, agent, etc.)
	Note      string    `json:"note,omitempty"`       // optional note about why the tag was added
}

// EntityFilter contains filters for querying entities
type EntityFilter struct {
	EntityType string // function, type, etc.
	Status     string // active, archived
	FilePath   string // filter by file path (prefix match)
	Name       string // filter by name (contains match)
	Language   string // go, typescript, python, rust, java
	Limit      int    // max results (0 = no limit)
	Offset     int    // pagination offset
}

// DependencyFilter contains filters for querying dependencies
type DependencyFilter struct {
	FromID  string // filter by source entity
	ToID    string // filter by target entity
	DepType string // filter by dependency type
}
