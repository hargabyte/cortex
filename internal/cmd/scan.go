// Package cmd implements the scan command for cx CLI.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/exclude"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/metrics"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a codebase and build the context graph",
	Long: `Scan traverses the specified directory (or current directory if none given),
parses source files, and builds a context graph of all code entities.

Auto-initializes the .cx directory if it doesn't exist.

The scan process:
  1. Discovers all source files matching supported languages
  2. Parses each file using tree-sitter grammars
  3. Extracts symbols (functions, classes, types, variables)
  4. Compares with existing entities (create/update/archive)
  5. Updates the .cx/cortex.db file index

Supported languages: Go, TypeScript, JavaScript, Java, Rust, Python, C, C++, C#, PHP, Kotlin, Ruby

Auto-excludes dependency directories (disable with --no-auto-exclude):
  - Rust target/ (when Cargo.toml exists)
  - Go vendor/ (when vendor/modules.txt exists)
  - Node node_modules/ (when package.json exists)
  - PHP vendor/ (when vendor/autoload.php exists)
  - Python virtual environments (directories with pyvenv.cfg)

Examples:
  cx scan                    # Scan current directory (auto-init if needed)
  cx scan ./src              # Scan specific directory
  cx scan --force            # Force full rescan
  cx scan --overview         # Scan + show project overview
  cx scan --lang go          # Scan only Go files
  cx scan --dry-run          # Show what would be created
  cx scan --no-auto-exclude  # Don't auto-exclude dependency directories
  cx scan -v                 # Verbose: shows auto-excluded directories`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

// Command-line flags
var (
	scanLang          string
	scanExclude       []string
	scanDryRun        bool
	scanForce         bool
	scanOverview      bool
	scanNoAutoExclude bool
)

func init() {
	rootCmd.AddCommand(scanCmd)

	// Scan-specific flags
	scanCmd.Flags().StringVar(&scanLang, "lang", "", "Language (default: auto-detect from extensions)")
	scanCmd.Flags().StringSliceVar(&scanExclude, "exclude", nil, "Exclude patterns (comma-separated globs)")
	scanCmd.Flags().BoolVar(&scanDryRun, "dry-run", false, "Show what would be created")
	scanCmd.Flags().BoolVar(&scanForce, "force", false, "Rescan even if file unchanged")
	scanCmd.Flags().BoolVar(&scanOverview, "overview", false, "Show project overview after scan (replaces quickstart)")
	scanCmd.Flags().BoolVar(&scanNoAutoExclude, "no-auto-exclude", false, "Disable automatic exclusion of dependency directories")
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
	unchanged   bool // true if file was unchanged and skipped (entities should be preserved)
}

