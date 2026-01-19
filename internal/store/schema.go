package store

// schemaTables defines the MySQL/Dolt schema for the cortex database tables.
// Each statement is separate for compatibility with Dolt driver.
var schemaTables = []string{
	// entities table (replaces beads for code entities)
	// FULLTEXT index must be inline for Dolt compatibility
	`CREATE TABLE IF NOT EXISTS entities (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    kind VARCHAR(50),
    file_path VARCHAR(500) NOT NULL,
    line_start INT NOT NULL,
    line_end INT,
    signature TEXT,
    sig_hash VARCHAR(16),
    body_hash VARCHAR(16),
    receiver VARCHAR(255),
    visibility VARCHAR(20),
    fields TEXT,
    language VARCHAR(20) DEFAULT 'go',
    status VARCHAR(20) DEFAULT 'active',
    body_text LONGTEXT,
    doc_comment TEXT,
    skeleton TEXT,
    created_at VARCHAR(30) NOT NULL,
    updated_at VARCHAR(30) NOT NULL,
    FULLTEXT ft_search (name, body_text, doc_comment)
)`,

	// dependencies (call graph, type usage, etc.)
	`CREATE TABLE IF NOT EXISTS dependencies (
    from_id VARCHAR(255) NOT NULL,
    to_id VARCHAR(255) NOT NULL,
    dep_type VARCHAR(50) NOT NULL,
    created_at VARCHAR(30) NOT NULL,
    PRIMARY KEY (from_id, to_id, dep_type)
)`,

	// metrics cache
	`CREATE TABLE IF NOT EXISTS metrics (
    entity_id VARCHAR(255) PRIMARY KEY,
    pagerank DOUBLE DEFAULT 0,
    in_degree INT DEFAULT 0,
    out_degree INT DEFAULT 0,
    betweenness DOUBLE DEFAULT 0,
    computed_at VARCHAR(30)
)`,

	// file index (track scanned files)
	`CREATE TABLE IF NOT EXISTS file_index (
    file_path VARCHAR(500) PRIMARY KEY,
    scan_hash VARCHAR(64) NOT NULL,
    scanned_at VARCHAR(30) NOT NULL
)`,

	// external system links
	`CREATE TABLE IF NOT EXISTS entity_links (
    entity_id VARCHAR(255) NOT NULL,
    external_system VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    link_type VARCHAR(50) DEFAULT 'related',
    created_at VARCHAR(30) NOT NULL,
    PRIMARY KEY (entity_id, external_system, external_id)
)`,

	// entity tags (bookmarks/labels)
	`CREATE TABLE IF NOT EXISTS entity_tags (
    entity_id VARCHAR(255) NOT NULL,
    tag VARCHAR(100) NOT NULL,
    created_at VARCHAR(30) NOT NULL,
    created_by VARCHAR(100),
    note TEXT,
    PRIMARY KEY (entity_id, tag)
)`,

	// coverage data (imported from go test -coverprofile)
	`CREATE TABLE IF NOT EXISTS entity_coverage (
    entity_id VARCHAR(255) PRIMARY KEY,
    coverage_percent DOUBLE DEFAULT 0,
    covered_lines TEXT,
    uncovered_lines TEXT,
    last_run VARCHAR(30)
)`,

	// test to entity mapping (which tests cover which entities)
	`CREATE TABLE IF NOT EXISTS test_entity_map (
    test_file VARCHAR(500) NOT NULL,
    test_name VARCHAR(255) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (test_file, test_name, entity_id)
)`,

	// scan metadata (tracks scan history for version control)
	`CREATE TABLE IF NOT EXISTS scan_metadata (
    id INT AUTO_INCREMENT PRIMARY KEY,
    scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    git_commit VARCHAR(40),
    git_branch VARCHAR(255),
    files_scanned INT,
    entities_found INT,
    dependencies_found INT,
    scan_duration_ms INT
)`,
}

// schemaIndexes defines indexes to be created after tables.
// These are created separately to handle idempotency gracefully.
var schemaIndexes = []string{
	"CREATE INDEX idx_entities_type ON entities(entity_type)",
	"CREATE INDEX idx_entities_file ON entities(file_path)",
	"CREATE INDEX idx_entities_name ON entities(name)",
	"CREATE INDEX idx_entities_status ON entities(status)",
	"CREATE INDEX idx_entities_language ON entities(language)",
	"CREATE INDEX idx_deps_from ON dependencies(from_id)",
	"CREATE INDEX idx_deps_to ON dependencies(to_id)",
	"CREATE INDEX idx_deps_type ON dependencies(dep_type)",
	"CREATE INDEX idx_metrics_pagerank ON metrics(pagerank)",
	"CREATE INDEX idx_metrics_betweenness ON metrics(betweenness)",
	"CREATE INDEX idx_links_external ON entity_links(external_system, external_id)",
	"CREATE INDEX idx_tags_tag ON entity_tags(tag)",
	"CREATE INDEX idx_tags_entity ON entity_tags(entity_id)",
	"CREATE INDEX idx_coverage_percent ON entity_coverage(coverage_percent)",
	"CREATE INDEX idx_test_entity ON test_entity_map(entity_id)",
	// Note: FULLTEXT index is created inline in entities table for Dolt compatibility
}

// initSchema creates the database tables and indexes if they don't exist.
func (s *Store) initSchema() error {
	// Create tables one at a time (Dolt requires single-statement execution)
	for _, stmt := range schemaTables {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}

	// Create indexes (ignore "duplicate key" errors for idempotency)
	for _, idx := range schemaIndexes {
		_, err := s.db.Exec(idx)
		if err != nil {
			// Ignore duplicate index errors (MySQL error 1061)
			// This makes schema initialization idempotent
			if !isDuplicateIndexError(err) {
				return err
			}
		}
	}

	return nil
}

// isDuplicateIndexError checks if the error is a duplicate index error.
func isDuplicateIndexError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// MySQL error 1061: Duplicate key name
	// Dolt error 1105: "already exists as an index"
	return schemaContains(errStr, "Duplicate key name") ||
		schemaContains(errStr, "1061") ||
		schemaContains(errStr, "already exists")
}

// schemaContains checks if s contains substr (simple helper to avoid strings import).
func schemaContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && schemaFindSubstring(s, substr) >= 0))
}

func schemaFindSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
