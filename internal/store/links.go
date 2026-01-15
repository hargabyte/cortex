package store

import (
	"time"
)

// CreateLink creates a link between an entity and an external system.
func (s *Store) CreateLink(link *EntityLink) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if link.LinkType == "" {
		link.LinkType = "related"
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO entity_links
		(entity_id, external_system, external_id, link_type, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		link.EntityID, link.ExternalSystem, link.ExternalID, link.LinkType, now)
	return err
}

// GetLinks returns all links for an entity.
func (s *Store) GetLinks(entityID string) ([]*EntityLink, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, external_system, external_id, link_type, created_at
		FROM entity_links WHERE entity_id = ?
		ORDER BY external_system, external_id`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*EntityLink
	for rows.Next() {
		var link EntityLink
		var createdAt string
		if err := rows.Scan(&link.EntityID, &link.ExternalSystem, &link.ExternalID,
			&link.LinkType, &createdAt); err != nil {
			return nil, err
		}
		link.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		links = append(links, &link)
	}
	return links, rows.Err()
}

// GetLinksByExternalID returns all links to a specific external ID.
// Useful for finding all entities linked to a specific bead or issue.
func (s *Store) GetLinksByExternalID(externalSystem, externalID string) ([]*EntityLink, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, external_system, external_id, link_type, created_at
		FROM entity_links
		WHERE external_system = ? AND external_id = ?
		ORDER BY entity_id`, externalSystem, externalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*EntityLink
	for rows.Next() {
		var link EntityLink
		var createdAt string
		if err := rows.Scan(&link.EntityID, &link.ExternalSystem, &link.ExternalID,
			&link.LinkType, &createdAt); err != nil {
			return nil, err
		}
		link.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		links = append(links, &link)
	}
	return links, rows.Err()
}

// GetLinksBySystem returns all links to a specific external system.
func (s *Store) GetLinksBySystem(externalSystem string) ([]*EntityLink, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, external_system, external_id, link_type, created_at
		FROM entity_links WHERE external_system = ?
		ORDER BY entity_id, external_id`, externalSystem)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*EntityLink
	for rows.Next() {
		var link EntityLink
		var createdAt string
		if err := rows.Scan(&link.EntityID, &link.ExternalSystem, &link.ExternalID,
			&link.LinkType, &createdAt); err != nil {
			return nil, err
		}
		link.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		links = append(links, &link)
	}
	return links, rows.Err()
}

// DeleteLink removes a specific link.
func (s *Store) DeleteLink(entityID, externalSystem, externalID string) error {
	_, err := s.db.Exec(`
		DELETE FROM entity_links
		WHERE entity_id = ? AND external_system = ? AND external_id = ?`,
		entityID, externalSystem, externalID)
	return err
}

// DeleteLinksForEntity removes all links for an entity.
func (s *Store) DeleteLinksForEntity(entityID string) error {
	_, err := s.db.Exec(`DELETE FROM entity_links WHERE entity_id = ?`, entityID)
	return err
}

// CountLinks returns the total number of links.
func (s *Store) CountLinks() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM entity_links`).Scan(&count)
	return count, err
}
