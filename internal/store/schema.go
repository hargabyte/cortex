package store

// schemaSQL defines the SQLite schema for the cortex database.
const schemaSQL = `
-- entities table (replaces beads for code entities)
CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY,              -- sa-fn-a7f9b2-LoginUser
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,        -- function, type, constant, enum, var, import
    kind TEXT,                        -- struct, interface, alias (for types)
    file_path TEXT NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER,
    signature TEXT,                   -- (email:str,pass:str)->(*User,err)
    sig_hash TEXT,                    -- 8-char hash
    body_hash TEXT,                   -- 8-char hash
    receiver TEXT,                    -- *Server (for methods)
    visibility TEXT,                  -- pub, priv
    fields TEXT,                      -- JSON for type fields
    language TEXT DEFAULT 'go',       -- go, typescript, python, rust, java
    status TEXT DEFAULT 'active',     -- active, archived
    body_text TEXT,                   -- Function/method body for FTS search
    doc_comment TEXT,                 -- Documentation comment for FTS search
    skeleton TEXT,                    -- signature + doc comment + { ... } placeholder
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- dependencies (call graph, type usage, etc.)
CREATE TABLE IF NOT EXISTS dependencies (
    from_id TEXT NOT NULL,
    to_id TEXT NOT NULL,
    dep_type TEXT NOT NULL,           -- calls, uses_type, implements, extends, imports
    created_at TEXT NOT NULL,
    PRIMARY KEY (from_id, to_id, dep_type)
);

-- metrics cache
CREATE TABLE IF NOT EXISTS metrics (
    entity_id TEXT PRIMARY KEY,
    pagerank REAL DEFAULT 0,
    in_degree INTEGER DEFAULT 0,
    out_degree INTEGER DEFAULT 0,
    betweenness REAL DEFAULT 0,
    computed_at TEXT
);

-- file index (track scanned files)
CREATE TABLE IF NOT EXISTS file_index (
    file_path TEXT PRIMARY KEY,
    scan_hash TEXT NOT NULL,
    scanned_at TEXT NOT NULL
);

-- external system links
CREATE TABLE IF NOT EXISTS entity_links (
    entity_id TEXT NOT NULL,
    external_system TEXT NOT NULL,    -- beads, github, jira
    external_id TEXT NOT NULL,        -- bd-abc123, issue-456
    link_type TEXT DEFAULT 'related', -- related, implements, fixes, discovered-from
    created_at TEXT NOT NULL,
    PRIMARY KEY (entity_id, external_system, external_id)
);

-- entity tags (bookmarks/labels)
CREATE TABLE IF NOT EXISTS entity_tags (
    entity_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    created_at TEXT NOT NULL,
    created_by TEXT,                  -- who created the tag (user, agent, etc.)
    note TEXT,                        -- optional note about why the tag was added
    PRIMARY KEY (entity_id, tag),
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

-- coverage data (imported from go test -coverprofile)
CREATE TABLE IF NOT EXISTS entity_coverage (
    entity_id TEXT PRIMARY KEY,
    coverage_percent REAL DEFAULT 0,
    covered_lines TEXT,           -- JSON array of covered line numbers
    uncovered_lines TEXT,         -- JSON array of uncovered line numbers
    last_run TEXT,                -- Timestamp of last coverage run
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

-- test to entity mapping (which tests cover which entities)
CREATE TABLE IF NOT EXISTS test_entity_map (
    test_file TEXT NOT NULL,
    test_name TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    PRIMARY KEY (test_file, test_name, entity_id),
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type);
CREATE INDEX IF NOT EXISTS idx_entities_file ON entities(file_path);
CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
CREATE INDEX IF NOT EXISTS idx_entities_status ON entities(status);
CREATE INDEX IF NOT EXISTS idx_entities_language ON entities(language);
CREATE INDEX IF NOT EXISTS idx_deps_from ON dependencies(from_id);
CREATE INDEX IF NOT EXISTS idx_deps_to ON dependencies(to_id);
CREATE INDEX IF NOT EXISTS idx_deps_type ON dependencies(dep_type);
CREATE INDEX IF NOT EXISTS idx_metrics_pagerank ON metrics(pagerank DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_betweenness ON metrics(betweenness DESC);
CREATE INDEX IF NOT EXISTS idx_links_external ON entity_links(external_system, external_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON entity_tags(tag);
CREATE INDEX IF NOT EXISTS idx_tags_entity ON entity_tags(entity_id);
CREATE INDEX IF NOT EXISTS idx_coverage_percent ON entity_coverage(coverage_percent DESC);
CREATE INDEX IF NOT EXISTS idx_test_entity ON test_entity_map(entity_id);

-- FTS5 Virtual Table for full-text search
-- Indexes name, body_text, doc_comment, and file_path for searching by concept
CREATE VIRTUAL TABLE IF NOT EXISTS entity_fts USING fts5(
    name,
    body_text,
    doc_comment,
    file_path,
    content='entities',
    content_rowid='rowid'
);

-- Triggers to keep FTS index in sync with entities table
CREATE TRIGGER IF NOT EXISTS entities_fts_ai AFTER INSERT ON entities BEGIN
    INSERT INTO entity_fts(rowid, name, body_text, doc_comment, file_path)
    VALUES (new.rowid, new.name, new.body_text, new.doc_comment, new.file_path);
END;

CREATE TRIGGER IF NOT EXISTS entities_fts_ad AFTER DELETE ON entities BEGIN
    INSERT INTO entity_fts(entity_fts, rowid, name, body_text, doc_comment, file_path)
    VALUES ('delete', old.rowid, old.name, old.body_text, old.doc_comment, old.file_path);
END;

CREATE TRIGGER IF NOT EXISTS entities_fts_au AFTER UPDATE ON entities BEGIN
    INSERT INTO entity_fts(entity_fts, rowid, name, body_text, doc_comment, file_path)
    VALUES ('delete', old.rowid, old.name, old.body_text, old.doc_comment, old.file_path);
    INSERT INTO entity_fts(rowid, name, body_text, doc_comment, file_path)
    VALUES (new.rowid, new.name, new.body_text, new.doc_comment, new.file_path);
END;
`

// initSchema creates the database tables and indexes if they don't exist.
func (s *Store) initSchema() error {
	_, err := s.db.Exec(schemaSQL)
	return err
}
