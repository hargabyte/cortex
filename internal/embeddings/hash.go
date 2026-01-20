package embeddings

import (
	"encoding/hex"
	"hash/fnv"

	"github.com/anthropics/cx/internal/store"
)

// ContentHash generates a hash of entity content for change detection.
// Returns a 16-character hex string.
// Used to detect when an entity needs re-embedding.
func ContentHash(e *store.Entity) string {
	h := fnv.New64a()

	// Hash the same content that PrepareEntityContent uses
	h.Write([]byte(e.EntityType))
	h.Write([]byte(e.Name))
	h.Write([]byte(e.Signature))
	h.Write([]byte(e.DocComment))

	return hex.EncodeToString(h.Sum(nil))
}
