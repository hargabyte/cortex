package exclude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAutoExcludes_Empty(t *testing.T) {
	// Create temp dir with no project files
	tmpDir := t.TempDir()

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 0 {
		t.Errorf("expected 0 directories, got %d: %v", len(result.Directories), result.Directories)
	}
}

func TestDetectAutoExcludes_Rust(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Cargo.toml and target/
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "target"), 0755); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 1 {
		t.Errorf("expected 1 directory, got %d: %v", len(result.Directories), result.Directories)
	}
	if !contains(result.Directories, "target") {
		t.Errorf("expected 'target' in directories, got %v", result.Directories)
	}
	if result.Reasons["target"] == "" {
		t.Error("expected reason for target directory")
	}
}

func TestDetectAutoExcludes_Rust_NoTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Cargo.toml but no target/
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 0 {
		t.Errorf("expected 0 directories (no target/), got %d: %v", len(result.Directories), result.Directories)
	}
}

func TestDetectAutoExcludes_Go(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod and vendor/modules.txt
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vendor", "modules.txt"), []byte("# go.sum"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 1 {
		t.Errorf("expected 1 directory, got %d: %v", len(result.Directories), result.Directories)
	}
	if !contains(result.Directories, "vendor") {
		t.Errorf("expected 'vendor' in directories, got %v", result.Directories)
	}
}

func TestDetectAutoExcludes_Node(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.json and node_modules/
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 1 {
		t.Errorf("expected 1 directory, got %d: %v", len(result.Directories), result.Directories)
	}
	if !contains(result.Directories, "node_modules") {
		t.Errorf("expected 'node_modules' in directories, got %v", result.Directories)
	}
}

func TestDetectAutoExcludes_Node_NoModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.json but no node_modules/
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 0 {
		t.Errorf("expected 0 directories (no node_modules/), got %d: %v", len(result.Directories), result.Directories)
	}
}

func TestDetectAutoExcludes_PHP(t *testing.T) {
	tmpDir := t.TempDir()

	// Create composer.json and vendor/autoload.php
	if err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vendor", "autoload.php"), []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 1 {
		t.Errorf("expected 1 directory, got %d: %v", len(result.Directories), result.Directories)
	}
	if !contains(result.Directories, "vendor") {
		t.Errorf("expected 'vendor' in directories, got %v", result.Directories)
	}
}

