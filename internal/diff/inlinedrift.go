// Package diff provides diff-based context assembly for cx.
package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/semdiff"
	"github.com/anthropics/cx/internal/store"
)

// DriftType classifies the kind of drift detected.
type DriftType string

const (
	// DriftSignature indicates the entity's signature has changed.
	DriftSignature DriftType = "signature"
	// DriftBody indicates the entity's body has changed.
	DriftBody DriftType = "body"
	// DriftMissing indicates the entity no longer exists in the file.
	DriftMissing DriftType = "missing"
	// DriftNew indicates a new entity exists in the file but not in the store.
	DriftNew DriftType = "new"
	// DriftFileMissing indicates the file no longer exists.
	DriftFileMissing DriftType = "file_missing"
)

// EntityDrift represents drift detected for a single entity.
type EntityDrift struct {
	// Name is the entity name.
	Name string `yaml:"name" json:"name"`
	// Type is the entity type (function, method, struct, etc.).
	Type string `yaml:"type" json:"type"`
	// Location is the file:line location.
	Location string `yaml:"location" json:"location"`
	// DriftType classifies the kind of drift.
	DriftType DriftType `yaml:"drift_type" json:"drift_type"`
	// Detail provides additional information about the drift.
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
	// OldSignature is the previous signature (for signature changes).
	OldSignature string `yaml:"old_signature,omitempty" json:"old_signature,omitempty"`
	// NewSignature is the new signature (for signature changes).
	NewSignature string `yaml:"new_signature,omitempty" json:"new_signature,omitempty"`
	// OldSigHash is the previous signature hash.
	OldSigHash string `yaml:"old_sig_hash,omitempty" json:"old_sig_hash,omitempty"`
	// NewSigHash is the new signature hash.
	NewSigHash string `yaml:"new_sig_hash,omitempty" json:"new_sig_hash,omitempty"`
	// OldBodyHash is the previous body hash.
	OldBodyHash string `yaml:"old_body_hash,omitempty" json:"old_body_hash,omitempty"`
	// NewBodyHash is the new body hash.
	NewBodyHash string `yaml:"new_body_hash,omitempty" json:"new_body_hash,omitempty"`
	// EntityID is the stored entity ID.
	EntityID string `yaml:"entity_id,omitempty" json:"entity_id,omitempty"`
	// Breaking indicates if this drift is a breaking change.
	Breaking bool `yaml:"breaking" json:"breaking"`
	// CallerCount is the number of callers affected (for breaking changes).
	CallerCount int `yaml:"caller_count,omitempty" json:"caller_count,omitempty"`
}

// BrokenDependency represents a dependency that may be broken due to drift.
type BrokenDependency struct {
	// CallerName is the name of the entity that calls the drifted entity.
	CallerName string `yaml:"caller_name" json:"caller_name"`
	// CallerLocation is the file:line of the caller.
	CallerLocation string `yaml:"caller_location" json:"caller_location"`
	// CallsEntity is the name of the drifted entity being called.
	CallsEntity string `yaml:"calls_entity" json:"calls_entity"`
	// DriftType is the type of drift on the called entity.
	DriftType DriftType `yaml:"drift_type" json:"drift_type"`
}

// InlineDriftReport contains the complete inline drift analysis.
type InlineDriftReport struct {
	// Summary contains aggregate statistics.
	Summary *InlineDriftSummary `yaml:"summary" json:"summary"`
	// Drifted lists entities with detected drift.
	Drifted []EntityDrift `yaml:"drifted,omitempty" json:"drifted,omitempty"`
	// NewEntities lists entities found in file but not in store.
	NewEntities []EntityDrift `yaml:"new_entities,omitempty" json:"new_entities,omitempty"`
	// BrokenDependencies lists dependencies that may be broken.
	BrokenDependencies []BrokenDependency `yaml:"broken_dependencies,omitempty" json:"broken_dependencies,omitempty"`
	// Warnings contains any warnings during analysis.
	Warnings []string `yaml:"warnings,omitempty" json:"warnings,omitempty"`
	// Recommendations contains suggested actions.
	Recommendations []string `yaml:"recommendations" json:"recommendations"`
}

