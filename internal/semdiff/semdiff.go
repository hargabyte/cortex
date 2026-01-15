// Package semdiff provides semantic diff analysis for code changes.
//
// Unlike line-level diffs, semantic diff understands the structural meaning
// of code changes - distinguishing signature changes (breaking) from body
// changes (non-breaking), and identifying affected callers.
package semdiff

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
)

// ChangeType classifies the kind of semantic change.
type ChangeType string

const (
	// ChangeAdded indicates a new entity was added.
	ChangeAdded ChangeType = "added"
	// ChangeRemoved indicates an entity was removed.
	ChangeRemoved ChangeType = "removed"
	// ChangeSignature indicates the signature changed (breaking).
	ChangeSignature ChangeType = "signature_change"
	// ChangeBody indicates only the body changed (non-breaking).
	ChangeBody ChangeType = "body_change"
)

// SemanticChange represents a single semantic change to a code entity.
type SemanticChange struct {
	// Name is the entity name (qualified if method).
	Name string `yaml:"name" json:"name"`
	// Type is the entity type (function, method, struct, etc.).
	Type string `yaml:"type" json:"type"`
	// Location is the file:line location.
	Location string `yaml:"location" json:"location"`
	// ChangeType classifies the change.
	ChangeType ChangeType `yaml:"change_type" json:"change_type"`
	// Breaking indicates if this is a breaking change.
	Breaking bool `yaml:"breaking" json:"breaking"`
	// AffectedCallers is the count of entities that call this one.
	AffectedCallers int `yaml:"affected_callers,omitempty" json:"affected_callers,omitempty"`
	// CallerNames lists the names of affected callers (limited).
	CallerNames []string `yaml:"caller_names,omitempty" json:"caller_names,omitempty"`
	// OldSignature is the previous signature (for signature changes).
	OldSignature string `yaml:"old_signature,omitempty" json:"old_signature,omitempty"`
	// NewSignature is the new signature (for signature changes).
	NewSignature string `yaml:"new_signature,omitempty" json:"new_signature,omitempty"`
}

// SemanticDiff represents the complete semantic diff result.
type SemanticDiff struct {
	// Summary contains aggregate statistics.
	Summary SemanticSummary `yaml:"summary" json:"summary"`
	// Changes lists all semantic changes.
	Changes []SemanticChange `yaml:"changes" json:"changes"`
}

// SemanticSummary contains aggregate statistics about the diff.
type SemanticSummary struct {
	// TotalChanges is the total number of semantic changes.
	TotalChanges int `yaml:"total_changes" json:"total_changes"`
	// BreakingChanges is the count of breaking changes.
	BreakingChanges int `yaml:"breaking_changes" json:"breaking_changes"`
	// Added is the count of added entities.
	Added int `yaml:"added" json:"added"`
	// Removed is the count of removed entities.
	Removed int `yaml:"removed" json:"removed"`
	// SignatureChanges is the count of signature changes.
	SignatureChanges int `yaml:"signature_changes" json:"signature_changes"`
	// BodyChanges is the count of body-only changes.
	BodyChanges int `yaml:"body_changes" json:"body_changes"`
	// TotalAffectedCallers is the total count of affected callers across all changes.
	TotalAffectedCallers int `yaml:"total_affected_callers" json:"total_affected_callers"`
}

// Analyzer performs semantic diff analysis.
type Analyzer struct {
	store       *store.Store
	projectRoot string
	cfg         *config.Config
}

// NewAnalyzer creates a new semantic diff analyzer.
func NewAnalyzer(s *store.Store, projectRoot string, cfg *config.Config) *Analyzer {
	return &Analyzer{
		store:       s,
		projectRoot: projectRoot,
		cfg:         cfg,
	}
}

// Analyze performs semantic diff analysis on changed files.
// If filePath is non-empty, only analyzes that file/directory.
func (a *Analyzer) Analyze(filePath string) (*SemanticDiff, error) {
	// Get stored file entries
	fileEntries, err := a.store.GetAllFileEntries()
	if err != nil {
		return nil, fmt.Errorf("failed to get file entries: %w", err)
	}

	if len(fileEntries) == 0 {
		return nil, fmt.Errorf("no scan data found: run 'cx scan' first")
	}

	// Build map of stored file hashes
	storedHashes := make(map[string]string)
	for _, entry := range fileEntries {
		if filePath != "" && !matchesPath(entry.FilePath, filePath) {
			continue
		}
		storedHashes[entry.FilePath] = entry.ScanHash
	}

	var changes []SemanticChange

	// Check stored files for modifications and deletions
	for fp, storedHash := range storedHashes {
		absPath := filepath.Join(a.projectRoot, fp)
		content, err := os.ReadFile(absPath)
		if err != nil {
			// File deleted - mark all entities as removed
			removedChanges, err := a.analyzeRemovedFile(fp)
			if err != nil {
				continue
			}
			changes = append(changes, removedChanges...)
			continue
		}

		// Check if file content changed
		currentHash := extract.ComputeFileHash(content)
		if currentHash != storedHash {
			// File modified - do semantic analysis
			fileChanges, err := a.analyzeModifiedFile(fp, content)
			if err != nil {
				continue
			}
			changes = append(changes, fileChanges...)
		}
	}

	// Check for new files
	newFileChanges, err := a.findNewFiles(storedHashes, filePath)
	if err != nil {
		return nil, err
	}
	changes = append(changes, newFileChanges...)

	// Build summary
	summary := buildSummary(changes)

	return &SemanticDiff{
		Summary: summary,
		Changes: changes,
	}, nil
}

