package store

import (
	"database/sql"
	"strings"
	"time"
)

// AddTag adds a tag to an entity.
// If the tag already exists for the entity, it updates the note and created_by fields.
func (s *Store) AddTag(entityID, tag, createdBy string) error {
	return s.AddTagWithNote(entityID, tag, createdBy, "")
}

// AddTagWithNote adds a tag to an entity with an optional note.
// If the tag already exists for the entity, it updates the note and created_by fields.
func (s *Store) AddTagWithNote(entityID, tag, createdBy, note string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO entity_tags (entity_id, tag, created_at, created_by, note)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(entity_id, tag) DO UPDATE SET
			created_by = excluded.created_by,
			note = excluded.note`,
		entityID, tag, now, createdBy, note)
	return err
}

// RemoveTag removes a tag from an entity.
func (s *Store) RemoveTag(entityID, tag string) error {
	result, err := s.db.Exec(`DELETE FROM entity_tags WHERE entity_id = ? AND tag = ?`, entityID, tag)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetTags returns all tags for an entity.
func (s *Store) GetTags(entityID string) ([]*EntityTag, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, tag, created_at, created_by, note
		FROM entity_tags WHERE entity_id = ?
		ORDER BY tag`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*EntityTag
	for rows.Next() {
		var t EntityTag
		var createdAt string
		var createdBy, note sql.NullString

		if err := rows.Scan(&t.EntityID, &t.Tag, &createdAt, &createdBy, &note); err != nil {
			return nil, err
		}

		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if createdBy.Valid {
			t.CreatedBy = createdBy.String
		}
		if note.Valid {
			t.Note = note.String
		}

		tags = append(tags, &t)
	}
	return tags, rows.Err()
}

// FindByTag returns all entities with a specific tag.
func (s *Store) FindByTag(tag string) ([]*Entity, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.name, e.entity_type, e.kind, e.file_path, e.line_start, e.line_end,
			e.signature, e.sig_hash, e.body_hash, e.receiver, e.visibility, e.fields, e.language, e.status,
			e.body_text, e.doc_comment, e.skeleton, e.created_at, e.updated_at
		FROM entities e
		INNER JOIN entity_tags t ON e.id = t.entity_id
		WHERE t.tag = ?
		ORDER BY e.file_path, e.line_start`, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntities(rows)
}

// FindByTags returns entities that have all (matchAll=true) or any (matchAll=false) of the given tags.
func (s *Store) FindByTags(tags []string, matchAll bool) ([]*Entity, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(tags))
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		placeholders[i] = "?"
		args[i] = tag
	}

	var query string
	if matchAll {
		// Entities must have ALL specified tags
		query = `
			SELECT e.id, e.name, e.entity_type, e.kind, e.file_path, e.line_start, e.line_end,
				e.signature, e.sig_hash, e.body_hash, e.receiver, e.visibility, e.fields, e.language, e.status,
				e.body_text, e.doc_comment, e.skeleton, e.created_at, e.updated_at
			FROM entities e
			WHERE e.id IN (
				SELECT entity_id FROM entity_tags
				WHERE tag IN (` + strings.Join(placeholders, ",") + `)
				GROUP BY entity_id
				HAVING COUNT(DISTINCT tag) = ?
			)
			ORDER BY e.file_path, e.line_start`
		args = append(args, len(tags))
	} else {
		// Entities must have ANY of the specified tags
		query = `
			SELECT DISTINCT e.id, e.name, e.entity_type, e.kind, e.file_path, e.line_start, e.line_end,
				e.signature, e.sig_hash, e.body_hash, e.receiver, e.visibility, e.fields, e.language, e.status,
				e.body_text, e.doc_comment, e.skeleton, e.created_at, e.updated_at
			FROM entities e
			INNER JOIN entity_tags t ON e.id = t.entity_id
			WHERE t.tag IN (` + strings.Join(placeholders, ",") + `)
			ORDER BY e.file_path, e.line_start`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntities(rows)
}

// ListAllTags returns all unique tags in the database with their usage counts.
func (s *Store) ListAllTags() (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT tag, COUNT(*) as count
		FROM entity_tags
		GROUP BY tag
		ORDER BY count DESC, tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		result[tag] = count
	}
	return result, rows.Err()
}

// DeleteTagsForEntity removes all tags for an entity.
func (s *Store) DeleteTagsForEntity(entityID string) error {
	_, err := s.db.Exec(`DELETE FROM entity_tags WHERE entity_id = ?`, entityID)
	return err
}

// CountTags returns the total number of tag assignments.
func (s *Store) CountTags() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM entity_tags`).Scan(&count)
	return count, err
}

// CountUniqueTags returns the number of unique tags.
func (s *Store) CountUniqueTags() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT tag) FROM entity_tags`).Scan(&count)
	return count, err
}

// scanEntities is a helper to scan entity rows into a slice.
func scanEntities(rows *sql.Rows) ([]*Entity, error) {
	var entities []*Entity
	for rows.Next() {
		var e Entity
		var lineEnd sql.NullInt64
		var language, bodyText, docComment, skeleton sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&e.ID, &e.Name, &e.EntityType, &e.Kind, &e.FilePath, &e.LineStart, &lineEnd,
			&e.Signature, &e.SigHash, &e.BodyHash, &e.Receiver, &e.Visibility, &e.Fields, &language, &e.Status,
			&bodyText, &docComment, &skeleton, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		// Parse timestamps
		createdAtTime, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			createdAtTime = time.Now().UTC()
		}
		e.CreatedAt = createdAtTime

		updatedAtTime, err := time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			updatedAtTime = time.Now().UTC()
		}
		e.UpdatedAt = updatedAtTime

		// Handle nullable LineEnd
		if lineEnd.Valid {
			v := int(lineEnd.Int64)
			e.LineEnd = &v
		}

		// Handle nullable Language (default to "go" for backwards compatibility)
		if language.Valid {
			e.Language = language.String
		} else {
			e.Language = "go"
		}

		// Handle nullable FTS fields
		if bodyText.Valid {
			e.BodyText = bodyText.String
		}
		if docComment.Valid {
			e.DocComment = docComment.String
		}
		if skeleton.Valid {
			e.Skeleton = skeleton.String
		}

		entities = append(entities, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}
