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

The output is organized by workflow:
  - Essential: Commands to run every session
  - Discovery: Finding and understanding code
  - Analysis: Impact and importance assessment
  - Quality: Test coverage and verification
  - Maintenance: Database and scanning

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

## Essential Commands (Every Session)

` + "```bash" + `
# Start of session - context recovery
cx prime

# Before ANY coding task - get focused context
cx context --smart "<task description>" --budget 8000

# Before modifying code - check blast radius
cx impact <file>

# New project setup
cx quickstart
` + "```" + `

## Discovery Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| cx find <name> | Name search | --type=F/T/M, --exact |
| cx search "query" | Concept search | --top N, --lang go |
| cx show <name> | Entity details | --density dense, --coverage |
| cx near <name> | Neighborhood | --depth N, --direction in/out |
| cx map [path] | Project skeleton | --filter F/T/M, ~10k tokens |

## Analysis Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| cx rank | Top entities | --keystones, --bottlenecks, --top N |
| cx graph <name> | Dependencies | --hops N, --direction in/out |
| cx impact <file> | Blast radius | --depth N, --create-task |
| cx diff | Changes since scan | --file <path>, --detailed |

## Quality Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| cx coverage import <file> | Import coverage | coverage.out or GOCOVERDIR |
| cx gaps | Coverage gaps | --keystones-only, --create-tasks |
| cx test-impact | Smart tests | --diff, --output-command |
| cx verify | Check drift | --strict (CI), --fix |

## Context Assembly

` + "```bash" + `
# Task-focused context (most useful)
cx context --smart "add auth to API" --budget 8000

# Entity-focused context
cx context <entity> --hops 2 --include deps,callers

# Budget modes: importance (default) | distance
cx context --smart "task" --budget-mode distance
` + "```" + `

## Output Control

` + "```bash" + `
--format yaml|json|jsonl    # Output format
--density sparse|medium|dense|smart  # Detail level

# Token estimates per entity:
# sparse: 50-100 tokens
# medium: 200-300 tokens (default)
# dense:  400-600 tokens
` + "```" + `

## Quick Patterns

` + "```bash" + `
# Understand codebase
cx map && cx rank --keystones --top 10

# Before refactoring
cx impact <file> && cx gaps --keystones-only

# Smart testing
cx test-impact --diff --output-command | sh

# Find critical untested code
cx gaps --keystones-only --create-tasks
` + "```" + `

## Supported Languages
Go, TypeScript, JavaScript, Java, Rust, Python
`
}

func generateAgentReferenceJSON() string {
	return `{
  "version": "` + Version + `",
  "essential": {
    "prime": "cx prime - Context recovery at session start",
    "context": "cx context --smart \"<task>\" --budget 8000 - Focused context",
    "impact": "cx impact <file> - Blast radius before changes",
    "quickstart": "cx quickstart - Full project setup"
  },
  "discovery": {
    "find": {"usage": "cx find <name>", "flags": ["--type=F/T/M", "--exact"]},
    "search": {"usage": "cx search \"query\"", "flags": ["--top N", "--lang"]},
    "show": {"usage": "cx show <name>", "flags": ["--density", "--coverage"]},
    "near": {"usage": "cx near <name>", "flags": ["--depth", "--direction"]},
    "map": {"usage": "cx map [path]", "flags": ["--filter F/T/M"], "note": "~10k tokens"}
  },
  "analysis": {
    "rank": {"usage": "cx rank", "flags": ["--keystones", "--bottlenecks", "--top N"]},
    "graph": {"usage": "cx graph <name>", "flags": ["--hops", "--direction"]},
    "impact": {"usage": "cx impact <file>", "flags": ["--depth", "--create-task"]},
    "diff": {"usage": "cx diff", "flags": ["--file", "--detailed"]}
  },
  "quality": {
    "coverage_import": {"usage": "cx coverage import <file>"},
    "gaps": {"usage": "cx gaps", "flags": ["--keystones-only", "--create-tasks"]},
    "test_impact": {"usage": "cx test-impact", "flags": ["--diff", "--output-command"]},
    "verify": {"usage": "cx verify", "flags": ["--strict", "--fix"]}
  },
  "patterns": {
    "understand": "cx map && cx rank --keystones --top 10",
    "before_refactor": "cx impact <file> && cx gaps --keystones-only",
    "smart_test": "cx test-impact --diff --output-command",
    "find_untested": "cx gaps --keystones-only --create-tasks"
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
