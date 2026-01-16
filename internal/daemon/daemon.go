// Package daemon provides the CX daemon process for persistent graph operations.
// The daemon runs in the background, keeping the code graph warm and fresh,
// and handles queries from cx commands via Unix socket communication.
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/store"
)

// DefaultIdleTimeout is the default duration after which the daemon shuts down
// if no activity is detected.
const DefaultIdleTimeout = 30 * time.Minute

// DefaultSocketPath returns the default Unix socket path for the daemon.
// The socket is stored in the user's home directory under .cx/daemon.sock
func DefaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/cx-daemon.sock"
	}
	return filepath.Join(home, ".cx", "daemon.sock")
}

// DefaultPIDPath returns the default PID file path for the daemon.
func DefaultPIDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/cx-daemon.pid"
	}
	return filepath.Join(home, ".cx", "daemon.pid")
}

// Config holds daemon configuration options.
type Config struct {
	// SocketPath is the Unix socket path for client connections.
	// Defaults to ~/.cx/daemon.sock
	SocketPath string

	// PIDPath is the path to store the daemon's PID file.
	// Defaults to ~/.cx/daemon.pid
	PIDPath string

	// IdleTimeout is the duration after which the daemon shuts down if idle.
	// Set to 0 to disable auto-shutdown.
	IdleTimeout time.Duration

	// ProjectRoot is the root directory of the project to manage.
	// If empty, uses current working directory.
	ProjectRoot string

	// CXDir is the .cx directory path. If empty, derived from ProjectRoot.
	CXDir string

	// Verbose enables detailed logging.
	Verbose bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		SocketPath:  DefaultSocketPath(),
		PIDPath:     DefaultPIDPath(),
		IdleTimeout: DefaultIdleTimeout,
	}
}

// Status represents the daemon's current status.
type Status struct {
	// Running indicates if the daemon is currently running.
	Running bool `json:"running"`

	// PID is the daemon's process ID (0 if not running).
	PID int `json:"pid"`

	// StartedAt is when the daemon started (zero if not running).
	StartedAt time.Time `json:"started_at,omitempty"`

	// Uptime is how long the daemon has been running.
	Uptime time.Duration `json:"uptime,omitempty"`

	// LastActivity is when the last query or file change was processed.
	LastActivity time.Time `json:"last_activity,omitempty"`

	// IdleTime is how long since the last activity.
	IdleTime time.Duration `json:"idle_time,omitempty"`

	// IdleTimeout is the configured idle timeout duration.
	IdleTimeout time.Duration `json:"idle_timeout,omitempty"`

	// TimeUntilShutdown is how long until auto-shutdown (0 if disabled).
	TimeUntilShutdown time.Duration `json:"time_until_shutdown,omitempty"`

	// SocketPath is the Unix socket path.
	SocketPath string `json:"socket_path"`

	// ProjectRoot is the project root being managed.
	ProjectRoot string `json:"project_root,omitempty"`

	// EntityCount is the number of entities in the graph.
	EntityCount int `json:"entity_count,omitempty"`

	// GraphFresh indicates if the graph is up-to-date with the filesystem.
	GraphFresh bool `json:"graph_fresh"`

	// StaleFiles is the count of files that have changed since last scan.
	StaleFiles int `json:"stale_files,omitempty"`
}

// Daemon is the main CX daemon process.
type Daemon struct {
	config Config

	// Core components
	store       *store.Store
	graph       *graph.Graph
	socket      *Socket
	projectRoot string
	cxDir       string

	// Lifecycle management
	startedAt    time.Time
	lastActivity time.Time
	idleTimer    *time.Timer
	shutdown     chan struct{}
	shutdownOnce sync.Once

	// Thread safety
	mu sync.RWMutex

	// Logger function (allows custom logging)
	logFunc func(format string, args ...interface{})
}

