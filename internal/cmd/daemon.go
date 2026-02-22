// Package cmd contains all CLI commands for cx.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/daemon"
	"github.com/anthropics/cx/internal/mcp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// daemonControlCmd represents the daemon command group
var daemonControlCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Control the CX daemon and MCP server",
	Long: `Control the CX background daemon for live code graph updates and MCP server.

The daemon runs in the background and keeps the code graph up-to-date.
Commands automatically start the daemon when needed.

Subcommands:
  status   Show daemon status
  start    Start the daemon
  stop     Stop the running daemon
  mcp      Start MCP server for AI agent integration

Examples:
  cx daemon status              # Show daemon status
  cx daemon stop                # Stop the running daemon
  cx daemon start --background  # Start daemon in background
  cx daemon mcp                 # Start MCP server`,
}

// daemonControlStatusCmd shows daemon status (subcommand of daemon)
var daemonControlStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long: `Show the current status of the CX daemon.

Displays:
  - Whether the daemon is running
  - Process ID (PID)
  - Uptime and idle time
  - Time until auto-shutdown
  - Entity count and graph freshness

Examples:
  cx daemon status              # Show status (default YAML format)
  cx daemon status --format json # Show status as JSON`,
	RunE: runDaemonControlStatus,
}

// daemonControlStopCmd stops the daemon
var daemonControlStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	Long: `Stop the running CX daemon gracefully.

If no daemon is running, this command does nothing.

Examples:
  cx daemon stop`,
	RunE: runDaemonControlStop,
}

// daemonControlStartCmd starts the daemon
var daemonControlStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	Long: `Start the CX daemon.

By default, the daemon runs in the foreground. Use --background to
detach and run as a background process.

The daemon automatically shuts down after the idle timeout if no
queries are received.

Examples:
  cx daemon start                        # Start in foreground
  cx daemon start --background           # Start in background
  cx daemon start --idle-timeout 1h      # Custom idle timeout`,
	RunE: runDaemonControlStart,
}

// daemonMCPCmd starts the MCP server
var daemonMCPCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI agent integration",
	Long: `Start an MCP (Model Context Protocol) server for AI agent integration.

This allows AI agents like Claude Code to query the code graph through MCP tools
instead of spawning CLI commands. Use this when doing heavy iterative work
where repeated CLI calls would be wasteful.

Philosophy: CLI for discovery, MCP for iteration.

Usage Pattern:
  Session start:        cx prime, cx map...        (fast CLI)
  Heavy iterative work: cx daemon mcp              (start server)
  Work work work...     (tools stay loaded)
  Done:                 cx daemon mcp --stop       (or auto-timeout)

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
  cx daemon mcp                           # Start with default tools
  cx daemon mcp --watch                   # Start with filesystem watching
  cx daemon mcp --tools diff,impact       # Start with specific tools only
  cx daemon mcp --timeout 30m             # Auto-stop after 30 minutes
  cx daemon mcp --status                  # Check if server is running
  cx daemon mcp --stop                    # Stop running server
  cx daemon mcp --list-tools              # Show available tools`,
	RunE: runDaemonMCP,
}

// Flags for daemon start
var (
	daemonStartBackground      bool
	daemonStartForegroundChild bool // Internal flag for spawned child process
	daemonStartIdleTimeout     string
	daemonStartProject         string
	daemonStartCXDir           string
)

// Flags for daemon mcp
var (
	daemonMCPTools     string
	daemonMCPTimeout   string
	daemonMCPStatus    bool
	daemonMCPStop      bool
	daemonMCPListTools bool
	daemonMCPWatch     bool
)

