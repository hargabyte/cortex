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

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for AI agent integration",
	Long: `Start an MCP (Model Context Protocol) server for AI agent integration.

This allows AI agents like Claude Code to query the code graph through MCP tools
instead of spawning CLI commands. Use this when doing heavy iterative work
where repeated CLI calls would be wasteful.

Philosophy: CLI for discovery, MCP for iteration.

Usage Pattern:
  Session start:        cx prime, cx map...        (fast CLI)
  Heavy iterative work: cx serve --mcp             (start server)
  Work work work...     (tools stay loaded)
  Done:                 cx serve --stop            (or auto-timeout)

Available Tools:
  cx_diff      Show changes since last scan
  cx_impact    Analyze blast radius of changes
  cx_context   Smart context assembly
  cx_show      Entity details
  cx_find      Search entities
  cx_gaps      Coverage gap analysis

Examples:
  cx serve --mcp                           # Start with default tools
  cx serve --mcp --tools diff,impact       # Start with specific tools only
  cx serve --mcp --timeout 30m             # Auto-stop after 30 minutes
  cx serve --status                        # Check if server is running
  cx serve --stop                          # Stop running server
  cx serve --list-tools                    # Show available tools`,
	RunE: runServe,
}

var (
	serveMCP       bool
	serveTools     string
	serveTimeout   string
	serveStatus    bool
	serveStop      bool
	serveListTools bool
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().BoolVar(&serveMCP, "mcp", false, "Start MCP server (stdio transport)")
	serveCmd.Flags().StringVar(&serveTools, "tools", "", "Comma-separated list of tools to expose (default: diff,impact,context,show)")
	serveCmd.Flags().StringVar(&serveTimeout, "timeout", "30m", "Inactivity timeout (0 for no timeout)")
	serveCmd.Flags().BoolVar(&serveStatus, "status", false, "Check if server is running")
	serveCmd.Flags().BoolVar(&serveStop, "stop", false, "Stop running server")
	serveCmd.Flags().BoolVar(&serveListTools, "list-tools", false, "List available tools")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Handle --list-tools
	if serveListTools {
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
	if serveStatus {
		return checkServerStatus()
	}

	// Handle --stop
	if serveStop {
		return stopServer()
	}

	// Start MCP server
	if !serveMCP {
		return fmt.Errorf("use --mcp to start the MCP server, or --help for usage")
	}

	// Parse timeout
	timeout, err := parseDuration(serveTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Parse tools
	var tools []string
	if serveTools != "" {
		for _, t := range strings.Split(serveTools, ",") {
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
		fmt.Fprintf(os.Stderr, "\ncx serve: shutting down\n")
		server.Close()
		removePIDFile()
		os.Exit(0)
	}()

	// Log startup info to stderr (stdout is for MCP protocol)
	fmt.Fprintf(os.Stderr, "cx serve: starting MCP server\n")
	fmt.Fprintf(os.Stderr, "cx serve: tools: %v\n", server.ListTools())
	if timeout > 0 {
		fmt.Fprintf(os.Stderr, "cx serve: timeout: %v\n", timeout)
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