// analyzeRemovedFile analyzes a deleted file and returns removal changes.
func (a *Analyzer) analyzeRemovedFile(filePath string) ([]SemanticChange, error) {
	entities, err := a.store.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})
	if err != nil {
		return nil, err
	}

	var changes []SemanticChange
	for _, e := range entities {
		// Skip imports - they're not interesting for semantic diff
		if e.EntityType == "import" {
			continue
		}

		// Get callers to determine if removal is safe
		callers, callerNames := a.getCallers(e.ID)

		change := SemanticChange{
			Name:            e.Name,
			Type:            e.EntityType,
			Location:        formatStoreLocation(e),
			ChangeType:      ChangeRemoved,
			Breaking:        callers > 0, // Breaking if anything calls it
			AffectedCallers: callers,
			CallerNames:     callerNames,
		}
		changes = append(changes, change)
	}

	return changes, nil
}

// analyzeModifiedFile performs semantic analysis on a modified file.
func (a *Analyzer) analyzeModifiedFile(filePath string, content []byte) ([]SemanticChange, error) {
	// Get stored entities for this file
	storedEntities, err := a.store.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})
	if err != nil {
		return nil, err
	}

	// Build map by name for lookup
	storedByName := make(map[string]*store.Entity)
	for _, e := range storedEntities {
		storedByName[e.Name] = e
	}

	// Extract current entities from the file
	currentEntities, err := a.extractCurrentEntities(filePath, content)
	if err != nil {
		return nil, err
	}

	// Build map of current entities by name
	currentByName := make(map[string]*extract.Entity)
	for i := range currentEntities {
		e := &currentEntities[i]
		currentByName[e.Name] = e
	}

	var changes []SemanticChange

	// Check for modified and removed entities
	for name, stored := range storedByName {
		// Skip imports
		if stored.EntityType == "import" {
			continue
		}

		current, exists := currentByName[name]
		if !exists {
			// Entity removed
			callers, callerNames := a.getCallers(stored.ID)
			changes = append(changes, SemanticChange{
				Name:            stored.Name,
				Type:            stored.EntityType,
				Location:        formatStoreLocation(stored),
				ChangeType:      ChangeRemoved,
				Breaking:        callers > 0,
				AffectedCallers: callers,
				CallerNames:     callerNames,
			})
			continue
		}

		// Entity exists - check for changes
		change := a.compareEntities(stored, current)
		if change != nil {
			changes = append(changes, *change)
		}
	}

	// Check for added entities
	for name, current := range currentByName {
		// Skip imports
		if current.Kind == extract.ImportEntity {
			continue
		}

		if _, exists := storedByName[name]; !exists {
			changes = append(changes, SemanticChange{
				Name:       current.Name,
				Type:       string(current.Kind),
				Location:   fmt.Sprintf("%s:%d", current.File, current.StartLine),
				ChangeType: ChangeAdded,
				Breaking:   false, // Additions are never breaking
			})
		}
	}

	return changes, nil
}

// extractCurrentEntities extracts entities from file content.
func (a *Analyzer) extractCurrentEntities(filePath string, content []byte) ([]extract.Entity, error) {
	lang := detectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}

	// Get the parser for this language
	p, err := parser.NewParser(parser.Language(lang))
	if err != nil {
		return nil, fmt.Errorf("unsupported language %s: %w", lang, err)
	}

	// Parse the file
	result, err := p.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer result.Close()

	result.FilePath = filePath

	// Extract entities based on language
	var entities []extract.Entity
	switch p.Language() {
	case parser.Go:
		extractor := extract.NewExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Python:
		extractor := extract.NewPythonExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.TypeScript, parser.JavaScript:
		extractor := extract.NewTypeScriptExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Rust:
		extractor := extract.NewRustExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Java:
		extractor := extract.NewJavaExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.C:
		extractor := extract.NewCExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Cpp:
		extractor := extract.NewCppExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.CSharp:
		extractor := extract.NewCSharpExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.PHP:
		extractor := extract.NewPHPExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Ruby:
		extractor := extract.NewRubyExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	case parser.Kotlin:
		extractor := extract.NewKotlinExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	default:
		// Fall back to Go extractor
		extractor := extract.NewExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	}

	if err != nil {
		return nil, err
	}

	// Normalize file path and compute hashes
	for i := range entities {
		entities[i].File = filePath
		entities[i].ComputeHashes()
	}

	return entities, nil
}

