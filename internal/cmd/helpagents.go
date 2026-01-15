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
  - show, find, check, context, test, map, scan, db, serve

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

## 9 Core Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| cx show | Understand code | --related, --graph, --coverage |
| cx find | Discover code | --important, --keystones, "multi word" |
| cx check | Pre-change safety | --quick, --coverage, --drift, --changes |
| cx context | Get context | --smart "<task>", --budget N |
| cx test | Smart testing | --diff, --gaps, --coverage, --run |
| cx map | Project overview | --filter F/T/M, --lang |
| cx scan | Build graph | --force, --overview |
| cx db | Maintenance | info, doctor, compact, export |
| cx serve | MCP server | (for IDE integration) |

## Essential Workflow

` + "```bash" + `
# Start of session
cx context                    # Session recovery

# Before ANY coding task
cx context --smart "<task>" --budget 8000

# Before modifying code
cx check <file>               # Full safety assessment
cx check <file> --quick       # Just blast radius

# New project setup
cx scan --overview
` + "```" + `

## Command Details

### cx show - Understand Code
` + "```bash" + `
cx show <entity>              # Entity details
cx show <entity> --related    # + neighborhood (was: cx near)
cx show <entity> --graph      # + dependency graph (was: cx graph)
cx show file.go:45            # Entity at line
` + "```" + `

### cx find - Discover Code
` + "```bash" + `
cx find AuthService           # Name search
cx find "auth validation"     # Concept search (was: cx search)
cx find --important --top 10  # Top by importance (was: cx rank)
cx find --keystones           # Critical entities
` + "```" + `

### cx check - Pre-Change Safety
` + "```bash" + `
cx check <file>               # Full assessment (impact + coverage + drift)
cx check <file> --quick       # Just blast radius (was: cx impact)
cx check --coverage           # Coverage gaps (was: cx gaps)
cx check --drift              # Staleness check (was: cx verify)
cx check --changes            # What changed (was: cx diff)
` + "```" + `

### cx context - Get Context
` + "```bash" + `
cx context                    # Session recovery (was: cx prime)
cx context --smart "<task>" --budget 8000  # Task-focused
cx context <entity> --hops 2  # Entity-focused
` + "```" + `

### cx test - Smart Testing
` + "```bash" + `
cx test                       # Tests for uncommitted changes
cx test --diff                # Same as above, explicit
cx test <file>                # Tests for specific file
cx test --run                 # Actually run the tests
cx test --gaps                # Coverage gaps (uses cx check --coverage)
cx test coverage import <file> # Import coverage data
` + "```" + `

## Output Control

` + "```bash" + `
--format yaml|json|jsonl      # Output format
--density sparse|medium|dense # Detail level (50-600 tokens/entity)
` + "```" + `

## Quick Patterns

` + "```bash" + `
# Understand codebase
cx map && cx find --keystones --top 10

# Before refactoring
cx check <file>

# Smart testing
cx test --diff --run

# Find critical untested code
cx check --coverage --keystones-only
` + "```" + `

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
    "check": {
      "purpose": "Pre-change safety",
      "usage": "cx check <file>",
      "flags": ["--quick", "--coverage", "--drift", "--changes"],
      "replaces": ["cx impact", "cx gaps", "cx verify", "cx diff"]
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
    "before_modify": "cx check <file>",
    "understand": "cx map && cx find --keystones --top 10",
    "smart_test": "cx test --diff --run",
    "find_untested": "cx check --coverage --keystones-only"
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
