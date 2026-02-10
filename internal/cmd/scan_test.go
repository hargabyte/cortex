package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
)

func TestScanFilePass1_IncrementalFlagControlsSkipping(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "main.go")
	content := []byte("package main\n\nfunc main() {}\n")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	st, err := store.Open(filepath.Join(tmpDir, ".cx"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	relPath := getRelativePath(filePath, tmpDir)
	if err := st.SetFileScanned(relPath, extract.ComputeFileHash(content)); err != nil {
		t.Fatalf("seed file index: %v", err)
	}

	p, err := parser.NewParser(parser.Go)
	if err != nil {
		t.Fatalf("new parser: %v", err)
	}
	defer p.Close()

	origIncremental, origForce := scanIncremental, scanForce
	defer func() {
		scanIncremental, scanForce = origIncremental, origForce
	}()

	stats := &scanStats{}

	scanIncremental = false
	scanForce = false
	fullResult := scanFilePass1(filePath, tmpDir, p, st, stats)
	if fullResult == nil {
		t.Fatal("expected non-nil result in full scan mode")
	}
	if fullResult.unchanged {
		t.Fatal("expected file to be parsed in full scan mode")
	}
	if fullResult.parseResult == nil {
		t.Fatal("expected parse result in full scan mode")
	}
	fullResult.parseResult.Close()

	scanIncremental = true
	scanForce = false
	incrementalResult := scanFilePass1(filePath, tmpDir, p, st, stats)
	if incrementalResult == nil {
		t.Fatal("expected non-nil marker result in incremental mode")
	}
	if !incrementalResult.unchanged {
		t.Fatal("expected unchanged marker when incremental mode is enabled")
	}
	if incrementalResult.parseResult != nil {
		t.Fatal("did not expect parse result for unchanged file in incremental mode")
	}
}