// runScan implements the scan command logic
func runScan(cmd *cobra.Command, args []string) error {
	// Track scan start time for duration calculation
	scanStartTime := time.Now()

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

	// Auto-exclude dependency directories (unless disabled)
	var autoExcludeResult *exclude.AutoExcludeResult
	if !scanNoAutoExclude {
		autoExcludeResult = exclude.DetectAutoExcludes(absPath)
		excludes = append(excludes, autoExcludeResult.Directories...)

		// Verbose output for auto-excludes
		if verbose && len(autoExcludeResult.Directories) > 0 && !quiet {
			fmt.Println("Auto-excluded dependency directories:")
			for _, dir := range autoExcludeResult.Directories {
				fmt.Printf("  %s: %s\n", dir, autoExcludeResult.Reasons[dir])
			}
			fmt.Println()
		}
	}

	// Determine language(s) to scan
	var languages []parser.Language
	if scanLang != "" {
		// Explicit language specified
		lang, err := parseLanguageFlag(scanLang)
		if err != nil {
			return err
		}
		languages = []parser.Language{lang}
	} else {
		// Auto-detect languages from file extensions
		languages = detectLanguages(scanPath, excludes)
		if len(languages) == 0 {
			// Fallback to Go if no recognizable source files found
			languages = []parser.Language{parser.Go}
		}
	}

	// Find existing .cx directory or create one
	// First, look for an existing project by walking up from scanPath
	var cxDir string
	var projectRoot string
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	existingCxDir, err := config.FindConfigDir(scanPath)
	if err == nil {
		foundProjectRoot := filepath.Dir(existingCxDir)
		// Check if the found .cx is in cwd or a direct parent of scanPath
		// If .cx is in a distant parent (like home directory), prefer creating a new local one
		// This prevents accidentally using a stale/unrelated database
		cwdHasCx := existingCxDir == filepath.Join(cwd, config.ConfigDirName)
		scanPathHasCx := existingCxDir == filepath.Join(scanPath, config.ConfigDirName)

		if cwdHasCx || scanPathHasCx || foundProjectRoot == cwd {
			// Found .cx in cwd or scanPath - use it for incremental scan
			cxDir = existingCxDir
			projectRoot = foundProjectRoot
		} else {
			// Found .cx in a distant parent directory
			// Create a new local .cx to avoid path mismatches
			cxDir, err = config.EnsureConfigDir(cwd)
			if err != nil {
				return fmt.Errorf("failed to create .cx directory: %w", err)
			}
			projectRoot = cwd
		}
	} else {
		// No existing .cx found - create new project
		// Use current working directory as project root (not scanPath)
		// This ensures `cx scan ./src` creates .cx in cwd, not in ./src
		cxDir, err = config.EnsureConfigDir(cwd)
		if err != nil {
			return fmt.Errorf("failed to create .cx directory: %w", err)
		}
		projectRoot = cwd
	}

	// Compute scan path relative to project root for archival scoping
	relScanPath, err := filepath.Rel(projectRoot, scanPath)
	if err != nil {
		relScanPath = "." // Fall back to full scan
	}
	// Normalize paths to forward slashes for cross-platform consistency
	relScanPath = filepath.ToSlash(relScanPath)
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
	// Iterate through all detected languages to support multi-language projects
	// ============================================================
	var fileResults []fileScanResult

	// Track files already scanned to avoid processing the same file with multiple languages.
	// This is important for .h files which can match both C and C++ in mixed projects.
	scannedFiles := make(map[string]bool)

	for _, lang := range languages {
		var filePaths []string

		// Collect all file paths for this language
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

			// Skip files already scanned by a previous language
			if scannedFiles[path] {
				return nil
			}

			filePaths = append(filePaths, path)
			return nil
		})

		if err != nil {
			return fmt.Errorf("walking directory: %w", err)
		}

		// Skip if no files found for this language
		if len(filePaths) == 0 {
			continue
		}

		// Create parser for this language
		p, err := parser.NewParser(lang)
		if err != nil {
			// Log warning but continue with other languages
			if verbose {
				w.WriteComment(fmt.Sprintf("Warning: failed to create parser for %s: %v", lang, err))
			}
			continue
		}

		// Process each file in pass 1
		// Use projectRoot as base path so entity file paths match existing DB entries
		for _, path := range filePaths {
			result := scanFilePass1(path, projectRoot, p, storeDB, stats)
			if result != nil {
				fileResults = append(fileResults, *result)
			}
			// Mark file as scanned to avoid re-processing with another language
			scannedFiles[path] = true
		}

		// Close parser for this language before moving to next
		p.Close()
	}

	// ============================================================
	// Process entities and persist to store
	// ============================================================
	// Collect entities for bulk insert
	var entitiesToCreate []*store.Entity
	var entitiesToUpdate []*store.Entity

	for _, fr := range fileResults {
		// Handle unchanged files: preserve their existing entities
		if fr.unchanged {
			// Query existing entities for this file and mark them as scanned
			// This prevents them from being archived
			fileEntities, err := storeDB.QueryEntities(store.EntityFilter{
				Status:   "active",
				FilePath: fr.relPath,
			})
			if err == nil {
				for _, e := range fileEntities {
					// Only include entities that exactly match this file (not prefix matches)
					if e.FilePath == fr.relPath {
						scannedEntityIDs[e.ID] = true
					}
				}
			}
			continue // Skip further processing for unchanged files
		}

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
		if err := storeDB.CreateEntitiesBulk(entitiesToCreate); err != nil {
			w.WriteComment(fmt.Sprintf("Error: bulk entity creation failed: %v", err))
		}
	}

	// Update changed entities
	if len(entitiesToUpdate) > 0 && !scanDryRun {
		for _, e := range entitiesToUpdate {
			if err := storeDB.UpdateEntity(e); err != nil {
				w.WriteComment(fmt.Sprintf("Error: entity update failed for %s: %v", e.ID, err))
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

	// Build global entity lookup maps ONCE for cross-file resolution
	// These maps are shared by all extractors to avoid O(files × entities) overhead
	entityByName := make(map[string]*extract.CallGraphEntity)
	entityByID := make(map[string]*extract.CallGraphEntity)

	// First pass: collect all entities into a slice to avoid loop variable aliasing
	var allEntities []extract.CallGraphEntity
	for _, fr := range fileResults {
		for _, ewn := range fr.entities {
			cge := ewn.Entity.ToCallGraphEntity()
			cge.Node = ewn.Node // Set the AST node
			allEntities = append(allEntities, cge)
		}
	}

	// Second pass: build lookup maps with stable pointers
	for i := range allEntities {
		e := &allEntities[i]
		entityByName[e.Name] = e
		if e.QualifiedName != "" {
			entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			entityByID[e.ID] = e
		}
	}

	// Extract dependencies from each file using shared lookup maps
	for _, fr := range fileResults {
		if fr.parseResult == nil {
			continue
		}

		// Convert this file's entities to CallGraphEntity slice (only for iteration)
		var fileEntities []extract.CallGraphEntity
		for _, ewn := range fr.entities {
			cge := ewn.Entity.ToCallGraphEntity()
			cge.Node = ewn.Node
			fileEntities = append(fileEntities, cge)
		}

		// Create call graph extractor with shared lookup maps
		// Dispatch to language-specific extractor
		var deps []extract.Dependency
		var extractErr error

		switch fr.parseResult.Language {
		case parser.Go:
			extractor := extract.NewCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.TypeScript, parser.JavaScript:
			extractor := extract.NewTypeScriptCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Python:
			extractor := extract.NewPythonCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Java:
			extractor := extract.NewJavaCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Rust:
			extractor := extract.NewRustCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.C:
			extractor := extract.NewCCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Cpp:
			extractor := extract.NewCppCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.CSharp:
			extractor := extract.NewCSharpCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.PHP:
			extractor := extract.NewPHPCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Ruby:
			extractor := extract.NewRubyCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		case parser.Kotlin:
			extractor := extract.NewKotlinCallGraphExtractorWithMaps(fr.parseResult, fileEntities, entityByName, entityByID)
			deps, extractErr = extractor.ExtractDependencies()
		default:
			// Unsupported language for call graph extraction
			continue
		}

		err := extractErr
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

	// Show overview if requested (--overview flag)
	if scanOverview && !scanDryRun {
		if err := showScanOverview(storeDB, projectRoot, cfg); err != nil {
			// Non-fatal: just log warning if overview fails
			if !quiet {
				fmt.Fprintf(os.Stderr, "Warning: overview failed: %v\n", err)
			}
		}
	}

	// Create Dolt commit after successful scan (skip for dry-run)
	if !scanDryRun && stats.errors == 0 {
		// Calculate scan duration
		scanDuration := time.Since(scanStartTime)

		// Get git context from the project being scanned
		gitCommit := getGitCommit(projectRoot)
		gitBranch := getGitBranch(projectRoot)

		// Save scan metadata
		meta := &store.ScanMetadata{
			GitCommit:         gitCommit,
			GitBranch:         gitBranch,
			FilesScanned:      stats.filesScanned,
			EntitiesFound:     stats.entitiesTotal,
			DependenciesFound: stats.depsPersisted,
			DurationMs:        int(scanDuration.Milliseconds()),
		}
		if err := storeDB.SaveScanMetadata(meta); err != nil {
			if verbose {
				w.WriteComment(fmt.Sprintf("Warning: failed to save scan metadata: %v", err))
			}
		}

		// Format commit message: cx scan: {entities} entities, {deps} deps [{branch}@{commit}]
		commitMsg := fmt.Sprintf("cx scan: %d entities, %d deps", stats.entitiesTotal, stats.depsPersisted)
		if gitBranch != "" || gitCommit != "" {
			commitMsg += fmt.Sprintf(" [%s@%s]", gitBranch, gitCommit)
		}

		// Create Dolt commit
		if _, err := storeDB.DoltCommit(commitMsg); err != nil {
			if verbose {
				w.WriteComment(fmt.Sprintf("Warning: failed to create Dolt commit: %v", err))
			}
		} else if verbose {
			w.WriteComment(fmt.Sprintf("Dolt commit: %s", commitMsg))
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
			// Return a marker for unchanged file so its entities can be preserved
			return &fileScanResult{
				path:      path,
				relPath:   relPath,
				fileHash:  fileHash,
				unchanged: true,
			}
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
	case parser.C:
		extractor := extract.NewCExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.CSharp:
		extractor := extract.NewCSharpExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.PHP:
		extractor := extract.NewPHPExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Cpp:
		extractor := extract.NewCppExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Kotlin:
		extractor := extract.NewKotlinExtractorWithBase(result, basePath)
		entitiesWithNodes, err = extractor.ExtractAllWithNodes()
	case parser.Ruby:
		extractor := extract.NewRubyExtractorWithBase(result, basePath)
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
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".kt", ".kts":
		return "kotlin"
	case ".rb", ".rake":
		return "ruby"
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
	case parser.C:
		return ext == ".c" || ext == ".h"
	case parser.CSharp:
		return ext == ".cs"
	case parser.PHP:
		return ext == ".php"
	case parser.Kotlin:
		return ext == ".kt" || ext == ".kts"
	case parser.Ruby:
		return ext == ".rb" || ext == ".rake"
	case parser.Cpp:
		// Note: .h files are included here for pure C++ projects.
		// The detectLanguages function handles C/C++ disambiguation by removing C
		// from the language list when .h files exist with C++ sources but no .c files.
		return ext == ".cpp" || ext == ".cc" || ext == ".cxx" || ext == ".hpp" || ext == ".hh" || ext == ".hxx" || ext == ".h"
	default:
		return false
	}
}

// getRelativePath returns path relative to basePath.
// Always uses forward slashes for cross-platform consistency.
func getRelativePath(path, basePath string) string {
	rel, err := filepath.Rel(basePath, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
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

// showScanOverview displays a project overview after scanning (--overview flag).
// This consolidates the quickstart functionality into scan.
func showScanOverview(s *store.Store, projectRoot string, cfg *config.Config) error {
	fmt.Println()
	fmt.Println("Computing importance metrics...")

	// Get all active entities for metrics computation
	entities, err := s.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("query entities: %w", err)
	}

	if len(entities) == 0 {
		fmt.Println("No entities found.")
		return nil
	}

	// Check if metrics need computing
	needRecompute := false
	for _, e := range entities {
		if m, _ := s.GetMetrics(e.ID); m == nil {
			needRecompute = true
			break
		}
	}

	if needRecompute {
		// Build graph from store
		g, err := graph.BuildFromStore(s)
		if err != nil {
			return fmt.Errorf("build graph: %w", err)
		}

		adjacency := g.Edges

		// Compute PageRank
		prConfig := metrics.PageRankConfig{
			Damping:       cfg.Metrics.PageRankDamping,
			MaxIterations: cfg.Metrics.PageRankIterations,
			Tolerance:     0.0001,
		}
		pagerank := metrics.ComputePageRank(adjacency, prConfig)

		// Compute betweenness
		betweenness := metrics.ComputeBetweenness(adjacency)

		// Compute degrees
		inDegree, outDegree := metrics.ComputeInOutDegree(adjacency)

		// Save to store
		bulkMetrics := make([]*store.Metrics, 0, len(entities))
		for _, e := range entities {
			m := &store.Metrics{
				EntityID:    e.ID,
				PageRank:    pagerank[e.ID],
				Betweenness: betweenness[e.ID],
				InDegree:    inDegree[e.ID],
				OutDegree:   outDegree[e.ID],
				ComputedAt:  time.Now(),
			}
			bulkMetrics = append(bulkMetrics, m)
		}

		if err := s.SaveBulkMetrics(bulkMetrics); err != nil {
			return fmt.Errorf("save metrics: %w", err)
		}
	}

	// Print overview
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	printScanOverviewSummary(s, projectRoot)

	return nil
}

// printScanOverviewSummary prints the project summary for --overview.
func printScanOverviewSummary(s *store.Store, projectRoot string) {
	// Get stats
	activeCount, _ := s.CountEntities(store.EntityFilter{Status: "active"})
	depCount, _ := s.CountDependencies()
	fileCount, _ := s.CountFileIndex()

	// Count by type
	funcCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "function"})
	methodCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "method"})
	typeCount, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "struct"})
	typeCount2, _ := s.CountEntities(store.EntityFilter{Status: "active", EntityType: "interface"})
	typeCount += typeCount2

	fmt.Printf("Project: %s\n", filepath.Base(projectRoot))
	fmt.Printf("Entities: %d (functions: %d, methods: %d, types: %d)\n",
		activeCount, funcCount, methodCount, typeCount)
	fmt.Printf("Dependencies: %d\n", depCount)
	fmt.Printf("Files indexed: %d\n", fileCount)
	fmt.Println()

	// Get top keystones by PageRank
	topMetrics, err := s.GetTopByPageRank(5)
	if err == nil && len(topMetrics) > 0 {
		fmt.Println("Top Keystones:")
		for i, m := range topMetrics {
			e, err := s.GetEntity(m.EntityID)
			if err != nil || e == nil {
				continue
			}
			loc := formatOverviewLocation(e.FilePath, e.LineStart)
			fmt.Printf("  %d. %s (%s) @ %s\n", i+1, e.Name, e.EntityType, loc)
		}
		fmt.Println()
	}

	fmt.Println("Next steps:")
	fmt.Println("  cx context --smart \"<your task>\" --budget 8000")
	fmt.Println("  cx impact <file-to-modify>")
	fmt.Println("  cx map                    # project skeleton")
}

