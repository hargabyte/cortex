// Package cmd implements the scan command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/parser"
	"github.com/spf13/cobra"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a codebase and build the context graph",
	Long: `Scan traverses the specified directory (or current directory if none given),
parses source files, and builds a context graph of all code entities.

The scan process:
  1. Discovers all source files matching supported languages
  2. Parses each file using tree-sitter grammars
  3. Extracts symbols (functions, classes, types, variables)
  4. Compares with existing entities (create/update/archive)
  5. Updates the .cx/cortex.db file index

Supported languages: Go, TypeScript, JavaScript, Java, Rust, Python

Examples:
  cx scan                    # Scan current directory
  cx scan ./src              # Scan specific directory
  cx scan --lang go          # Scan only Go files
  cx scan --dry-run          # Show what would be created`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

// Command-line flags
var (
	scanLang    string
	scanExclude []string
	scanDryRun  bool
	scanForce   bool
)

func init() {
	rootCmd.AddCommand(scanCmd)

	// Scan-specific flags
	scanCmd.Flags().StringVar(&scanLang, "lang", "", "Language (default: auto-detect from extensions)")
	scanCmd.Flags().StringSliceVar(&scanExclude, "exclude", nil, "Exclude patterns (comma-separated globs)")
	scanCmd.Flags().BoolVar(&scanDryRun, "dry-run", false, "Show what would be created")
	scanCmd.Flags().BoolVar(&scanForce, "force", false, "Rescan even if file unchanged")
}

// scanStats tracks scan statistics for summary output
type scanStats struct {
	filesScanned   int
	entitiesTotal  int
	created        int
	updated        int
	unchanged      int
	archived       int
	skipped        int
	errors         int
	depsExtracted  int
	depsResolved   int
	depsPersisted  int
}

// fileScanResult holds the results from scanning a single file.
// Used in the two-pass architecture to keep parse results alive for call graph extraction.
type fileScanResult struct {
	path        string
	relPath     string
	fileHash    string
	parseResult *parser.ParseResult
	entities    []extract.EntityWithNode
	language    parser.Language
}

