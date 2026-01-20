package embeddings

import "context"

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts efficiently.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// ModelVersion returns the model identifier for cache invalidation.
	ModelVersion() string

	// Dimensions returns the embedding vector dimension.
	Dimensions() int

	// Close releases resources held by the embedder.
	Close() error
}
