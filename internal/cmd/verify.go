// Package cmd provides the verify command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// verifyCmd represents the verify command
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify entity staleness via AST hashes",
	Long: `Check entity staleness by comparing stored hashes with current AST.
Detects signature changes, body changes, and missing entities.

This is useful for:
  - Detecting when code has changed since last scan
  - Finding entities that need re-analysis
  - CI integration to catch drift

Examples:
  cx verify                         # Check all entities
  cx verify --type=function         # Check only functions
  cx verify --strict                # Exit non-zero on any drift (for CI)
  cx verify --fix                   # Update hashes for drifted entities
  cx verify --create-task           # Create beads task for verification failures`,
	RunE: runVerify,
}

var (
	verifyType       string
	verifyStrict     bool
	verifyFix        bool
	verifyDryRun     bool
	verifyCreateTask bool
)

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&verifyType, "type", "all", "Entity types to verify (function|type|all)")
	verifyCmd.Flags().BoolVar(&verifyStrict, "strict", false, "Exit non-zero on any drift (for CI)")
	verifyCmd.Flags().BoolVar(&verifyFix, "fix", false, "Update hashes for drifted entities")
	verifyCmd.Flags().BoolVar(&verifyDryRun, "dry-run", false, "Show what --fix would do without making changes")
	verifyCmd.Flags().BoolVar(&verifyCreateTask, "create-task", false, "Create a beads task for verification failures")
}

// verifyResult holds the categorized verification results
type verifyResult struct {
	valid   []verifyEntry
	drifted []verifyEntry
	missing []verifyEntry
}

// verifyEntry holds details about a single entity verification
type verifyEntry struct {
	entity  *store.Entity
	file    string
	line    string
	oldSig  string
	newSig  string
	oldBody string
	newBody string
	reason  string
	detail  string
}

// VerifyOutput represents the verification results in YAML/JSON format
type VerifyOutput struct {
	// Verification contains the overall verification status and results
	Verification *VerificationData `yaml:"verification" json:"verification"`
}

