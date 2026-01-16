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

This command outputs a curated reference optimized for AI agent context
windows. Designed to give you exactly what you need to use cx effectively.

Examples:
  cx help-agents              # Markdown output (default)
  cx help-agents --format json  # JSON output for parsing`,
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
	return `# CX Command Reference for AI Agents

> This tool exists because exploring codebases burns tokens.
> Query what you need directly instead.

## Quick Start Workflow

` + "```bash" + `
# 1. Starting a task - get focused context
cx context --smart "your task description" --budget 8000

# 2. Before modifying any file - check impact
cx safe <file>

# 3. Understanding specific code
cx show <entity> --related

# 4. After changes - run affected tests
cx test --diff --run
` + "```" + `

---

## Essential Commands

### cx context
Get task-relevant context. **Use this first.**

` + "```bash" + `
cx context                                    # Session recovery
cx context --smart "task" --budget 8000       # Intent-aware context (MOST USEFUL)
cx context --full                             # Extended with keystones
cx context --diff                             # Context for uncommitted changes
cx context --staged                           # Context for staged changes
` + "```" + `

**Key flags:**
- ` + "`--smart <task>`" + ` - Natural language task description
- ` + "`--budget <N>`" + ` - Token budget (default: 4000)
- ` + "`--depth <N>`" + ` - Max hops from entry points (default: 2)
- ` + "`--with-coverage`" + ` - Include test coverage data

---

### cx safe
Pre-flight safety check. **Use before editing files.**

` + "```bash" + `
cx safe <file>                    # Full assessment (impact + coverage + drift)
cx safe <file> --quick            # Just blast radius
cx safe --coverage                # Coverage gaps only
cx safe --coverage --keystones-only  # Critical coverage gaps
cx safe --drift                   # Is the graph stale?
cx safe --changes                 # What changed since scan?
` + "```" + `

**Risk levels:** critical > high > medium > low

---

### cx show
Understand a specific entity.

` + "```bash" + `
cx show <entity>                  # Entity details
cx show <entity> --related        # + neighborhood (calls, callers, same-file)
cx show <entity> --graph          # + dependency graph
cx show <entity> --graph --hops 3 # Deeper graph
cx show file.go:45                # Entity at specific line
cx show <entity> --coverage       # Include test coverage
` + "```" + `

**Target formats:** ` + "`Name`" + `, ` + "`pkg.Name`" + `, ` + "`path/file.Name`" + `, ` + "`sa-fn-xxx-Name`" + `, ` + "`file:line`" + `

---

### cx find
Discover code by name, concept, or importance.

` + "```bash" + `
cx find <name>                    # Name search (prefix match)
cx find "multi word query"        # Concept/FTS search
cx find --keystones               # Most important entities
cx find --important --top 20      # Top by PageRank
cx find --type F Login            # Functions only (F|T|M|C)
cx find --tag critical            # Filter by tag
` + "```" + `

---

### cx rank
Find critical code by importance metrics.

` + "```bash" + `
cx rank                           # Top 20 by PageRank
cx rank --keystones               # Most depended-on entities
cx rank --bottlenecks             # Central to call paths
cx rank --leaves                  # Leaf nodes (no dependents)
cx rank --top 50                  # More results
` + "```" + `

---

### cx trace
Trace call paths between entities.

` + "```bash" + `
cx trace <from> <to>              # Shortest path between entities
cx trace <from> <to> --all        # All paths
cx trace <entity> --callers       # What calls this?
cx trace <entity> --callees       # What does this call?
` + "```" + `

---

### cx map
Project skeleton view (~10k tokens).

` + "```bash" + `
cx map                            # Full project
cx map <path>                     # Specific directory
cx map --filter F                 # Functions only
cx map --filter T --lang go       # Go types only
` + "```" + `

---

### cx test
Smart test selection and coverage analysis.

` + "```bash" + `
cx test                           # Tests for uncommitted changes
cx test --diff --run              # Find and run tests
cx test <file>                    # Tests for specific file
cx test --gaps                    # Show coverage gaps
cx test --gaps --keystones-only   # Critical coverage gaps
cx test suggest --top 10          # Prioritized test suggestions
` + "```" + `

---

### cx dead
Find provably dead code.

` + "```bash" + `
cx dead                           # Dead private code
cx dead --include-exports         # Include unused exports
cx dead --by-file                 # Group by file
cx dead --type F                  # Functions only
` + "```" + `

---

### cx tag
Tag entities for organization.

` + "```bash" + `
cx tag <entity> <tags...>         # Add tags
cx tag <entity> -n "note"         # Tag with note
cx tag list <entity>              # List entity's tags
cx tag find --tag critical        # Find tagged entities
cx tag export -o                  # Export to .cx/tags.yaml
cx tag import tags.yaml           # Import tags
` + "```" + `

---

### cx scan
Build or update the code graph.

` + "```bash" + `
cx scan                           # Scan (auto-init if needed)
cx scan --force                   # Force full rescan
cx scan --overview                # Show overview after
` + "```" + `

---

### cx db / cx doctor / cx reset
Database maintenance.

` + "```bash" + `
cx db info                        # Statistics
cx doctor                         # Health check
cx doctor --fix                   # Auto-fix issues
cx reset                          # Reset with backup
cx reset --hard --force           # Delete everything
` + "```" + `

---

### cx daemon
Background daemon control.

` + "```bash" + `
cx daemon status                  # Check status
cx daemon start --background      # Start in background
cx daemon stop                    # Stop daemon
` + "```" + `

---

### cx status
Quick status check.

` + "```bash" + `
cx status                         # Daemon and graph status
cx status --json                  # JSON output
` + "```" + `

---

## Global Flags

` + "```" + `
--format yaml|json|jsonl          Output format (default: yaml)
--density sparse|medium|dense     Detail level:
  sparse: 50-100 tokens   - type, location only
  medium: 200-300 tokens  - + signature, dependencies (default)
  dense:  400-600 tokens  - + metrics, hashes, timestamps
-q, --quiet                       Exit code only
-v, --verbose                     Verbose output
` + "```" + `

---

## When to Use Each Command

| Need | Command |
|------|---------|
| Starting a task | ` + "`cx context --smart \"task\"`" + ` |
| Before editing | ` + "`cx safe <file>`" + ` |
| Understand entity | ` + "`cx show <entity> --related`" + ` |
| Find code | ` + "`cx find <name>`" + ` or ` + "`cx find \"concept\"`" + ` |
| Project overview | ` + "`cx map`" + ` |
| Critical code | ` + "`cx rank --keystones`" + ` |
| Call paths | ` + "`cx trace <from> <to>`" + ` |
| Dead code | ` + "`cx dead`" + ` |
| Run tests | ` + "`cx test --diff --run`" + ` |
| Bookmark code | ` + "`cx tag <entity> <tags>`" + ` |

---

## Supported Languages

Go, TypeScript, JavaScript, Python, Java, Rust, C, C#, PHP

---

*This tool gives you a map. Without it, you're exploring blind.*
`
}

