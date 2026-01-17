package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/daemon"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// daemonStatusCmd represents the cx status command (daemon/graph status)
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon and graph status",
	Long: `Show the current status of the CX daemon and code graph.

Displays information about:
- Daemon status (running, idle time, auto-shutdown timer)
- Graph freshness (stale files, last scan time)
- Entity count and database info

The daemon runs automatically in the background to keep the graph fresh.
Users don't need to manage it manually - it starts on first cx command
and shuts down after 30 minutes of inactivity.

Examples:
  cx status                # Show status
  cx status --json         # JSON output for scripts
  cx status --watch        # Continuously update status`,
	RunE: runDaemonStatus,
}

var (
	statusJSON  bool
	statusWatch bool
)

func init() {
	rootCmd.AddCommand(daemonStatusCmd)
	daemonStatusCmd.Hidden = true // Daemon is currently broken, hide status command

	daemonStatusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
	daemonStatusCmd.Flags().BoolVar(&statusWatch, "watch", false, "Continuously update status")
}

// StatusOutput represents the status output structure
type StatusOutput struct {
	// Daemon information
	Daemon DaemonStatus `json:"daemon" yaml:"daemon"`

	// Graph information
	Graph GraphStatus `json:"graph" yaml:"graph"`

	// Database information
	Database DatabaseStatus `json:"database" yaml:"database"`
}

// DaemonStatus represents daemon-specific status
type DaemonStatus struct {
	Running           bool   `json:"running" yaml:"running"`
	Connected         bool   `json:"connected" yaml:"connected"`
	PID               int    `json:"pid,omitempty" yaml:"pid,omitempty"`
	Uptime            string `json:"uptime,omitempty" yaml:"uptime,omitempty"`
	IdleTime          string `json:"idle_time,omitempty" yaml:"idle_time,omitempty"`
	IdleTimeout       string `json:"idle_timeout,omitempty" yaml:"idle_timeout,omitempty"`
	TimeUntilShutdown string `json:"time_until_shutdown,omitempty" yaml:"time_until_shutdown,omitempty"`
	SocketPath        string `json:"socket_path" yaml:"socket_path"`
}

// GraphStatus represents graph-specific status
type GraphStatus struct {
	Fresh       bool   `json:"fresh" yaml:"fresh"`
	StaleFiles  int    `json:"stale_files,omitempty" yaml:"stale_files,omitempty"`
	LastScan    string `json:"last_scan,omitempty" yaml:"last_scan,omitempty"`
	EntityCount int    `json:"entity_count" yaml:"entity_count"`
}

// DatabaseStatus represents database-specific status
type DatabaseStatus struct {
	Path        string `json:"path" yaml:"path"`
	Initialized bool   `json:"initialized" yaml:"initialized"`
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	if statusWatch {
		return runStatusWatch(cmd)
	}

	return runStatusOnce(cmd)
}

