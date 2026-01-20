package embeddings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	// DefaultModel is the HuggingFace model ID for embeddings.
	DefaultModel = "sentence-transformers/all-MiniLM-L6-v2"
	// ModelVersion is the identifier for cache invalidation.
	ModelVersion = "all-MiniLM-L6-v2"
	// EmbeddingDimensions is the output dimension of all-MiniLM-L6-v2.
	EmbeddingDimensions = 384
)

// HugotEmbedder implements Embedder using the Hugot library with pure Go inference.
type HugotEmbedder struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
}

// NewLocalEmbedder creates an embedder using Hugot with the pure Go backend.
// Models are downloaded to ~/.cx/models/ on first use.
func NewLocalEmbedder() (Embedder, error) {
	return NewHugotEmbedder()
}

// NewHugotEmbedder creates a new HugotEmbedder with the all-MiniLM-L6-v2 model.
func NewHugotEmbedder() (*HugotEmbedder, error) {
	// Use pure Go session (no CGO, no external dependencies)
	session, err := hugot.NewGoSession()
	if err != nil {
		return nil, fmt.Errorf("create hugot session: %w", err)
	}

	// Determine model cache directory
	modelDir := getModelCacheDir()

	// Download model if not cached
	// Specify the standard ONNX file (model has multiple variants)
	downloadOpts := hugot.NewDownloadOptions()
	downloadOpts.OnnxFilePath = "onnx/model.onnx"
	modelPath, err := hugot.DownloadModel(DefaultModel, modelDir, downloadOpts)
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("download model %s: %w", DefaultModel, err)
	}

	// Create feature extraction pipeline
	config := hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      "cxEmbeddings",
	}

	pipeline, err := hugot.NewPipeline(session, config)
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("create embedding pipeline: %w", err)
	}

	return &HugotEmbedder{
		session:  session,
		pipeline: pipeline,
	}, nil
}

// getModelCacheDir returns the directory for caching downloaded models.
func getModelCacheDir() string {
	// Check for custom model directory
	if dir := os.Getenv("CX_MODEL_DIR"); dir != "" {
		return dir
	}

	// Default to ~/.cx/models/
	home, err := os.UserHomeDir()
	if err != nil {
		return "./models"
	}
	return filepath.Join(home, ".cx", "models")
}

// Embed generates an embedding vector for the given text.
func (e *HugotEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result, err := e.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, fmt.Errorf("compute embedding: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return result.Embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts efficiently.
func (e *HugotEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(texts) == 0 {
		return nil, nil
	}

	// Process in batches of 32 (recommended for Go backend)
	const batchSize = 32
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		result, err := e.pipeline.RunPipeline(batch)
		if err != nil {
			return nil, fmt.Errorf("compute batch embeddings: %w", err)
		}

		allEmbeddings = append(allEmbeddings, result.Embeddings...)
	}

	return allEmbeddings, nil
}

// ModelVersion returns the model identifier for cache invalidation.
func (e *HugotEmbedder) ModelVersion() string {
	return ModelVersion
}

// Dimensions returns the embedding vector dimension.
func (e *HugotEmbedder) Dimensions() int {
	return EmbeddingDimensions
}

// Close releases resources held by the embedder.
func (e *HugotEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session != nil {
		err := e.session.Destroy()
		e.session = nil
		e.pipeline = nil
		return err
	}
	return nil
}
