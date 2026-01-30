package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultStoreProviderOptions(t *testing.T) {
	opts := DefaultStoreProviderOptions()

	// UseDaemon is disabled due to spawn storm bug (see cortex-6uc)
	if opts.UseDaemon {
		t.Error("UseDaemon should be false by default (daemon disabled due to spawn storm bug)")
	}

	if opts.RequireDaemon {
		t.Error("RequireDaemon should be false by default")
	}
}

func TestStoreProviderDirectMode(t *testing.T) {
	// Create a temp directory with .cx structure
	tmpDir, err := os.MkdirTemp("", "cx-storeproxy-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("create .cx dir: %v", err)
	}

	// Test with direct mode only (no daemon)
	opts := StoreProviderOptions{
		CXDir:     cxDir,
		UseDaemon: false, // Force direct mode
	}

	provider, err := NewStoreProvider(opts)
	if err != nil {
		t.Fatalf("create store provider: %v", err)
	}
	defer provider.Close()

	if provider.IsDaemonMode() {
		t.Error("should not be in daemon mode")
	}

	if provider.Store() == nil {
		t.Error("store should not be nil in direct mode")
	}

	if provider.Client() != nil {
		t.Error("client should be nil in direct mode")
	}

	if provider.CXDir() != cxDir {
		t.Errorf("CXDir expected %s, got %s", cxDir, provider.CXDir())
	}
}

func TestStoreProviderWithDaemonFallback(t *testing.T) {
	// Create a temp directory with .cx structure
	tmpDir, err := os.MkdirTemp("", "cx-storeproxy-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("create .cx dir: %v", err)
	}

	// Test with daemon mode enabled but no daemon running
	// Should fallback to direct mode
	opts := StoreProviderOptions{
		CXDir:         cxDir,
		UseDaemon:     true,  // Try daemon first
		RequireDaemon: false, // Allow fallback
	}

	provider, err := NewStoreProvider(opts)
	if err != nil {
		t.Fatalf("create store provider: %v", err)
	}
	defer provider.Close()

	// Without a daemon running, should fall back to direct mode
	// Note: This test may behave differently if a daemon happens to be running
	if provider.IsDaemonMode() {
		t.Log("Note: Daemon mode active - a daemon may be running")
	} else {
		if provider.Store() == nil {
			t.Error("store should not be nil in fallback mode")
		}
	}
}

func TestGetDirectStore(t *testing.T) {
	// Create a temp directory with .cx structure
	tmpDir, err := os.MkdirTemp("", "cx-directstore-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("create .cx dir: %v", err)
	}

	store, err := GetDirectStore(cxDir)
	if err != nil {
		t.Fatalf("get direct store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Error("store should not be nil")
	}

	// Verify we can use the store
	if store.Path() == "" {
		t.Error("store path should not be empty")
	}
}

func TestGetDirectStoreNotInitialized(t *testing.T) {
	// Create a temp directory WITHOUT .cx structure
	tmpDir, err := os.MkdirTemp("", "cx-directstore-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	// GetDirectStore with empty path should try to find config dir
	// and fail because there's no .cx directory
	_, err = GetDirectStore("")
	if err == nil {
		t.Error("expected error when cx is not initialized")
	}
}

func TestStoreProviderClose(t *testing.T) {
	// Create a temp directory with .cx structure
	tmpDir, err := os.MkdirTemp("", "cx-storeproxy-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("create .cx dir: %v", err)
	}

	opts := StoreProviderOptions{
		CXDir:     cxDir,
		UseDaemon: false,
	}

	provider, err := NewStoreProvider(opts)
	if err != nil {
		t.Fatalf("create store provider: %v", err)
	}

	// Close should not error
	err = provider.Close()
	if err != nil {
		t.Errorf("close provider: %v", err)
	}

	// Double close should also be safe
	err = provider.Close()
	if err != nil {
		t.Errorf("double close provider: %v", err)
	}
}