// InlineDriftSummary contains aggregate statistics about drift.
type InlineDriftSummary struct {
	// Target is the file being analyzed.
	Target string `yaml:"target" json:"target"`
	// Status is the overall status (clean, drifted).
	Status string `yaml:"status" json:"status"`
	// EntitiesChecked is the number of entities analyzed.
	EntitiesChecked int `yaml:"entities_checked" json:"entities_checked"`
	// SignatureChanges is the count of signature changes.
	SignatureChanges int `yaml:"signature_changes" json:"signature_changes"`
	// BodyChanges is the count of body-only changes.
	BodyChanges int `yaml:"body_changes" json:"body_changes"`
	// MissingEntities is the count of entities no longer in file.
	MissingEntities int `yaml:"missing_entities" json:"missing_entities"`
	// NewEntities is the count of new entities not in store.
	NewEntities int `yaml:"new_entities" json:"new_entities"`
	// BreakingChanges is the count of breaking changes.
	BreakingChanges int `yaml:"breaking_changes" json:"breaking_changes"`
	// TotalAffectedCallers is the total callers affected by breaking changes.
	TotalAffectedCallers int `yaml:"total_affected_callers" json:"total_affected_callers"`
}

// InlineDriftAnalyzer performs inline drift analysis for a file.
type InlineDriftAnalyzer struct {
	store       *store.Store
	projectRoot string
}

// NewInlineDriftAnalyzer creates a new inline drift analyzer.
func NewInlineDriftAnalyzer(s *store.Store, projectRoot string) *InlineDriftAnalyzer {
	return &InlineDriftAnalyzer{
		store:       s,
		projectRoot: projectRoot,
	}
}

// ParseWithoutStore parses a file and extracts entities without storing to the database.
// This is the core of cortex-dsr.2.1 - in-memory entity extraction.
func (a *InlineDriftAnalyzer) ParseWithoutStore(filePath string) ([]extract.Entity, error) {
	// Determine absolute path
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(a.projectRoot, filePath)
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect language from file extension
	lang := semdiff.DetectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}

	// Create parser
	p, err := parser.NewParser(parser.Language(lang))
	if err != nil {
		return nil, fmt.Errorf("failed to create parser for %s: %w", lang, err)
	}
	defer p.Close()

	// Parse content
	result, err := p.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
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
		extractor := extract.NewExtractorWithBase(result, a.projectRoot)
		entities, err = extractor.ExtractAll()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	// Normalize file paths and compute hashes
	relPath := filePath
	if filepath.IsAbs(filePath) {
		if rel, err := filepath.Rel(a.projectRoot, filePath); err == nil {
			relPath = rel
		}
	}

	for i := range entities {
		entities[i].File = relPath
		entities[i].ComputeHashes()
	}

	return entities, nil
}