func init() {
	rootCmd.AddCommand(daemonControlCmd)
	// Daemon is safe to use — spawn storm guard (CX_DAEMON_CHILD) prevents recursive spawning
	daemonControlCmd.AddCommand(daemonControlStatusCmd)
	daemonControlCmd.AddCommand(daemonControlStopCmd)
	daemonControlCmd.AddCommand(daemonControlStartCmd)
	daemonControlCmd.AddCommand(daemonMCPCmd)

	// Start command flags
	daemonControlStartCmd.Flags().BoolVar(&daemonStartBackground, "background", false, "Run daemon in background")
	daemonControlStartCmd.Flags().BoolVar(&daemonStartForegroundChild, "foreground-child", false, "Internal: run as foreground child of background spawn")
	daemonControlStartCmd.Flags().MarkHidden("foreground-child") // Hide from help - internal use only
	daemonControlStartCmd.Flags().StringVar(&daemonStartIdleTimeout, "idle-timeout", "30m", "Idle timeout before auto-shutdown")
	daemonControlStartCmd.Flags().StringVar(&daemonStartProject, "project", "", "Project root path")
	daemonControlStartCmd.Flags().StringVar(&daemonStartCXDir, "cx-dir", "", "CX directory path (.cx)")

	// MCP command flags
	daemonMCPCmd.Flags().StringVar(&daemonMCPTools, "tools", "", "Comma-separated list of tools to expose (default: diff,impact,context,show)")
	daemonMCPCmd.Flags().StringVar(&daemonMCPTimeout, "timeout", "30m", "Inactivity timeout (0 for no timeout)")
	daemonMCPCmd.Flags().BoolVar(&daemonMCPStatus, "status", false, "Check if MCP server is running")
	daemonMCPCmd.Flags().BoolVar(&daemonMCPStop, "stop", false, "Stop running MCP server")
	daemonMCPCmd.Flags().BoolVar(&daemonMCPListTools, "list-tools", false, "List available MCP tools")
	daemonMCPCmd.Flags().BoolVar(&daemonMCPWatch, "watch", false, "Enable filesystem watching for auto-rescan")
}

// runDaemonControlStatus shows the daemon status
func runDaemonControlStatus(cmd *cobra.Command, args []string) error {
	socketPath := daemon.DefaultSocketPath()

	status, err := daemon.GetDaemonStatus(socketPath)
	if err != nil {
		return fmt.Errorf("get daemon status: %w", err)
	}

	// Build output structure
	output := buildStatusOutput(status)

	// Format output based on global format flag
	switch outputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	case "jsonl":
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(output)
	default: // yaml
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(output)
	}
}

