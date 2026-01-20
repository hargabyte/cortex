package store

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// EmbeddingStore manages entity embeddings in the database.
type EmbeddingStore interface {
	// SaveEmbedding stores an embedding for an entity.
	SaveEmbedding(entityID string, embedding []float32, modelVersion, contentHash string) error

	// GetEmbedding retrieves an embedding for an entity.
	GetEmbedding(entityID string) (*EntityEmbedding, error)

	// GetAllEmbeddings retrieves all embeddings for similarity search.
	GetAllEmbeddings() ([]*EntityEmbedding, error)

	// DeleteEmbedding removes an embedding.
	DeleteEmbedding(entityID string) error

	// FindSimilar finds the top-K most similar entities to a query vector.
	FindSimilar(queryVec []float32, limit int) ([]SimilarityResult, error)

	// NeedsEmbedding returns entity IDs that need (re)embedding.
	NeedsEmbedding(modelVersion string) ([]string, error)

	// EmbeddingCount returns the number of stored embeddings.
	EmbeddingCount() (int, error)

	// GetEmbeddingAt retrieves an embedding at a specific commit/ref (time travel).
	GetEmbeddingAt(entityID, ref string) (*EntityEmbedding, error)
}

// EntityEmbedding represents a stored embedding for a code entity.
type EntityEmbedding struct {
	EntityID     string    `json:"entity_id"`
	Embedding    []float32 `json:"embedding"`
	ModelVersion string    `json:"model_version"`
	ContentHash  string    `json:"content_hash"`
	CreatedAt    string    `json:"created_at"`
}

// SimilarityResult represents a search result with similarity score.
type SimilarityResult struct {
	EntityID   string  `json:"entity_id"`
	Similarity float64 `json:"similarity"`
}

// SaveEmbedding stores an embedding for an entity using INSERT ... ON DUPLICATE KEY UPDATE.
func (s *Store) SaveEmbedding(entityID string, embedding []float32, modelVersion, contentHash string) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO entity_embeddings (entity_id, embedding, model_version, content_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			embedding = VALUES(embedding),
			model_version = VALUES(model_version),
			content_hash = VALUES(content_hash),
			created_at = VALUES(created_at)
	`, entityID, string(embJSON), modelVersion, contentHash, time.Now().Format(time.RFC3339))
	return err
}

// GetEmbedding retrieves an embedding for an entity by querying the database and unmarshaling JSON.
func (s *Store) GetEmbedding(entityID string) (*EntityEmbedding, error) {
	var e EntityEmbedding
	var embJSON string
	err := s.db.QueryRow(`
		SELECT entity_id, embedding, model_version, content_hash, created_at
		FROM entity_embeddings WHERE entity_id = ?
	`, entityID).Scan(&e.EntityID, &embJSON, &e.ModelVersion, &e.ContentHash, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(embJSON), &e.Embedding); err != nil {
		return nil, fmt.Errorf("unmarshal embedding: %w", err)
	}
	return &e, nil
}

// GetAllEmbeddings retrieves all embeddings for similarity search with batch unmarshaling.
func (s *Store) GetAllEmbeddings() ([]*EntityEmbedding, error) {
	rows, err := s.db.Query(`
		SELECT entity_id, embedding, model_version, content_hash, created_at
		FROM entity_embeddings
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*EntityEmbedding
	for rows.Next() {
		var e EntityEmbedding
		var embJSON string
		if err := rows.Scan(&e.EntityID, &embJSON, &e.ModelVersion, &e.ContentHash, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(embJSON), &e.Embedding); err != nil {
			return nil, fmt.Errorf("unmarshal embedding for %s: %w", e.EntityID, err)
		}
		results = append(results, &e)
	}
	return results, rows.Err()
}

// DeleteEmbedding removes an embedding from the database.
func (s *Store) DeleteEmbedding(entityID string) error {
	_, err := s.db.Exec(`DELETE FROM entity_embeddings WHERE entity_id = ?`, entityID)
	return err
}

// FindSimilar finds the top-K most similar entities to a query vector using cosine similarity.
func (s *Store) FindSimilar(queryVec []float32, limit int) ([]SimilarityResult, error) {
	embeddings, err := s.GetAllEmbeddings()
	if err != nil {
		return nil, err
	}

	results := make([]SimilarityResult, 0, len(embeddings))
	for _, e := range embeddings {
		sim := cosineSimilarity(queryVec, e.Embedding)
		results = append(results, SimilarityResult{
			EntityID:   e.EntityID,
			Similarity: sim,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// NeedsEmbedding returns entity IDs that need (re)embedding for a given model version.
func (s *Store) NeedsEmbedding(modelVersion string) ([]string, error) {
	// Find entities without embeddings OR with different model version
	rows, err := s.db.Query(`
		SELECT e.id FROM entities e
		LEFT JOIN entity_embeddings ee ON e.id = ee.entity_id
		WHERE e.status = 'active'
		AND (ee.entity_id IS NULL OR ee.model_version != ?)
	`, modelVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// EmbeddingCount returns the number of stored embeddings.
func (s *Store) EmbeddingCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM entity_embeddings`).Scan(&count)
	return count, err
}

// GetEmbeddingAt retrieves an embedding at a specific commit/ref using time travel queries.
// Supports short commit hashes which are automatically resolved to full hashes.
func (s *Store) GetEmbeddingAt(entityID, ref string) (*EntityEmbedding, error) {
	// Resolve short commit hashes to full hashes
	resolvedRef, err := s.ResolveRef(ref)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
	}

	var e EntityEmbedding
	var embJSON string
	query := fmt.Sprintf(`
		SELECT entity_id, embedding, model_version, content_hash, created_at
		FROM entity_embeddings AS OF '%s'
		WHERE entity_id = ?
	`, resolvedRef)
	err = s.db.QueryRow(query, entityID).Scan(&e.EntityID, &embJSON, &e.ModelVersion, &e.ContentHash, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(embJSON), &e.Embedding); err != nil {
		return nil, fmt.Errorf("unmarshal embedding: %w", err)
	}
	return &e, nil
}