// CompareSignatures compares extracted entities with indexed versions.
// This is the core of cortex-dsr.2.2 - signature comparison.
func (a *InlineDriftAnalyzer) CompareSignatures(filePath string, currentEntities []extract.Entity) ([]EntityDrift, []EntityDrift, error) {
	// Normalize path for store query
	relPath := filePath
	if filepath.IsAbs(filePath) {
		if rel, err := filepath.Rel(a.projectRoot, filePath); err == nil {
			relPath = rel
		}
	}

	// Get stored entities for this file
	storedEntities, err := a.store.QueryEntities(store.EntityFilter{
		FilePath: relPath,
		Status:   "active",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query stored entities: %w", err)
	}

	// Build lookup maps
	storedByName := make(map[string]*store.Entity)
	for _, e := range storedEntities {
		storedByName[e.Name] = e
	}

	currentByName := make(map[string]*extract.Entity)
	for i := range currentEntities {
		e := &currentEntities[i]
		currentByName[e.Name] = e
	}

	var drifted []EntityDrift
	var newEntities []EntityDrift

	// Check for modified and missing entities
	for name, stored := range storedByName {
		// Skip imports
		if stored.EntityType == "import" {
			continue
		}

		current, exists := currentByName[name]
		if !exists {
			// Entity no longer exists - MISSING drift
			drifted = append(drifted, EntityDrift{
				Name:      stored.Name,
				Type:      stored.EntityType,
				Location:  formatStoredLocation(stored),
				DriftType: DriftMissing,
				Detail:    "Entity no longer exists in file",
				EntityID:  stored.ID,
				Breaking:  true, // Missing is always breaking
			})
			continue
		}

		// Compare signatures
		current.ComputeHashes()

		sigChanged := stored.SigHash != "" && current.SigHash != "" && stored.SigHash != current.SigHash
		bodyChanged := stored.BodyHash != "" && current.BodyHash != "" && stored.BodyHash != current.BodyHash

		if sigChanged {
			// Signature changed - breaking change
			drifted = append(drifted, EntityDrift{
				Name:         stored.Name,
				Type:         stored.EntityType,
				Location:     formatStoredLocation(stored),
				DriftType:    DriftSignature,
				Detail:       "Signature has changed (parameters or return types)",
				OldSignature: stored.Signature,
				NewSignature: current.FormatSignature(),
				OldSigHash:   stored.SigHash,
				NewSigHash:   current.SigHash,
				EntityID:     stored.ID,
				Breaking:     true,
			})
		} else if bodyChanged {
			// Body changed - non-breaking change
			drifted = append(drifted, EntityDrift{
				Name:        stored.Name,
				Type:        stored.EntityType,
				Location:    formatStoredLocation(stored),
				DriftType:   DriftBody,
				Detail:      "Implementation has changed",
				OldBodyHash: stored.BodyHash,
				NewBodyHash: current.BodyHash,
				EntityID:    stored.ID,
				Breaking:    false,
			})
		}
	}

	// Check for new entities not in store
	for name, current := range currentByName {
		// Skip imports
		if current.Kind == extract.ImportEntity {
			continue
		}

		if _, exists := storedByName[name]; !exists {
			newEntities = append(newEntities, EntityDrift{
				Name:      current.Name,
				Type:      string(current.Kind),
				Location:  fmt.Sprintf("%s:%d", current.File, current.StartLine),
				DriftType: DriftNew,
				Detail:    "Entity not found in index (new or renamed)",
				Breaking:  false,
			})
		}
	}

	// Sort by entity name for consistent output
	sort.Slice(drifted, func(i, j int) bool {
		return drifted[i].Name < drifted[j].Name
	})
	sort.Slice(newEntities, func(i, j int) bool {
		return newEntities[i].Name < newEntities[j].Name
	})

	return drifted, newEntities, nil
}

// AnalyzeFile performs complete inline drift analysis for a file.
// This is the main entry point for cortex-dsr.2.3 and cortex-dsr.2.4.
func (a *InlineDriftAnalyzer) AnalyzeFile(filePath string) (*InlineDriftReport, error) {
	report := &InlineDriftReport{
		Summary: &InlineDriftSummary{
			Target: filePath,
			Status: "clean",
		},
	}

	// Check if file exists
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(a.projectRoot, filePath)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// File doesn't exist - check if we have stored entities
		relPath := filePath
		if filepath.IsAbs(filePath) {
			if rel, err := filepath.Rel(a.projectRoot, filePath); err == nil {
				relPath = rel
			}
		}

		storedEntities, _ := a.store.QueryEntities(store.EntityFilter{
			FilePath: relPath,
			Status:   "active",
		})

		if len(storedEntities) > 0 {
			// We had entities but file is gone
			for _, stored := range storedEntities {
				if stored.EntityType == "import" {
					continue
				}
				report.Drifted = append(report.Drifted, EntityDrift{
					Name:      stored.Name,
					Type:      stored.EntityType,
					Location:  formatStoredLocation(stored),
					DriftType: DriftFileMissing,
					Detail:    "File no longer exists",
					EntityID:  stored.ID,
					Breaking:  true,
				})
				report.Summary.MissingEntities++
				report.Summary.BreakingChanges++
			}
			report.Summary.Status = "drifted"
			report.Summary.EntitiesChecked = len(storedEntities)
		}

		report.Warnings = append(report.Warnings, fmt.Sprintf("File not found: %s", filePath))
		report.Recommendations = a.buildRecommendations(report)
		return report, nil
	}

	// Parse file without storing to DB
	currentEntities, err := a.ParseWithoutStore(filePath)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Parse error: %v", err))
		report.Recommendations = []string{
			"Fix syntax errors in the file",
			"Check that the file is a supported language",
		}
		return report, nil
	}

	// Compare with stored entities
	drifted, newEntities, err := a.CompareSignatures(filePath, currentEntities)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Comparison error: %v", err))
		report.Recommendations = []string{
			"Run 'cx scan' to rebuild the index",
		}
		return report, nil
	}

	report.Drifted = drifted
	report.NewEntities = newEntities

	// Enrich with caller information for breaking changes
	a.enrichWithCallerInfo(report)

	// Find broken dependencies
	report.BrokenDependencies = a.findBrokenDependencies(report.Drifted)

	// Update summary
	for _, d := range drifted {
		switch d.DriftType {
		case DriftSignature:
			report.Summary.SignatureChanges++
			report.Summary.BreakingChanges++
			report.Summary.TotalAffectedCallers += d.CallerCount
		case DriftBody:
			report.Summary.BodyChanges++
		case DriftMissing, DriftFileMissing:
			report.Summary.MissingEntities++
			report.Summary.BreakingChanges++
			report.Summary.TotalAffectedCallers += d.CallerCount
		}
	}
	report.Summary.NewEntities = len(newEntities)

	// Count total entities checked
	relPath := filePath
	if filepath.IsAbs(filePath) {
		if rel, err := filepath.Rel(a.projectRoot, filePath); err == nil {
			relPath = rel
		}
	}
	storedEntities, _ := a.store.QueryEntities(store.EntityFilter{
		FilePath: relPath,
		Status:   "active",
	})
	report.Summary.EntitiesChecked = len(storedEntities)

	// Set status
	if len(drifted) > 0 || len(newEntities) > 0 {
		report.Summary.Status = "drifted"
	}

	// Build recommendations
	report.Recommendations = a.buildRecommendations(report)

	return report, nil
}

