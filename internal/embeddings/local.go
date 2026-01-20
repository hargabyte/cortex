package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	// DefaultModel is the default embedding model to use.
	DefaultModel = "all-minilm"
	// DefaultOllamaURL is the default Ollama API endpoint.
	DefaultOllamaURL = "http://localhost:11434"
	// EmbeddingDimensions is the output dimension of all-minilm.
	EmbeddingDimensions = 384
)

// OllamaEmbedder implements Embedder using the Ollama API.
type OllamaEmbedder struct {
	client   *http.Client
	baseURL  string
	model    string
	mu       sync.Mutex
}

// ollamaEmbedRequest is the request body for Ollama embeddings API.
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string or []string
}

// ollamaEmbedResponse is the response from Ollama embeddings API.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// NewOllamaEmbedder creates a new OllamaEmbedder with default settings.
func NewOllamaEmbedder() *OllamaEmbedder {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	model := os.Getenv("CX_EMBEDDING_MODEL")
	if model == "" {
		model = DefaultModel
	}
	return &OllamaEmbedder{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
		model:   model,
	}
}

// NewOllamaEmbedderWithConfig creates an OllamaEmbedder with custom settings.
func NewOllamaEmbedderWithConfig(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
		model:   model,
	}
}

// Embed generates an embedding vector for the given text.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.doEmbed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts efficiently.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	return e.doEmbed(ctx, texts)
}

// doEmbed calls the Ollama API with either a single string or slice of strings.
func (e *OllamaEmbedder) doEmbed(ctx context.Context, input any) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	reqBody := ollamaEmbedRequest{
		Model: e.model,
		Input: input,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Embeddings, nil
}

// ModelVersion returns the model identifier for cache invalidation.
func (e *OllamaEmbedder) ModelVersion() string {
	return e.model
}

// Dimensions returns the embedding vector dimension.
func (e *OllamaEmbedder) Dimensions() int {
	return EmbeddingDimensions
}

// IsAvailable checks if Ollama is running and has the embedding model.
func (e *OllamaEmbedder) IsAvailable(ctx context.Context) bool {
	// Try a simple embed to check availability
	_, err := e.Embed(ctx, "test")
	return err == nil
}

// Close is a no-op for the HTTP-based embedder.
func (e *OllamaEmbedder) Close() error {
	return nil
}

// NewLocalEmbedder creates an embedder (defaults to Ollama).
// This is the primary constructor for getting an Embedder instance.
func NewLocalEmbedder() (Embedder, error) {
	embedder := NewOllamaEmbedder()
	return embedder, nil
}
