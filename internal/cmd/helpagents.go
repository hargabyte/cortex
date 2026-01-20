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
cx scan --tag v1.0                # Create tag after scan (usable with --at)
` + "```" + `

---

### Dolt Time Travel Commands
Query the codebase at any point in history.

` + "```bash" + `
cx show Entity --at HEAD~5        # Entity state 5 commits ago
cx show Entity --at v1.0          # Entity at tagged release
cx show Entity --history          # Entity's full change history
cx find Login --at HEAD~10        # Find entities at older state
cx history --stats                # Commit history with entity counts
cx diff --from HEAD~1             # What changed since last commit
cx blame Entity                   # Who changed this and when?
cx stale --scans 5                # Unchanged for 5+ scans
cx catchup --since v1.0           # Changes since tag/ref
cx safe <file> --trend            # Entity count trends over time
cx branch                         # List/create/delete branches
cx rollback HEAD~3 --hard         # Reset to previous state
cx sql "SELECT * FROM entities"   # Direct SQL access
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

### cx serve
MCP server for AI IDE integration.

` + "```bash" + `
cx serve                          # Start MCP server (stdio)
cx serve --list-tools             # Show available tools
cx serve --tools=context,safe     # Limit to specific tools
` + "```" + `

**Available MCP Tools:**
- ` + "`cx_context`" + ` - Smart context assembly
- ` + "`cx_safe`" + ` - Pre-flight safety check
- ` + "`cx_find`" + ` - Search entities
- ` + "`cx_show`" + ` - Entity details
- ` + "`cx_map`" + ` - Project skeleton
- ` + "`cx_diff`" + ` - Changes since scan
- ` + "`cx_impact`" + ` - Blast radius analysis
- ` + "`cx_gaps`" + ` - Coverage gaps