// runScan implements the scan command logic
func runScan(cmd *cobra.Command, args []string) error {
	// Determine scan path
	scanPath := "."
	if len(args) > 0 {
		scanPath = args[0]
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	scanPath = absPath

	// Load config
	cfg, _ := config.Load(scanPath)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Merge exclude patterns (CLI flags take precedence)
	excludes := cfg.Scan.Exclude
	if len(scanExclude) > 0 {
		excludes = append(excludes, scanExclude...)
	}

	// Determine language to scan
	lang := parser.Go // Default to Go
	if scanLang != "" {
		switch strings.ToLower(scanLang) {
		case "go":
			lang = parser.Go
		case "typescript", "ts":
			lang = parser.TypeScript
		case "javascript", "js":
			lang = parser.JavaScript
		case "java":
			lang = parser.Java
		case "rust", "rs":
			lang = parser.Rust
		case "python", "py":
			lang = parser.Python
		default:
			return fmt.Errorf("unsupported language: %s", scanLang)
		}
	}

	// Find existing .cx directory or create one
	// First, look for an existing project by walking up from scanPath
	var cxDir string
	var projectRoot string
	existingCxDir, err := config.FindConfigDir(scanPath)
	if err == nil {
		// Found existing .cx - this is an incremental scan
		cxDir = existingCxDir
		projectRoot = filepath.Dir(cxDir) // Project root is parent of .cx
	} else {
		// No existing .cx found - create new project at scanPath
		cxDir, err = config.EnsureConfigDir(scanPath)
		if err != nil {
			return fmt.Errorf("failed to create .cx directory: %w", err)
		}
		projectRoot = scanPath
	}

	// Compute scan path relative to project root for archival scoping
	relScanPath, err := filepath.Rel(projectRoot, scanPath)
	if err != nil {
		relScanPath = "." // Fall back to full scan
	}
	// Normalize: "." means full project scan
	isFullScan := relScanPath == "."

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Output writer (CGF sparse format for scan summary)
	w := output.NewCGFWriter(os.Stdout, output.CGFSparse)
	if !quiet {
		if err := w.WriteHeader(); err != nil {
			return fmt.Errorf("writing header: %w", err)
		}
	}

	stats := &scanStats{}

	// Track existing entity IDs to detect deletions
	// For incremental scans, only track entities within the scan path
	existingEntityIDs := make(map[string]bool)
	scannedEntityIDs := make(map[string]bool)

	// Get existing entities from store
	existingEntities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active"})
	if err == nil {
		for _, e := range existingEntities {
			// For incremental scans (subdirectory), only consider entities within scan path
			// This prevents archiving entities outside the scanned directory
			if isFullScan || strings.HasPrefix(e.FilePath, relScanPath+"/") || e.FilePath == relScanPath {
				existingEntityIDs[e.ID] = true
			}
		}
	}

	// ============================================================
	// PASS 1: Scan all files, extract entities, keep ASTs in memory
	// ============================================================
	var fileResults []fileScanResult
	var filePaths []string

	// Collect all file paths first
	err = filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			stats.errors++
			return nil
		}

		if info.IsDir() {
			if shouldExcludeDir(path, scanPath, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		if !isSourceFile(path, lang) {
			return nil
		}

		if shouldExcludeFile(path, scanPath, excludes) {
			stats.skipped++
			return nil
		}

		filePaths = append(filePaths, path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	// Create a single parser instance for efficiency
	p, err := parser.NewParser(lang)
	if err != nil {
		return fmt.Errorf("creating parser: %w", err)
	}
	defer p.Close()

	// Process each file in pass 1
	// Use projectRoot as base path so entity file paths match existing DB entries
	for _, path := range filePaths {
		result := scanFilePass1(path, projectRoot, p, storeDB, stats)
		if result != nil {
			fileResults = append(fileResults, *result)
		}
	}

	// ============================================================
	// Process entities and persist to store
	// ============================================================
	// Collect entities for bulk insert
	var entitiesToCreate []*store.Entity
	var entitiesToUpdate []*store.Entity

	for _, fr := range fileResults {
		for _, ewn := range fr.entities {
			entity := ewn.Entity
			entityID := entity.GenerateEntityID()
			scannedEntityIDs[entityID] = true
			stats.entitiesTotal++

			status, storeEntity := processEntityWithStore(entity, entityID, storeDB, stats, existingEntityIDs)
			writeEntityWithStatus(w, entity, status)

			if storeEntity != nil {
				if status == "new" {
					entitiesToCreate = append(entitiesToCreate, storeEntity)
				} else if strings.HasPrefix(status, "updated") {
					entitiesToUpdate = append(entitiesToUpdate, storeEntity)
				}
			}
		}

		// Update file index
		if !scanDryRun {
			if err := storeDB.SetFileScanned(fr.relPath, fr.fileHash); err != nil && verbose {
				w.WriteComment(fmt.Sprintf("Warning: file index update failed for %s: %v", fr.relPath, err))
			}
		}
	}

	// Bulk create new entities
	if len(entitiesToCreate) > 0 && !scanDryRun {
		if err := storeDB.CreateEntitiesBulk(entitiesToCreate); err != nil && verbose {
			w.WriteComment(fmt.Sprintf("Warning: bulk entity creation failed: %v", err))
		}
	}

	// Update changed entities
	if len(entitiesToUpdate) > 0 && !scanDryRun {
		for _, e := range entitiesToUpdate {
			if err := storeDB.UpdateEntity(e); err != nil && verbose {
				w.WriteComment(fmt.Sprintf("Warning: entity update failed for %s: %v", e.ID, err))
			}
		}
	}

	// ============================================================
	// PASS 2: Extract dependencies using global entity map
	// ============================================================
	if verbose {
		w.WriteBlankLine()
		w.WriteComment("Extracting call graph dependencies...")
	}

	// Build global entity map for cross-file resolution
	var allCallGraphEntities []extract.CallGraphEntity
	for _, fr := range fileResults {
		for _, ewn := range fr.entities {
			cge := ewn.Entity.ToCallGraphEntity()
			cge.Node = ewn.Node // Set the AST node
			allCallGraphEntities = append(allCallGraphEntities, cge)
		}
	}

	// Extract dependencies from each file using the global entity map
	for _, fr := range fileResults {
		if fr.parseResult == nil {
			continue
		}

		// Create call graph extractor with global entities for cross-file resolution
		extractor := extract.NewCallGraphExtractor(fr.parseResult, allCallGraphEntities)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			if verbose {
				w.WriteComment(fmt.Sprintf("Warning: call graph extraction failed for %s: %v", fr.relPath, err))
			}
			continue
		}

		stats.depsExtracted += len(deps)

		// Collect resolved dependencies for bulk insertion
		var depsToCreate []*store.Dependency
		for _, dep := range deps {
			if dep.ToID != "" {
				stats.depsResolved++
				depsToCreate = append(depsToCreate, &store.Dependency{
					FromID:  dep.FromID,
					ToID:    dep.ToID,
					DepType: string(dep.DepType),
				})
			}
		}

		// Persist dependencies in bulk
		if len(depsToCreate) > 0 && !scanDryRun {
			if err := storeDB.CreateDependenciesBulk(depsToCreate); err == nil {
				stats.depsPersisted += len(depsToCreate)
				if verbose {
					for _, dep := range depsToCreate {
						w.WriteComment(fmt.Sprintf("  %s -> %s (%s)", dep.FromID, dep.ToID, dep.DepType))
					}
				}
			}
		}
	}

	// Clean up parse results
	for _, fr := range fileResults {
		if fr.parseResult != nil {
			fr.parseResult.Close()
		}
	}

	// Handle archived entities (existed before but not scanned now)
	if !scanDryRun {
		for entityID := range existingEntityIDs {
			if !scannedEntityIDs[entityID] {
				if err := storeDB.ArchiveEntity(entityID); err == nil {
					stats.archived++
					if verbose {
						w.WriteComment(fmt.Sprintf("Archived: %s", entityID))
					}
				}
			}
		}
	}

	// Print summary (unless quiet mode)
	if !quiet {
		w.WriteBlankLine()
		w.WriteComment(fmt.Sprintf("Scanned %d files, %d entities", stats.filesScanned, stats.entitiesTotal))
		w.WriteComment(fmt.Sprintf("Created: %d, Updated: %d, Unchanged: %d, Archived: %d",
			stats.created, stats.updated, stats.unchanged, stats.archived))
		if stats.depsExtracted > 0 {
			w.WriteComment(fmt.Sprintf("Dependencies: %d extracted, %d resolved, %d persisted",
				stats.depsExtracted, stats.depsResolved, stats.depsPersisted))
		}
		if stats.skipped > 0 || stats.errors > 0 {
			w.WriteComment(fmt.Sprintf("Skipped: %d, Errors: %d", stats.skipped, stats.errors))
		}
	}

	return nil
}

// scanFilePass1 handles the first pass of scanning: parse file and extract entities with AST nodes.
// Returns nil if file should be skipped (unchanged or error).
func scanFilePass1(path, basePath string, p *parser.Parser, storeDB *store.Store, stats *scanStats) *fileScanResult {
	stats.filesScanned++

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		stats.errors++
		return nil
	}

	// Check if file changed (via store file index)
	fileHash := extract.ComputeFileHash(content)
	relPath := getRelativePath(path, basePath)

	if storeDB != nil && !scanForce {
		changed, err := storeDB.IsFileChanged(relPath, fileHash)
		if err == nil && !changed {
			stats.skipped++
			stats.filesScanned-- // Don't count skipped files
			return nil
		}
	}

	// Parse file
	result, err := p.Parse(content)
	if err != nil {
		stats.errors++
		return nil
	}

	result.FilePath = relPath

	// Extract entities with AST nodes based on language
	var entitiesWithNodes []extract.EntityWithNode
	switch p.Language() {
	case parser.Go:
		extractor := extract.NewExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Python:
		extractor := extract.NewPythonExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.TypeScript, parser.JavaScript:
		extractor := extract.NewTypeScriptExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Rust:
		extractor := extract.NewRustExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Java:
		extractor := extract.NewJavaExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	default:
		// Fall back to Go extractor for unsupported languages
		extractor := extract.NewExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	}
	if err != nil {
		stats.errors++
		result.Close()
		return nil
	}

	return &fileScanResult{
		path:        path,
		relPath:     relPath,
		fileHash:    fileHash,
		parseResult: result,
		entities:    entitiesWithNodes,
		language:    p.Language(),
	}
}

// processEntityWithStore compares entity with existing and prepares create/update operations.
// Returns the status string and a store.Entity object if an operation is needed.
func processEntityWithStore(entity *extract.Entity, entityID string, storeDB *store.Store,
	stats *scanStats, existingEntityIDs map[string]bool) (string, *store.Entity) {

	// Detect language from file extension
	lang := detectLanguageFromPath(entity.File)

	// Check if entity exists in store
	existing, err := storeDB.GetEntity(entityID)

	if err != nil || existing == nil {
		// Entity doesn't exist - prepare for creation
		if !existingEntityIDs[entityID] {
			stats.created++
			endLine := int(entity.EndLine)
			return "new", &store.Entity{
				ID:         entityID,
				Name:       entity.Name,
				EntityType: string(entity.Kind),
				FilePath:   entity.File,
				LineStart:  int(entity.StartLine),
				LineEnd:    &endLine,
				Signature:  entity.FormatSignature(),
				SigHash:    entity.SigHash,
				BodyHash:   entity.BodyHash,
				Receiver:   entity.Receiver,
				Visibility: string(entity.Visibility),
				Language:   lang,
				Status:     "active",
				BodyText:   entity.RawBody,
				DocComment: entity.DocComment,
				Skeleton:   entity.Skeleton,
			}
		}
		stats.unchanged++
		return "unchanged", nil
	}

	// Entity exists - check for changes
	if existing.SigHash != entity.SigHash || existing.BodyHash != entity.BodyHash {
		// Changed - prepare for update
		stats.updated++
		endLine := int(entity.EndLine)
		storeEntity := &store.Entity{
			ID:         entityID,
			Name:       entity.Name,
			EntityType: string(entity.Kind),
			FilePath:   entity.File,
			LineStart:  int(entity.StartLine),
			LineEnd:    &endLine,
			Signature:  entity.FormatSignature(),
			SigHash:    entity.SigHash,
			BodyHash:   entity.BodyHash,
			Receiver:   entity.Receiver,
			Visibility: string(entity.Visibility),
			Language:   lang,
			Status:     "active",
			BodyText:   entity.RawBody,
			DocComment: entity.DocComment,
			Skeleton:   entity.Skeleton,
		}

		if existing.SigHash != entity.SigHash {
			return "updated:sig", storeEntity
		}
		return "updated:body", storeEntity
	}

	stats.unchanged++
	return "unchanged", nil
}

// detectLanguageFromPath detects the programming language from a file path.
func detectLanguageFromPath(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	default:
		return "unknown"
	}
}

