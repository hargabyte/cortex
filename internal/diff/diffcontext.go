// Package diff provides diff-based context assembly for cx.
package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/semdiff"
	"github.com/anthropics/cx/internal/store"
)

// DiffContextOptions configures diff-based context assembly.
type DiffContextOptions struct {
	// Staged only analyzes staged changes.
	Staged bool
	// CommitRange specifies a commit range (e.g., "HEAD~3").
	CommitRange string
	// Budget is the token budget for context output.
	Budget int
	// Depth is the max hops to trace from modified entities.
	Depth int
	// IncludeCallers includes entities that call modified entities.
	IncludeCallers bool
	// IncludeCallees includes entities called by modified entities.
	IncludeCallees bool
}

// DefaultDiffContextOptions returns sensible defaults.
func DefaultDiffContextOptions() DiffContextOptions {
	return DiffContextOptions{
		Budget:         8000,
		Depth:          2,
		IncludeCallers: true,
		IncludeCallees: true,
	}
}

// EntityChange represents a semantic change to an entity.
type EntityChange struct {
	// Name is the entity name.
	Name string `yaml:"name" json:"name"`
	// Type is the entity type (function, method, struct, etc.).
	Type string `yaml:"type" json:"type"`
	// Location is the file:line location.
	Location string `yaml:"location" json:"location"`
	// ChangeType is the kind of change (added, removed, signature_change, body_change).
	ChangeType string `yaml:"change_type" json:"change_type"`
	// Breaking indicates if this is a breaking change.
	Breaking bool `yaml:"breaking,omitempty" json:"breaking,omitempty"`
	// EntityID is the entity ID for graph lookups.
	EntityID string `yaml:"entity_id,omitempty" json:"entity_id,omitempty"`
	// OldSignature is the previous signature (for signature changes).
	OldSignature string `yaml:"old_signature,omitempty" json:"old_signature,omitempty"`
	// NewSignature is the new signature (for signature changes).
	NewSignature string `yaml:"new_signature,omitempty" json:"new_signature,omitempty"`
}

// AffectedCaller represents an entity affected by changes.
type AffectedCaller struct {
	// Name is the caller entity name.
	Name string `yaml:"name" json:"name"`
	// Type is the entity type.
	Type string `yaml:"type" json:"type"`
	// Location is the file:line location.
	Location string `yaml:"location" json:"location"`
	// CallsModified lists the modified entities this caller depends on.
	CallsModified []string `yaml:"calls_modified" json:"calls_modified"`
	// EntityID is the entity ID.
	EntityID string `yaml:"entity_id,omitempty" json:"entity_id,omitempty"`
}

// DiffContextResult contains the assembled diff-based context.
type DiffContextResult struct {
	// Summary contains aggregate statistics.
	Summary *DiffContextSummary `yaml:"summary" json:"summary"`
	// ChangedFiles lists the files with changes.
	ChangedFiles []FileChange `yaml:"changed_files" json:"changed_files"`
	// EntitiesModified lists entities with modifications.
	EntitiesModified []EntityChange `yaml:"entities_modified,omitempty" json:"entities_modified,omitempty"`
	// EntitiesAdded lists newly added entities.
	EntitiesAdded []EntityChange `yaml:"entities_added,omitempty" json:"entities_added,omitempty"`
	// EntitiesRemoved lists removed entities.
	EntitiesRemoved []EntityChange `yaml:"entities_removed,omitempty" json:"entities_removed,omitempty"`
	// CallersAffected lists entities that call modified entities.
	CallersAffected []AffectedCaller `yaml:"callers_affected,omitempty" json:"callers_affected,omitempty"`
	// TokensUsed is the estimated token count.
	TokensUsed int `yaml:"tokens_used" json:"tokens_used"`
	// TokensBudget is the configured budget.
	TokensBudget int `yaml:"tokens_budget" json:"tokens_budget"`
	// Warnings contains any warnings during analysis.
	Warnings []string `yaml:"warnings,omitempty" json:"warnings,omitempty"`
}

