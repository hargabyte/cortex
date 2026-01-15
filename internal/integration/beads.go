package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BeadsAvailable checks if beads CLI is available and a .beads directory exists.
func BeadsAvailable() bool {
	return BeadsAvailableIn("")
}

// BeadsAvailableIn checks if beads is available in a specific directory.
func BeadsAvailableIn(dir string) bool {
	// Check if bd command exists
	_, err := exec.LookPath("bd")
	if err != nil {
		return false
	}

	// Check if .beads directory exists
	beadsDir := ".beads"
	if dir != "" {
		beadsDir = filepath.Join(dir, ".beads")
	}

	info, err := os.Stat(beadsDir)
	if err != nil || !info.IsDir() {
		return false
	}

	return true
}

// BeadInfo contains information about a bead/issue.
type BeadInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
}

// GetBead retrieves information about a bead by ID.
func GetBead(id string) (*BeadInfo, error) {
	return GetBeadIn("", id)
}

// GetBeadIn retrieves information about a bead in a specific directory.
func GetBeadIn(dir, id string) (*BeadInfo, error) {
	cmd := exec.Command("bd", "show", id, "--json")
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bd show %s failed: %w: %s", id, err, stderr.String())
	}

	// bd show --json returns an array
	var beads []BeadInfo
	if err := json.Unmarshal(stdout.Bytes(), &beads); err != nil {
		return nil, fmt.Errorf("parse bead info: %w", err)
	}

	if len(beads) == 0 {
		return nil, fmt.Errorf("bead not found: %s", id)
	}

	return &beads[0], nil
}

// CreateBeadOptions contains options for creating a new bead.
type CreateBeadOptions struct {
	Title       string
	Description string
	Type        string // task, bug, feature
	Priority    int
	Labels      []string
	Parent      string
}

// CreateBead creates a new bead and returns its ID.
func CreateBead(opts CreateBeadOptions) (string, error) {
	return CreateBeadIn("", opts)
}

// CreateBeadIn creates a new bead in a specific directory.
func CreateBeadIn(dir string, opts CreateBeadOptions) (string, error) {
	args := []string{"create", opts.Title}

	if opts.Type != "" {
		args = append(args, "-t", opts.Type)
	}

	if opts.Priority >= 0 && opts.Priority <= 4 {
		args = append(args, "-p", fmt.Sprintf("%d", opts.Priority))
	}

	if opts.Description != "" {
		args = append(args, "-d", opts.Description)
	}

	if opts.Parent != "" {
		args = append(args, "--parent", opts.Parent)
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	cmd := exec.Command("bd", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("bd create failed: %w: %s", err, stderr.String())
	}

	// Parse created ID from output
	output := stdout.String()
	id := extractBeadID(output)
	if id == "" {
		return "", fmt.Errorf("could not parse created bead ID from: %s", output)
	}

	return id, nil
}

// AddDependency creates a dependency between beads.
func AddDependency(fromID, toID, depType string) error {
	return AddDependencyIn("", fromID, toID, depType)
}

// AddDependencyIn creates a dependency in a specific directory.
func AddDependencyIn(dir, fromID, toID, depType string) error {
	args := []string{"dep", "add", fromID, toID}
	if depType != "" {
		args = append(args, "--type", depType)
	}

	cmd := exec.Command("bd", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd dep add failed: %w: %s", err, stderr.String())
	}

	return nil
}

// extractBeadID parses a bead ID from command output.
func extractBeadID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for common ID patterns
		if strings.Contains(line, "-") {
			parts := strings.Fields(line)
			for _, part := range parts {
				part = strings.Trim(part, ".,;:\"'")
				if looksLikeBeadID(part) {
					return part
				}
			}
		}
	}
	return ""
}

// looksLikeBeadID checks if a string looks like a bead ID.
func looksLikeBeadID(s string) bool {
	// Bead IDs have format: prefix-shortcode (e.g., bd-abc123, Project-hi5)
	parts := strings.Split(s, "-")
	if len(parts) < 2 {
		return false
	}
	// Last part should be alphanumeric
	last := parts[len(parts)-1]
	if len(last) < 2 {
		return false
	}
	for _, ch := range last {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '.') {
			return false
		}
	}
	return true
}
