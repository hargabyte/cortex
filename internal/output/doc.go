// Package output provides YAML/JSON output schema types for CX 2.0.
//
// # Overview
//
// This package replaces the compact CGF format with self-documenting YAML output
// that minimizes cognitive overhead for AI agents. The new format uses:
//   - Self-documenting keys (no abbreviations to decode)
//   - Full type names in signatures (string not str, error not err)
//   - Entity names as YAML keys (no redundant name: field)
//   - Multiple density modes for token efficiency
//
// # Output Types
//
// The package defines structured output for all cx commands:
//
//   - EntityOutput: Single entity details (cx show)
//   - ListOutput: Multiple entities (cx find, cx rank)
//   - GraphOutput: Graph traversal (cx graph)
//   - ImpactOutput: Impact analysis (cx impact)
//   - ContextOutput: Context assembly (cx context)
//
// # Format Types
//
// Three output formats are supported:
//
//   - YAML (default): Self-documenting, human-readable
//   - JSON: Machine-readable, same structure as YAML
//   - CGF (deprecated): Legacy compact format, will be removed in v2.0
//
// # Density Modes
//
// Four density levels control the amount of detail in output:
//
//   - Sparse: Minimal one-line format for token efficiency
//     Example: LoginUser: function @ internal/auth/login.go:45-89
//
//   - Medium (default): Balanced detail for most use cases
//     Includes: type, location, signature, visibility, basic dependencies
//
//   - Dense: Full detail for debugging and verification
//     Includes: everything + hashes, metrics, timestamps, all edges
//
//   - Smart: Importance-based density
//     Keystones get dense format, normal entities get medium, leaves get sparse
//
// # Example Usage
//
// Creating entity output in YAML format:
//
//	output := &EntityOutput{
//	    Type: "function",
//	    Location: "internal/auth/login.go:45-89",
//	    Signature: "(email: string, password: string) -> (*User, error)",
//	    Visibility: "public",
//	    Dependencies: &Dependencies{
//	        Calls: []string{"ValidateEmail", "HashPassword"},
//	        CalledBy: []CalledByEntry{
//	            {Name: "HandleLogin"},
//	            {Name: "HandleRegister"},
//	        },
//	    },
//	}
//
// # Design Principles
//
// 1. Self-documenting keys - No abbreviations, no position-dependent parsing
// 2. Readable signatures - Full type names: string not str
// 3. Names over hashes - Entity name is the ID; hash is internal metadata
// 4. Consistent structure - Same keys in same order for all entity types
// 5. Density where safe - Flow style for short lists, block for depth
//
// # Migration from CGF
//
// The CGF format is deprecated but available via --format=cgf for backward
// compatibility. A warning is printed when CGF format is used. CGF will be
// removed entirely in v2.0.
//
// YAML format is now the default and provides better token efficiency for
// AI agents by eliminating comprehension overhead.
//
// # Token Economics
//
// Format comparison for a typical function entity:
//
//   - CGF: ~97 bytes + ~50 tokens comprehension overhead = ~147 total cost
//   - YAML Medium: ~165 bytes + ~5 tokens comprehension = ~170 total cost
//   - YAML Sparse: ~52 bytes + ~2 tokens comprehension = ~54 total cost
//
// YAML Sparse is actually more efficient than CGF for listings, while YAML
// Medium is only marginally more expensive but vastly more readable.
package output