// writeEntityWithStatus writes an entity line in CGF format with status annotation
func writeEntityWithStatus(w *output.CGFWriter, e *extract.Entity, status string) {
	// Skip all output in quiet mode
	if quiet {
		return
	}

	entityType := mapEntityKindToCGFType(e.Kind)
	location := formatLocation(e)

	// Build the line: TYPE location Name [status]
	var line string
	if status == "unchanged" {
		// Don't print unchanged unless verbose
		if !verbose {
			return
		}
		line = fmt.Sprintf("%s %s %s [%s]", entityType, location, e.Name, status)
	} else {
		line = fmt.Sprintf("%s %s %s [%s]", entityType, location, e.Name, status)
	}

	fmt.Fprintln(os.Stdout, line)
}

// mapEntityKindToCGFType maps extract.EntityKind to CGF entity type
func mapEntityKindToCGFType(kind extract.EntityKind) output.CGFEntityType {
	switch kind {
	case extract.FunctionEntity, extract.MethodEntity:
		return output.CGFFunction
	case extract.TypeEntity:
		return output.CGFType
	case extract.ConstEntity, extract.VarEntity:
		return output.CGFConstant
	case extract.EnumEntity:
		return output.CGFEnum
	case extract.ImportEntity:
		return output.CGFImport
	default:
		return output.CGFExternal
	}
}

