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

	// Start a mock daemon that responds to health checks
	handler := func(req *Request) *Response {
		return &Response{
			Success: true,
			Data: map[string]interface{}{
				"healthy": true,
				"pid":     float64(99999),
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

	opts := EnsureDaemonOptions{
		SocketPath:   socketPath,
		PIDPath:      filepath.Join(tmpDir, "test.pid"),
		StartTimeout: 1 * time.Second,
		WithFallback: false,
	}

	result, err := EnsureDaemon(opts)
	if err != nil {
		t.Fatalf("EnsureDaemon should connect to existing daemon: %v", err)
	}
	if result.UsingFallback {
		t.Error("should not be using fallback when daemon is running")
	}
	if result.Client == nil {
		t.Error("Client should not be nil when daemon is running")
	}
	if result.WasStarted {
		t.Error("WasStarted should be false — daemon was already running")
	}
	if result.PID != 99999 {
		t.Errorf("PID expected 99999, got %d", result.PID)
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

	// No daemon running, no fallback — should error because we can't start one
	// (no real cx binary in test env, or it will fail to connect)
	opts := EnsureDaemonOptions{
		SocketPath:   socketPath,
		PIDPath:      filepath.Join(tmpDir, "test.pid"),
		StartTimeout: 500 * time.Millisecond,
		WithFallback: false,
	}

	_, err = EnsureDaemon(opts)
	if err == nil {
		t.Error("EnsureDaemon should error when no daemon running and fallback disabled")
	}
}

func TestEnsureDaemonChildGuard(t *testing.T) {
	// Set the env var that marks us as a daemon child
	os.Setenv("CX_DAEMON_CHILD", "1")
	defer os.Unsetenv("CX_DAEMON_CHILD")

	// With fallback: should return fallback immediately
	result, err := EnsureDaemon(EnsureDaemonOptions{WithFallback: true})
	if err != nil {
		t.Fatalf("child guard with fallback should not error: %v", err)
	}
	if !result.UsingFallback {
		t.Error("child guard should force fallback")
	}
	if result.Client != nil {
		t.Error("child guard should not return a client")
	}

	// Without fallback: should return error
	_, err = EnsureDaemon(EnsureDaemonOptions{WithFallback: false})
	if err == nil {
		t.Error("child guard without fallback should error")
	}
}
