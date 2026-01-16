// Package daemon storeproxy.go provides daemon-aware store access utilities.
// Commands can use GetStore() to transparently get store access - either through
// the daemon if running, or via direct database access as fallback.
package daemon

import (
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

// StoreProvider provides access to the code graph store.
// It abstracts whether access is via daemon or direct database.
type StoreProvider struct {
	// Store is the store instance (for direct mode)
	store *store.Store

	// Client is the daemon client (for daemon mode)
	client *Client

	// isDaemon indicates if we're using daemon mode
	isDaemon bool

	// cxDir is the .cx directory path
	cxDir string
}

// StoreProviderOptions configures store provider behavior.
type StoreProviderOptions struct {
	// CXDir is the .cx directory path. If empty, will be auto-detected.
	CXDir string

	// ProjectRoot is the project root. If empty, uses current directory.
	ProjectRoot string

	// UseDaemon controls whether to try daemon first.
	// If false, always uses direct mode.
	UseDaemon bool

	// RequireDaemon fails if daemon is not available (no fallback).
	RequireDaemon bool
}

// DefaultStoreProviderOptions returns the default options.
func DefaultStoreProviderOptions() StoreProviderOptions {
	return StoreProviderOptions{
		UseDaemon:     true,
		RequireDaemon: false,
	}
}

// NewStoreProvider creates a new store provider with the given options.
// It automatically determines whether to use daemon or direct mode.
func NewStoreProvider(opts StoreProviderOptions) (*StoreProvider, error) {
	// Find CX directory if not provided
	cxDir := opts.CXDir
	if cxDir == "" {
		projectRoot := opts.ProjectRoot
		if projectRoot == "" {
			projectRoot = "."
		}

		var err error
		cxDir, err = config.FindConfigDir(projectRoot)
		if err != nil {
			return nil, fmt.Errorf("cx not initialized: run 'cx scan' first")
		}
	}

	provider := &StoreProvider{
		cxDir: cxDir,
	}

	// Try daemon mode if enabled
	if opts.UseDaemon {
		result, err := EnsureDaemon(EnsureDaemonOptions{
			WithFallback: !opts.RequireDaemon,
			CXDir:        cxDir,
			ProjectRoot:  opts.ProjectRoot,
		})

		if err != nil {
			return nil, fmt.Errorf("daemon connection failed: %w", err)
		}

		if result.UsingFallback {
			// Fall through to direct mode
		} else {
			provider.client = result.Client
			provider.isDaemon = true
			return provider, nil
		}
	}

	// Direct mode - open store directly
	storeDB, err := store.Open(cxDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	provider.store = storeDB
	provider.isDaemon = false
	return provider, nil
}

// Store returns the underlying store instance.
// Returns nil if using daemon mode.
func (p *StoreProvider) Store() *store.Store {
	return p.store
}

// Client returns the daemon client.
// Returns nil if using direct mode.
func (p *StoreProvider) Client() *Client {
	return p.client
}

// IsDaemonMode returns true if using daemon mode.
func (p *StoreProvider) IsDaemonMode() bool {
	return p.isDaemon
}

// Close closes the store provider and releases resources.
func (p *StoreProvider) Close() error {
	if p.store != nil {
		return p.store.Close()
	}
	// Client connections are not long-lived, nothing to close
	return nil
}

// CXDir returns the .cx directory path.
func (p *StoreProvider) CXDir() string {
	return p.cxDir
}

// GetDirectStore opens a direct store connection, bypassing the daemon.
// Use this for operations that must run locally or when daemon is not suitable.
func GetDirectStore(cxDir string) (*store.Store, error) {
	if cxDir == "" {
		var err error
		cxDir, err = config.FindConfigDir(".")
		if err != nil {
			return nil, fmt.Errorf("cx not initialized: run 'cx scan' first")
		}
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	return storeDB, nil
}

// GetStore is a convenience function that returns a store with daemon fallback.
// This is the recommended way for commands to get store access.
// The caller is responsible for calling Close() on the returned StoreProvider.
//
// Usage:
//
//	provider, err := daemon.GetStore()
//	if err != nil {
//	    return err
//	}
//	defer provider.Close()
//
//	if provider.IsDaemonMode() {
//	    // Query via daemon client
//	    resp, _ := provider.Client().Query(...)
//	} else {
//	    // Direct store access
//	    entities, _ := provider.Store().QueryEntities(...)
//	}
func GetStore() (*StoreProvider, error) {
	return NewStoreProvider(DefaultStoreProviderOptions())
}

// GetStoreWithOptions is like GetStore but with custom options.
func GetStoreWithOptions(opts StoreProviderOptions) (*StoreProvider, error) {
	return NewStoreProvider(opts)
}

// MustGetDirectStore opens a direct store or panics.
// Use sparingly, prefer GetStore for graceful error handling.
func MustGetDirectStore() *store.Store {
	storeDB, err := GetDirectStore("")
	if err != nil {
		panic(err)
	}
	return storeDB
}
