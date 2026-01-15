// Package cmd contains all CLI commands for cx.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Version is the current version of cx
	Version = "0.1.0"

	// Global flags
	verbose      bool
	configPath   string
	forAgents    bool
	outputFormat string
	outputDensity string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cx",
	Short: "Context graph CLI for codebase analysis",
	Long: `cx is a codebase context tool that builds and queries a graph of code entities.

It helps developers and AI agents understand code structure, find relevant context,
and analyze dependencies across a codebase. cx scans source files, builds a graph
of symbols and their relationships, and provides commands to explore and query
this graph.

Output Format:
  All commands output YAML format by default with adjustable detail levels.
  Use --format flag to switch to JSON or deprecated CGF format.
  Use --density flag to control detail level (sparse|medium|dense|smart).

Main capabilities:
  - Scan codebases to build a context graph
  - Find symbols and their relationships
  - Show detailed information about code entities
  - Visualize dependency graphs in YAML format
  - Rank symbols by importance using PageRank
  - Analyze impact of changes
  - Verify graph consistency
  - Export context for AI consumption

Global Flags:
  --format    Output format: yaml (default) | json | cgf (deprecated)
  --density   Output detail level: sparse | medium (default) | dense | smart

Examples:
  cx find LoginUser                  # Find entities by name
  cx show sa-fn-a7f9b2-LoginUser     # Show entity details
  cx graph LoginUser                 # Visualize dependencies
  cx rank --keystones                # Find critical entities
  cx impact src/auth/               # Analyze change impact
  cx context bd-task-123             # Gather task context

See 'cx <command> --help' for command-specific options.`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: .cx/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "yaml", "Output format (yaml|json|cgf)")
	rootCmd.PersistentFlags().StringVar(&outputDensity, "density", "medium", "Output density (sparse|medium|dense|smart)")
	rootCmd.Flags().BoolVar(&forAgents, "for-agents", false, "Output machine-readable capability discovery JSON")

	// Set custom help function to intercept --for-agents flag
	originalHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if forAgents {
			outputAgentHelp(cmd)
			return
		}
		originalHelp(cmd, args)
	})
}

// CommandInfo represents a command for agent discovery
type CommandInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Usage       string        `json:"usage"`
	Flags       []FlagInfo    `json:"flags,omitempty"`
	Subcommands []CommandInfo `json:"subcommands,omitempty"`
	Examples    []string      `json:"examples,omitempty"`
}

// FlagInfo represents a command flag for agent discovery
type FlagInfo struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
}

// outputAgentHelp outputs machine-readable JSON describing all commands
func outputAgentHelp(cmd *cobra.Command) {
	root := buildCommandInfo(cmd.Root())

	output := map[string]interface{}{
		"version":      Version,
		"commands":     root.Subcommands,
		"global_flags": root.Flags,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

// buildCommandInfo recursively builds command information for agent discovery
func buildCommandInfo(cmd *cobra.Command) CommandInfo {
	info := CommandInfo{
		Name:        cmd.Name(),
		Description: cmd.Short,
		Usage:       cmd.UseLine(),
	}

	// Collect flags
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		info.Flags = append(info.Flags, FlagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Description: f.Usage,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
		})
	})

	// Collect subcommands
	for _, sub := range cmd.Commands() {
		if !sub.Hidden {
			info.Subcommands = append(info.Subcommands, buildCommandInfo(sub))
		}
	}

	// Extract examples from Example field if available
	if cmd.Example != "" {
		// Split by newline and filter empty lines
		lines := strings.Split(cmd.Example, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				info.Examples = append(info.Examples, trimmed)
			}
		}
	}

	return info
}
