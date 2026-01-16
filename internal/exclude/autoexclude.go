// Package exclude provides automatic detection and exclusion of dependency directories.
package exclude

import (
	"os"
	"path/filepath"
	"strings"
)

// AutoExcludeResult contains the directories to exclude and why.
type AutoExcludeResult struct {
	// Directories to exclude (relative to project root)
	Directories []string
	// Reasons maps each directory to why it was excluded
	Reasons map[string]string
}

// DetectAutoExcludes scans the project root for dependency directories that should be excluded.
// Only uses 100% confidence detection methods (file existence checks).
// Recursively detects nested projects (e.g., tools/svg-generator/src-tauri/target/).
func DetectAutoExcludes(projectRoot string) *AutoExcludeResult {
	result := &AutoExcludeResult{
		Directories: []string{},
		Reasons:     make(map[string]string),
	}

	// Walk the directory tree to find marker files at any depth
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip directories we can't read
		}

		// Skip if this is the project root itself
		if path == projectRoot {
			return nil
		}

		// Get relative path for this entry
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}

		// If this is a directory, check if it's already excluded or should be skipped
		if d.IsDir() {
			// Skip if this directory is already in our exclude list
			if contains(result.Directories, relPath) {
				return filepath.SkipDir
			}

			// Skip if any parent directory is already excluded
			for _, excluded := range result.Directories {
				if strings.HasPrefix(relPath, excluded+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}

			// Don't descend into common dependency directories even if not yet excluded
			dirName := d.Name()
			if dirName == "node_modules" || dirName == "target" || dirName == "vendor" {
				return filepath.SkipDir
			}

			return nil
		}

		// This is a file - check if it's a marker file
		dirPath := filepath.Dir(path)
		relDirPath, err := filepath.Rel(projectRoot, dirPath)
		if err != nil {
			return nil
		}

		fileName := d.Name()

		switch fileName {
		case "Cargo.toml":
			// Rust: target/ sibling if it exists
			targetDir := filepath.Join(relDirPath, "target")
			if relDirPath == "." {
				targetDir = "target"
			}
			absTargetDir := filepath.Join(projectRoot, targetDir)
			if dirExists(absTargetDir) && !contains(result.Directories, targetDir) {
				result.Directories = append(result.Directories, targetDir)
				result.Reasons[targetDir] = "Rust build artifacts (Cargo.toml detected)"
			}

		case "package.json":
			// Node: node_modules/ sibling if it exists
			nodeModulesDir := filepath.Join(relDirPath, "node_modules")
			if relDirPath == "." {
				nodeModulesDir = "node_modules"
			}
			absNodeModulesDir := filepath.Join(projectRoot, nodeModulesDir)
			if dirExists(absNodeModulesDir) && !contains(result.Directories, nodeModulesDir) {
				result.Directories = append(result.Directories, nodeModulesDir)
				result.Reasons[nodeModulesDir] = "Node.js dependencies (package.json detected)"
			}

		case "go.mod":
			// Go: vendor/ sibling if vendor/modules.txt exists
			vendorDir := filepath.Join(relDirPath, "vendor")
			if relDirPath == "." {
				vendorDir = "vendor"
			}
			absVendorModules := filepath.Join(projectRoot, vendorDir, "modules.txt")
			if fileExists(absVendorModules) && !contains(result.Directories, vendorDir) {
				result.Directories = append(result.Directories, vendorDir)
				result.Reasons[vendorDir] = "Go vendored dependencies (vendor/modules.txt detected)"
			}

		case "composer.json":
			// PHP: vendor/ sibling if vendor/autoload.php exists
			vendorDir := filepath.Join(relDirPath, "vendor")
			if relDirPath == "." {
				vendorDir = "vendor"
			}
			absVendorAutoload := filepath.Join(projectRoot, vendorDir, "autoload.php")
			if fileExists(absVendorAutoload) && !contains(result.Directories, vendorDir) {
				result.Directories = append(result.Directories, vendorDir)
				result.Reasons[vendorDir] = "PHP Composer dependencies (vendor/autoload.php detected)"
			}

		case "pyvenv.cfg":
			// Python: the directory containing pyvenv.cfg is the venv
			venvDir := relDirPath
			if !contains(result.Directories, venvDir) {
				result.Directories = append(result.Directories, venvDir)
				result.Reasons[venvDir] = "Python virtual environment (pyvenv.cfg detected)"
			}
		}

		return nil
	})

	return result
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// contains checks if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