// enrichWithCallerInfo adds caller count information to breaking changes.
func (a *InlineDriftAnalyzer) enrichWithCallerInfo(report *InlineDriftReport) {
	for i := range report.Drifted {
		drift := &report.Drifted[i]
		if !drift.Breaking || drift.EntityID == "" {
			continue
		}

		// Get dependencies (callers)
		deps, err := a.store.GetDependenciesTo(drift.EntityID)
		if err != nil {
			continue
		}

		callerCount := 0
		for _, dep := range deps {
			if dep.DepType == "calls" {
				callerCount++
			}
		}
		drift.CallerCount = callerCount
	}
}

// findBrokenDependencies finds callers that may be affected by drifted entities.
func (a *InlineDriftAnalyzer) findBrokenDependencies(drifted []EntityDrift) []BrokenDependency {
	var broken []BrokenDependency

	for _, drift := range drifted {
		if !drift.Breaking || drift.EntityID == "" {
			continue
		}

		// Get dependencies (callers)
		deps, err := a.store.GetDependenciesTo(drift.EntityID)
		if err != nil {
			continue
		}

		for _, dep := range deps {
			if dep.DepType != "calls" {
				continue
			}

			caller, err := a.store.GetEntity(dep.FromID)
			if err != nil || caller == nil {
				continue
			}

			// Skip imports
			if caller.EntityType == "import" {
				continue
			}

			broken = append(broken, BrokenDependency{
				CallerName:     caller.Name,
				CallerLocation: formatStoredLocation(caller),
				CallsEntity:    drift.Name,
				DriftType:      drift.DriftType,
			})
		}
	}

	// Sort by caller name for consistent output
	sort.Slice(broken, func(i, j int) bool {
		return broken[i].CallerName < broken[j].CallerName
	})

	// Limit to first 20 broken dependencies
	if len(broken) > 20 {
		broken = broken[:20]
	}

	return broken
}

