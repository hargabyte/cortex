// Package cmd implements the help-agents command for cx CLI.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// helpAgentsCmd represents the help-agents command
var helpAgentsCmd = &cobra.Command{
	Use:   "help-agents",
	Short: "Output agent-optimized command reference",
	Long: `Output a concise, token-efficient command reference for AI agents.

Unlike --help or --for-agents, this command outputs a curated reference
optimized for AI agent context windows (~1500 tokens).

The output is organized by workflow with the 9 core commands:
  - show, find, safe, context, test, map, scan, db, serve

Examples:
  cx help-agents              # YAML output (default)
  cx help-agents --format json  # JSON output`,
	Run: runHelpAgents,
}

func init() {
	rootCmd.AddCommand(helpAgentsCmd)
}

func runHelpAgents(cmd *cobra.Command, args []string) {
	content := generateAgentReference()

	if outputFormat == "json" {
		fmt.Print(generateAgentReferenceJSON())
	} else {
		fmt.Print(content)
	}
}

func generateAgentReference() string {
	return `# Cortex Agent Reference (v` + Version + `)

> I'm an AI agent. This tool exists because I'm bad at exploring codebases.
> Instead of burning context on exploration, I query what I need directly.

## The Commands I Actually Use

**Every session:**
` + "```bash" + `
cx context                            # Where am I? What's here?
cx context --smart "my task" --budget 8000  # What code matters for this task?
` + "```" + `

**Before modifying code:**
` + "```bash" + `
cx safe <file>                        # What breaks if I change this?
cx safe <file> --quick                # Just the blast radius
` + "```" + `

**Understanding code:**
` + "```bash" + `
cx show <entity>                      # What is this? What does it call?
cx show <entity> --related            # What's around it?
cx find --keystones --top 10          # What's critical in this codebase?
` + "```" + `

---

## Why These Commands Help Me

| Problem | Solution |
|---------|----------|
| I don't know where to start | ` + "`cx context --smart \"task\"`" + ` finds entry points |
| I might break something | ` + "`cx safe <file>`" + ` shows impact before I edit |
| I need to understand dependencies | ` + "`cx show <entity> --graph`" + ` visualizes them |
| I'm wasting tokens exploring | ` + "`cx map`" + ` gives project overview in ~10k tokens |
| I need to find specific code | ` + "`cx find <name>`" + ` or ` + "`cx find \"concept\"`" + ` |
| I need to bookmark critical code | ` + "`cx tag <entity> critical`" + ` for later reference |

---

## Global Flags

` + "```" + `
--format yaml|json|jsonl      Output format (default: yaml)
--density sparse|medium|dense Detail level:
  sparse: 50-100 tokens   - type, location only
  medium: 200-300 tokens  - + signature, dependencies
  dense:  400-600 tokens  - + metrics, hashes, everything
-q, --quiet                   Exit code only (for scripts)
` + "```" + `

---

## cx context
**My most-used command.** Get focused context for a task.

` + "```bash" + `
cx context                            # Session recovery - what's in this codebase?
cx context --full                     # Extended with keystones
cx context --smart "task" --budget N  # THE KEY COMMAND: task-relevant context
cx context <file.go> --hops 2         # Context around a specific file
` + "```" + `

**Flags:**
` + "```" + `
--smart <task>        Natural language task description (triggers intent parsing)
--budget <N>          Token budget (default: 4000)
--hops <N>            Graph expansion depth (default: 1)
--depth <N>           Max hops from entry points (default: 2)
--budget-mode <mode>  importance|distance - how to prune when over budget
--include <what>      deps,callers,types
--exclude <what>      tests,mocks (default)
--with-coverage       Include test coverage data
--full                Extended session recovery
` + "```" + `

**What --smart returns:**
` + "```yaml" + `
intent:
  keywords: [extracted, keywords]
  pattern: add_feature|fix|modify|etc
entry_points:
  FunctionName:
    type: function
    location: file:line
    note: "Why this is relevant"
relevant_entities:
  EntityName:
    relevance: high|medium|low
    reason: "Why included"
` + "```" + `

---

## cx find
**Discover code** by name, concept, or importance.

` + "```bash" + `
cx find <name>                # Name search (prefix match)
cx find "multi word query"    # Concept search (full-text)
cx find --keystones           # Critical entities only
cx find --important --top 20  # Top by PageRank
` + "```" + `

**Name resolution formats:**
` + "```" + `
HandleLogin                   # Simple name (prefix match)
auth.HandleLogin              # Qualified: package.symbol
internal/auth.HandleLogin     # Path-qualified: path.symbol
sa-fn-abc123-HandleLogin      # Direct entity ID
` + "```" + `

**Flags:**
` + "```" + `
--type <T>            F=function, T=type, M=method, C=constant
--exact               Exact match only (no prefix)
--lang <lang>         go|typescript|python|rust|java|c|csharp|php
--limit <N>           Max results (default: 100)
--important           Sort by PageRank
--keystones           Only highly-depended-on entities
--bottlenecks         Only entities central to call paths
--top <N>             Number of results for ranking flags (default: 20)
` + "```" + `

---

## cx show
**Understand a specific entity** - what it is, what it calls, what calls it.

` + "```bash" + `
cx show <entity>              # Entity details
cx show <entity> --related    # + neighborhood (calls, callers, same-file)
cx show <entity> --graph      # + dependency graph visualization
cx show file.go:45            # Entity at specific line
cx show name@path/file.go     # Disambiguate with file hint
` + "```" + `

**Flags:**
` + "```" + `
--related             Show neighborhood
--depth <N>           Hop count for neighborhood (default: 1)
--graph               Show dependency graph
--hops <N>            Graph traversal depth (default: 2)
--direction <dir>     in|out|both (default: both)
--type <T>            Edge filter: calls|uses_type|implements|all
--coverage            Include test coverage info
--include-metrics     Add PageRank scores
` + "```" + `

**Output includes:**
` + "```yaml" + `
EntityName:
  type: function|struct|method|etc
  location: file:line-line
  signature: "(params) -> return"
  visibility: public|private
  dependencies:
    calls: [entity_ids]
    called_by: [{name, location}]
    uses_types: [entity_ids]
` + "```" + `

---

## cx safe
**Pre-flight check before modifying code.** This prevents me from breaking things.

` + "```bash" + `
cx safe <file>                # Full assessment (impact + coverage + drift)
cx safe <file> --quick        # Just blast radius
cx safe --coverage            # Coverage gaps only
cx safe --drift               # Is the graph stale?
cx safe --changes             # What changed since scan?
` + "```" + `

**Risk levels:**
- ` + "`critical`" + `: Multiple undertested keystones affected
- ` + "`high`" + `: Keystones affected with coverage gaps
- ` + "`medium`" + `: Multiple entities affected
- ` + "`low`" + `: Isolated changes

**Flags:**
` + "```" + `
--quick               Just impact analysis
--coverage            Coverage gaps mode
--drift               Staleness check
--changes             What changed since scan
--depth <N>           Transitive impact depth (default: 3)
--keystones-only      Only keystones (with --coverage)
--strict              Exit non-zero on drift (for CI)
--fix                 Update hashes for drifted entities
` + "```" + `

---

## cx map
**Project skeleton view.** ~10k tokens for entire project structure.

` + "```bash" + `
cx map                        # Full project
cx map <path>                 # Specific directory
cx map --filter F             # Functions only
cx map --filter T --lang go   # Go types only
` + "```" + `

**Flags:**
` + "```" + `
--filter <T>          F=functions, T=types, M=methods, C=constants
--lang <lang>         Filter by language
` + "```" + `

---

## cx test
**Smart test selection** - find which tests to run for changes.

` + "```bash" + `
cx test                       # Tests for uncommitted changes
cx test <file>                # Tests for specific file
cx test --diff --run          # Find and actually run tests
cx test --gaps                # Show coverage gaps
` + "```" + `

**Subcommands:**
` + "```bash" + `
cx test coverage import coverage.out  # Import coverage data
cx test coverage import .coverage/    # Import per-test coverage
` + "```" + `

---

## cx scan
**Build or update the code graph.** Run after major changes.

` + "```bash" + `
cx scan                       # Scan current directory
cx scan --force               # Force full rescan
cx scan --overview            # Show project overview after
` + "```" + `

---

## cx db / cx doctor / cx reset
**Database maintenance.**

` + "```bash" + `
cx db info                    # Statistics
cx doctor                     # Health check
cx doctor --fix               # Auto-fix issues
cx reset                      # Reset with backup
cx reset --hard --force       # Delete everything
` + "```" + `

---

## cx guard
**Pre-commit hook.** Catches problems before commit.

` + "```bash" + `
cx guard                      # Check staged files
cx guard --all                # Check all modified
cx guard --fail-on-warnings   # Strict mode

# Install as git hook:
echo 'cx guard --staged' >> .git/hooks/pre-commit
` + "```" + `

---

## cx serve
**MCP server** for tool-based integration.

` + "```bash" + `
cx serve --mcp                # Start server
cx serve --status             # Check if running
cx serve --list-tools         # Available tools
` + "```" + `

---

## cx tag / cx tags
**Tag entities** for organization and quick access.

` + "```bash" + `
cx tag <entity> <tags...>             # Add tags
cx tag UserAuth critical auth         # Multiple tags
cx tag LoginUser -n "needs review"    # Tag with note
cx untag <entity> <tag>               # Remove tag
cx tags <entity>                      # List tags for entity
cx tags                               # All tags with counts
cx tags --find critical               # Find entities by tag
cx tags --find auth --find api --all  # ALL tags (AND)
cx tags --find auth --find api --any  # ANY tag (OR)
` + "```" + `

---

## cx status
**Graph and daemon status.**

` + "```bash" + `
cx status                             # Database statistics (alias: cx db info)
` + "```" + `

---

## cx link
**Link code to external systems** (beads, github, jira).

` + "```bash" + `
cx link <entity> <external-id>        # Create link
cx link --list <entity>               # List links
cx link --remove <entity> <ext-id>    # Remove link
` + "```" + `

---

## Supported Languages
Go, TypeScript, JavaScript, Python, Java, Rust, C, C#, PHP

---

## My Workflow

` + "```bash" + `
# 1. Starting a task
cx context --smart "add rate limiting to API" --budget 8000

# 2. Before modifying
cx safe src/api/handler.go

# 3. Understanding dependencies
cx show RateLimiter --graph --hops 2

# 4. Finding related code
cx find "rate limit" --type F

# 5. After changes
cx test --diff --run
` + "```" + `

---

*This tool gives me a map. Without it, I'm exploring blind.*
`
}