// New creates a new Daemon instance with the given configuration.
func New(cfg Config) (*Daemon, error) {
	// Ensure .cx directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	cxHomeDir := filepath.Join(home, ".cx")
	if err := os.MkdirAll(cxHomeDir, 0755); err != nil {
		return nil, fmt.Errorf("create .cx directory: %w", err)
	}

	// Set defaults
	if cfg.SocketPath == "" {
		cfg.SocketPath = DefaultSocketPath()
	}
	if cfg.PIDPath == "" {
		cfg.PIDPath = DefaultPIDPath()
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}

	// Determine project root
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		projectRoot, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Determine .cx directory
	cxDir := cfg.CXDir
	if cxDir == "" {
		cxDir = filepath.Join(projectRoot, ".cx")
	}

	d := &Daemon{
		config:      cfg,
		projectRoot: projectRoot,
		cxDir:       cxDir,
		shutdown:    make(chan struct{}),
		logFunc:     defaultLog,
	}

	return d, nil
}

// defaultLog is the default logging function that writes to stderr.
func defaultLog(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[daemon] "+format+"\n", args...)
}

// SetLogger sets a custom logging function.
func (d *Daemon) SetLogger(fn func(format string, args ...interface{})) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logFunc = fn
}

// log writes a log message using the configured logger.
func (d *Daemon) log(format string, args ...interface{}) {
	d.mu.RLock()
	fn := d.logFunc
	d.mu.RUnlock()
	if fn != nil {
		fn(format, args...)
	}
}

// Start initializes and starts the daemon.
// It opens the store, builds the graph, starts the socket server,
// and begins handling requests.
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if already running
	if d.startedAt != (time.Time{}) {
		return fmt.Errorf("daemon already running")
	}

	// Check for existing daemon via PID file
	if existingPID, err := readPIDFile(d.config.PIDPath); err == nil {
		if isProcessRunning(existingPID) {
			return fmt.Errorf("daemon already running with PID %d", existingPID)
		}
		// Stale PID file, remove it
		os.Remove(d.config.PIDPath)
	}

	// Open store
	storeDB, err := store.Open(d.cxDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	d.store = storeDB

	// Build graph
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		d.store.Close()
		return fmt.Errorf("build graph: %w", err)
	}
	d.graph = g

	// Write PID file
	if err := writePIDFile(d.config.PIDPath, os.Getpid()); err != nil {
		d.cleanup()
		return fmt.Errorf("write PID file: %w", err)
	}

	// Create and start socket server
	d.socket, err = NewSocket(d.config.SocketPath, d.handleRequest)
	if err != nil {
		d.cleanup()
		return fmt.Errorf("create socket: %w", err)
	}

	if err := d.socket.Start(); err != nil {
		d.cleanup()
		return fmt.Errorf("start socket: %w", err)
	}

	// Initialize timing
	d.startedAt = time.Now()
	d.lastActivity = d.startedAt

	// Start idle timer if timeout is configured
	if d.config.IdleTimeout > 0 {
		d.idleTimer = time.AfterFunc(d.config.IdleTimeout, d.onIdleTimeout)
	}

	// Setup signal handlers
	go d.handleSignals()

	d.log("started (pid=%d, socket=%s, idle_timeout=%v)",
		os.Getpid(), d.config.SocketPath, d.config.IdleTimeout)

	return nil
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.stopLocked()
}

// stopLocked performs the actual stop. Caller must hold the lock.
func (d *Daemon) stopLocked() error {
	// Signal shutdown (only once)
	d.shutdownOnce.Do(func() {
		close(d.shutdown)
	})

	// Stop idle timer
	if d.idleTimer != nil {
		d.idleTimer.Stop()
		d.idleTimer = nil
	}

	// Cleanup resources
	d.cleanup()

	d.log("stopped")
	return nil
}

// cleanup releases all resources. Caller must hold the lock.
func (d *Daemon) cleanup() {
	if d.socket != nil {
		d.socket.Stop()
		d.socket = nil
	}

	if d.store != nil {
		d.store.Close()
		d.store = nil
	}

	d.graph = nil

	// Remove PID file
	os.Remove(d.config.PIDPath)
}

// Wait blocks until the daemon shuts down.
func (d *Daemon) Wait() {
	<-d.shutdown
}

// Run starts the daemon and blocks until shutdown.
// This is a convenience method combining Start() and Wait().
func (d *Daemon) Run() error {
	if err := d.Start(); err != nil {
		return err
	}
	d.Wait()
	return nil
}

// resetIdleTimer resets the idle timeout timer.
// This should be called whenever there is activity.
func (d *Daemon) resetIdleTimer() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastActivity = time.Now()
	if d.idleTimer != nil && d.config.IdleTimeout > 0 {
		d.idleTimer.Reset(d.config.IdleTimeout)
	}
}

