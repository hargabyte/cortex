package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/mcp"
	"github.com/spf13/cobra"
)

// liveCmd represents the live command
var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Start MCP server for AI agent integration",
	Long: `Start an MCP (Model Context Protocol) server for AI agent integration.

This allows AI agents like Claude Code to query the code graph through MCP tools
instead of spawning CLI commands. Use this when doing heavy iterative work
where repeated CLI calls would be wasteful.

Philosophy: CLI for discovery, MCP for iteration.

Usage Pattern:
  Session start:        cx prime, cx map...        (fast CLI)
  Heavy iterative work: cx live --mcp              (start server)
  Work work work...     (tools stay loaded)
  Done:                 cx live --stop             (or auto-timeout)

Available Tools:
  cx_diff      Show changes since last scan
  cx_impact    Analyze blast radius of changes
  cx_context   Smart context assembly
  cx_show      Entity details
  cx_find      Search entities
  cx_gaps      Coverage gap analysis

Daemon Mode:
  Use --watch to enable filesystem watching. The server will automatically
  rescan changed files and keep the code graph up-to-date in real-time.

Examples:
  cx live --mcp                           # Start with default tools
  cx live --mcp --watch                   # Start with filesystem watching
  cx live --mcp --tools diff,impact       # Start with specific tools only
  cx live --mcp --timeout 30m             # Auto-stop after 30 minutes
  cx live --status                        # Check if server is running
  cx live --stop                          # Stop running server
  cx live --list-tools                    # Show available tools`,
	RunE: runLive,
}

// serveCmd represents the deprecated serve command (alias for live)
var serveCmd = &cobra.Command{
	Use:    "serve",
	Hidden: true,
	Short:  "Deprecated: Use 'cx live' instead",
	Long:   "Deprecated: 'cx serve' is an alias for 'cx live'. Please use 'cx live' instead.",
	RunE:   runLive,
}

var (
	liveMCP       bool
	liveTools     string
	liveTimeout   string
	liveStatus    bool
	liveStop      bool
	liveListTools bool
	liveWatch     bool
)

func init() {
	rootCmd.AddCommand(liveCmd)
	rootCmd.AddCommand(serveCmd)

	liveCmd.Flags().BoolVar(&liveMCP, "mcp", false, "Start MCP server (stdio transport)")
	liveCmd.Flags().StringVar(&liveTools, "tools", "", "Comma-separated list of tools to expose (default: diff,impact,context,show)")
	liveCmd.Flags().StringVar(&liveTimeout, "timeout", "30m", "Inactivity timeout (0 for no timeout)")
	liveCmd.Flags().BoolVar(&liveStatus, "status", false, "Check if server is running")
	liveCmd.Flags().BoolVar(&liveStop, "stop", false, "Stop running server")
	liveCmd.Flags().BoolVar(&liveListTools, "list-tools", false, "List available tools")
	liveCmd.Flags().BoolVar(&liveWatch, "watch", false, "Enable filesystem watching for auto-rescan (daemon mode)")

	// Share flags with serve alias
	serveCmd.Flags().BoolVar(&liveMCP, "mcp", false, "Start MCP server (stdio transport)")
	serveCmd.Flags().StringVar(&liveTools, "tools", "", "Comma-separated list of tools to expose (default: diff,impact,context,show)")
	serveCmd.Flags().StringVar(&liveTimeout, "timeout", "30m", "Inactivity timeout (0 for no timeout)")
	serveCmd.Flags().BoolVar(&liveStatus, "status", false, "Check if server is running")
	serveCmd.Flags().BoolVar(&liveStop, "stop", false, "Stop running server")
	serveCmd.Flags().BoolVar(&liveListTools, "list-tools", false, "List available tools")
	serveCmd.Flags().BoolVar(&liveWatch, "watch", false, "Enable filesystem watching for auto-rescan (daemon mode)")
}

func runLive(cmd *cobra.Command, args []string) error {
	// Handle --list-tools
	if liveListTools {
		fmt.Println("Available MCP tools:")
		fmt.Println()
		fmt.Println("  cx_diff      Show changes since last scan")
		fmt.Println("  cx_impact    Analyze blast radius of changes")
		fmt.Println("  cx_context   Smart context assembly")
		fmt.Println("  cx_show      Entity details")
		fmt.Println("  cx_find      Search entities")
		fmt.Println("  cx_gaps      Coverage gap analysis")
		fmt.Println()
		fmt.Println("Default set: diff, impact, context, show")
		return nil
	}

	// Handle --status
	if liveStatus {
		return checkServerStatus()
	}

	// Handle --stop
	if liveStop {
		return stopServer()
	}

	// Start MCP server
	if !liveMCP {
		return fmt.Errorf("use --mcp to start the MCP server, or --help for usage")
	}

	// Parse timeout
	timeout, err := parseDuration(liveTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Parse tools
	var tools []string
	if liveTools != "" {
		for _, t := range strings.Split(liveTools, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				// Allow shorthand (diff -> cx_diff)
				if !strings.HasPrefix(t, "cx_") {
					t = "cx_" + t
				}
				tools = append(tools, t)
			}
		}
	}

	// Create and start server
	cfg := mcp.Config{
		Tools:   tools,
		Timeout: timeout,
	}

	server, err := mcp.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}
	defer server.Close()

	// Write PID file
	if err := writePIDFile(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}
	defer removePIDFile()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\ncx live: shutting down\n")
		server.Close()
		removePIDFile()
		os.Exit(0)
	}()

	// Log startup info to stderr (stdout is for MCP protocol)
	fmt.Fprintf(os.Stderr, "cx live: starting MCP server\n")
	fmt.Fprintf(os.Stderr, "cx live: tools: %v\n", server.ListTools())
	if timeout > 0 {
		fmt.Fprintf(os.Stderr, "cx live: timeout: %v\n", timeout)
	}
	if liveWatch {
		fmt.Fprintf(os.Stderr, "cx live: watch mode enabled - will auto-rescan on file changes\n")
	}

	// Start serving
	return server.ServeStdio()
}

func parseDuration(s string) (time.Duration, error) {
	if s == "0" || s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

func getPIDFilePath() (string, error) {
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return "", err
	}
	return filepath.Join(cxDir, "serve.pid"), nil
}

func writePIDFile() error {
	pidPath, err := getPIDFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func removePIDFile() {
	pidPath, err := getPIDFilePath()
	if err != nil {
		return
	}
	os.Remove(pidPath)
}

func checkServerStatus() error {
	pidPath, err := getPIDFilePath()
	if err != nil {
		fmt.Println("Status: not running (cx not initialized)")
		return nil
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("Status: not running")
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("Status: not running (invalid PID file)")
		return nil
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Status: not running")
		removePIDFile()
		return nil
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		fmt.Println("Status: not running (stale PID file)")
		removePIDFile()
		return nil
	}

	fmt.Printf("Status: running (PID %d)\n", pid)
	return nil
}

func stopServer() error {
	pidPath, err := getPIDFilePath()
	if err != nil {
		return fmt.Errorf("cx not initialized")
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("No server running")
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		removePIDFile()
		return fmt.Errorf("invalid PID file")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		removePIDFile()
		fmt.Println("No server running")
		return nil
	}

	// Send SIGTERM for graceful shutdown
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		removePIDFile()
		fmt.Println("Server already stopped")
		return nil
	}

	fmt.Printf("Stopped server (PID %d)\n", pid)
	return nil
}
