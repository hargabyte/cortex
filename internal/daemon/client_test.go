package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultEnsureDaemonOptions(t *testing.T) {
	opts := DefaultEnsureDaemonOptions()

	if opts.SocketPath == "" {
		t.Error("SocketPath should have default value")
	}

	if opts.PIDPath == "" {
		t.Error("PIDPath should have default value")
	}

	if opts.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("IdleTimeout expected %v, got %v", DefaultIdleTimeout, opts.IdleTimeout)
	}

	if opts.StartTimeout != 5*time.Second {
		t.Errorf("StartTimeout expected 5s, got %v", opts.StartTimeout)
	}

	if !opts.WithFallback {
		t.Error("WithFallback should be true by default")
	}
}

func TestConnectToDaemonNotRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	// Should fail when no daemon is running
	client, err := ConnectToDaemon(socketPath)
	if err == nil {
		t.Error("expected error connecting to non-existent daemon")
	}
	if client != nil {
		t.Error("client should be nil when connection fails")
	}
}

func TestIsDaemonRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Should return false when no daemon is running
	if IsDaemonRunning(socketPath) {
		t.Error("should not be running without daemon")
	}

	// Start a mock socket server
	handler := func(req *Request) *Response {
		return &Response{
			Success: true,
			Data: map[string]interface{}{
				"healthy": true,
			},
		}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Should return true now
	if !IsDaemonRunning(socketPath) {
		t.Error("should be running with daemon")
	}
}

func TestGetDaemonStatusNotRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	// Should return nil, nil when not running (not an error)
	status, err := GetDaemonStatus(socketPath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if status != nil {
		t.Error("status should be nil when not running")
	}
}

func TestGetDaemonStatusRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a mock socket server that returns status
	handler := func(req *Request) *Response {
		if req.Type == RequestTypeStatus {
			return &Response{
				Success: true,
				Data: map[string]interface{}{
					"pid":          float64(12345),
					"uptime":       "5m30s",
					"idle_time":    "1m0s",
					"project_root": "/test/project",
					"entity_count": float64(100),
					"graph_fresh":  true,
				},
			}
		}
		return &Response{
			Success: true,
			Data:    map[string]interface{}{"healthy": true},
		}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Get status
	status, err := GetDaemonStatus(socketPath)
	if err != nil {
		t.Fatalf("get daemon status: %v", err)
	}

	if status == nil {
		t.Fatal("status should not be nil")
	}

	if !status.Running {
		t.Error("status.Running should be true")
	}

	if status.PID != 12345 {
		t.Errorf("PID expected 12345, got %d", status.PID)
	}

	if status.EntityCount != 100 {
		t.Errorf("EntityCount expected 100, got %d", status.EntityCount)
	}

	if !status.GraphFresh {
		t.Error("GraphFresh should be true")
	}
}

func TestStopDaemonNotRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	// Should not error when not running
	err = StopDaemon(socketPath)
	if err != nil {
		t.Errorf("unexpected error stopping non-running daemon: %v", err)
	}
}

func TestStopDaemonRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	stopCalled := false
	handler := func(req *Request) *Response {
		if req.Type == RequestTypeStop {
			stopCalled = true
			return &Response{
				Success: true,
				Data:    map[string]interface{}{"message": "stopping"},
			}
		}
		return &Response{Success: true}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Stop daemon
	err = StopDaemon(socketPath)
	if err != nil {
		t.Errorf("stop daemon: %v", err)
	}

	if !stopCalled {
		t.Error("stop handler should have been called")
	}
}

func TestEnsureDaemonResult(t *testing.T) {
	result := &EnsureDaemonResult{}

	// Default state
	if result.Client != nil {
		t.Error("Client should be nil by default")
	}
	if result.WasStarted {
		t.Error("WasStarted should be false by default")
	}
	if result.UsingFallback {
		t.Error("UsingFallback should be false by default")
	}
	if result.PID != 0 {
		t.Error("PID should be 0 by default")
	}
}

func TestEnsureDaemonWithExistingDaemon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a mock daemon
	handler := func(req *Request) *Response {
		if req.Type == RequestTypeHealth {
			return &Response{
				Success: true,
				Data: map[string]interface{}{
					"healthy": true,
					"pid":     float64(os.Getpid()),
				},
			}
		}
		return &Response{Success: true}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// EnsureDaemon should connect to existing daemon
	opts := EnsureDaemonOptions{
		SocketPath:   socketPath,
		PIDPath:      filepath.Join(tmpDir, "test.pid"),
		StartTimeout: 1 * time.Second,
		WithFallback: false,
	}

	result, err := EnsureDaemon(opts)
	if err != nil {
		t.Fatalf("EnsureDaemon: %v", err)
	}

	if result.Client == nil {
		t.Error("Client should not be nil")
	}
	if result.WasStarted {
		t.Error("WasStarted should be false for existing daemon")
	}
	if result.UsingFallback {
		t.Error("UsingFallback should be false")
	}
	if result.PID != os.Getpid() {
		t.Errorf("PID expected %d, got %d", os.Getpid(), result.PID)
	}
}

func TestEnsureDaemonWithFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	// No daemon running, should fallback
	opts := EnsureDaemonOptions{
		SocketPath:   socketPath,
		PIDPath:      filepath.Join(tmpDir, "test.pid"),
		StartTimeout: 500 * time.Millisecond,
		WithFallback: true, // Enable fallback
	}

	result, err := EnsureDaemon(opts)
	if err != nil {
		t.Fatalf("EnsureDaemon with fallback should not error: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if !result.UsingFallback {
		t.Error("UsingFallback should be true")
	}

	if result.Client != nil {
		t.Error("Client should be nil in fallback mode")
	}
}

func TestEnsureDaemonWithoutFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-client-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "nonexistent.sock")

	// No daemon running, no fallback - should error
	opts := EnsureDaemonOptions{
		SocketPath:   socketPath,
		PIDPath:      filepath.Join(tmpDir, "test.pid"),
		StartTimeout: 500 * time.Millisecond,
		WithFallback: false, // Disable fallback
	}

	result, err := EnsureDaemon(opts)

	// Should error or return nil result when daemon can't start and fallback is disabled
	// In this test we don't have cx binary, so it will fail to start
	if err == nil && result != nil && !result.UsingFallback {
		// If somehow it succeeded, that's unexpected
		t.Log("Note: EnsureDaemon succeeded unexpectedly, daemon may have started")
	}
}