func generateAgentReferenceJSON() string {
	return `{
  "version": "` + Version + `",
  "why": "This tool exists because AI agents are bad at exploring codebases. Instead of burning context on exploration, we query what we need directly.",
  "essential_commands": {
    "context": {
      "purpose": "Get focused context for a task (MOST IMPORTANT)",
      "usage": "cx context --smart \"<task>\" --budget 8000",
      "what_it_returns": "entry_points, relevant_entities with relevance scores",
      "when": "Every session, before starting any task"
    },
    "safe": {
      "purpose": "Check what breaks before modifying code",
      "usage": "cx safe <file>",
      "what_it_returns": "risk_level, impact_radius, affected_keystones",
      "when": "Before editing any file"
    },
    "show": {
      "purpose": "Understand a specific entity and its dependencies",
      "usage": "cx show <entity> --related",
      "what_it_returns": "type, signature, calls, called_by, uses_types",
      "flags": ["--related", "--graph", "--hops N", "--direction in|out"]
    },
    "find": {
      "purpose": "Discover code by name or concept",
      "usage": "cx find <name> OR cx find \"multi word concept\"",
      "flags": ["--keystones", "--important", "--type F|T|M", "--lang"]
    },
    "map": {
      "purpose": "Project skeleton in ~10k tokens",
      "usage": "cx map",
      "flags": ["--filter F|T|M", "--lang"]
    },
    "tag": {
      "purpose": "Tag entities for organization and quick access",
      "usage": "cx tag <entity> <tags...>",
      "examples": ["cx tag UserAuth critical", "cx tags --find critical"]
    }
  },
  "workflow": {
    "1_start_task": "cx context --smart \"<task description>\" --budget 8000",
    "2_before_modify": "cx safe <file>",
    "3_understand_deps": "cx show <entity> --graph --hops 2",
    "4_find_related": "cx find \"<concept>\" --type F",
    "5_after_changes": "cx test --diff --run"
  },
  "output": {
    "formats": ["yaml", "json", "jsonl"],
    "densities": {
      "sparse": "50-100 tokens - type, location only",
      "medium": "200-300 tokens - + signature, dependencies (default)",
      "dense": "400-600 tokens - + metrics, hashes, everything"
    }
  },
  "languages": ["go", "typescript", "javascript", "python", "java", "rust", "c", "csharp", "php"],
  "note": "This tool gives me a map. Without it, I'm exploring blind."
}
`
}
