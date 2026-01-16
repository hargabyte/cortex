package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CreateEntity inserts a single entity into the database.
func (s *Store) CreateEntity(e *Entity) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if e.Status == "" {
		e.Status = "active"
	}
	if e.Language == "" {
		e.Language = "go"
	}
	_, err := s.db.Exec(`
		INSERT INTO entities (id, name, entity_type, kind, file_path, line_start, line_end,
			signature, sig_hash, body_hash, receiver, visibility, fields, language, status,
			body_text, doc_comment, skeleton, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Name, e.EntityType, e.Kind, e.FilePath, e.LineStart, e.LineEnd,
		e.Signature, e.SigHash, e.BodyHash, e.Receiver, e.Visibility, e.Fields, e.Language, e.Status,
		e.BodyText, e.DocComment, e.Skeleton, now, now)
	return err
}

// CreateEntitiesBulk inserts multiple entities in a single transaction.
// Much faster than calling CreateEntity repeatedly.
func (s *Store) CreateEntitiesBulk(entities []*Entity) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO entities (id, name, entity_type, kind, file_path, line_start, line_end,
			signature, sig_hash, body_hash, receiver, visibility, fields, language, status,
			body_text, doc_comment, skeleton, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare statement: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i, e := range entities {
		if e.Status == "" {
			e.Status = "active"
		}
		if e.Language == "" {
			e.Language = "go"
		}
		_, err := stmt.Exec(
			e.ID, e.Name, e.EntityType, e.Kind, e.FilePath, e.LineStart, e.LineEnd,
			e.Signature, e.SigHash, e.BodyHash, e.Receiver, e.Visibility, e.Fields, e.Language, e.Status,
			e.BodyText, e.DocComment, e.Skeleton, now, now)
		if err != nil {
			stmt.Close()
			tx.Rollback()
			return fmt.Errorf("insert entity %d (%s): %w", i, e.ID, err)
		}
	}

	stmt.Close()
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction (%d entities): %w", len(entities), err)
	}

	return nil
}

// GetEntity retrieves an entity by ID.
// Returns sql.ErrNoRows if not found.
func (s *Store) GetEntity(id string) (*Entity, error) {
	var e Entity
	var lineEnd sql.NullInt64
	var language, bodyText, docComment, skeleton sql.NullString
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, name, entity_type, kind, file_path, line_start, line_end,
			signature, sig_hash, body_hash, receiver, visibility, fields, language, status,
			body_text, doc_comment, skeleton, created_at, updated_at
		FROM entities WHERE id = ?`, id).Scan(
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

	return &e, nil
}

// QueryEntities returns entities matching the filter.
func (s *Store) QueryEntities(filter EntityFilter) ([]*Entity, error) {
	query := `
		SELECT id, name, entity_type, kind, file_path, line_start, line_end,
			signature, sig_hash, body_hash, receiver, visibility, fields, language, status,
			body_text, doc_comment, skeleton, created_at, updated_at
		FROM entities WHERE 1=1`
	args := []interface{}{}

	if filter.EntityType != "" {
		query += " AND entity_type = ?"
		args = append(args, filter.EntityType)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.FilePath != "" {
		query += " AND file_path LIKE ?"
		args = append(args, filter.FilePath+"%")
	}
	if filter.Name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Language != "" {
		query += " AND language = ?"
		args = append(args, filter.Language)
	}

	query += " ORDER BY file_path, line_start"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

// UpdateEntity updates an existing entity.
// Only non-zero fields are updated.
func (s *Store) UpdateEntity(e *Entity) error {
	if e.ID == "" {
		return fmt.Errorf("entity ID is required")
	}

	// Build dynamic UPDATE statement
	var updates []string
	var args []interface{}

	if e.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, e.Name)
	}
	if e.EntityType != "" {
		updates = append(updates, "entity_type = ?")
		args = append(args, e.EntityType)
	}
	if e.Kind != "" {
		updates = append(updates, "kind = ?")
		args = append(args, e.Kind)
	}
	if e.FilePath != "" {
		updates = append(updates, "file_path = ?")
		args = append(args, e.FilePath)
	}
	if e.LineStart != 0 {
		updates = append(updates, "line_start = ?")
		args = append(args, e.LineStart)
	}
	if e.LineEnd != nil {
		updates = append(updates, "line_end = ?")
		args = append(args, *e.LineEnd)
	}
	if e.Signature != "" {
		updates = append(updates, "signature = ?")
		args = append(args, e.Signature)
	}
	if e.SigHash != "" {
		updates = append(updates, "sig_hash = ?")
		args = append(args, e.SigHash)
	}
	if e.BodyHash != "" {
		updates = append(updates, "body_hash = ?")
		args = append(args, e.BodyHash)
	}
	if e.Receiver != "" {
		updates = append(updates, "receiver = ?")
		args = append(args, e.Receiver)
	}
	if e.Visibility != "" {
		updates = append(updates, "visibility = ?")
		args = append(args, e.Visibility)
	}
	if e.Fields != "" {
		updates = append(updates, "fields = ?")
		args = append(args, e.Fields)
	}
	if e.Status != "" {
		updates = append(updates, "status = ?")
		args = append(args, e.Status)
	}
	if e.Language != "" {
		updates = append(updates, "language = ?")
		args = append(args, e.Language)
	}
	if e.BodyText != "" {
		updates = append(updates, "body_text = ?")
		args = append(args, e.BodyText)
	}
	if e.DocComment != "" {
		updates = append(updates, "doc_comment = ?")
		args = append(args, e.DocComment)
	}
	if e.Skeleton != "" {
		updates = append(updates, "skeleton = ?")
		args = append(args, e.Skeleton)
	}

	if len(updates) == 0 {
		// Nothing to update
		return nil
	}

	// Always update the updated_at timestamp
	now := time.Now().UTC().Format(time.RFC3339)
	updates = append(updates, "updated_at = ?")
	args = append(args, now)

	query := "UPDATE entities SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	args = append(args, e.ID)

	result, err := s.db.Exec(query, args...)
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

// DeleteEntity removes an entity by ID.
// Use ArchiveEntity for soft delete instead.
func (s *Store) DeleteEntity(id string) error {
	result, err := s.db.Exec(`DELETE FROM entities WHERE id = ?`, id)
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

// ArchiveEntity marks an entity as archived (soft delete).
func (s *Store) ArchiveEntity(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		UPDATE entities SET status = ?, updated_at = ? WHERE id = ?`,
		"archived", now, id)
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

// CountEntities returns the total count of entities matching the filter.
func (s *Store) CountEntities(filter EntityFilter) (int, error) {
	query := `SELECT COUNT(*) FROM entities WHERE 1=1`
	args := []interface{}{}

	if filter.EntityType != "" {
		query += " AND entity_type = ?"
		args = append(args, filter.EntityType)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.FilePath != "" {
		query += " AND file_path LIKE ?"
		args = append(args, filter.FilePath+"%")
	}
	if filter.Name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Language != "" {
		query += " AND language = ?"
		args = append(args, filter.Language)
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteEntitiesByFile removes all entities from a given file path.
// Used when rescanning a file.
func (s *Store) DeleteEntitiesByFile(filePath string) error {
	result, err := s.db.Exec(`DELETE FROM entities WHERE file_path = ?`, filePath)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		// This is not an error - the file might not have any entities
		return nil
	}

	return nil
}
