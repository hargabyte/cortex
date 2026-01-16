// Package daemon client.go provides the EnsureDaemon function and related utilities
// for automatically starting and connecting to the CX daemon.
//
// The daemon is an implementation detail - users never think about it.
// Commands automatically connect to an existing daemon or start a new one.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// EnsureDaemonOptions configures the behavior of EnsureDaemon.
type EnsureDaemonOptions struct {
	// SocketPath is the Unix socket path for the daemon.
	// Defaults to ~/.cx/daemon.sock
	SocketPath string

	// PIDPath is the path to the daemon's PID file.
	// Defaults to ~/.cx/daemon.pid
	PIDPath string

	// IdleTimeout is the idle timeout for the daemon.
	// Passed to the daemon when starting it.
	// Defaults to 30 minutes.
	IdleTimeout time.Duration

	// StartTimeout is how long to wait for the daemon to start.
	// Defaults to 5 seconds.
	StartTimeout time.Duration

	// ProjectRoot is the project root for the daemon.
	// If empty, uses current working directory.
	ProjectRoot string

	// CXDir is the .cx directory path.
	// If empty, uses ProjectRoot/.cx
	CXDir string

	// Verbose enables verbose logging when starting the daemon.
	Verbose bool

	// WithFallback enables fallback to direct DB access if daemon fails.
	// When true, EnsureDaemon returns (nil, nil) instead of an error,
	// signaling the caller should use direct store access.
	WithFallback bool
}

// DefaultEnsureDaemonOptions returns options with default values.
func DefaultEnsureDaemonOptions() EnsureDaemonOptions {
	return EnsureDaemonOptions{
		SocketPath:   DefaultSocketPath(),
		PIDPath:      DefaultPIDPath(),
		IdleTimeout:  DefaultIdleTimeout,
		StartTimeout: 5 * time.Second,
		WithFallback: true, // Default to graceful degradation
	}
}

// EnsureDaemonResult contains the result of EnsureDaemon.
type EnsureDaemonResult struct {
	// Client is the connected daemon client.
	// nil if using fallback mode.
	Client *Client

	// WasStarted is true if a new daemon was started.
	WasStarted bool

	// UsingFallback is true if daemon connection failed and
	// the caller should use direct DB access instead.
	UsingFallback bool

	// PID is the daemon's process ID (0 if using fallback).
	PID int
}

// EnsureDaemon ensures a daemon is running and returns a connected client.
// If no daemon is running, it starts one in the background and waits for it to be ready.
//
// DISABLED: Auto-spawning is disabled due to spawn storm bug. Always returns fallback.
// See: cortex-6uc
//
// If WithFallback is true and the daemon cannot be started/connected,
// it returns a result with UsingFallback=true instead of an error.
// The caller should then use direct store access.
//
// Usage:
//
//	result, err := daemon.EnsureDaemon(daemon.DefaultEnsureDaemonOptions())
//	if err != nil {
//	    return err
//	}
//	if result.UsingFallback {
//	    // Use direct store access
//	    store, _ := store.Open(cxDir)
//	    ...
//	} else {
//	    // Use daemon client
//	    resp, _ := result.Client.Query(...)
//	}
func EnsureDaemon(opts EnsureDaemonOptions) (*EnsureDaemonResult, error) {
	// DISABLED: Daemon auto-spawn causes spawn storms. Always use fallback.
	// When re-enabling, restore the original implementation from git history.
	// See: cortex-6uc
	if opts.WithFallback {
		return &EnsureDaemonResult{UsingFallback: true}, nil
	}
	return nil, fmt.Errorf("daemon auto-spawn disabled (spawn storm bug)")
}

