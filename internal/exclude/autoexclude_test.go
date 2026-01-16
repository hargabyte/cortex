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

	// Create vendor/modules.txt
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

	// Create vendor/autoload.php
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

	// Create both vendor/modules.txt and vendor/autoload.php
	// vendor/ should only appear once
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