**IDE Setup:**
- Cursor: Settings → MCP → ` + "`{\"mcpServers\":{\"cortex\":{\"command\":\"cx\",\"args\":[\"serve\"]}}}`" + `
- Windsurf: ` + "`~/.windsurf/mcp.json`" + ` → ` + "`{\"servers\":{\"cortex\":{\"command\":\"cx\",\"args\":[\"serve\"]}}}`" + `

---

### cx report
Generate structured data for AI-powered codebase reports.

` + "```bash" + `
cx report overview --data                    # System stats, keystones, architecture diagram
cx report feature "authentication" --data    # Feature deep-dive with call flow diagram
cx report changes --since HEAD~50 --data     # What changed (Dolt time-travel)
cx report changes --since v1.0 --until v2.0 --data
cx report health --data                      # Risk score, complexity hotspots

# Output options
cx report overview --data -o overview.yaml   # Write to file
cx report overview --data --format json      # JSON output
` + "```" + `

**Report types:**
- ` + "`overview`" + ` - System statistics, keystones, modules, architecture diagram
- ` + "`feature <query>`" + ` - Semantic search deep-dive with call flow diagram
- ` + "`changes`" + ` - Added/modified/deleted entities with change diagram (requires ` + "`--since`" + `)
- ` + "`health`" + ` - Risk score, untested keystones, dead code, complexity hotspots

**Output includes D2 diagram code** for visualizations.

#### Report Skill Setup

For interactive report generation with user preferences, install the /report skill:

` + "```bash" + `
# Create the skill (first time only)
cx report --init-skill > ~/.claude/commands/report.md

# Then use /report in Claude Code for interactive reports
` + "```" + `

The /report skill provides:
- Interactive preference gathering (audience, format, focus areas)
- Consistent report structure and naming convention
- Multiple output formats (HTML, Markdown, terminal)
- Diagram rendering options

**Report Naming Convention:**
Reports save to ` + "`reports/`" + ` with pattern: ` + "`<type>_<YYYY-MM-DD>[_<query>].<ext>`" + `
- ` + "`overview_2026-01-20.html`" + `
- ` + "`feature_2026-01-20_authentication.html`" + `
- ` + "`health_2026-01-20.html`" + `

---

### cx render
Render D2 diagrams to images.

` + "```bash" + `
cx render diagram.d2                    # → diagram.svg
cx render diagram.d2 -f png -o out.png  # → PNG output
echo "a -> b" | cx render -             # → stdout SVG (stdin input)
cx render report.html --embed           # Inline SVGs in HTML (replaces D2 code blocks)
` + "```" + `

**Flags:**
- ` + "`-o, --output`" + ` - Output file path
- ` + "`-f, --format`" + ` - Output format: svg (default) or png
- ` + "`--embed`" + ` - Embed inline SVGs in HTML file (processes D2 code blocks)
- ` + "`--layout`" + ` - Layout engine: elk (default), dagre, tala

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
| Critical code | ` + "`cx find --keystones`" + ` |
| Call paths | ` + "`cx trace <from> <to>`" + ` |
| Dead code | ` + "`cx dead`" + ` |
| Run tests | ` + "`cx test --diff --run`" + ` |
| Bookmark code | ` + "`cx tag <entity> <tags>`" + ` |
| Entity history | ` + "`cx blame <entity>`" + ` or ` + "`cx show <entity> --history`" + ` |
| Compare states | ` + "`cx diff --from HEAD~1`" + ` |
| Old code state | ` + "`cx show <entity> --at v1.0`" + ` |
| What changed? | ` + "`cx catchup --since v1.0`" + ` |
| MCP IDE integration | ` + "`cx serve`" + ` |
| Generate report data | ` + "`cx report overview --data`" + ` |
| Render D2 to image | ` + "`cx render diagram.d2`" + ` |

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
      "flags": ["--quick", "--coverage", "--drift", "--changes", "--keystones-only", "--depth", "--trend"]
    },
    "show": {
      "purpose": "Understand a specific entity",
      "usage": "cx show <entity>",
      "flags": ["--related", "--graph", "--hops", "--direction", "--coverage", "--include-metrics", "--at", "--history"]
    },
    "find": {
      "purpose": "Discover code by name, concept, or importance",
      "usage": "cx find <query>",
      "flags": ["--type", "--exact", "--keystones", "--important", "--tag", "--lang", "--limit", "--at"]
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
      "flags": ["--force", "--overview", "--lang", "--exclude", "--tag", "--diff"]
    },
    "history": {
      "purpose": "Show Dolt commit history",
      "usage": "cx history",
      "flags": ["--limit", "--stats"]
    },
    "diff": {
      "purpose": "Show changes between commits/refs",
      "usage": "cx diff --from HEAD~1",
      "flags": ["--from", "--to", "--entities", "--summary"]
    },
    "blame": {
      "purpose": "Show entity change history",
      "usage": "cx blame <entity>",
      "flags": ["--limit"]
    },
    "stale": {
      "purpose": "Find entities unchanged for N scans",
      "usage": "cx stale --scans 5",
      "flags": ["--scans", "--since"]
    },
    "catchup": {
      "purpose": "Show changes since a ref",
      "usage": "cx catchup --since v1.0",
      "flags": ["--since", "--summary"]
    },
    "branch": {
      "purpose": "List/create/delete Dolt branches",
      "usage": "cx branch [name]",
      "flags": ["-c", "-d", "--from"]
    },
    "rollback": {
      "purpose": "Reset to previous state",
      "usage": "cx rollback [ref]",
      "flags": ["--hard", "--yes"]
    },
    "sql": {
      "purpose": "Execute SQL directly",
      "usage": "cx sql \"<query>\"",
      "flags": ["--format"]
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
    "serve": {
      "purpose": "MCP server for AI IDE integration",
      "usage": "cx serve",
      "flags": ["--tools", "--list-tools"],
      "mcp_tools": ["cx_context", "cx_safe", "cx_find", "cx_show", "cx_map", "cx_diff", "cx_impact", "cx_gaps"],
      "ide_setup": {
        "cursor": "Settings → MCP → mcpServers.cortex = {command: cx, args: [serve]}",
        "windsurf": "~/.windsurf/mcp.json → servers.cortex = {command: cx, args: [serve]}"
      }
    },
    "report": {
      "purpose": "Generate structured data for AI-powered reports",
      "usage": "cx report <type> --data",
      "subcommands": ["overview", "feature", "changes", "health"],
      "flags": ["--data", "-o/--output", "--format", "--since", "--until", "--init-skill"],
      "report_types": {
        "overview": "System stats, keystones, modules, architecture diagram",
        "feature": "Semantic search deep-dive with call flow diagram",
        "changes": "Added/modified/deleted entities with change diagram (requires --since)",
        "health": "Risk score, untested keystones, dead code, complexity hotspots"
      },
      "skill_setup": {
        "install": "cx report --init-skill > ~/.claude/commands/report.md",
        "features": ["Interactive preference gathering", "Consistent naming", "Multiple formats", "Diagram rendering"],
        "naming": "reports/<type>_<YYYY-MM-DD>[_<query>].<ext>"
      }
    },
    "render": {
      "purpose": "Render D2 diagrams to images",
      "usage": "cx render <file.d2>",
      "flags": ["-o/--output", "-f/--format", "--embed", "--layout"],
      "formats": ["svg", "png"],
      "layouts": ["elk", "dagre", "tala"],
      "examples": {
        "file_to_svg": "cx render diagram.d2",
        "file_to_png": "cx render diagram.d2 -f png -o out.png",
        "stdin_to_stdout": "echo \"a -> b\" | cx render -",
        "embed_in_html": "cx render report.html --embed"
      }
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
