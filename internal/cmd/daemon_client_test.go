package cmd

import (
	"testing"

	"github.com/anthropics/cx/internal/daemon"
)

// TestEnsureDaemon tests the ensureDaemon function
func TestEnsureDaemon(t *testing.T) {
	// Reset any cached client
	resetDaemonClient()

	// Stop any running daemon
	daemon.StopDaemon("")

	// First call should return nil (daemon not running, fallback mode)
	client := ensureDaemon()
	if client != nil {
		t.Errorf("Expected nil client when daemon not running (fallback mode), got %v", client)
	}

	// Verify useDaemon returns false
	if useDaemon() {
		t.Errorf("Expected useDaemon() to return false when daemon not running")
	}

	// Note: We can't easily test the daemon running case in a unit test
	// because it requires starting an actual daemon process.
	// Integration tests should cover that scenario.
}

// TestResetDaemonClient verifies reset clears the cached client
func TestResetDaemonClient(t *testing.T) {
	// Set a dummy client (even though we shouldn't do this in real code)
	globalDaemonClient = daemon.NewClient("")

	// Verify it's set
	if globalDaemonClient == nil {
		t.Fatal("Failed to set test client")
	}

	// Reset
	resetDaemonClient()

	// Verify it's cleared
	if globalDaemonClient != nil {
		t.Errorf("Expected globalDaemonClient to be nil after reset, got %v", globalDaemonClient)
	}
}
