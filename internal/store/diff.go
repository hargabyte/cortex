package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// DiffChange represents a single entity change from a Dolt diff.
type DiffChange struct {
	DiffType   string  // "added", "modified", "removed"
	EntityID   string  // the entity ID (to_id for added/modified, from_id for removed)
	EntityName string  // entity name
	EntityType string  // function, type, etc.
	FilePath   string  // file path
	LineStart  int     // line start
	OldSigHash *string // previous signature hash (for modified/removed)
	NewSigHash *string // new signature hash (for added/modified)
}

// DiffResult contains the complete diff output with categorized changes.
type DiffResult struct {
	FromRef  string       // the "from" reference (commit, branch, tag)
	ToRef    string       // the "to" reference
	Added    []DiffChange // entities added
	Modified []DiffChange // entities modified
	Removed  []DiffChange // entities removed
}

// DiffOptions specifies options for the Dolt diff query.
type DiffOptions struct {
	FromRef    string // "from" commit/branch/tag (default: "HEAD~1")
	ToRef      string // "to" commit/branch/tag (default: "HEAD" or "WORKING")
	Table      string // table to diff (default: "entities")
	EntityName string // filter to specific entity name (optional)
	EntityID   string // filter to specific entity ID (optional)
}

// DoltDiff queries the Dolt diff between two refs for a given table.
// Returns categorized changes (added, modified, removed).
func (s *Store) DoltDiff(opts DiffOptions) (*DiffResult, error) {
	// Set defaults
	if opts.FromRef == "" {
		opts.FromRef = "HEAD~1"
	}
	if opts.ToRef == "" {
		opts.ToRef = "WORKING"
	}
	if opts.Table == "" {
		opts.Table = "entities"
	}

	result := &DiffResult{
		FromRef:  opts.FromRef,
		ToRef:    opts.ToRef,
		Added:    []DiffChange{},
		Modified: []DiffChange{},
		Removed:  []DiffChange{},
	}

	// Validate refs to prevent SQL injection (only allow safe characters)
	if !isValidRef(opts.FromRef) || !isValidRef(opts.ToRef) || !isValidRef(opts.Table) {
		return nil, fmt.Errorf("invalid ref format")
	}

	// Check if we have enough commit history for HEAD~N refs
	if strings.HasPrefix(opts.FromRef, "HEAD~") {
		count, err := s.commitCount()
		if err != nil {
			return result, nil // No history, return empty
		}
		var n int
		if _, err := fmt.Sscanf(opts.FromRef, "HEAD~%d", &n); err == nil {
			if count <= n {
				return result, nil // Not enough history
			}
		}
	}

	// Build the DOLT_DIFF query
	// Note: DOLT_DIFF doesn't support bind variables, so we use string formatting
	// with validated inputs for the table function arguments
	query := fmt.Sprintf(`
		SELECT
			diff_type,
			COALESCE(to_id, from_id) as entity_id,
			COALESCE(to_name, from_name) as entity_name,
			COALESCE(to_entity_type, from_entity_type) as entity_type,
			COALESCE(to_file_path, from_file_path) as file_path,
			COALESCE(to_line_start, from_line_start) as line_start,
			from_sig_hash,
			to_sig_hash
		FROM DOLT_DIFF('%s', '%s', '%s')
		WHERE 1=1
	`, opts.FromRef, opts.ToRef, opts.Table)

	var args []interface{}

	// Add filters if specified (these can use bind variables)
	if opts.EntityName != "" {
		query += " AND (to_name LIKE ? OR from_name LIKE ?)"
		pattern := "%" + opts.EntityName + "%"
		args = append(args, pattern, pattern)
	}
	if opts.EntityID != "" {
		query += " AND (to_id = ? OR from_id = ?)"
		args = append(args, opts.EntityID, opts.EntityID)
	}

	query += " ORDER BY diff_type, entity_name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		// Check if this is a "no commits" error (new repo with no history)
		if strings.Contains(err.Error(), "cannot resolve") ||
			strings.Contains(err.Error(), "HEAD~1") ||
			strings.Contains(err.Error(), "no such commit") {
			// No history yet - return empty result
			return result, nil
		}
		return nil, fmt.Errorf("dolt diff query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var change DiffChange
		var oldHash, newHash sql.NullString
		var lineStart sql.NullInt64

		err := rows.Scan(
			&change.DiffType,
			&change.EntityID,
			&change.EntityName,
			&change.EntityType,
			&change.FilePath,
			&lineStart,
			&oldHash,
			&newHash,
		)
		if err != nil {
			return nil, fmt.Errorf("scan diff row: %w", err)
		}

		if lineStart.Valid {
			change.LineStart = int(lineStart.Int64)
		}
		if oldHash.Valid {
			change.OldSigHash = &oldHash.String
		}
		if newHash.Valid {
			change.NewSigHash = &newHash.String
		}

		// Categorize by diff type
		switch change.DiffType {
		case "added":
			result.Added = append(result.Added, change)
		case "modified":
			result.Modified = append(result.Modified, change)
		case "removed":
			result.Removed = append(result.Removed, change)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate diff rows: %w", err)
	}

	return result, nil
}

// isValidRef checks if a ref string is safe to use in a query.
// Refs can contain alphanumeric, _, -, ., /, ~, and ^ characters.
func isValidRef(ref string) bool {
	if ref == "" {
		return false
	}
	for _, c := range ref {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' ||
			c == '.' || c == '/' || c == '~' || c == '^') {
			return false
		}
	}
	return true
}

