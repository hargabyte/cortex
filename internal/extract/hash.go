// Package extract provides entity extraction and hash computation for code analysis.
//
// Hash computation is used for staleness detection - determining whether
// a function signature or body has changed since the last scan.
//
// Hash Format: "sig_hash:body_hash" (8 hex chars each)
// - Signature hash: sha256(normalize(name + param_types + return_types))[:8]
// - Body hash: sha256(normalize(function_body_ast))[:8]
package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// HashLength is the number of hex characters in a truncated hash.
// Hashes are truncated to 8 hex chars (32 bits) for compact storage.
const HashLength = 8

// ComputeBodyHashFromAST computes a hash of the function body AST.
// This is used to detect implementation changes without signature changes.
//
// The AST is normalized to ignore formatting/whitespace differences.
// Returns "00000000" for entities without a body (interface methods, etc.)
func ComputeBodyHashFromAST(funcNode *sitter.Node, source []byte) string {
	if funcNode == nil {
		return emptyHash()
	}

	// Find the block (function body)
	bodyNode := funcNode.ChildByFieldName("body")
	if bodyNode == nil {
		return emptyHash() // No body (interface method declaration, etc.)
	}

	// Normalize the body AST
	normalized := normalizeAST(bodyNode, source)
	return truncateHash(hashBytes([]byte(normalized)))
}

// ComputeFileHash computes a hash of file content for change detection.
// This is used at the file level to skip unchanged files during scanning.
func ComputeFileHash(content []byte) string {
	return truncateHash(hashBytes(content))
}

// CompareHashes checks if two hash pairs (sig:body format) differ.
// Returns: sigChanged, bodyChanged
//
// This is useful for determining what kind of change occurred:
//   - sigChanged=true: API-breaking change (signature modified)
//   - bodyChanged=true: Implementation change (body modified)
func CompareHashes(old, new string) (sigChanged, bodyChanged bool) {
	oldSig, oldBody := ParseHashPair(old)
	newSig, newBody := ParseHashPair(new)

	// Invalid format (missing colon) is considered changed
	if oldSig == "" && oldBody == "" {
		return true, true
	}
	if newSig == "" && newBody == "" {
		return true, true
	}

	sigChanged = oldSig != newSig
	bodyChanged = oldBody != newBody

	return sigChanged, bodyChanged
}

// FormatHashPair formats signature and body hashes as "sig:body".
// This is the canonical storage format for entity hashes.
func FormatHashPair(sigHash, bodyHash string) string {
	return sigHash + ":" + bodyHash
}

// ParseHashPair parses a "sig:body" format hash pair.
// Returns empty strings for bodyHash if the format is invalid.
func ParseHashPair(hashPair string) (sigHash, bodyHash string) {
	idx := strings.Index(hashPair, ":")
	if idx == -1 {
		// Invalid format - return the whole string as sigHash
		return hashPair, ""
	}
	return hashPair[:idx], hashPair[idx+1:]
}

// normalizeAST normalizes an AST node to a string for hashing.
// This captures the structure and content while ignoring:
//   - Position information (line numbers, columns)
//   - Formatting (whitespace, indentation)
//   - Comments
func normalizeAST(node *sitter.Node, source []byte) string {
	var sb strings.Builder

	// Recursive walk that captures structure but not formatting
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		// Skip comments entirely - they shouldn't affect the hash
		if n.Type() == "comment" {
			return
		}

		// Include node type for structural identity
		sb.WriteString(n.Type())
		sb.WriteString("(")

		// For leaf nodes (identifiers, literals), include content
		if n.ChildCount() == 0 {
			content := string(source[n.StartByte():n.EndByte()])
			sb.WriteString(content)
		}

		// Recurse into all children
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			walk(child)
		}

		sb.WriteString(")")
	}

	walk(node)
	return sb.String()
}

// hashBytes computes SHA-256 hash of bytes and returns hex string.
func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// truncateHash truncates a hash string to HashLength characters.
func truncateHash(hash string) string {
	if len(hash) <= HashLength {
		return hash
	}
	return hash[:HashLength]
}

// emptyHash returns the hash value used for entities without a body.
func emptyHash() string {
	return "00000000"
}

// IsEmptyHash checks if a hash represents an empty/missing body.
func IsEmptyHash(hash string) bool {
	return hash == "00000000"
}

// ComputeEntityHashes computes both signature and body hashes for an entity.
// This is a convenience function that delegates to the Entity's methods
// and also computes AST-based body hash when a node is available.
func ComputeEntityHashes(entity *Entity, node *sitter.Node, source []byte) {
	// Compute signature hash using entity's method
	entity.ComputeHashes()

	// If we have an AST node, compute body hash from AST for more accurate results
	if node != nil && (entity.Kind == FunctionEntity || entity.Kind == MethodEntity) {
		entity.BodyHash = ComputeBodyHashFromAST(node, source)
	}
}

// NormalizeSignatureForHash normalizes signature components for hashing.
// This ensures consistent hashing regardless of whitespace variations.
func NormalizeSignatureForHash(name string, params []Param, returns []string, receiver string) string {
	var sb strings.Builder

	sb.WriteString(name)

	// Add parameter types
	for _, p := range params {
		sb.WriteByte(',')
		sb.WriteString(p.Type)
	}

	sb.WriteString("->")

	// Add return types
	for i, r := range returns {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(r)
	}

	// Add receiver for methods
	if receiver != "" {
		sb.WriteByte('|')
		sb.WriteString(receiver)
	}

	return sb.String()
}

// NormalizeTypeForHash normalizes type definition for hashing.
func NormalizeTypeForHash(name string, typeKind TypeKind, fields []Field) string {
	var sb strings.Builder

	sb.WriteString(name)
	sb.WriteByte('|')
	sb.WriteString(string(typeKind))

	// Add fields
	for _, f := range fields {
		sb.WriteByte(',')
		sb.WriteString(f.Name)
		sb.WriteByte(':')
		sb.WriteString(f.Type)
	}

	return sb.String()
}
