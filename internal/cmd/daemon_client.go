package cmd

import (
	"github.com/anthropics/cx/internal/daemon"
)

// Global daemon client connection.
// This is lazily initialized on first use by commands.
var globalDaemonClient *daemon.Client

// ensureDaemon ensures a daemon connection is available.
// It tries to connect to an existing daemon or start a new one.
// Returns a connected client or nil if falling back to direct DB mode.
//
// This function is safe to call multiple times - it caches the connection
// after the first successful connection.
//
// Usage in commands:
//
//	client := ensureDaemon()
//	if client != nil {
//	    // Use daemon client for queries
//	    resp, err := client.Query(...)
//	} else {
//	    // Fallback to direct store access
//	    store, _ := store.Open(cxDir)
//	    defer store.Close()
//	    ...
//	}
func ensureDaemon() *daemon.Client {
	// Return cached client if available
	if globalDaemonClient != nil {
		return globalDaemonClient
	}

	result, err := daemon.EnsureDaemon(daemon.DefaultEnsureDaemonOptions())
	if err != nil {
		return nil
	}
	if result.UsingFallback {
		return nil
	}

	// Cache for subsequent calls
	globalDaemonClient = result.Client
	return globalDaemonClient
}

// useDaemon returns true if daemon mode is available.
// This is a convenience wrapper around ensureDaemon for
// simple presence checks.
func useDaemon() bool {
	return ensureDaemon() != nil
}

// resetDaemonClient clears the cached daemon client.
// This forces the next ensureDaemon() call to reconnect.
// Useful for testing or after a daemon restart.
func resetDaemonClient() {
	globalDaemonClient = nil
}