// formatOverviewLocation formats a short location string for overview output.
func formatOverviewLocation(filePath string, line int) string {
	parts := strings.Split(filePath, "/")
	if len(parts) > 2 {
		filePath = strings.Join(parts[len(parts)-2:], "/")
	}
	return fmt.Sprintf("%s:%d", filePath, line)
}

// parseLanguageFlag parses the --lang flag value into a parser.Language.
func parseLanguageFlag(langStr string) (parser.Language, error) {
	switch strings.ToLower(langStr) {
	case "go":
		return parser.Go, nil
	case "typescript", "ts":
		return parser.TypeScript, nil
	case "javascript", "js":
		return parser.JavaScript, nil
	case "java":
		return parser.Java, nil
	case "rust", "rs":
		return parser.Rust, nil
	case "python", "py":
		return parser.Python, nil
	case "c":
		return parser.C, nil
	case "csharp", "cs":
		return parser.CSharp, nil
	case "php":
		return parser.PHP, nil
	case "kotlin", "kt":
		return parser.Kotlin, nil
	case "ruby", "rb":
		return parser.Ruby, nil
	case "cpp", "c++":
		return parser.Cpp, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", langStr)
	}
}

// getGitCommit returns the current git HEAD commit hash (short form).
func getGitCommit(projectRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getGitBranch returns the current git branch name.
func getGitBranch(projectRoot string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectLanguages walks the directory and returns detected languages based on file extensions.
// Returns languages sorted by file count (most common first).
// Handles the C/C++ ambiguity for .h files: if C++ source files exist but no .c files,
// .h files are treated as C++ headers and C is not added to the language list.
func detectLanguages(scanPath string, excludes []string) []parser.Language {
	// Count files by language and track specific extensions for C/C++ disambiguation
	langCounts := make(map[parser.Language]int)
	hasDotC := false   // Has .c files (definitively C)
	hasDotH := false   // Has .h files (ambiguous)
	hasCppSrc := false // Has .cpp/.cc/.cxx files (definitively C++)

	filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			if shouldExcludeDir(path, scanPath, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded files
		if shouldExcludeFile(path, scanPath, excludes) {
			return nil
		}

		// Detect language from extension
		ext := filepath.Ext(path)

		// Track specific extensions for C/C++ disambiguation
		switch ext {
		case ".c":
			hasDotC = true
		case ".h":
			hasDotH = true
		case ".cpp", ".cc", ".cxx":
			hasCppSrc = true
		}

		lang := parser.LanguageFromExtension(ext)
		if lang != "" {
			langCounts[lang]++
		}

		return nil
	})

	// Handle C/C++ .h file ambiguity:
	// If we have .h files and C++ source files, but NO .c files,
	// the .h files should be parsed as C++ (not C).
	// Remove C from the language list in this case.
	if hasDotH && hasCppSrc && !hasDotC {
		delete(langCounts, parser.C)
	}

	// Convert to slice and sort by count (descending)
	type langCount struct {
		lang  parser.Language
		count int
	}
	var counts []langCount
	for lang, count := range langCounts {
		counts = append(counts, langCount{lang, count})
	}

	// Sort by count descending
	for i := 0; i < len(counts); i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].count > counts[i].count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}

	// Extract just the languages
	var languages []parser.Language
	for _, lc := range counts {
		languages = append(languages, lc.lang)
	}

	return languages
}
