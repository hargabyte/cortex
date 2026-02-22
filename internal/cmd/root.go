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
	// Version is the current version of cx, injected at build time via:
	// go build -ldflags="-X github.com/anthropics/cx/internal/cmd.Version=v1.0.0"
	Version = "dev"

	// Global flags
	verbose       bool
	quiet         bool
	configPath    string
	forAgents     bool
	outputFormat  string
	outputDensity string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cx [entity-or-file]",
	Short: "Context graph CLI for codebase analysis",
	Long: `cx is a codebase context tool that builds and queries a graph of code entities.

Five commands cover everything:

  cx [target]       Understand code (auto-detects entity/file/query)
  cx find <pattern> Search and discover entities
  cx check [file]   Quality gate (safety, guard, tests)
  cx scan           Build or rebuild the code graph
  cx call           Machine gateway (MCP tools)

The bare 'cx' command auto-detects what you need:
  cx                           Session recovery context
  cx LoginUser                 Show entity details
  cx src/auth/login.go         Safety check on file
  cx --smart "add auth"        Smart context for task
  cx --map                     Project skeleton
  cx --diff                    Context for uncommitted changes
  cx --trace LoginUser         Trace call chains
  cx --blame LoginUser         Entity commit history

Run 'cx admin' to see all administrative commands (db, tags, branches, etc.)

For advanced trace/blame flags, use the full command:
  cx trace Foo --callees --depth 3
  cx blame Foo --limit 50 --deps

Global Flags:
  --format    Output format: yaml (default) | json | cgf (deprecated)
  --density   Output detail level: sparse | medium (default) | dense | smart`,
	Version:      Version,
	SilenceUsage: true, // Don't dump usage text on errors — just print the error
	// Allow bare args for auto-detection (entity names, file paths)
	Args: cobra.ArbitraryArgs,
	RunE: runCx,
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
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress output (exit code only)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: .cx/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "yaml", "Output format (yaml|json|jsonl|cgf)")
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

	// Register bare-cx dispatcher flags (--map, --trace, --blame, --smart, etc.)
	registerCxFlags()

	// Hide old commands from help output.
	// Setting Hidden on the command structs works regardless of init() order
	// because Hidden is a property of the command, not its parent.
	hideOldCommands()
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
