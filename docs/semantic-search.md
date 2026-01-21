# Semantic Search

Cortex generates vector embeddings for every entity using pure Go (no Python, no external APIs). This enables concept-based code discovery.

## Usage

```bash
# Find by meaning, not just keywords
cx find --semantic "validate user credentials"    # Finds: LoginUser, AuthMiddleware, CheckPassword
cx find --semantic "database connection pooling"  # Finds: NewPool, GetConn, releaseConn
cx find --semantic "error handling for HTTP"      # Finds: HandleError, WriteErrorResponse

# Hybrid search combines multiple signals
cx context --smart "add rate limiting" --budget 8000
# Uses: 50% semantic similarity + 30% keyword match + 20% PageRank importance
```

## How It Works

1. During `cx scan`, entity signatures and doc comments are embedded using all-MiniLM-L6-v2
2. Embeddings are stored in Dolt with full version history
3. Queries are embedded and compared using cosine similarity
4. Results are ranked by conceptual relevance, not just string matching

## Requirements

Embeddings are generated automatically during scan. No API keys or external services needed—everything runs locally using Hugot (pure Go inference).

## Search Types

| Type | Command | Use When |
|------|---------|----------|
| **Name search** | `cx find Login` | You know the entity name |
| **Full-text** | `cx find "authentication JWT"` | Searching by keywords |
| **Semantic** | `cx find --semantic "validate credentials"` | Searching by concept/meaning |
| **Hybrid** | `cx context --smart "task"` | Getting task-relevant context |

## Tips

- Semantic search finds conceptually related code even when keywords differ
- Combine with `--type` and `--lang` filters for precision
- Use 2-4 focused keywords for best results with `--smart`