func generateAgentReferenceJSON() string {
	return `{
  "version": "` + Version + `",
  "purpose": "AI agents waste tokens exploring codebases. This tool lets you query what you need directly.",
  "workflow": {
    "1_start_task": "cx context --smart \"<task>\" --budget 8000",
    "2_before_modify": "cx safe <file>",
    "3_understand": "cx show <entity> --related",
    "4_after_changes": "cx test --diff --run"
  },
  "commands": {
    "context": {
      "purpose": "Get task-relevant context (MOST IMPORTANT)",
      "usage": "cx context --smart \"<task>\" --budget 8000",
      "flags": ["--smart", "--budget", "--depth", "--diff", "--staged", "--with-coverage", "--full"]
    },
    "safe": {
      "purpose": "Pre-flight safety check before modifying code",
      "usage": "cx safe <file>",
      "flags": ["--quick", "--coverage", "--drift", "--changes", "--keystones-only", "--depth"]
    },
    "show": {
      "purpose": "Understand a specific entity",
      "usage": "cx show <entity>",
      "flags": ["--related", "--graph", "--hops", "--direction", "--coverage", "--include-metrics"]
    },
    "find": {
      "purpose": "Discover code by name, concept, or importance",
      "usage": "cx find <query>",
      "flags": ["--type", "--exact", "--keystones", "--important", "--tag", "--lang", "--limit"]
    },
    "rank": {
      "purpose": "Find critical code by importance",
      "usage": "cx rank --keystones",
      "flags": ["--keystones", "--bottlenecks", "--leaves", "--top"]
    },
    "trace": {
      "purpose": "Trace call paths between entities",
      "usage": "cx trace <from> <to>",
      "flags": ["--callers", "--callees", "--all", "--depth"]
    },
    "map": {
      "purpose": "Project skeleton (~10k tokens)",
      "usage": "cx map",
      "flags": ["--filter", "--lang", "--depth"]
    },
    "test": {
      "purpose": "Smart test selection and coverage",
      "usage": "cx test --diff --run",
      "flags": ["--diff", "--run", "--gaps", "--keystones-only", "--affected"],
      "subcommands": ["coverage import", "discover", "list", "impact", "suggest"]
    },
    "dead": {
      "purpose": "Find provably dead code",
      "usage": "cx dead",
      "flags": ["--include-exports", "--by-file", "--type"]
    },
    "tag": {
      "purpose": "Tag entities for organization",
      "usage": "cx tag <entity> <tags...>",
      "flags": ["--note", "--by"],
      "subcommands": ["list", "find", "remove", "export", "import"]
    },
    "scan": {
      "purpose": "Build/update code graph",
      "usage": "cx scan",
      "flags": ["--force", "--overview", "--lang", "--exclude"]
    },
    "db": {
      "purpose": "Database management",
      "subcommands": ["info", "doctor", "compact", "export"]
    },
    "doctor": {
      "purpose": "Health check",
      "flags": ["--fix", "--deep", "--yes"]
    },
    "reset": {
      "purpose": "Reset database",
      "flags": ["--scan-only", "--hard", "--force"]
    },
    "daemon": {
      "purpose": "Background daemon control",
      "subcommands": ["start", "status", "stop"]
    },
    "status": {
      "purpose": "Quick status check",
      "flags": ["--json", "--watch"]
    }
  },
  "global_flags": {
    "--format": "yaml|json|jsonl (default: yaml)",
    "--density": "sparse|medium|dense (default: medium)",
    "--quiet": "Exit code only",
    "--verbose": "Verbose output"
  },
  "density_tokens": {
    "sparse": "50-100 per entity",
    "medium": "200-300 per entity",
    "dense": "400-600 per entity"
  },
  "languages": ["go", "typescript", "javascript", "python", "java", "rust", "c", "csharp", "php"]
}
`
}
