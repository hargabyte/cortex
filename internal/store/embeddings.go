package store

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
