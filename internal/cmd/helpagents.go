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
	return `# CX Agent Reference (v` + Version + `)

## Quick Start
` + "```bash" + `
cx                            # Session recovery (run at start of every session)
cx scan                       # First-time setup / rescan after major changes
` + "```" + `

## Global Flags (all commands)
` + "```" + `
--format yaml|json|jsonl      Output format (default: yaml)
--density sparse|medium|dense Detail level (default: medium)
  sparse: 50-100 tokens   - type, location only
  medium: 200-300 tokens  - + signature, basic deps
  dense:  400-600 tokens  - + metrics, hashes, all deps
-v, --verbose                 Enable verbose output
-q, --quiet                   Suppress output (exit code only)
` + "```" + `

---

## cx (no args) / cx context
Session recovery and context assembly.

` + "```bash" + `
cx                                    # Session recovery (status + quick reference)
cx context                            # Same as above
cx context --full                     # Extended with keystones and map
cx context --smart "task" --budget N  # Intent-aware context assembly
cx context <entity> --hops N          # Entity-focused context
` + "```" + `

**Flags:**
` + "```" + `
--smart <task>        Natural language task for intent-aware assembly
--budget <N>          Token budget (default: 4000)
--hops <N>            Graph expansion depth (default: 1)
--depth <N>           Max hops from entry points in smart mode (default: 2)
--budget-mode <mode>  importance|distance (default: importance)
--include <what>      deps,callers,types
--exclude <what>      tests,mocks
--with-coverage       Include coverage data for each entity
--full                Extended session recovery
` + "```" + `

**Examples:**
` + "```bash" + `
cx context --smart "add rate limiting to API" --budget 8000
cx context --smart "fix JWT validation bug" --budget 6000
cx context HandleLogin --hops 2
cx context src/auth/login.go
` + "```" + `

---

## cx find
Discover code by name, concept, or importance.

` + "```bash" + `
cx find <name>                # Name search (prefix match)
cx find "multi word query"    # Concept search (FTS)
cx find --keystones           # Critical entities
cx find --important --top N   # Top by PageRank
` + "```" + `

**Flags:**
` + "```" + `
--type <T>            Filter: F=function, T=type, M=method, C=constant
--exact               Exact match only (no prefix)
--lang <lang>         Filter by language (go, typescript, python, rust, java)
--file <path>         Filter by file path
--limit <N>           Max results (default: 100)
--important           Sort by PageRank importance
--keystones           Only keystone entities (highly depended-on)
--bottlenecks         Only bottleneck entities (central to paths)
--top <N>             Number of results for importance flags (default: 20)
--qualified           Show qualified names in output
` + "```" + `

**Examples:**
` + "```bash" + `
cx find HandleLogin                   # Name search
cx find "authentication JWT"          # Concept search
cx find --type=F Login                # Functions matching "Login"
cx find --keystones --top 10          # Top 10 critical entities
cx find --lang=go --important         # Important Go entities
cx find auth.LoginUser                # Qualified name: package.symbol
cx find internal/auth.LoginUser       # Path-qualified: path.symbol
` + "```" + `

---

## cx show
Understand a specific entity.

` + "```bash" + `
cx show <entity>              # Entity details
cx show <entity> --related    # + neighborhood (calls, callers, same-file)
cx show <entity> --graph      # + dependency graph visualization
cx show file.go:45            # Entity at specific line
` + "```" + `

**Flags:**
` + "```" + `
--related             Show neighborhood (calls, callers, same-file)
--depth <N>           Hop count for neighborhood (default: 1)
--graph               Show dependency graph visualization
--hops <N>            Graph traversal depth (default: 2)
--direction <dir>     in|out|both (default: both)
--type <T>            Edge filter: calls|uses_type|implements|all
--coverage            Include test coverage information
--include-metrics     Add importance scores to medium density
` + "```" + `

**Examples:**
` + "```bash" + `
cx show HandleLogin                   # Basic details
cx show HandleLogin --related         # Who calls it, what it calls
cx show HandleLogin --related --depth 2  # 2-hop neighborhood
cx show HandleLogin --graph --hops 3  # 3-level dependency graph
cx show HandleLogin --graph --direction in  # Only callers
cx show internal/auth/jwt.go:45       # Entity at line 45
cx show store.Store                   # Qualified name lookup
cx show HandleLogin --coverage        # Include test coverage
` + "```" + `

---

## cx safe
Pre-flight safety check before modifying code.

` + "```bash" + `
cx safe <file>                # Full assessment (impact + coverage + drift)
cx safe <file> --quick        # Just blast radius
cx safe --coverage            # Coverage gaps only
cx safe --drift               # Staleness check only
cx safe --changes             # What changed since scan
` + "```" + `

**Flags:**
` + "```" + `
--quick               Just impact analysis (blast radius)
--coverage            Coverage gaps mode
--drift               Staleness check mode
--changes             Show what changed since scan
--keystones-only      Only show keystones (with --coverage)
--threshold <N>       Coverage threshold % (default: 75)
--depth <N>           Transitive impact depth (default: 3)
--strict              Exit non-zero on drift (for CI)
--fix                 Update hashes for drifted entities (with --drift)
--detailed            Show hash changes (with --changes)
--semantic            Show semantic analysis (with --changes)
--create-task         Create beads task for findings
` + "```" + `

**Examples:**
` + "```bash" + `
cx safe internal/auth/jwt.go          # Full safety check
cx safe internal/auth/jwt.go --quick  # Just blast radius
cx safe --coverage --keystones-only   # Undertested critical code
cx safe --drift --strict              # CI: fail if stale
cx safe --changes --detailed          # What changed with hashes
cx safe src/core/ --depth 5           # Deep transitive analysis
` + "```" + `

---

## cx test
Smart test selection and coverage analysis.

` + "```bash" + `
cx test                       # Tests for uncommitted changes
cx test <file>                # Tests for specific file
cx test --diff --run          # Find and run affected tests
cx test --gaps                # Show coverage gaps
` + "```" + `

**Flags:**
` + "```" + `
--diff                Use git diff HEAD (default behavior)
--commit <ref>        Use specific commit
--file <path>         Specify file directly
--depth <N>           Indirect test discovery depth (default: 2)
--run                 Actually run the selected tests
--output-command      Output go test command only
--gaps                Show coverage gaps
--coverage            Show coverage summary
--keystones-only      With --gaps: only keystones
--threshold <N>       Coverage threshold % (default: 75)
` + "```" + `

**Subcommands:**
` + "```bash" + `
cx coverage import coverage.out       # Import Go coverage file
cx coverage import .coverage/         # Import per-test coverage dir
cx coverage status                    # Show coverage statistics
` + "```" + `

**Examples:**
` + "```bash" + `
cx test                               # What tests for uncommitted changes
cx test internal/auth/                # Tests for auth directory
cx test --diff --run                  # Find and run tests
cx test --commit HEAD~1               # Tests for specific commit
cx test --gaps --keystones-only       # Undertested keystones
cx coverage import coverage.out && cx test --gaps  # Full workflow
` + "```" + `

---

## cx map
Project structure overview (skeleton view).

` + "```bash" + `
cx map                        # Full project skeleton (~10k tokens)
cx map <path>                 # Specific directory
cx map --filter F             # Functions only
` + "```" + `

**Flags:**
` + "```" + `
--filter <T>          F=functions, T=types, M=methods, C=constants
--lang <lang>         Filter by language
--depth <N>           Nested type expansion depth (0=no limit)
` + "```" + `

**Examples:**
` + "```bash" + `
cx map                                # Full project
cx map internal/store                 # Just store package
cx map --filter F                     # Functions only
cx map --filter T --lang go           # Go types only
` + "```" + `

---

## cx scan
Build or update the code graph.

` + "```bash" + `
cx scan                       # Scan current directory (auto-init)
cx scan <path>                # Scan specific directory
cx scan --force               # Force full rescan
` + "```" + `

**Flags:**
` + "```" + `
--force               Rescan even if files unchanged
--overview            Show project overview after scan
--lang <lang>         Scan only specific language
--exclude <patterns>  Comma-separated globs to exclude
--dry-run             Show what would be scanned
` + "```" + `

**Examples:**
` + "```bash" + `
cx scan                               # Normal scan
cx scan --force                       # Force full rescan
cx scan --overview                    # Scan + show overview
cx scan --exclude "vendor/*,*_test.go"  # Skip patterns
` + "```" + `

---

## cx db
Database management.

` + "```bash" + `
cx db info                    # Statistics
cx db doctor                  # Health check
cx db doctor --fix            # Auto-fix issues
cx db compact                 # Reclaim space
cx db export -o file.jsonl    # Export to JSONL
` + "```" + `

**Subcommands:**
` + "```" + `
info                  Database statistics
doctor                Health check (--fix, --deep, --yes)
compact               VACUUM (--remove-archived, --dry-run)
export                Export to JSONL (-o/--output <file>)
` + "```" + `

---

## cx doctor
Database health check (top-level alias).

` + "```bash" + `
cx doctor                     # Run all checks
cx doctor --fix               # Auto-fix issues
cx doctor --deep              # Full graph validation
` + "```" + `

---

## cx reset
Reset database to clean state.

` + "```bash" + `
cx reset                      # Safe reset with backup
cx reset --scan-only          # Clear file index only
cx reset --hard --force       # Delete everything
` + "```" + `

**Flags:**
` + "```" + `
--scan-only           Only clear file index (keeps entities)
--hard                Delete database file (requires --force)
--force               Skip confirmation prompt
--no-backup           Skip backup before reset
--dry-run             Show what would be done
` + "```" + `

---

## cx guard
Pre-commit hook for quality checks.

` + "```bash" + `
cx guard                      # Check staged files
cx guard --all                # Check all modified files
cx guard --fail-on-warnings   # Strict mode
` + "```" + `

**Hook installation:**
` + "```bash" + `
echo 'cx guard --staged' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
` + "```" + `

---

## cx serve
MCP server for AI agent integration.

` + "```bash" + `
cx serve --mcp                # Start MCP server
cx serve --status             # Check if running
cx serve --stop               # Stop server
` + "```" + `

**Flags:**
` + "```" + `
--mcp                 Start MCP server (stdio transport)
--tools <list>        Comma-separated tools (default: diff,impact,context,show)
--timeout <duration>  Inactivity timeout (default: 30m, 0=none)
--status              Check server status
--stop                Stop running server
--list-tools          Show available tools
` + "```" + `

---

## cx link
Link code entities to external systems.

` + "```bash" + `
cx link <entity> <external-id>        # Create link
cx link --list <entity>               # List links
cx link --remove <entity> <ext-id>    # Remove link
` + "```" + `

**Flags:**
` + "```" + `
--system <sys>        beads|github|jira (default: beads)
--type <type>         related|implements|fixes|discovered-from
--list                List links for entity
--remove              Remove a link
` + "```" + `

---

## Supported Languages
Go, TypeScript, JavaScript, Java, Rust, Python
`
}

