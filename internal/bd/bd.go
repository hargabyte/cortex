package bd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps bd CLI operations
type Client struct {
	// WorkDir is the directory to run bd commands in
	WorkDir string
}

// NewClient creates a BD client for the given directory.
// It validates that bd is available in PATH.
func NewClient(workDir string) (*Client, error) {
	// Verify bd is in PATH
	_, err := exec.LookPath("bd")
	if err != nil {
		return nil, fmt.Errorf("bd command not found in PATH: %w", err)
	}

	return &Client{
		WorkDir: workDir,
	}, nil
}

// execBD runs a bd command and returns stdout
func (c *Client) execBD(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	if c.WorkDir != "" {
		cmd.Dir = c.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include stderr in error for better debugging
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("bd %s failed: %w: %s", strings.Join(args, " "), err, errMsg)
		}
		return nil, fmt.Errorf("bd %s failed: %w", strings.Join(args, " "), err)
	}

	return stdout.Bytes(), nil
}

// execBDJSON runs a bd command with --json and parses output into result
func (c *Client) execBDJSON(result interface{}, args ...string) error {
	// Append --json flag
	args = append(args, "--json")

	output, err := c.execBD(args...)
	if err != nil {
		return err
	}

	if len(output) == 0 {
		return nil
	}

	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("failed to parse bd JSON output: %w", err)
	}

	return nil
}

// Version returns the bd version string
func (c *Client) Version() (string, error) {
	output, err := c.execBD("--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Ready returns entities that are ready to work on (unblocked)
func (c *Client) Ready(limit int) ([]Entity, error) {
	args := []string{"ready"}
	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	var entities []Entity
	if err := c.execBDJSON(&entities, args...); err != nil {
		return nil, err
	}
	return entities, nil
}

// Sync synchronizes the beads database with git
func (c *Client) Sync() error {
	_, err := c.execBD("sync")
	return err
}

// Doctor runs health checks on the beads database
func (c *Client) Doctor(fix bool) error {
	args := []string{"doctor"}
	if fix {
		args = append(args, "--fix", "--yes")
	}
	_, err := c.execBD(args...)
	return err
}

// Init initializes a new beads database in the working directory.
// Uses: bd init --prefix <prefix> --quiet
func (c *Client) Init(prefix string) error {
	args := []string{"init", "--quiet"}
	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}
	_, err := c.execBD(args...)
	return err
}

// HasBeadsDB checks if a .beads directory exists in the working directory
func (c *Client) HasBeadsDB() bool {
	var beadsDir string
	if c.WorkDir == "" || c.WorkDir == "." {
		beadsDir = ".beads"
	} else {
		beadsDir = filepath.Join(c.WorkDir, ".beads")
	}
	info, err := os.Stat(beadsDir)
	return err == nil && info.IsDir()
}
