package cache

// schemaSQL defines the SQLite schema for the cache database.
// Tables:
//   - metrics: stores computed graph metrics per entity (PageRank, betweenness, degrees)
//   - file_index: tracks file scan state for incremental scanning
const schemaSQL = `
CREATE TABLE IF NOT EXISTS metrics (
    entity_id TEXT PRIMARY KEY,
    pagerank REAL NOT NULL DEFAULT 0,
    in_degree INTEGER NOT NULL DEFAULT 0,
    out_degree INTEGER NOT NULL DEFAULT 0,
    betweenness REAL NOT NULL DEFAULT 0,
    computed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS file_index (
    file_path TEXT PRIMARY KEY,
    scan_hash TEXT NOT NULL,
    scanned_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_metrics_pagerank ON metrics(pagerank DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_betweenness ON metrics(betweenness DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_in_degree ON metrics(in_degree DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_out_degree ON metrics(out_degree DESC);
`

// initSchema creates the database tables and indexes if they don't exist.
func (c *Cache) initSchema() error {
	_, err := c.db.Exec(schemaSQL)
	return err
}