// buildRecommendations generates actionable recommendations based on the drift report.
func (a *InlineDriftAnalyzer) buildRecommendations(report *InlineDriftReport) []string {
	var recs []string

	if report.Summary.Status == "clean" {
		recs = append(recs, "No drift detected - safe to proceed with modifications")
		return recs
	}

	// Breaking changes
	if report.Summary.BreakingChanges > 0 {
		recs = append(recs, fmt.Sprintf("WARNING: %d breaking changes detected", report.Summary.BreakingChanges))
	}

	// Signature changes
	if report.Summary.SignatureChanges > 0 {
		recs = append(recs, fmt.Sprintf("Run 'cx scan' to update index after %d signature changes", report.Summary.SignatureChanges))
		if report.Summary.TotalAffectedCallers > 0 {
			recs = append(recs, fmt.Sprintf("Review %d affected callers for compatibility", report.Summary.TotalAffectedCallers))
		}
	}

	// Missing entities
	if report.Summary.MissingEntities > 0 {
		recs = append(recs, fmt.Sprintf("Run 'cx scan --force' to clean up %d removed entities", report.Summary.MissingEntities))
	}

	// New entities
	if report.Summary.NewEntities > 0 {
		recs = append(recs, fmt.Sprintf("Run 'cx scan' to index %d new entities", report.Summary.NewEntities))
	}

	// Body-only changes
	if report.Summary.BodyChanges > 0 && report.Summary.BreakingChanges == 0 {
		recs = append(recs, fmt.Sprintf("%d body-only changes detected - run 'cx safe --drift --fix' to update hashes", report.Summary.BodyChanges))
	}

	// Broken dependencies
	if len(report.BrokenDependencies) > 0 {
		recs = append(recs, fmt.Sprintf("Check %d callers that may be affected by these changes", len(report.BrokenDependencies)))
	}

	// General recommendation
	if len(recs) > 0 {
		recs = append(recs, "Run 'cx scan' to synchronize the index with your changes")
	}

	return recs
}

// formatStoredLocation formats the location of a stored entity.
func formatStoredLocation(e *store.Entity) string {
	if e.LineEnd != nil && *e.LineEnd > e.LineStart {
		return fmt.Sprintf("%s:%d-%d", e.FilePath, e.LineStart, *e.LineEnd)
	}
	return fmt.Sprintf("%s:%d", e.FilePath, e.LineStart)
}

// AnalyzeMultipleFiles performs inline drift analysis for multiple files.
func (a *InlineDriftAnalyzer) AnalyzeMultipleFiles(filePaths []string) (*InlineDriftReport, error) {
	combinedReport := &InlineDriftReport{
		Summary: &InlineDriftSummary{
			Target: fmt.Sprintf("%d files", len(filePaths)),
			Status: "clean",
		},
	}

	var allTargets []string

	for _, filePath := range filePaths {
		report, err := a.AnalyzeFile(filePath)
		if err != nil {
			combinedReport.Warnings = append(combinedReport.Warnings, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		allTargets = append(allTargets, filePath)

		// Merge results
		combinedReport.Drifted = append(combinedReport.Drifted, report.Drifted...)
		combinedReport.NewEntities = append(combinedReport.NewEntities, report.NewEntities...)
		combinedReport.BrokenDependencies = append(combinedReport.BrokenDependencies, report.BrokenDependencies...)
		combinedReport.Warnings = append(combinedReport.Warnings, report.Warnings...)

		// Merge summary
		combinedReport.Summary.EntitiesChecked += report.Summary.EntitiesChecked
		combinedReport.Summary.SignatureChanges += report.Summary.SignatureChanges
		combinedReport.Summary.BodyChanges += report.Summary.BodyChanges
		combinedReport.Summary.MissingEntities += report.Summary.MissingEntities
		combinedReport.Summary.NewEntities += report.Summary.NewEntities
		combinedReport.Summary.BreakingChanges += report.Summary.BreakingChanges
		combinedReport.Summary.TotalAffectedCallers += report.Summary.TotalAffectedCallers

		if report.Summary.Status == "drifted" {
			combinedReport.Summary.Status = "drifted"
		}
	}

	// Update target to show file list
	if len(allTargets) > 0 {
		if len(allTargets) <= 3 {
			combinedReport.Summary.Target = strings.Join(allTargets, ", ")
		}
	}

	// Deduplicate broken dependencies
	seen := make(map[string]bool)
	var uniqueBroken []BrokenDependency
	for _, bd := range combinedReport.BrokenDependencies {
		key := fmt.Sprintf("%s->%s", bd.CallerName, bd.CallsEntity)
		if !seen[key] {
			seen[key] = true
			uniqueBroken = append(uniqueBroken, bd)
		}
	}
	combinedReport.BrokenDependencies = uniqueBroken

	// Limit broken dependencies
	if len(combinedReport.BrokenDependencies) > 20 {
		combinedReport.BrokenDependencies = combinedReport.BrokenDependencies[:20]
	}

	// Build recommendations
	combinedReport.Recommendations = a.buildRecommendations(combinedReport)

	return combinedReport, nil
}