// VerificationData holds the detailed verification information
type VerificationData struct {
	// Status is the overall verification result: "passed" or "failed"
	Status string `yaml:"status" json:"status"`

	// EntitiesChecked is the total number of entities verified
	EntitiesChecked int `yaml:"entities_checked" json:"entities_checked"`

	// Valid is the count of entities with no drift
	Valid int `yaml:"valid" json:"valid"`

	// Drifted is the count of entities with signature or body changes
	Drifted int `yaml:"drifted" json:"drifted"`

	// Missing is the count of entities that no longer exist
	Missing int `yaml:"missing" json:"missing"`

	// Issues contains detailed information about each drifted or missing entity
	Issues []VerifyIssue `yaml:"issues" json:"issues"`

	// Actions contains suggested remediation steps
	Actions []string `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// VerifyIssue represents a single verification issue
type VerifyIssue struct {
	// Entity is the name of the entity with an issue
	Entity string `yaml:"entity" json:"entity"`

	// Type is the issue type: "drifted" or "missing"
	Type string `yaml:"type" json:"type"`

	// Location is the file and line information
	Location string `yaml:"location" json:"location"`

	// Reason describes the issue: "sig_hash changed", "body_hash changed", "symbol not found", etc.
	Reason string `yaml:"reason" json:"reason"`

	// Detail provides additional context about the issue
	Detail string `yaml:"detail" json:"detail"`

	// Expected contains the expected hash (for drifted entities)
	Expected string `yaml:"expected,omitempty" json:"expected,omitempty"`

	// Actual contains the actual hash (for drifted entities)
	Actual string `yaml:"actual,omitempty" json:"actual,omitempty"`

	// HashType indicates which hash changed: "signature" or "body"
	HashType string `yaml:"hash_type,omitempty" json:"hash_type,omitempty"`
}

func runVerify(cmd *cobra.Command, args []string) error {
	// Open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Get entities based on type filter
	filter := store.EntityFilter{Status: "active"}
	if verifyType != "all" {
		filter.EntityType = mapVerifyTypeToStore(verifyType)
	}

	entities, err := storeDB.QueryEntities(filter)
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		// Return empty verification result
		verifyOut := &VerifyOutput{
			Verification: &VerificationData{
				Status:          "passed",
				EntitiesChecked: 0,
				Valid:           0,
				Drifted:         0,
				Missing:         0,
				Issues:          []VerifyIssue{},
			},
		}

		formatter, err := output.GetFormatter(output.FormatYAML)
		if err != nil {
			return fmt.Errorf("failed to get formatter: %w", err)
		}

		return formatter.FormatToWriter(cmd.OutOrStdout(), verifyOut, output.DensityMedium)
	}

	result := &verifyResult{}

	// Group entities by file for efficient parsing
	byFile := groupByFileStore(entities)

	// Get base directory (working directory)
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}

	for filePath, fileEntities := range byFile {
		// Resolve file path
		absPath := filePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(baseDir, filePath)
		}

		// Check if file exists
		content, err := os.ReadFile(absPath)
		if err != nil {
			// File missing
			for _, entity := range fileEntities {
				e := entity
				result.missing = append(result.missing, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "file deleted",
					detail: fmt.Sprintf("File not found: %s", filePath),
				})
			}
			continue
		}

		// Parse file
		p, err := parser.NewParser(parser.Go)
		if err != nil {
			// Parser creation error
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "parse error",
					detail: fmt.Sprintf("Cannot create parser: %v", err),
				})
			}
			continue
		}

		parseResult, err := p.Parse(content)
		p.Close()

		if err != nil {
			// Parse error - mark all as drifted
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "parse error",
					detail: fmt.Sprintf("Parse failed: %v", err),
				})
			}
			continue
		}

		// Extract current entities
		extractor := extract.NewExtractor(parseResult)
		currentEntities, err := extractor.ExtractAll()
		if err != nil {
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "extract error",
					detail: fmt.Sprintf("Extraction failed: %v", err),
				})
			}
			continue
		}

		// Build lookup by name for current entities
		currentMap := buildEntityLookup(currentEntities)

		// Verify each stored entity
		for _, stored := range fileEntities {
			s := stored
			current := findMatchingEntityByName(stored.Name, currentMap)

			if current == nil {
				result.missing = append(result.missing, verifyEntry{
					entity: s,
					file:   filePath,
					line:   fmt.Sprintf("%d", s.LineStart),
					reason: "symbol not found",
					detail: "Symbol no longer exists in file",
				})
				continue
			}

			// Compare hashes - store entity has structured hash fields
			entry := verifyEntry{
				entity:  s,
				file:    filePath,
				line:    fmt.Sprintf("%d", s.LineStart),
				oldSig:  s.SigHash,
				newSig:  current.SigHash,
				oldBody: s.BodyHash,
				newBody: current.BodyHash,
			}

			if s.SigHash == "" && s.BodyHash == "" {
				entry.reason = "no stored hashes"
				entry.detail = "Entity has no stored hashes"
				result.drifted = append(result.drifted, entry)
			} else if s.SigHash != current.SigHash {
				entry.reason = "sig_hash changed"
				entry.detail = "Parameters or return types changed"
				result.drifted = append(result.drifted, entry)
			} else if s.BodyHash != current.BodyHash {
				entry.reason = "body_hash changed"
				entry.detail = "Logic changed"
				result.drifted = append(result.drifted, entry)
			} else {
				result.valid = append(result.valid, entry)
			}
		}

		parseResult.Close()
	}

	// Fix drifted if requested (or show what would be fixed in dry-run mode)
	if (verifyFix || verifyDryRun) && len(result.drifted) > 0 {
		if verifyDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "\n[dry-run] Would fix %d drifted entities:\n", len(result.drifted))
		}
		for _, entry := range result.drifted {
			if entry.newSig != "" && entry.newBody != "" {
				if verifyDryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run]   - %s (%s)\n",
						entry.entity.Name, entry.entity.ID)
				} else {
					entry.entity.SigHash = entry.newSig
					entry.entity.BodyHash = entry.newBody
					if err := storeDB.UpdateEntity(entry.entity); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to update %s: %v\n",
							entry.entity.ID, err)
					}
				}
			}
		}
		if verifyDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] No changes made\n\n")
		}
	}

	// Build issues list for output
	issues := []VerifyIssue{}

	// Add drifted issues
	for _, entry := range result.drifted {
		issue := VerifyIssue{
			Entity:   entry.entity.Name,
			Type:     "drifted",
			Location: formatStoreLocation(entry.entity),
			Reason:   entry.reason,
			Detail:   entry.detail,
		}

		// Add hash comparison if available
		if entry.reason == "sig_hash changed" && entry.oldSig != "" && entry.newSig != "" {
			issue.HashType = "signature"
			issue.Expected = entry.oldSig
			issue.Actual = entry.newSig
		} else if entry.reason == "body_hash changed" && entry.oldBody != "" && entry.newBody != "" {
			issue.HashType = "body"
			issue.Expected = entry.oldBody
			issue.Actual = entry.newBody
		}

		issues = append(issues, issue)
	}

	// Add missing issues
	for _, entry := range result.missing {
		issue := VerifyIssue{
			Entity:   entry.entity.Name,
			Type:     "missing",
			Location: formatStoreLocation(entry.entity),
			Reason:   entry.reason,
			Detail:   entry.detail,
		}
		issues = append(issues, issue)
	}

	// Determine overall status
	status := "passed"
	if len(result.drifted) > 0 || len(result.missing) > 0 {
		status = "failed"
	}

	// Build actions
	actions := []string{}
	if len(result.drifted) > 0 || len(result.missing) > 0 {
		if verifyFix {
			actions = append(actions, "Fixed drifted entities")
		} else {
			actions = append(actions, "Run `cx verify --fix` to update hashes for drifted entities")
		}
		actions = append(actions, "Run `cx scan --force` to re-scan the codebase")
	}

	// Create output
	verifyOut := &VerifyOutput{
		Verification: &VerificationData{
			Status:          status,
			EntitiesChecked: len(result.valid) + len(result.drifted) + len(result.missing),
			Valid:           len(result.valid),
			Drifted:         len(result.drifted),
			Missing:         len(result.missing),
			Issues:          issues,
			Actions:         actions,
		},
	}

	// Output in YAML format
	formatter, err := output.GetFormatter(output.FormatYAML)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	if err := formatter.FormatToWriter(cmd.OutOrStdout(), verifyOut, output.DensityMedium); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Create beads task if requested and there are issues
	if verifyCreateTask && (len(result.drifted) > 0 || len(result.missing) > 0) {
		if !integration.BeadsAvailable() {
			return fmt.Errorf("--create-task requires beads integration (bd CLI and .beads/ directory)")
		}

		// Build task title and description
		title := fmt.Sprintf("Verify: %d drifted, %d missing entities need attention",
			len(result.drifted), len(result.missing))

		var desc strings.Builder
		desc.WriteString("## Verification Results\n\n")
		desc.WriteString(fmt.Sprintf("- **Drifted**: %d entities\n", len(result.drifted)))
		desc.WriteString(fmt.Sprintf("- **Missing**: %d entities\n", len(result.missing)))
		desc.WriteString(fmt.Sprintf("- **Valid**: %d entities\n\n", len(result.valid)))

		// List drifted entities (top 10)
		if len(result.drifted) > 0 {
			desc.WriteString("### Drifted Entities\n")
			for i, e := range result.drifted {
				if i >= 10 {
					desc.WriteString(fmt.Sprintf("... and %d more\n", len(result.drifted)-10))
					break
				}
				desc.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", e.entity.Name, e.file, e.reason))
			}
			desc.WriteString("\n")
		}

		// List missing entities (top 10)
		if len(result.missing) > 0 {
			desc.WriteString("### Missing Entities\n")
			for i, e := range result.missing {
				if i >= 10 {
					desc.WriteString(fmt.Sprintf("... and %d more\n", len(result.missing)-10))
					break
				}
				desc.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", e.entity.Name, e.file, e.reason))
			}
			desc.WriteString("\n")
		}

		desc.WriteString("### Suggested Actions\n")
		desc.WriteString("- Run `cx scan --force` to update the database\n")
		desc.WriteString("- Run `cx verify --fix` to update hashes for drifted entities\n")

		// Determine priority based on severity
		priority := 2 // Medium
		if len(result.missing) > 10 || len(result.drifted) > 20 {
			priority = 1 // High
		}

		// Create the task
		opts := integration.CreateBeadOptions{
			Title:       title,
			Description: desc.String(),
			Type:        "task",
			Priority:    priority,
			Labels:      []string{"cx:verify", "cx:stale"},
		}

		beadID, err := integration.CreateBead(opts)
		if err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n# Created task: %s\n", beadID)
	}

	// Exit with error if strict mode and drift detected
	if verifyStrict && (len(result.drifted) > 0 || len(result.missing) > 0) {
		return fmt.Errorf("verification failed: %d drifted, %d missing",
			len(result.drifted), len(result.missing))
	}

	return nil
}

// mapVerifyTypeToStore converts verify --type flag to store entity type
func mapVerifyTypeToStore(t string) string {
	switch strings.ToLower(t) {
	case "function", "func", "f":
		return "function"
	case "type", "t":
		return "type"
	default:
		return ""
	}
}

// groupByFileStore groups entities by their source file
func groupByFileStore(entities []*store.Entity) map[string][]*store.Entity {
	result := make(map[string][]*store.Entity)
	for _, e := range entities {
		if e.FilePath != "" {
			result[e.FilePath] = append(result[e.FilePath], e)
		}
	}
	return result
}

// buildEntityLookup builds a map from entity name to extracted entity
func buildEntityLookup(entities []extract.Entity) map[string]*extract.Entity {
	result := make(map[string]*extract.Entity)

	for i := range entities {
		e := &entities[i]
		// Key by name (primary lookup)
		result[e.Name] = e
		// Also key by name with receiver for methods
		if e.Receiver != "" {
			result[e.Receiver+"."+e.Name] = e
		}
	}

	return result
}

// findMatchingEntityByName finds the current entity matching a stored entity by name
func findMatchingEntityByName(name string, currentMap map[string]*extract.Entity) *extract.Entity {
	if current, ok := currentMap[name]; ok {
		return current
	}
	return nil
}