// compareEntities compares stored and current entity, returning a change if different.
func (a *Analyzer) compareEntities(stored *store.Entity, current *extract.Entity) *SemanticChange {
	// Compare signature hashes
	sigChanged := stored.SigHash != current.SigHash
	bodyChanged := stored.BodyHash != current.BodyHash

	if !sigChanged && !bodyChanged {
		return nil // No change
	}

	callers, callerNames := a.getCallers(stored.ID)

	if sigChanged {
		return &SemanticChange{
			Name:            stored.Name,
			Type:            stored.EntityType,
			Location:        formatStoreLocation(stored),
			ChangeType:      ChangeSignature,
			Breaking:        true, // Signature changes are always breaking
			AffectedCallers: callers,
			CallerNames:     callerNames,
			OldSignature:    stored.Signature,
			NewSignature:    current.FormatSignature(),
		}
	}

	// Body-only change
	return &SemanticChange{
		Name:            stored.Name,
		Type:            stored.EntityType,
		Location:        formatStoreLocation(stored),
		ChangeType:      ChangeBody,
		Breaking:        false, // Body changes are not breaking
		AffectedCallers: callers,
		CallerNames:     callerNames,
	}
}

// getCallers returns the count and names of entities that call the given entity.
func (a *Analyzer) getCallers(entityID string) (int, []string) {
	deps, err := a.store.GetDependenciesTo(entityID)
	if err != nil {
		return 0, nil
	}

	var callerNames []string
	callCount := 0
	for _, dep := range deps {
		if dep.DepType == "calls" {
			callCount++
			// Get caller entity name
			caller, err := a.store.GetEntity(dep.FromID)
			if err == nil && caller != nil {
				callerNames = append(callerNames, caller.Name)
			}
		}
	}

	// Limit caller names to first 5
	if len(callerNames) > 5 {
		callerNames = callerNames[:5]
	}

	return callCount, callerNames
}

// findNewFiles finds new source files not in the stored hashes.
func (a *Analyzer) findNewFiles(storedHashes map[string]string, filterPath string) ([]SemanticChange, error) {
	var changes []SemanticChange

	err := filepath.Walk(a.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == ".cx" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check source files
		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}

		relPath, _ := filepath.Rel(a.projectRoot, path)

		// Apply filter if specified
		if filterPath != "" && !matchesPath(relPath, filterPath) {
			return nil
		}

		// Check if this file is new
		if _, exists := storedHashes[relPath]; !exists {
			// New file - extract entities and mark as added
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			entities, err := a.extractCurrentEntities(relPath, content)
			if err != nil {
				return nil
			}

			for _, e := range entities {
				if e.Kind == extract.ImportEntity {
					continue
				}
				changes = append(changes, SemanticChange{
					Name:       e.Name,
					Type:       string(e.Kind),
					Location:   fmt.Sprintf("%s:%d", e.File, e.StartLine),
					ChangeType: ChangeAdded,
					Breaking:   false,
				})
			}
		}

		return nil
	})

	return changes, err
}

// buildSummary builds aggregate statistics from changes.
func buildSummary(changes []SemanticChange) SemanticSummary {
	summary := SemanticSummary{
		TotalChanges: len(changes),
	}

	for _, c := range changes {
		if c.Breaking {
			summary.BreakingChanges++
		}
		summary.TotalAffectedCallers += c.AffectedCallers

		switch c.ChangeType {
		case ChangeAdded:
			summary.Added++
		case ChangeRemoved:
			summary.Removed++
		case ChangeSignature:
			summary.SignatureChanges++
		case ChangeBody:
			summary.BodyChanges++
		}
	}

	return summary
}

// formatStoreLocation formats entity location as file:line.
func formatStoreLocation(e *store.Entity) string {
	if e.LineEnd != nil && *e.LineEnd > e.LineStart {
		return fmt.Sprintf("%s:%d-%d", e.FilePath, e.LineStart, *e.LineEnd)
	}
	return fmt.Sprintf("%s:%d", e.FilePath, e.LineStart)
}

// matchesPath checks if filePath matches or is under filterPath.
func matchesPath(filePath, filterPath string) bool {
	if filePath == filterPath {
		return true
	}
	// Check if filePath is under filterPath directory
	if len(filePath) > len(filterPath) && filePath[:len(filterPath)] == filterPath {
		next := filePath[len(filterPath)]
		return next == '/' || next == filepath.Separator
	}
	return false
}

// detectLanguage detects programming language from file extension.
func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb", ".rake":
		return "ruby"
	case ".kt", ".kts":
		return "kotlin"
	default:
		return ""
	}
}