// startDaemonProcess starts the daemon in the background.
// It spawns a child process with --foreground-child flag which runs the daemon
// in foreground mode within the detached child process.
func startDaemonProcess(opts EnsureDaemonOptions) error {
	// Find the cx executable
	cxPath, err := os.Executable()
	if err != nil {
		// Fallback to looking in PATH
		cxPath, err = exec.LookPath("cx")
		if err != nil {
			return fmt.Errorf("find cx executable: %w", err)
		}
	}

	// Build command arguments
	// Use --foreground-child instead of --background to avoid recursion
	// The child process will run the daemon in foreground mode, but since
	// it's detached from the parent, it effectively runs in the background.
	args := []string{
		"daemon", "start",
		"--foreground-child",
		"--idle-timeout", opts.IdleTimeout.String(),
	}

	if opts.ProjectRoot != "" {
		args = append(args, "--project", opts.ProjectRoot)
	}

	if opts.CXDir != "" {
		args = append(args, "--cx-dir", opts.CXDir)
	}

	// Start the daemon process
	cmd := exec.Command(cxPath, args...)

	// Detach from parent process
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Set process group for proper detachment on Unix
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
	}

	// Detach - don't wait for process
	if err := cmd.Process.Release(); err != nil {
		// Non-fatal, process is already running
	}

	return nil
}

// ConnectToDaemon attempts to connect to an existing daemon.
// Returns nil, error if daemon is not running or not responding.
func ConnectToDaemon(socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}

	client := NewClient(socketPath)

	// Verify daemon is responding
	resp, err := client.Health()
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("daemon not healthy: %s", resp.Error)
	}

	return client, nil
}

// IsDaemonRunning checks if a daemon is currently running and responding.
func IsDaemonRunning(socketPath string) bool {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}

	client := NewClient(socketPath)
	resp, err := client.Health()
	return err == nil && resp.Success
}

// GetDaemonStatus returns the current daemon status.
// Returns nil if daemon is not running.
func GetDaemonStatus(socketPath string) (*Status, error) {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}

	client := NewClient(socketPath)
	resp, err := client.Status()
	if err != nil {
		// Check if it's just not running
		if !client.IsConnectable() {
			return nil, nil // Not running is not an error
		}
		return nil, fmt.Errorf("get daemon status: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("daemon status error: %s", resp.Error)
	}

	// Parse status from response data
	status := &Status{
		Running:    true,
		SocketPath: socketPath,
	}

	if data := resp.Data; data != nil {
		if pid, ok := data["pid"].(float64); ok {
			status.PID = int(pid)
		}
		if uptime, ok := data["uptime"].(string); ok {
			if d, err := time.ParseDuration(uptime); err == nil {
				status.Uptime = d
			}
		}
		if idleTime, ok := data["idle_time"].(string); ok {
			if d, err := time.ParseDuration(idleTime); err == nil {
				status.IdleTime = d
			}
		}
		if idleTimeout, ok := data["idle_timeout"].(float64); ok {
			status.IdleTimeout = time.Duration(idleTimeout)
		}
		if timeUntil, ok := data["time_until_shutdown"].(float64); ok {
			status.TimeUntilShutdown = time.Duration(timeUntil)
		}
		if projectRoot, ok := data["project_root"].(string); ok {
			status.ProjectRoot = projectRoot
		}
		if entityCount, ok := data["entity_count"].(float64); ok {
			status.EntityCount = int(entityCount)
		}
		if graphFresh, ok := data["graph_fresh"].(bool); ok {
			status.GraphFresh = graphFresh
		}
		if staleFiles, ok := data["stale_files"].(float64); ok {
			status.StaleFiles = int(staleFiles)
		}
	}

	return status, nil
}

// StopDaemon sends a stop request to the running daemon.
func StopDaemon(socketPath string) error {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}

	client := NewClient(socketPath)
	resp, err := client.Stop()
	if err != nil {
		// Check if daemon is just not running
		if !client.IsConnectable() {
			return nil // Already stopped
		}
		return fmt.Errorf("stop daemon: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("daemon stop error: %s", resp.Error)
	}

	return nil
}