// formatLocation formats file:line[-end] location
func formatLocation(e *extract.Entity) string {
	if e.EndLine > e.StartLine {
		return fmt.Sprintf("%s:%d-%d", e.File, e.StartLine, e.EndLine)
	}
	return fmt.Sprintf("%s:%d", e.File, e.StartLine)
}

// shouldExcludeDir checks if a directory should be excluded from scanning
func shouldExcludeDir(path, basePath string, patterns []string) bool {
	relPath := getRelativePath(path, basePath)

	// Always exclude hidden directories
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}

	// Check against exclude patterns
	for _, pattern := range patterns {
		// Remove trailing /**
		dirPattern := strings.TrimSuffix(pattern, "/**")
		dirPattern = strings.TrimSuffix(dirPattern, "/*")

		// Simple directory name match
		if base == dirPattern || relPath == dirPattern {
			return true
		}

		// Check if pattern matches relative path
		matched, _ := filepath.Match(dirPattern, relPath)
		if matched {
			return true
		}

		matched, _ = filepath.Match(dirPattern, base)
		if matched {
			return true
		}
	}

	return false
}

// shouldExcludeFile checks if a file should be excluded from scanning
func shouldExcludeFile(path, basePath string, patterns []string) bool {
	relPath := getRelativePath(path, basePath)
	base := filepath.Base(path)

	// Always exclude hidden files
	if strings.HasPrefix(base, ".") {
		return true
	}

	// Check against exclude patterns
	for _, pattern := range patterns {
		// Handle ** patterns by checking both full path and filename
		if strings.Contains(pattern, "**") {
			// Convert ** pattern to simpler matching
			simplePattern := strings.Replace(pattern, "**/", "", -1)
			simplePattern = strings.Replace(simplePattern, "**", "", -1)

			// Match against filename
			matched, _ := filepath.Match(simplePattern, base)
			if matched {
				return true
			}
		}

		// Direct pattern match against relative path
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return true
		}

		// Match against filename only
		matched, _ = filepath.Match(pattern, base)
		if matched {
			return true
		}
	}

	return false
}

// isSourceFile checks if a file is a source file for the given language
func isSourceFile(path string, lang parser.Language) bool {
	ext := filepath.Ext(path)

	switch lang {
	case parser.Go:
		return ext == ".go"
	case parser.TypeScript:
		return ext == ".ts" || ext == ".tsx"
	case parser.JavaScript:
		return ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs"
	case parser.Java:
		return ext == ".java"
	case parser.Rust:
		return ext == ".rs"
	case parser.Python:
		return ext == ".py"
	default:
		return false
	}
}

// getRelativePath returns path relative to basePath
func getRelativePath(path, basePath string) string {
	rel, err := filepath.Rel(basePath, path)
	if err != nil {
		return path
	}
	return rel
}

// isFileChanged checks if a file has changed since last scan
func isFileChanged(storeDB *store.Store, path, hash string) bool {
	if storeDB == nil {
		return true // No store, assume changed
	}

	changed, err := storeDB.IsFileChanged(path, hash)
	if err != nil {
		return true // Error reading store, assume changed
	}

	return changed
}