func TestDetectAutoExcludes_Python(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom-venv/pyvenv.cfg
	if err := os.Mkdir(filepath.Join(tmpDir, "my-custom-venv"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "my-custom-venv", "pyvenv.cfg"), []byte("home = /usr/bin"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 1 {
		t.Errorf("expected 1 directory, got %d: %v", len(result.Directories), result.Directories)
	}
	if !contains(result.Directories, "my-custom-venv") {
		t.Errorf("expected 'my-custom-venv' in directories, got %v", result.Directories)
	}
}

func TestDetectAutoExcludes_GoAndPHP(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod, composer.json, and both vendor/modules.txt and vendor/autoload.php
	// vendor/ should only appear once
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vendor", "modules.txt"), []byte("# go.sum"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vendor", "autoload.php"), []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	// Count occurrences of "vendor"
	vendorCount := 0
	for _, dir := range result.Directories {
		if dir == "vendor" {
			vendorCount++
		}
	}

	if vendorCount != 1 {
		t.Errorf("expected vendor to appear exactly once, got %d times in: %v", vendorCount, result.Directories)
	}
}

func TestDetectAutoExcludes_MultipleEcosystems(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Rust project with target/
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "target"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create Node project with node_modules/
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create Python venv
	if err := os.Mkdir(filepath.Join(tmpDir, "venv"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "venv", "pyvenv.cfg"), []byte("home = /usr"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	if len(result.Directories) != 3 {
		t.Errorf("expected 3 directories, got %d: %v", len(result.Directories), result.Directories)
	}

	expected := []string{"target", "node_modules", "venv"}
	for _, exp := range expected {
		if !contains(result.Directories, exp) {
			t.Errorf("expected '%s' in directories, got %v", exp, result.Directories)
		}
	}
}

// --- Nested Project Tests ---

func TestDetectAutoExcludes_NestedRust(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Rust project: tools/svg-generator/src-tauri/
	nestedPath := filepath.Join(tmpDir, "tools", "svg-generator", "src-tauri")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "Cargo.toml"), []byte("[package]\nname = \"tauri-app\""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(nestedPath, "target"), 0755); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expectedDir := filepath.Join("tools", "svg-generator", "src-tauri", "target")
	if !contains(result.Directories, expectedDir) {
		t.Errorf("expected '%s' in directories, got %v", expectedDir, result.Directories)
	}
	if result.Reasons[expectedDir] == "" {
		t.Errorf("expected reason for %s", expectedDir)
	}
}

func TestDetectAutoExcludes_NestedNode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Node project: apps/frontend/
	nestedPath := filepath.Join(tmpDir, "apps", "frontend")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(nestedPath, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expectedDir := filepath.Join("apps", "frontend", "node_modules")
	if !contains(result.Directories, expectedDir) {
		t.Errorf("expected '%s' in directories, got %v", expectedDir, result.Directories)
	}
}

func TestDetectAutoExcludes_NestedGo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Go project: services/api/
	nestedPath := filepath.Join(tmpDir, "services", "api")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "go.mod"), []byte("module example.com/api"), 0644); err != nil {
		t.Fatal(err)
	}
	vendorPath := filepath.Join(nestedPath, "vendor")
	if err := os.Mkdir(vendorPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorPath, "modules.txt"), []byte("# go.sum"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expectedDir := filepath.Join("services", "api", "vendor")
	if !contains(result.Directories, expectedDir) {
		t.Errorf("expected '%s' in directories, got %v", expectedDir, result.Directories)
	}
}

func TestDetectAutoExcludes_NestedPHP(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested PHP project: packages/laravel-app/
	nestedPath := filepath.Join(tmpDir, "packages", "laravel-app")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	vendorPath := filepath.Join(nestedPath, "vendor")
	if err := os.Mkdir(vendorPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorPath, "autoload.php"), []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expectedDir := filepath.Join("packages", "laravel-app", "vendor")
	if !contains(result.Directories, expectedDir) {
		t.Errorf("expected '%s' in directories, got %v", expectedDir, result.Directories)
	}
}

func TestDetectAutoExcludes_NestedPython(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Python venv: scripts/analysis/.venv/
	nestedPath := filepath.Join(tmpDir, "scripts", "analysis", ".venv")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "pyvenv.cfg"), []byte("home = /usr/bin"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expectedDir := filepath.Join("scripts", "analysis", ".venv")
	if !contains(result.Directories, expectedDir) {
		t.Errorf("expected '%s' in directories, got %v", expectedDir, result.Directories)
	}
}

func TestDetectAutoExcludes_MultipleNestedProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create root-level Rust project
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "target"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create nested Tauri project: tools/desktop/src-tauri/
	tauriPath := filepath.Join(tmpDir, "tools", "desktop", "src-tauri")
	if err := os.MkdirAll(tauriPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tauriPath, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tauriPath, "target"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create nested Node frontend: tools/desktop/
	frontendPath := filepath.Join(tmpDir, "tools", "desktop")
	if err := os.WriteFile(filepath.Join(frontendPath, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(frontendPath, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	result := DetectAutoExcludes(tmpDir)

	expected := []string{
		"target",
		filepath.Join("tools", "desktop", "src-tauri", "target"),
		filepath.Join("tools", "desktop", "node_modules"),
	}

	if len(result.Directories) != len(expected) {
		t.Errorf("expected %d directories, got %d: %v", len(expected), len(result.Directories), result.Directories)
	}

	for _, exp := range expected {
		if !contains(result.Directories, exp) {
			t.Errorf("expected '%s' in directories, got %v", exp, result.Directories)
		}
	}
}

func TestDetectAutoExcludes_NestedWithoutSiblingDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Cargo.toml without target/ directory
	nestedPath := filepath.Join(tmpDir, "libs", "core")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	// No target/ directory created

	result := DetectAutoExcludes(tmpDir)

	// Should not detect anything since target/ doesn't exist
	if len(result.Directories) != 0 {
		t.Errorf("expected 0 directories (no target/), got %d: %v", len(result.Directories), result.Directories)
	}
}
