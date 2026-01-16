package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPaths(t *testing.T) {
	socketPath := DefaultSocketPath()
	if socketPath == "" {
		t.Error("DefaultSocketPath returned empty string")
	}

	pidPath := DefaultPIDPath()
	if pidPath == "" {
		t.Error("DefaultPIDPath returned empty string")
	}

	// Verify they're under .cx directory
	if !contains(socketPath, ".cx") {
		t.Errorf("socket path should contain .cx: %s", socketPath)
	}
	if !contains(pidPath, ".cx") {
		t.Errorf("pid path should contain .cx: %s", pidPath)
	}
}

func TestPIDFileOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-daemon-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write PID file
	testPID := 12345
	if err := writePIDFile(pidPath, testPID); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	// Read PID file
	readPID, err := readPIDFile(pidPath)
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}

	if readPID != testPID {
		t.Errorf("expected PID %d, got %d", testPID, readPID)
	}

	// Read non-existent PID file
	_, err = readPIDFile(filepath.Join(tmpDir, "nonexistent.pid"))
	if err == nil {
		t.Error("expected error reading non-existent PID file")
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Current process should be running
	currentPID := os.Getpid()
	if !isProcessRunning(currentPID) {
		t.Error("current process should be running")
	}

	// Non-existent PID should not be running
	// Use a high PID that's unlikely to exist
	if isProcessRunning(999999) {
		t.Error("non-existent process should not be running")
	}
}

func TestNewDaemon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-daemon-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		SocketPath:  filepath.Join(tmpDir, "daemon.sock"),
		PIDPath:     filepath.Join(tmpDir, "daemon.pid"),
		IdleTimeout: 5 * time.Minute,
		ProjectRoot: tmpDir,
		CXDir:       filepath.Join(tmpDir, ".cx"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	if d.config.SocketPath != cfg.SocketPath {
		t.Errorf("socket path mismatch: expected %s, got %s",
			cfg.SocketPath, d.config.SocketPath)
	}

	if d.config.PIDPath != cfg.PIDPath {
		t.Errorf("PID path mismatch: expected %s, got %s",
			cfg.PIDPath, d.config.PIDPath)
	}

	if d.config.IdleTimeout != cfg.IdleTimeout {
		t.Errorf("idle timeout mismatch: expected %v, got %v",
			cfg.IdleTimeout, d.config.IdleTimeout)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("expected default idle timeout %v, got %v",
			DefaultIdleTimeout, cfg.IdleTimeout)
	}

	if cfg.SocketPath == "" {
		t.Error("default socket path should not be empty")
	}

	if cfg.PIDPath == "" {
		t.Error("default PID path should not be empty")
	}
}

func TestStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-daemon-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		SocketPath:  filepath.Join(tmpDir, "daemon.sock"),
		PIDPath:     filepath.Join(tmpDir, "daemon.pid"),
		IdleTimeout: 5 * time.Minute,
		ProjectRoot: tmpDir,
		CXDir:       filepath.Join(tmpDir, ".cx"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	// Before starting
	status := d.GetStatus()
	if status.Running {
		t.Error("daemon should not be running before Start()")
	}

	if status.SocketPath != cfg.SocketPath {
		t.Errorf("socket path mismatch in status: expected %s, got %s",
			cfg.SocketPath, status.SocketPath)
	}

	if status.IdleTimeout != cfg.IdleTimeout {
		t.Errorf("idle timeout mismatch in status: expected %v, got %v",
			cfg.IdleTimeout, status.IdleTimeout)
	}
}

func TestIsRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-daemon-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// No PID file - should not be running
	running, pid, err := IsRunning(pidPath)
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if running {
		t.Error("should not be running without PID file")
	}
	if pid != 0 {
		t.Errorf("PID should be 0, got %d", pid)
	}

	// Write PID file with current PID - should be running
	currentPID := os.Getpid()
	if err := writePIDFile(pidPath, currentPID); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	running, pid, err = IsRunning(pidPath)
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if !running {
		t.Error("should be running with valid PID")
	}
	if pid != currentPID {
		t.Errorf("expected PID %d, got %d", currentPID, pid)
	}

	// Write PID file with non-existent PID - should not be running
	// and file should be cleaned up
	if err := writePIDFile(pidPath, 999999); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	running, pid, err = IsRunning(pidPath)
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if running {
		t.Error("should not be running with stale PID")
	}

	// PID file should have been removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale PID file should have been removed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
