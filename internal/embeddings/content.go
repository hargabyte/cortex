package embeddings

import (
	"strings"

	"github.com/anthropics/cx/internal/store"
)

// PrepareEntityContent generates the text to embed for an entity.
// Combines name, type, signature, and doc comment into a single string.
func PrepareEntityContent(e *store.Entity) string {
	var parts []string

	// Type and name (e.g., "function LoginUser")
	parts = append(parts, e.EntityType+" "+e.Name)

	// Full signature if available
	if e.Signature != "" {
		parts = append(parts, e.Signature)
	}

	// Doc comment if present
	if e.DocComment != "" {
		parts = append(parts, e.DocComment)
	}

	return strings.Join(parts, "\n")
}
