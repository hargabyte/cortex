// Package exclude provides automatic detection and exclusion of dependency directories.
package exclude

import (
	"os"
	"path/filepath"
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
func DetectAutoExcludes(projectRoot string) *AutoExcludeResult {
	result := &AutoExcludeResult{
		Directories: []string{},
		Reasons:     make(map[string]string),
	}

	// Check for each language ecosystem
	// Order doesn't matter since we check independent conditions

	// Rust: target/ if Cargo.toml exists
	if fileExists(filepath.Join(projectRoot, "Cargo.toml")) {
		targetDir := "target"
		if dirExists(filepath.Join(projectRoot, targetDir)) {
			result.Directories = append(result.Directories, targetDir)
			result.Reasons[targetDir] = "Rust build artifacts (Cargo.toml detected)"
		}
	}

	// Go: vendor/ if vendor/modules.txt exists
	vendorModules := filepath.Join(projectRoot, "vendor", "modules.txt")
	if fileExists(vendorModules) {
		vendorDir := "vendor"
		result.Directories = append(result.Directories, vendorDir)
		result.Reasons[vendorDir] = "Go vendored dependencies (vendor/modules.txt detected)"
	}

	// Node/TypeScript: node_modules/ if package.json exists
	if fileExists(filepath.Join(projectRoot, "package.json")) {
		nodeModules := "node_modules"
		if dirExists(filepath.Join(projectRoot, nodeModules)) {
			result.Directories = append(result.Directories, nodeModules)
			result.Reasons[nodeModules] = "Node.js dependencies (package.json detected)"
		}
	}

	// PHP: vendor/ if vendor/autoload.php exists
	vendorAutoload := filepath.Join(projectRoot, "vendor", "autoload.php")
	if fileExists(vendorAutoload) {
		vendorDir := "vendor"
		// Avoid duplicate if Go already added vendor/
		if !contains(result.Directories, vendorDir) {
			result.Directories = append(result.Directories, vendorDir)
			result.Reasons[vendorDir] = "PHP Composer dependencies (vendor/autoload.php detected)"
		}
	}

	// Python: Find any top-level directory containing pyvenv.cfg (virtual environments)
	// Scan all top-level directories rather than hardcoding names, since users can name
	// their virtual environments anything (e.g., "myproject-env", "py311-venv", etc.)
	entries, err := os.ReadDir(projectRoot)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				dirName := entry.Name()
				pyvenvCfg := filepath.Join(projectRoot, dirName, "pyvenv.cfg")
				if fileExists(pyvenvCfg) {
					if !contains(result.Directories, dirName) {
						result.Directories = append(result.Directories, dirName)
						result.Reasons[dirName] = "Python virtual environment (pyvenv.cfg detected)"
					}
				}
			}
		}
	}

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