// onIdleTimeout is called when the idle timer expires.
func (d *Daemon) onIdleTimeout() {
	d.log("idle timeout reached after %v, shutting down", d.config.IdleTimeout)
	d.Stop()
}

// handleSignals sets up signal handlers for graceful shutdown.
func (d *Daemon) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		d.log("received signal %v, shutting down", sig)
		d.Stop()
	case <-d.shutdown:
		// Already shutting down
	}
}

// handleRequest processes an incoming request from a client.
func (d *Daemon) handleRequest(req *Request) *Response {
	// Reset idle timer on any activity
	d.resetIdleTimer()

	switch req.Type {
	case RequestTypeHealth:
		return d.handleHealthRequest()
	case RequestTypeStatus:
		return d.handleStatusRequest()
	case RequestTypeQuery:
		return d.handleQueryRequest(req)
	case RequestTypeStop:
		return d.handleStopRequest()
	default:
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("unknown request type: %s", req.Type),
		}
	}
}

// handleHealthRequest returns a simple health check response.
func (d *Daemon) handleHealthRequest() *Response {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return &Response{
		Success: true,
		Data: map[string]interface{}{
			"healthy":  true,
			"pid":      os.Getpid(),
			"uptime":   time.Since(d.startedAt).String(),
			"uptime_s": int64(time.Since(d.startedAt).Seconds()),
		},
	}
}

// handleStatusRequest returns detailed daemon status.
func (d *Daemon) handleStatusRequest() *Response {
	status := d.GetStatus()

	data, err := json.Marshal(status)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("marshal status: %v", err),
		}
	}

	var dataMap map[string]interface{}
	json.Unmarshal(data, &dataMap)

	return &Response{
		Success: true,
		Data:    dataMap,
	}
}

// handleQueryRequest handles a cx query request.
func (d *Daemon) handleQueryRequest(req *Request) *Response {
	// Query handling will be implemented in future tasks
	// For now, return a placeholder
	return &Response{
		Success: false,
		Error:   "query handling not yet implemented",
	}
}

// handleStopRequest handles a request to stop the daemon.
func (d *Daemon) handleStopRequest() *Response {
	// Schedule stop after sending response
	go func() {
		time.Sleep(100 * time.Millisecond)
		d.Stop()
	}()

	return &Response{
		Success: true,
		Data: map[string]interface{}{
			"message": "daemon shutting down",
		},
	}
}

// GetStatus returns the current daemon status.
func (d *Daemon) GetStatus() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()
	status := Status{
		Running:     d.startedAt != (time.Time{}),
		PID:         os.Getpid(),
		SocketPath:  d.config.SocketPath,
		ProjectRoot: d.projectRoot,
		IdleTimeout: d.config.IdleTimeout,
		GraphFresh:  true, // Will be updated when incremental scanning is implemented
	}

	if status.Running {
		status.StartedAt = d.startedAt
		status.Uptime = now.Sub(d.startedAt)
		status.LastActivity = d.lastActivity
		status.IdleTime = now.Sub(d.lastActivity)

		if d.config.IdleTimeout > 0 {
			remaining := d.config.IdleTimeout - status.IdleTime
			if remaining < 0 {
				remaining = 0
			}
			status.TimeUntilShutdown = remaining
		}

		// Get entity count from store if available
		if d.store != nil {
			if entities, err := d.store.QueryEntities(store.EntityFilter{Limit: 0}); err == nil {
				status.EntityCount = len(entities)
			}
		}
	}

	return status
}

// IsRunning checks if a daemon is currently running by checking the PID file.
func IsRunning(pidPath string) (bool, int, error) {
	if pidPath == "" {
		pidPath = DefaultPIDPath()
	}

	pid, err := readPIDFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	if isProcessRunning(pid) {
		return true, pid, nil
	}

	// PID file exists but process is not running - stale file
	os.Remove(pidPath)
	return false, 0, nil
}

// readPIDFile reads the PID from the PID file.
func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, fmt.Errorf("parse PID: %w", err)
	}

	return pid, nil
}

// writePIDFile writes the PID to the PID file.
func writePIDFile(path string, pid int) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create PID directory: %w", err)
	}

	return os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// isProcessRunning checks if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds; send signal 0 to check if process exists
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