// DiffContextSummary contains aggregate statistics.
type DiffContextSummary struct {
	// FilesChanged is the number of changed files.
	FilesChanged int `yaml:"files_changed" json:"files_changed"`
	// EntitiesModified is the count of modified entities.
	EntitiesModified int `yaml:"entities_modified" json:"entities_modified"`
	// EntitiesAdded is the count of added entities.
	EntitiesAdded int `yaml:"entities_added" json:"entities_added"`
	// EntitiesRemoved is the count of removed entities.
	EntitiesRemoved int `yaml:"entities_removed" json:"entities_removed"`
	// SignatureChanges is the count of signature changes.
	SignatureChanges int `yaml:"signature_changes" json:"signature_changes"`
	// BodyChanges is the count of body-only changes.
	BodyChanges int `yaml:"body_changes" json:"body_changes"`
	// BreakingChanges is the count of breaking changes.
	BreakingChanges int `yaml:"breaking_changes" json:"breaking_changes"`
	// TotalCallersAffected is the count of affected callers.
	TotalCallersAffected int `yaml:"total_callers_affected" json:"total_callers_affected"`
	// Mode indicates the diff mode (staged, uncommitted, commit-range).
	Mode string `yaml:"mode" json:"mode"`
}

// DiffContext assembles context based on git diff.
type DiffContext struct {
	store       *store.Store
	graph       *graph.Graph
	projectRoot string
	cfg         *config.Config
	options     DiffContextOptions
}

// NewDiffContext creates a new diff-based context assembler.
func NewDiffContext(s *store.Store, g *graph.Graph, projectRoot string, cfg *config.Config, opts DiffContextOptions) *DiffContext {
	if opts.Budget <= 0 {
		opts.Budget = 8000
	}
	if opts.Depth <= 0 {
		opts.Depth = 2
	}

	return &DiffContext{
		store:       s,
		graph:       g,
		projectRoot: projectRoot,
		cfg:         cfg,
		options:     opts,
	}
}

// Assemble builds the diff-based context.
func (dc *DiffContext) Assemble() (*DiffContextResult, error) {
	result := &DiffContextResult{
		Summary:      &DiffContextSummary{},
		TokensBudget: dc.options.Budget,
	}

	// Get changed files based on options
	gitDiff := NewGitDiff(dc.projectRoot)
	var changedFiles []FileChange
	var err error

	if dc.options.Staged {
		changedFiles, err = gitDiff.GetStagedChanges()
		result.Summary.Mode = "staged"
	} else if dc.options.CommitRange != "" {
		changedFiles, err = gitDiff.GetCommitRangeChanges(dc.options.CommitRange)
		result.Summary.Mode = "commit-range:" + dc.options.CommitRange
	} else {
		changedFiles, err = gitDiff.GetUncommittedChanges()
		result.Summary.Mode = "uncommitted"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		result.Warnings = append(result.Warnings, "No changed files detected")
		return result, nil
	}

	result.ChangedFiles = changedFiles
	result.Summary.FilesChanged = len(changedFiles)

	// Analyze each changed file
	modifiedEntityIDs := make(map[string]bool)

	for _, fc := range changedFiles {
		switch fc.Status {
		case "D":
			// Deleted file - mark all entities as removed
			removedChanges := dc.analyzeRemovedFile(fc.Path)
			result.EntitiesRemoved = append(result.EntitiesRemoved, removedChanges...)
			result.Summary.EntitiesRemoved += len(removedChanges)
			for _, rc := range removedChanges {
				if rc.EntityID != "" {
					modifiedEntityIDs[rc.EntityID] = true
				}
			}

		case "A":
			// Added file - mark all entities as added
			addedChanges := dc.analyzeAddedFile(fc.Path)
			result.EntitiesAdded = append(result.EntitiesAdded, addedChanges...)
			result.Summary.EntitiesAdded += len(addedChanges)

		default:
			// Modified file - semantic diff
			modified, added, removed := dc.analyzeModifiedFile(fc.Path)
			result.EntitiesModified = append(result.EntitiesModified, modified...)
			result.EntitiesAdded = append(result.EntitiesAdded, added...)
			result.EntitiesRemoved = append(result.EntitiesRemoved, removed...)

			result.Summary.EntitiesModified += len(modified)
			result.Summary.EntitiesAdded += len(added)
			result.Summary.EntitiesRemoved += len(removed)

			// Track modified entity IDs for caller tracing
			for _, m := range modified {
				if m.EntityID != "" {
					modifiedEntityIDs[m.EntityID] = true
				}
				if m.ChangeType == "signature_change" {
					result.Summary.SignatureChanges++
					result.Summary.BreakingChanges++
				} else if m.ChangeType == "body_change" {
					result.Summary.BodyChanges++
				}
			}
			for _, r := range removed {
				if r.EntityID != "" {
					modifiedEntityIDs[r.EntityID] = true
				}
				if r.Breaking {
					result.Summary.BreakingChanges++
				}
			}
		}
	}

	// Trace callers of modified entities
	if dc.options.IncludeCallers && len(modifiedEntityIDs) > 0 {
		callers := dc.traceCallers(modifiedEntityIDs)
		result.CallersAffected = callers
		result.Summary.TotalCallersAffected = len(callers)
	}

	// Estimate tokens
	result.TokensUsed = dc.estimateTokens(result)

	return result, nil
}