// DaemonStatusOutput represents the YAML/JSON output for daemon status
type DaemonStatusOutput struct {
	Running           bool   `yaml:"running" json:"running"`
	PID               int    `yaml:"pid,omitempty" json:"pid,omitempty"`
	SocketPath        string `yaml:"socket_path" json:"socket_path"`
	ProjectRoot       string `yaml:"project_root,omitempty" json:"project_root,omitempty"`
	Uptime            string `yaml:"uptime,omitempty" json:"uptime,omitempty"`
	IdleTime          string `yaml:"idle_time,omitempty" json:"idle_time,omitempty"`
	IdleTimeout       string `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	TimeUntilShutdown string `yaml:"time_until_shutdown,omitempty" json:"time_until_shutdown,omitempty"`
	EntityCount       int    `yaml:"entity_count,omitempty" json:"entity_count,omitempty"`
	GraphFresh        bool   `yaml:"graph_fresh,omitempty" json:"graph_fresh,omitempty"`
	StaleFiles        int    `yaml:"stale_files,omitempty" json:"stale_files,omitempty"`
}

// buildStatusOutput builds the output structure from daemon status
func buildStatusOutput(status *daemon.Status) DaemonStatusOutput {
	if status == nil {
		return DaemonStatusOutput{
			Running:    false,
			SocketPath: daemon.DefaultSocketPath(),
		}
	}

	output := DaemonStatusOutput{
		Running:     status.Running,
		PID:         status.PID,
		SocketPath:  status.SocketPath,
		ProjectRoot: status.ProjectRoot,
		EntityCount: status.EntityCount,
		GraphFresh:  status.GraphFresh,
		StaleFiles:  status.StaleFiles,
	}

	if status.Running {
		output.Uptime = formatDuration(status.Uptime)
		output.IdleTime = formatDuration(status.IdleTime)
		if status.IdleTimeout > 0 {
			output.IdleTimeout = formatDuration(status.IdleTimeout)
		}
		if status.TimeUntilShutdown > 0 {
			output.TimeUntilShutdown = formatDuration(status.TimeUntilShutdown)
		}
	}

	return output
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}

	// Round to seconds for cleaner output
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%ds", mins, secs)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// runDaemonControlStop stops the running daemon
func runDaemonControlStop(cmd *cobra.Command, args []string) error {
	socketPath := daemon.DefaultSocketPath()

	// Check if daemon is running
	if !daemon.IsDaemonRunning(socketPath) {
		fmt.Println("Daemon is not running")
		return nil
	}

	// Stop the daemon
	if err := daemon.StopDaemon(socketPath); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}

	fmt.Println("Daemon stopped")
	return nil
}

// runDaemonControlStart starts the daemon
func runDaemonControlStart(cmd *cobra.Command, args []string) error {
	// Parse idle timeout
	idleTimeout, err := time.ParseDuration(daemonStartIdleTimeout)
	if err != nil {
		return fmt.Errorf("invalid idle timeout: %w", err)
	}

	// Build config
	cfg := daemon.Config{
		IdleTimeout: idleTimeout,
		ProjectRoot: daemonStartProject,
		CXDir:       daemonStartCXDir,
		Verbose:     verbose,
	}

	// Case 1: --foreground-child - we're the spawned child process, run in foreground
	// This is used internally by EnsureDaemon to start the actual daemon process
	if daemonStartForegroundChild {
		d, err := daemon.New(cfg)
		if err != nil {
			return fmt.Errorf("create daemon: %w", err)
		}
		// Run blocks until shutdown (no message since we're a background child)
		return d.Run()
	}

	// Case 2: --background - spawn a detached child process and exit
	if daemonStartBackground {
		// Start daemon in background using EnsureDaemon
		opts := daemon.EnsureDaemonOptions{
			IdleTimeout:  idleTimeout,
			ProjectRoot:  daemonStartProject,
			CXDir:        daemonStartCXDir,
			WithFallback: false, // We want errors if it fails
		}

		result, err := daemon.EnsureDaemon(opts)
		if err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}

		if result.WasStarted {
			fmt.Printf("Daemon started (PID %d)\n", result.PID)
		} else {
			fmt.Printf("Daemon already running (PID %d)\n", result.PID)
		}
		return nil
	}

	// Case 3: No flags - foreground mode for interactive use
	d, err := daemon.New(cfg)
	if err != nil {
		return fmt.Errorf("create daemon: %w", err)
	}

	// Run blocks until shutdown
	fmt.Fprintln(os.Stderr, "Starting daemon in foreground (Ctrl+C to stop)...")
	return d.Run()
}

// runDaemonMCP runs the MCP server
func runDaemonMCP(cmd *cobra.Command, args []string) error {
	// Deprecation warning
	fmt.Fprintln(os.Stderr, "DEPRECATED: 'cx daemon mcp' is deprecated. Use 'cx serve' instead.")
	fmt.Fprintln(os.Stderr, "The new 'cx serve' command is simpler and has more tools available.")
	fmt.Fprintln(os.Stderr, "This command will be removed in a future version.")
	fmt.Fprintln(os.Stderr, "")

	// Handle --list-tools
	if daemonMCPListTools {
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
	if daemonMCPStatus {
		return checkMCPServerStatus()
	}

	// Handle --stop
	if daemonMCPStop {
		return stopMCPServer()
	}

	// Parse timeout
	timeout, err := parseMCPDuration(daemonMCPTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Parse tools
	var tools []string
	if daemonMCPTools != "" {
		for _, t := range strings.Split(daemonMCPTools, ",") {
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
	if err := writeMCPPIDFile(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}
	defer removeMCPPIDFile()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\ncx daemon mcp: shutting down\n")
		server.Close()
		removeMCPPIDFile()
		os.Exit(0)
	}()

	// Log startup info to stderr (stdout is for MCP protocol)
	fmt.Fprintf(os.Stderr, "cx daemon mcp: starting MCP server\n")
	fmt.Fprintf(os.Stderr, "cx daemon mcp: tools: %v\n", server.ListTools())
	if timeout > 0 {
		fmt.Fprintf(os.Stderr, "cx daemon mcp: timeout: %v\n", timeout)
	}
	if daemonMCPWatch {
		fmt.Fprintf(os.Stderr, "cx daemon mcp: watch mode enabled - will auto-rescan on file changes\n")

		// Start file watcher in background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			if err := watchForChanges(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "cx daemon mcp: watcher error: %v\n", err)
			}
		}()

		// Cancel watcher on shutdown
		go func() {
			<-sigChan
			cancel()
		}()
	}

	// Start serving
	return server.ServeStdio()
}

func parseMCPDuration(s string) (time.Duration, error) {
	if s == "0" || s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

func getMCPPIDFilePath() (string, error) {
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return "", err
	}
	return filepath.Join(cxDir, "mcp.pid"), nil
}

func writeMCPPIDFile() error {
	pidPath, err := getMCPPIDFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func removeMCPPIDFile() {
	pidPath, err := getMCPPIDFilePath()
	if err != nil {
		return
	}
	os.Remove(pidPath)
}

func checkMCPServerStatus() error {
	pidPath, err := getMCPPIDFilePath()
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
		removeMCPPIDFile()
		return nil
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		fmt.Println("Status: not running (stale PID file)")
		removeMCPPIDFile()
		return nil
	}

	fmt.Printf("Status: running (PID %d)\n", pid)
	return nil
}

func stopMCPServer() error {
	pidPath, err := getMCPPIDFilePath()
	if err != nil {
		return fmt.Errorf("cx not initialized")
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("No MCP server running")
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		removeMCPPIDFile()
		return fmt.Errorf("invalid PID file")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		removeMCPPIDFile()
		fmt.Println("No MCP server running")
		return nil
	}

	// Send SIGTERM for graceful shutdown
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		removeMCPPIDFile()
		fmt.Println("MCP server already stopped")
		return nil
	}

	fmt.Printf("Stopped MCP server (PID %d)\n", pid)
	return nil
}

// watchForChanges watches for file changes and triggers rescans.
// Uses polling-based approach to detect changes without external dependencies.
func watchForChanges(ctx context.Context) error {
	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Track last modification times
	fileModTimes := make(map[string]time.Time)

	// Initial scan to populate file mod times
	scanFiles := func() (map[string]time.Time, error) {
		modTimes := make(map[string]time.Time)
		err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Skip directories
			if info.IsDir() {
				base := filepath.Base(path)
				// Skip hidden directories, vendor, node_modules
				if strings.HasPrefix(base, ".") && base != "." {
					return filepath.SkipDir
				}
				if base == "vendor" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			// Only track source files
			if isWatchableSourceFile(path) {
				modTimes[path] = info.ModTime()
			}

			return nil
		})
		return modTimes, err
	}

	// Initial scan
	fileModTimes, err = scanFiles()
	if err != nil {
		return fmt.Errorf("initial file scan: %w", err)
	}

	fmt.Fprintf(os.Stderr, "cx daemon mcp: watching %d source files for changes (polling every 2s)\n", len(fileModTimes))

	// Poll for changes every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Debounce: wait 500ms after last change before triggering rescan
	var debounceTimer *time.Timer
	var pendingRescan bool

	triggerRescan := func() {
		fmt.Fprintf(os.Stderr, "cx daemon mcp: file changes detected, rescanning...\n")

		// Run cx scan
		if err := triggerScanCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "cx daemon mcp: rescan failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "cx daemon mcp: rescan complete\n")
		}
	}

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case <-ticker.C:
			// Check for file changes
			currentModTimes, err := scanFiles()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cx daemon mcp: file scan error: %v\n", err)
				continue
			}

			// Detect changes
			hasChanges := false
			var changedFiles []string

			// Check for modified or new files
			for path, modTime := range currentModTimes {
				if lastModTime, exists := fileModTimes[path]; !exists {
					hasChanges = true
					changedFiles = append(changedFiles, filepath.Base(path))
				} else if modTime.After(lastModTime) {
					hasChanges = true
					changedFiles = append(changedFiles, filepath.Base(path))
				}
			}

			// Check for removed files
			if !hasChanges {
				for path := range fileModTimes {
					if _, exists := currentModTimes[path]; !exists {
						hasChanges = true
						changedFiles = append(changedFiles, filepath.Base(path))
						break
					}
				}
			}

			if hasChanges {
				fmt.Fprintf(os.Stderr, "cx daemon mcp: detected changes in %d file(s): %v\n", len(changedFiles), changedFiles)

				// Update our tracking
				fileModTimes = currentModTimes

				// Set pending rescan flag
				pendingRescan = true

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
					if pendingRescan {
						triggerRescan()
						pendingRescan = false
					}
				})
			}
		}
	}
}

// isWatchableSourceFile checks if a file is a source code file based on extension.
func isWatchableSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".java", ".rs", ".py", ".c", ".h", ".cpp", ".cc", ".cxx",
		".hpp", ".hh", ".hxx", ".cs", ".php", ".kt", ".kts", ".rb", ".rake":
		return true
	default:
		return false
	}
}

// triggerScanCommand triggers a cx scan command.
func triggerScanCommand() error {
	// Use the scan command from this package
	return runScan(scanCmd, []string{})
}