// DoltDiffSummary returns a quick summary of changes between two refs.
func (s *Store) DoltDiffSummary(fromRef, toRef string) (added, modified, removed int, err error) {
	if fromRef == "" {
		fromRef = "HEAD~1"
	}
	if toRef == "" {
		toRef = "WORKING"
	}

	// Validate refs to prevent SQL injection
	if !isValidRef(fromRef) || !isValidRef(toRef) {
		return 0, 0, 0, fmt.Errorf("invalid ref format")
	}

	// Check if we have enough commit history for HEAD~N refs
	if strings.HasPrefix(fromRef, "HEAD~") {
		count, err := s.commitCount()
		if err != nil {
			return 0, 0, 0, nil // No history, return empty
		}
		// Extract N from HEAD~N
		var n int
		if _, err := fmt.Sscanf(fromRef, "HEAD~%d", &n); err == nil {
			if count <= n {
				return 0, 0, 0, nil // Not enough history
			}
		}
	}

	// Note: DOLT_DIFF doesn't support bind variables
	query := fmt.Sprintf(`
		SELECT
			SUM(CASE WHEN diff_type = 'added' THEN 1 ELSE 0 END) as added,
			SUM(CASE WHEN diff_type = 'modified' THEN 1 ELSE 0 END) as modified,
			SUM(CASE WHEN diff_type = 'removed' THEN 1 ELSE 0 END) as removed
		FROM DOLT_DIFF('%s', '%s', 'entities')
	`, fromRef, toRef)

	var addedNull, modifiedNull, removedNull sql.NullInt64
	err = s.db.QueryRow(query).Scan(&addedNull, &modifiedNull, &removedNull)
	if err != nil {
		// Handle no history case
		if strings.Contains(err.Error(), "cannot resolve") ||
			strings.Contains(err.Error(), "HEAD~") ||
			strings.Contains(err.Error(), "no such commit") ||
			strings.Contains(err.Error(), "invalid ancestor spec") {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, fmt.Errorf("diff summary: %w", err)
	}

	if addedNull.Valid {
		added = int(addedNull.Int64)
	}
	if modifiedNull.Valid {
		modified = int(modifiedNull.Int64)
	}
	if removedNull.Valid {
		removed = int(removedNull.Int64)
	}

	return added, modified, removed, nil
}

// commitCount returns the number of commits in the Dolt log.
func (s *Store) commitCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM dolt_log").Scan(&count)
	return count, err
}

// DoltLog returns the Dolt commit history.
type DoltLogEntry struct {
	CommitHash string
	Committer  string
	Email      string
	Date       string
	Message    string
}

// DoltLog returns recent Dolt commits.
func (s *Store) DoltLog(limit int) ([]DoltLogEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT commit_hash, committer, email, date, message
		FROM dolt_log
		ORDER BY date DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("dolt log query: %w", err)
	}
	defer rows.Close()

	var entries []DoltLogEntry
	for rows.Next() {
		var entry DoltLogEntry
		err := rows.Scan(&entry.CommitHash, &entry.Committer, &entry.Email, &entry.Date, &entry.Message)
		if err != nil {
			return nil, fmt.Errorf("scan log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// DoltLogStatsResult contains entity/dependency counts at a commit.
type DoltLogStatsResult struct {
	Entities     int
	Dependencies int
}

// DoltLogStats returns entity and dependency counts at a specific commit.
// Uses AS OF to query the tables at the given commit point.
func (s *Store) DoltLogStats(commitHash string) (*DoltLogStatsResult, error) {
	if !isValidRef(commitHash) {
		return nil, fmt.Errorf("invalid commit hash")
	}

	result := &DoltLogStatsResult{}

	// Count entities at this commit using AS OF
	// Note: AS OF requires the ref in the table reference, not as a function argument
	entityQuery := fmt.Sprintf(`SELECT COUNT(*) FROM entities AS OF '%s'`, commitHash)
	err := s.db.QueryRow(entityQuery).Scan(&result.Entities)
	if err != nil {
		// Table might not exist at this commit
		result.Entities = 0
	}

	// Count dependencies at this commit
	depQuery := fmt.Sprintf(`SELECT COUNT(*) FROM dependencies AS OF '%s'`, commitHash)
	err = s.db.QueryRow(depQuery).Scan(&result.Dependencies)
	if err != nil {
		// Table might not exist at this commit
		result.Dependencies = 0
	}

	return result, nil
}