// analyzeRemovedFile returns entity changes for a deleted file.
func (dc *DiffContext) analyzeRemovedFile(filePath string) []EntityChange {
	entities, err := dc.store.QueryEntities(store.EntityFilter{
		FilePath: filePath,
		Status:   "active",
	})
	if err != nil {
		return nil
	}

	var changes []EntityChange
	for _, e := range entities {
		if e.EntityType == "import" {
			continue
		}

		// Check if entity has callers (breaking if so)
		callerCount := len(dc.graph.Predecessors(e.ID))

		changes = append(changes, EntityChange{
			Name:       e.Name,
			Type:       e.EntityType,
			Location:   formatLocation(e),
			ChangeType: "removed",
			Breaking:   callerCount > 0,
			EntityID:   e.ID,
		})
	}

	return changes
}

// analyzeAddedFile returns entity changes for a new file.
func (dc *DiffContext) analyzeAddedFile(filePath string) []EntityChange {
	absPath := filepath.Join(dc.projectRoot, filePath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	entities, err := dc.extractEntitiesFromContent(filePath, content)
	if err != nil {
		return nil
	}

	var changes []EntityChange
	for _, e := range entities {
		if e.Kind == extract.ImportEntity {
			continue
		}

		changes = append(changes, EntityChange{
			Name:       e.Name,
			Type:       string(e.Kind),
			Location:   fmt.Sprintf("%s:%d", filePath, e.StartLine),
			ChangeType: "added",
			Breaking:   false,
		})
	}

	return changes
}

// analyzeModifiedFile returns entity changes for a modified file.
func (dc *DiffContext) analyzeModifiedFile(filePath string) (modified, added, removed []EntityChange) {
	absPath := filepath.Join(dc.projectRoot, filePath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, nil
	}

	// Get stored entities
	storedEntities, err := dc.store.QueryEntities(store.EntityFilter{
		FilePath: filePath,
		Status:   "active",
	})
	if err != nil {
		return nil, nil, nil
	}

	storedByName := make(map[string]*store.Entity)
	for _, e := range storedEntities {
		storedByName[e.Name] = e
	}

	// Extract current entities
	currentEntities, err := dc.extractEntitiesFromContent(filePath, content)
	if err != nil {
		return nil, nil, nil
	}

	currentByName := make(map[string]*extract.Entity)
	for i := range currentEntities {
		e := &currentEntities[i]
		currentByName[e.Name] = e
	}

	// Compare stored vs current
	for name, stored := range storedByName {
		if stored.EntityType == "import" {
			continue
		}

		current, exists := currentByName[name]
		if !exists {
			// Entity removed
			callerCount := len(dc.graph.Predecessors(stored.ID))
			removed = append(removed, EntityChange{
				Name:       stored.Name,
				Type:       stored.EntityType,
				Location:   formatLocation(stored),
				ChangeType: "removed",
				Breaking:   callerCount > 0,
				EntityID:   stored.ID,
			})
			continue
		}

		// Compare signatures and bodies
		current.ComputeHashes()

		sigChanged := stored.SigHash != current.SigHash
		bodyChanged := stored.BodyHash != current.BodyHash

		if sigChanged {
			modified = append(modified, EntityChange{
				Name:         stored.Name,
				Type:         stored.EntityType,
				Location:     formatLocation(stored),
				ChangeType:   "signature_change",
				Breaking:     true,
				EntityID:     stored.ID,
				OldSignature: stored.Signature,
				NewSignature: current.FormatSignature(),
			})
		} else if bodyChanged {
			modified = append(modified, EntityChange{
				Name:       stored.Name,
				Type:       stored.EntityType,
				Location:   formatLocation(stored),
				ChangeType: "body_change",
				Breaking:   false,
				EntityID:   stored.ID,
			})
		}
	}

	// Check for added entities
	for name, current := range currentByName {
		if current.Kind == extract.ImportEntity {
			continue
		}

		if _, exists := storedByName[name]; !exists {
			added = append(added, EntityChange{
				Name:       current.Name,
				Type:       string(current.Kind),
				Location:   fmt.Sprintf("%s:%d", filePath, current.StartLine),
				ChangeType: "added",
				Breaking:   false,
			})
		}
	}

	return modified, added, removed
}

// traceCallers finds entities that call the modified entities.
func (dc *DiffContext) traceCallers(modifiedIDs map[string]bool) []AffectedCaller {
	callerMap := make(map[string]*AffectedCaller)

	for entityID := range modifiedIDs {
		predecessors := dc.graph.Predecessors(entityID)
		for _, predID := range predecessors {
			// Skip if the caller is itself modified
			if modifiedIDs[predID] {
				continue
			}

			// Get the entity that's calling
			caller, err := dc.store.GetEntity(predID)
			if err != nil || caller == nil {
				continue
			}

			// Skip imports
			if caller.EntityType == "import" {
				continue
			}

			// Get the name of what's being called
			calledEntity, err := dc.store.GetEntity(entityID)
			calledName := entityID
			if err == nil && calledEntity != nil {
				calledName = calledEntity.Name
			}

			// Add or update caller entry
			if existing, ok := callerMap[predID]; ok {
				existing.CallsModified = append(existing.CallsModified, calledName)
			} else {
				callerMap[predID] = &AffectedCaller{
					Name:          caller.Name,
					Type:          caller.EntityType,
					Location:      formatLocation(caller),
					CallsModified: []string{calledName},
					EntityID:      predID,
				}
			}
		}
	}

	// Convert map to slice and sort by name
	var callers []AffectedCaller
	for _, caller := range callerMap {
		callers = append(callers, *caller)
	}

	sort.Slice(callers, func(i, j int) bool {
		return callers[i].Name < callers[j].Name
	})

	return callers
}

// extractEntitiesFromContent parses file content and extracts entities.
func (dc *DiffContext) extractEntitiesFromContent(filePath string, content []byte) ([]extract.Entity, error) {
	lang := semdiff.DetectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}

	p, err := parser.NewParser(parser.Language(lang))
	if err != nil {
		return nil, fmt.Errorf("unsupported language %s: %w", lang, err)
	}
	defer p.Close()

	result, err := p.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer result.Close()

	result.FilePath = filePath

	// Extract based on language
	var entities []extract.Entity
	switch p.Language() {
	case parser.Go:
		extractor := extract.NewExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Python:
		extractor := extract.NewPythonExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.TypeScript, parser.JavaScript:
		extractor := extract.NewTypeScriptExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Rust:
		extractor := extract.NewRustExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Java:
		extractor := extract.NewJavaExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.C:
		extractor := extract.NewCExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Cpp:
		extractor := extract.NewCppExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.CSharp:
		extractor := extract.NewCSharpExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.PHP:
		extractor := extract.NewPHPExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Ruby:
		extractor := extract.NewRubyExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Kotlin:
		extractor := extract.NewKotlinExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	default:
		extractor := extract.NewExtractorWithBase(result, dc.projectRoot)
		entities, err = extractor.ExtractAll()
	}

	if err != nil {
		return nil, err
	}

	// Normalize file paths and compute hashes
	for i := range entities {
		entities[i].File = filePath
		entities[i].ComputeHashes()
	}

	return entities, nil
}

// estimateTokens estimates the token count for the result.
func (dc *DiffContext) estimateTokens(result *DiffContextResult) int {
	tokens := 100 // Base overhead

	// Each file change
	tokens += len(result.ChangedFiles) * 20

	// Each entity change
	tokens += len(result.EntitiesModified) * 80
	tokens += len(result.EntitiesAdded) * 50
	tokens += len(result.EntitiesRemoved) * 50

	// Each affected caller
	tokens += len(result.CallersAffected) * 60

	return tokens
}

// formatLocation formats an entity location.
func formatLocation(e *store.Entity) string {
	if e.LineEnd != nil && *e.LineEnd > e.LineStart {
		return fmt.Sprintf("%s:%d-%d", e.FilePath, e.LineStart, *e.LineEnd)
	}
	return fmt.Sprintf("%s:%d", e.FilePath, e.LineStart)
}
