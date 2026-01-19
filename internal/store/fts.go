// Package store provides Dolt-backed persistence for cortex state and metadata.
// This file implements full-text search using MySQL FULLTEXT indexes.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SearchResult represents a single FTS search result with relevance scoring.
type SearchResult struct {
	Entity        *Entity `json:"entity"`
	FTSScore      float64 `json:"fts_score"`              // Raw FULLTEXT match score (higher = better match)
	PageRank      float64 `json:"pagerank"`               // PageRank from metrics
	CombinedScore float64 `json:"combined_score"`         // Weighted combination of FTS and PageRank
	MatchColumn   string  `json:"match_column,omitempty"` // Which column matched: name, body_text, doc_comment, file_path
}

// SearchOptions configures the search behavior.
type SearchOptions struct {
	Query          string  // The search query (required)
	Limit          int     // Max results to return (default: 10)
	Threshold      float64 // Minimum relevance score (default: 0.0)
	Language       string  // Filter by language (optional)
	EntityType     string  // Filter by entity type (optional)
	BoostPageRank  float64 // Weight for PageRank in combined score (default: 0.3)
	BoostFTS       float64 // Weight for FTS score in combined score (default: 0.7)
	BoostExactName float64 // Multiplier for exact name matches (default: 2.0)
}

// DefaultSearchOptions returns default search options.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit:          10,
		Threshold:      0.0,
		BoostPageRank:  0.3,
		BoostFTS:       0.7,
		BoostExactName: 2.0,
	}
}

// SearchEntities performs full-text search on entities using MySQL FULLTEXT.
// Returns entities sorted by combined relevance score (FTS + PageRank).
// Note: Dolt only supports NATURAL LANGUAGE MODE (no boolean operators).
func (s *Store) SearchEntities(opts SearchOptions) ([]*SearchResult, error) {
	if opts.Query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	// Set defaults
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	if opts.BoostFTS <= 0 {
		opts.BoostFTS = 0.7
	}
	if opts.BoostPageRank <= 0 {
		opts.BoostPageRank = 0.3
	}
	if opts.BoostExactName <= 0 {
		opts.BoostExactName = 2.0
	}

	// Normalize query for FULLTEXT matching
	ftsQuery := buildFTSQuery(opts.Query)

	// Build the SQL query using MySQL FULLTEXT syntax
	// MATCH() AGAINST() returns relevance score when used in SELECT
	query := `
		SELECT
			e.id, e.name, e.entity_type, e.kind, e.file_path, e.line_start, e.line_end,
			e.signature, e.sig_hash, e.body_hash, e.receiver, e.visibility, e.fields,
			e.language, e.status, e.body_text, e.doc_comment, e.skeleton,
			e.created_at, e.updated_at,
			MATCH(e.name, e.body_text, e.doc_comment) AGAINST(? IN NATURAL LANGUAGE MODE) as fts_score,
			COALESCE(m.pagerank, 0.0) as pagerank
		FROM entities e
		LEFT JOIN metrics m ON m.entity_id = e.id
		WHERE MATCH(e.name, e.body_text, e.doc_comment) AGAINST(? IN NATURAL LANGUAGE MODE)
		AND e.status = 'active'`

	args := []interface{}{ftsQuery, ftsQuery}

	// Add optional filters
	if opts.Language != "" {
		query += " AND e.language = ?"
		args = append(args, opts.Language)
	}
	if opts.EntityType != "" {
		query += " AND e.entity_type = ?"
		args = append(args, opts.EntityType)
	}

	// Order by FTS score (will reorder by combined score after)
	query += " ORDER BY fts_score DESC"

	// Get more results than needed for re-ranking
	query += fmt.Sprintf(" LIMIT %d", opts.Limit*3)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		result, err := scanSearchResult(rows, opts)
		if err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}

		// Apply exact name match boost
		if strings.EqualFold(result.Entity.Name, opts.Query) {
			result.CombinedScore *= opts.BoostExactName
		}

		// Filter by threshold
		if result.CombinedScore >= opts.Threshold {
			results = append(results, result)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search results: %w", err)
	}

	// Sort by combined score
	sortSearchResults(results)

	// Limit results
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// RebuildFTSIndex is a no-op for MySQL FULLTEXT indexes.
// FULLTEXT indexes are automatically maintained by the database engine.
// This function is kept for API compatibility.
func (s *Store) RebuildFTSIndex() error {
	// MySQL FULLTEXT indexes are automatically maintained
	// No manual rebuild needed like with SQLite FTS5
	return nil
}

