package embeddings

import (
	"context"
	"testing"
)

func TestHugotEmbedder(t *testing.T) {
	// Create embedder (will download model on first run)
	embedder, err := NewLocalEmbedder()
	if err != nil {
		t.Fatalf("NewLocalEmbedder() error: %v", err)
	}
	defer embedder.(*HugotEmbedder).Close()

	// Test single embedding
	ctx := context.Background()
	text := "function LoginUser authenticates a user with email and password"

	embedding, err := embedder.Embed(ctx, text)
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}

	// Verify dimensions
	if len(embedding) != EmbeddingDimensions {
		t.Errorf("Embed() got %d dimensions, want %d", len(embedding), EmbeddingDimensions)
	}

	// Verify non-zero values
	nonZero := 0
	for _, v := range embedding {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < len(embedding)/2 {
		t.Errorf("Embed() too many zero values: %d/%d non-zero", nonZero, len(embedding))
	}

	t.Logf("Embedding dimensions: %d, non-zero values: %d", len(embedding), nonZero)
}

func TestHugotEmbedderBatch(t *testing.T) {
	embedder, err := NewLocalEmbedder()
	if err != nil {
		t.Fatalf("NewLocalEmbedder() error: %v", err)
	}
	defer embedder.(*HugotEmbedder).Close()

	ctx := context.Background()
	texts := []string{
		"function LoginUser authenticates a user",
		"function ValidateEmail checks email format",
		"function ParseJSON decodes JSON data",
	}

	embeddings, err := embedder.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch() error: %v", err)
	}

	if len(embeddings) != len(texts) {
		t.Errorf("EmbedBatch() got %d embeddings, want %d", len(embeddings), len(texts))
	}

	for i, emb := range embeddings {
		if len(emb) != EmbeddingDimensions {
			t.Errorf("EmbedBatch()[%d] got %d dimensions, want %d", i, len(emb), EmbeddingDimensions)
		}
	}

	t.Logf("Generated %d embeddings of %d dimensions each", len(embeddings), EmbeddingDimensions)
}

func TestHugotEmbedderSimilarity(t *testing.T) {
	embedder, err := NewLocalEmbedder()
	if err != nil {
		t.Fatalf("NewLocalEmbedder() error: %v", err)
	}
	defer embedder.(*HugotEmbedder).Close()

	ctx := context.Background()

	// Similar texts should have high similarity
	text1 := "user authentication login"
	text2 := "authenticate user credentials"
	text3 := "parse JSON data structure"

	emb1, _ := embedder.Embed(ctx, text1)
	emb2, _ := embedder.Embed(ctx, text2)
	emb3, _ := embedder.Embed(ctx, text3)

	// Compute cosine similarities
	sim12 := cosineSim(emb1, emb2)
	sim13 := cosineSim(emb1, emb3)

	t.Logf("Similarity (auth vs auth): %.4f", sim12)
	t.Logf("Similarity (auth vs json): %.4f", sim13)

	// Auth texts should be more similar to each other than to JSON text
	if sim12 <= sim13 {
		t.Errorf("Expected auth texts to be more similar: sim12=%.4f, sim13=%.4f", sim12, sim13)
	}
}

func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
