package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/store"
)

func intPtr(i int) *int { return &i }

func TestRunForContext_FileTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	// Create store with test entities
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Create entities in a file
	err = st.CreateEntity(&store.Entity{
		ID:         "sa-fn-aaa-FuncA",
		Name:       "FuncA",
		EntityType: "function",
		FilePath:   "pkg/handler.go",
		LineStart:  10,
		LineEnd:    intPtr(20),
		Signature:  "func FuncA() error",
		Visibility: "public",
		Status:     "active",
		Language:   "go",
	})
	if err != nil {
		t.Fatalf("create entity A: %v", err)
	}

	err = st.CreateEntity(&store.Entity{
		ID:         "sa-fn-bbb-FuncB",
		Name:       "FuncB",
		EntityType: "function",
		FilePath:   "pkg/handler.go",
		LineStart:  25,
		LineEnd:    intPtr(35),
		Signature:  "func FuncB() string",
		Visibility: "public",
		Status:     "active",
		Language:   "go",
	})
	if err != nil {
		t.Fatalf("create entity B: %v", err)
	}

	// Create a caller entity in another file
	err = st.CreateEntity(&store.Entity{
		ID:         "sa-fn-ccc-Caller",
		Name:       "Caller",
		EntityType: "function",
		FilePath:   "cmd/main.go",
		LineStart:  5,
		LineEnd:    intPtr(15),
		Signature:  "func Caller()",
		Visibility: "public",
		Status:     "active",
		Language:   "go",
	})
	if err != nil {
		t.Fatalf("create caller entity: %v", err)
	}

	// Create dependency: Caller -> FuncA
	err = st.CreateDependency(&store.Dependency{
		FromID:  "sa-fn-ccc-Caller",
		ToID:    "sa-fn-aaa-FuncA",
		DepType: "calls",
	})
	if err != nil {
		t.Fatalf("create dep: %v", err)
	}

	st.Close()

	// Change to tmpDir so config.FindConfigDir finds .cx
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Run --for via the cobra command
	var buf bytes.Buffer
	contextCmd.SetOut(&buf)
	contextCmd.SetErr(&buf)

	// Reset flags
	contextFor = "pkg/handler.go"
	contextMaxTokens = 4000
	contextWithCoverage = false
	outputFormat = "yaml"
	outputDensity = "medium"
	contextSmart = ""
	contextDiff = false
	contextStaged = false
	contextCommitRange = ""

	err = runForContext(contextCmd, "pkg/handler.go")
	if err != nil {
		t.Fatalf("runForContext failed: %v", err)
	}

	out := buf.String()

	// Should contain our entities
	if !strings.Contains(out, "FuncA") {
		t.Errorf("output should contain FuncA, got:\n%s", out)
	}
	if !strings.Contains(out, "FuncB") {
		t.Errorf("output should contain FuncB, got:\n%s", out)
	}
	// Should contain the caller
	if !strings.Contains(out, "Caller") {
		t.Errorf("output should contain Caller (calls FuncA), got:\n%s", out)
	}
}

func TestRunForContext_EntityTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	err = st.CreateEntity(&store.Entity{
		ID:         "sa-fn-xyz-MyFunc",
		Name:       "MyFunc",
		EntityType: "function",
		FilePath:   "internal/core.go",
		LineStart:  1,
		LineEnd:    intPtr(10),
		Signature:  "func MyFunc() int",
		Visibility: "public",
		Status:     "active",
		Language:   "go",
	})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	contextCmd.SetOut(&buf)
	contextCmd.SetErr(&buf)

	err = runForContext(contextCmd, "sa-fn-xyz-MyFunc")
	if err != nil {
		t.Fatalf("runForContext entity failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MyFunc") {
		t.Errorf("output should contain MyFunc, got:\n%s", out)
	}
	if !strings.Contains(out, "Target entity") {
		t.Errorf("output should show 'Target entity' reason, got:\n%s", out)
	}
}

func TestRunForContext_DirectoryTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Two entities in same directory
	for _, name := range []string{"Alpha", "Beta"} {
		err = st.CreateEntity(&store.Entity{
			ID:         "sa-fn-" + strings.ToLower(name) + "-" + name,
			Name:       name,
			EntityType: "function",
			FilePath:   "pkg/utils/" + strings.ToLower(name) + ".go",
			LineStart:  1,
			LineEnd:    intPtr(5),
			Signature:  "func " + name + "()",
			Visibility: "public",
			Status:     "active",
			Language:   "go",
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	contextCmd.SetOut(&buf)

	err = runForContext(contextCmd, "pkg/utils/")
	if err != nil {
		t.Fatalf("runForContext dir failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alpha") || !strings.Contains(out, "Beta") {
		t.Errorf("output should contain both Alpha and Beta, got:\n%s", out)
	}
}

func TestRunForContext_NoEntitiesFound(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	err = runForContext(contextCmd, "nonexistent/file.go")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "no entities found") {
		t.Errorf("expected 'no entities found' error, got: %v", err)
	}
}