// CountFTSEntries returns the number of entities with searchable content.
// For MySQL FULLTEXT, this counts entities that have indexable text.
func (s *Store) CountFTSEntries() (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM entities
		WHERE status = 'active'
		AND (name IS NOT NULL OR body_text IS NOT NULL OR doc_comment IS NOT NULL)`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting FTS entries: %w", err)
	}
	return count, nil
}

// codeStopWords are generic terms that add noise in code search contexts.
// These are filtered out to improve search precision.
var codeStopWords = map[string]bool{
	"code":      true,
	"source":    true,
	"file":      true,
	"function":  true,
	"method":    true,
	"class":     true,
	"implement": true,
	"feature":   true,
	"new":       true,
	"existing":  true,
	"current":   true,
	"project":   true,
	"codebase":  true,
	"logic":     true,
	"system":    true,
	"module":    true,
	"component": true,
}

// buildFTSQuery converts a user query into FULLTEXT query syntax.
// For MySQL NATURAL LANGUAGE MODE, we simply pass clean keywords.
// Filters out code-generic stopwords that add noise.
//
// Examples:
// - "auth" -> "auth"
// - "rate limit" -> "rate limit"
// - "parsing source code" -> "parsing" (source, code filtered as stopwords)
func buildFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into words
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}

	// Filter out code stopwords and build query parts
	var parts []string
	for _, word := range words {
		// Clean the word for FULLTEXT
		word = cleanFTSWord(word)
		word = strings.TrimSpace(word)

		// Skip empty words and code stopwords
		if word == "" {
			continue
		}
		lowerWord := strings.ToLower(word)
		if codeStopWords[lowerWord] {
			continue
		}

		parts = append(parts, word)
	}

	// If all words were filtered, return original first word
	if len(parts) == 0 {
		for _, word := range words {
			word = cleanFTSWord(word)
			word = strings.TrimSpace(word)
			if word != "" {
				return word
			}
		}
		return query // Fall back to original
	}

	// Join words with spaces for natural language search
	return strings.Join(parts, " ")
}

// cleanFTSWord removes special characters that might interfere with FULLTEXT search.
func cleanFTSWord(s string) string {
	// Remove characters that might cause issues in FULLTEXT queries
	replacer := strings.NewReplacer(
		`"`, ``,
		`'`, ``,
		`(`, ``,
		`)`, ``,
		`*`, ``,
		`+`, ``,
		`-`, ``,
		`@`, ``,
		`<`, ``,
		`>`, ``,
		`~`, ``,
	)
	return replacer.Replace(s)
}

// scanSearchResult scans a database row into a SearchResult.
func scanSearchResult(rows *sql.Rows, opts SearchOptions) (*SearchResult, error) {
	var e Entity
	var lineEnd sql.NullInt64
	var language, bodyText, docComment, skeleton sql.NullString
	var createdAt, updatedAt string
	var ftsScore, pagerank float64

	err := rows.Scan(
		&e.ID, &e.Name, &e.EntityType, &e.Kind, &e.FilePath, &e.LineStart, &lineEnd,
		&e.Signature, &e.SigHash, &e.BodyHash, &e.Receiver, &e.Visibility, &e.Fields,
		&language, &e.Status, &bodyText, &docComment, &skeleton,
		&createdAt, &updatedAt,
		&ftsScore, &pagerank)
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if lineEnd.Valid {
		v := int(lineEnd.Int64)
		e.LineEnd = &v
	}
	if language.Valid {
		e.Language = language.String
	} else {
		e.Language = "go"
	}
	if bodyText.Valid {
		e.BodyText = bodyText.String
	}
	if docComment.Valid {
		e.DocComment = docComment.String
	}
	if skeleton.Valid {
		e.Skeleton = skeleton.String
	}

	// Parse timestamps (ignore errors, use current time as fallback)
	e.CreatedAt, _ = parseTimeOrNow(createdAt)
	e.UpdatedAt, _ = parseTimeOrNow(updatedAt)

	// Calculate combined score
	// Normalize FTS score to 0-1 range (BM25 scores can vary widely)
	normalizedFTS := normalizeBM25Score(ftsScore)
	combinedScore := opts.BoostFTS*normalizedFTS + opts.BoostPageRank*pagerank

	return &SearchResult{
		Entity:        &e,
		FTSScore:      ftsScore,
		PageRank:      pagerank,
		CombinedScore: combinedScore,
	}, nil
}

// normalizeBM25Score normalizes a BM25 score to roughly 0-1 range.
// BM25 scores are typically in the 0-20 range for good matches.
func normalizeBM25Score(score float64) float64 {
	if score <= 0 {
		return 0
	}
	// Use sigmoid-like normalization
	// Scores above 10 are considered excellent
	return score / (score + 5.0)
}

// sortSearchResults sorts results by combined score descending.
func sortSearchResults(results []*SearchResult) {
	// Simple insertion sort (results are typically small)
	for i := 1; i < len(results); i++ {
		j := i
		for j > 0 && results[j].CombinedScore > results[j-1].CombinedScore {
			results[j], results[j-1] = results[j-1], results[j]
			j--
		}
	}
}

// parseTimeOrNow parses a time string or returns current time on error.
func parseTimeOrNow(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC(), err
	}
	return t, nil
}