func generateAgentReferenceJSON() string {
	return `{
  "version": "` + Version + `",
  "commands": {
    "show": {
      "purpose": "Understand code",
      "usage": "cx show <entity>",
      "flags": ["--related", "--graph", "--coverage", "--hops N"],
      "replaces": ["cx near", "cx graph"]
    },
    "find": {
      "purpose": "Discover code",
      "usage": "cx find <query>",
      "flags": ["--important", "--keystones", "--bottlenecks", "--top N"],
      "note": "Multi-word query = concept search",
      "replaces": ["cx search", "cx rank"]
    },
    "safe": {
      "purpose": "Pre-change safety",
      "usage": "cx safe <file>",
      "flags": ["--quick", "--coverage", "--drift", "--changes"],
      "replaces": ["cx impact", "cx gaps", "cx verify", "cx diff", "cx check"]
    },
    "context": {
      "purpose": "Get context",
      "usage": "cx context",
      "flags": ["--smart \"<task>\"", "--budget N", "--hops N"],
      "replaces": ["cx prime"]
    },
    "test": {
      "purpose": "Smart testing",
      "usage": "cx test [file]",
      "flags": ["--diff", "--gaps", "--coverage", "--run"],
      "subcommands": ["coverage import <file>"],
      "replaces": ["cx test-impact"]
    },
    "map": {
      "purpose": "Project overview",
      "usage": "cx map [path]",
      "flags": ["--filter F/T/M", "--lang"],
      "note": "~10k tokens"
    },
    "scan": {
      "purpose": "Build graph",
      "usage": "cx scan [path]",
      "flags": ["--force", "--overview", "--lang"],
      "note": "Auto-initializes",
      "replaces": ["cx init", "cx quickstart"]
    },
    "db": {
      "purpose": "Maintenance",
      "usage": "cx db <subcommand>",
      "subcommands": ["info", "doctor", "compact", "export"],
      "replaces": ["cx doctor", "cx status"]
    },
    "serve": {
      "purpose": "MCP server",
      "usage": "cx serve"
    }
  },
  "patterns": {
    "session_start": "cx context",
    "before_task": "cx context --smart \"<task>\" --budget 8000",
    "before_modify": "cx safe <file>",
    "understand": "cx map && cx find --keystones --top 10",
    "smart_test": "cx test --diff --run",
    "find_untested": "cx safe --coverage --keystones-only"
  },
  "output": {
    "formats": ["yaml", "json", "jsonl"],
    "densities": {
      "sparse": "50-100 tokens/entity",
      "medium": "200-300 tokens/entity (default)",
      "dense": "400-600 tokens/entity"
    }
  },
  "languages": ["go", "typescript", "javascript", "java", "rust", "python"]
}
`
}