func runStatusOnce(cmd *cobra.Command) error {
	output := StatusOutput{}

	// Get CX directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		output.Database.Initialized = false
		output.Daemon.SocketPath = daemon.DefaultSocketPath()
		output.Daemon.Running = false

		if statusJSON {
			return outputStatusJSON(cmd, output)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "CX Status:")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "  Database: not initialized")
		fmt.Fprintln(cmd.OutOrStdout(), "            Run 'cx init && cx scan' to initialize")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "  Daemon:   not running")
		fmt.Fprintln(cmd.OutOrStdout(), "            Will auto-start on next command")
		return nil
	}

	output.Database.Initialized = true
	output.Database.Path = cxDir

	// Check daemon connection status
	client := ensureDaemon()
	output.Daemon.Connected = (client != nil)

	// Try to get daemon status
	daemonStatus, err := daemon.GetDaemonStatus("")
	if err == nil && daemonStatus != nil && daemonStatus.Running {
		output.Daemon.Running = true
		output.Daemon.PID = daemonStatus.PID
		output.Daemon.SocketPath = daemonStatus.SocketPath
		output.Daemon.Uptime = formatStatusDuration(daemonStatus.Uptime)
		output.Daemon.IdleTime = formatStatusDuration(daemonStatus.IdleTime)
		output.Daemon.IdleTimeout = formatStatusDuration(daemonStatus.IdleTimeout)
		output.Daemon.TimeUntilShutdown = formatStatusDuration(daemonStatus.TimeUntilShutdown)
		output.Graph.Fresh = daemonStatus.GraphFresh
		output.Graph.StaleFiles = daemonStatus.StaleFiles
		output.Graph.EntityCount = daemonStatus.EntityCount
	} else {
		output.Daemon.Running = false
		output.Daemon.SocketPath = daemon.DefaultSocketPath()

		// Get graph info from direct store access
		storeDB, err := store.Open(cxDir)
		if err == nil {
			defer storeDB.Close()

			// Get entity count
			entities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active", Limit: 0})
			if err == nil {
				output.Graph.EntityCount = len(entities)
			}

			// Get last scan info
			lastScan, err := getStatusLastScanTime(storeDB)
			if err == nil && !lastScan.IsZero() {
				output.Graph.LastScan = lastScan.Format(time.RFC3339)
				// Check freshness by looking at file modifications
				staleCount := countStatusStaleFiles(storeDB, lastScan)
				output.Graph.StaleFiles = staleCount
				output.Graph.Fresh = staleCount == 0
			}
		}
	}

	if statusJSON {
		return outputStatusJSON(cmd, output)
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "CX Status:")
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Daemon section
	if output.Daemon.Running {
		fmt.Fprintf(cmd.OutOrStdout(), "  Daemon:   running (pid %d)\n", output.Daemon.PID)
		if output.Daemon.Connected {
			fmt.Fprintln(cmd.OutOrStdout(), "            connected (commands use daemon mode)")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "            not connected (commands use direct DB mode)")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "            uptime %s, idle %s\n",
			output.Daemon.Uptime, output.Daemon.IdleTime)
		if output.Daemon.TimeUntilShutdown != "" && output.Daemon.TimeUntilShutdown != "0s" {
			fmt.Fprintf(cmd.OutOrStdout(), "            auto-shutdown in %s\n", output.Daemon.TimeUntilShutdown)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Daemon:   not running")
		if output.Daemon.Connected {
			fmt.Fprintln(cmd.OutOrStdout(), "            connected (will auto-start on next command)")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "            commands use direct DB mode")
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Graph section
	freshText := "stale"
	if output.Graph.Fresh {
		freshText = "fresh"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Graph:    %s", freshText)
	if !output.Graph.Fresh && output.Graph.StaleFiles > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " (%d files changed since scan)", output.Graph.StaleFiles)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "            %d entities indexed\n", output.Graph.EntityCount)
	if output.Graph.LastScan != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "            last scan: %s\n", output.Graph.LastScan)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Database section
	fmt.Fprintf(cmd.OutOrStdout(), "  Database: %s\n", output.Database.Path)

	return nil
}

func runStatusWatch(cmd *cobra.Command) error {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial display
	if err := runStatusOnce(cmd); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to exit")

	for range ticker.C {
		// Clear screen and move to top
		fmt.Print("\033[2J\033[H")
		if err := runStatusOnce(cmd); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to exit")
	}

	return nil
}

func outputStatusJSON(cmd *cobra.Command, output StatusOutput) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func formatStatusDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
	return fmt.Sprintf("%dh", hours)
}

func getStatusLastScanTime(storeDB *store.Store) (time.Time, error) {
	// Query for the most recent file scan time
	files, err := storeDB.GetAllFileEntries()
	if err != nil {
		return time.Time{}, err
	}

	var lastScan time.Time
	for _, f := range files {
		if f.ScannedAt.After(lastScan) {
			lastScan = f.ScannedAt
		}
	}

	return lastScan, nil
}

func countStatusStaleFiles(storeDB *store.Store, lastScan time.Time) int {
	// Get indexed files and check if any have been modified since last scan
	files, err := storeDB.GetAllFileEntries()
	if err != nil {
		return 0
	}

	staleCount := 0
	for _, f := range files {
		// Check if file has been modified since scan
		info, err := os.Stat(f.FilePath)
		if err != nil {
			continue // File might have been deleted
		}

		if info.ModTime().After(f.ScannedAt) {
			staleCount++
		}
	}

	return staleCount
}
