package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anthropics/cx/internal/mcp"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for AI IDE integration",
	Long: `Start an MCP server exposing Cortex tools to AI IDEs.

Supports: Cursor, Windsurf, Google Antigravity, and any MCP-compatible client.

This is the recommended way to integrate Cortex with AI IDEs. The server
exposes code graph tools through the Model Context Protocol (MCP), allowing
AI assistants to query your codebase structure, analyze impact, and get
smart context without spawning separate CLI processes.

Available Tools:
  cx_context   Smart context assembly for task-focused context
  cx_safe      Pre-flight safety check before modifying code
  cx_find      Search for entities by name pattern
  cx_show      Show detailed information about an entity
  cx_map       Project skeleton overview
  cx_diff      Show changes since last scan
  cx_impact    Analyze blast radius of changes
  cx_gaps      Find coverage gaps in critical code

Examples:
  cx serve                        # Start MCP server with default tools
  cx serve --tools=context,safe   # Limit to specific tools
  cx serve --list-tools           # Show available tools and exit

IDE Setup:
  Cursor:     Settings > MCP > Add server: cx serve
  Windsurf:   ~/.windsurf/mcp.json: {"servers":{"cortex":{"command":"cx","args":["serve"]}}}
  Antigravity: MCP config: {"mcpServers":{"cortex":{"command":"cx","args":["serve"]}}}`,
	RunE: runServe,
}

var (
	serveTools     string
	serveListTools bool
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveTools, "tools", "", "Comma-separated list of tools to expose (default: all)")
	serveCmd.Flags().BoolVar(&serveListTools, "list-tools", false, "Show available tools and exit")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Handle --list-tools
	if serveListTools {
		fmt.Println("Available MCP tools:")
		fmt.Println()
		fmt.Println("  cx_context   Smart context assembly for task-focused context")
		fmt.Println("  cx_safe      Pre-flight safety check before modifying code")
		fmt.Println("  cx_find      Search for entities by name pattern")
		fmt.Println("  cx_show      Show detailed information about an entity")
		fmt.Println("  cx_map       Project skeleton overview")
		fmt.Println("  cx_diff      Show changes since last scan")
		fmt.Println("  cx_impact    Analyze blast radius of changes")
		fmt.Println("  cx_gaps      Find coverage gaps in critical code")
		fmt.Println()
		fmt.Println("Default tools:", strings.Join(mcp.DefaultTools, ", "))
		fmt.Println("All tools:    ", strings.Join(mcp.AllTools, ", "))
		return nil
	}

	// Parse tools list
	var tools []string
	if serveTools != "" {
		for _, t := range strings.Split(serveTools, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			// Normalize tool names (allow "context" or "cx_context")
			if !strings.HasPrefix(t, "cx_") {
				t = "cx_" + t
			}
			// Validate tool name
			valid := false
			for _, all := range mcp.AllTools {
				if all == t {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown tool: %s (available: %s)", t, strings.Join(mcp.AllTools, ", "))
			}
			tools = append(tools, t)
		}
	}

	// If no tools specified, use all tools
	if len(tools) == 0 {
		tools = mcp.AllTools
	}

	// Create MCP server
	cfg := mcp.Config{
		Tools:   tools,
		Timeout: 0, // No timeout for cx serve
	}

	server, err := mcp.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}
	defer server.Close()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\ncx serve: shutting down\n")
		server.Close()
		os.Exit(0)
	}()

	// Log startup info to stderr (stdout is for MCP protocol)
	fmt.Fprintf(os.Stderr, "cx serve: starting MCP server\n")
	fmt.Fprintf(os.Stderr, "cx serve: tools: %v\n", server.ListTools())

	// Start serving on stdio
	return server.ServeStdio()
}
