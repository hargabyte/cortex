package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/store"
)

func TestRunImpact_BasicBlastRadius(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := tmpDir + "/.cx"

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Create target entity
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-target-DoWork", Name: "DoWork", EntityType: "function",
		FilePath: "pkg/core.go", LineStart: 10, LineEnd: intPtr(20),
		Signature: "func DoWork() error", Visibility: "public", Status: "active", Language: "go",
	})

	// Create direct caller
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-caller-HandleRequest", Name: "HandleRequest", EntityType: "function",
		FilePath: "pkg/handler.go", LineStart: 5, LineEnd: intPtr(15),
		Signature: "func HandleRequest()", Visibility: "public", Status: "active", Language: "go",
	})

	// Create transitive caller (calls HandleRequest)
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-trans-Main", Name: "Main", EntityType: "function",
		FilePath: "cmd/main.go", LineStart: 1, LineEnd: intPtr(10),
		Signature: "func main()", Visibility: "private", Status: "active", Language: "go",
	})

	// Dependencies: Main -> HandleRequest -> DoWork
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-caller-HandleRequest", ToID: "sa-fn-target-DoWork", DepType: "calls"})
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-trans-Main", ToID: "sa-fn-caller-HandleRequest", DepType: "calls"})

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	impactCmd.SetOut(&buf)
	impactCmd.SetErr(&buf)

	impactDepth = 2
	outputFormat = "yaml"
	outputDensity = "medium"

	err = runImpact(impactCmd, []string{"pkg/core.go"})
	if err != nil {
		t.Fatalf("runImpact failed: %v", err)
	}

	out := buf.String()

	// Should find direct caller
	if !strings.Contains(out, "HandleRequest") {
		t.Errorf("should find direct caller HandleRequest, got:\n%s", out)
	}
	// Should find transitive caller at depth 2
	if !strings.Contains(out, "Main") {
		t.Errorf("should find transitive caller Main, got:\n%s", out)
	}
	// Should show risk level
	if !strings.Contains(out, "risk_level") {
		t.Errorf("should show risk_level, got:\n%s", out)
	}
}

func TestRunImpact_EntityTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := tmpDir + "/.cx"

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	st.CreateEntity(&store.Entity{
		ID: "sa-fn-aaa-Parse", Name: "Parse", EntityType: "function",
		FilePath: "internal/parser.go", LineStart: 1, LineEnd: intPtr(10),
		Signature: "func Parse(s string) Node", Visibility: "public", Status: "active", Language: "go",
	})

	st.CreateEntity(&store.Entity{
		ID: "sa-fn-bbb-Compile", Name: "Compile", EntityType: "function",
		FilePath: "internal/compiler.go", LineStart: 1, LineEnd: intPtr(10),
		Signature: "func Compile(n Node) string", Visibility: "public", Status: "active", Language: "go",
	})

	st.CreateDependency(&store.Dependency{FromID: "sa-fn-bbb-Compile", ToID: "sa-fn-aaa-Parse", DepType: "calls"})

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	impactCmd.SetOut(&buf)

	impactDepth = 2
	outputFormat = "yaml"
	outputDensity = "medium"

	err = runImpact(impactCmd, []string{"sa-fn-aaa-Parse"})
	if err != nil {
		t.Fatalf("runImpact entity failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Compile") {
		t.Errorf("should find Compile as dependent, got:\n%s", out)
	}
}

func TestRunImpact_NoEntities(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := tmpDir + "/.cx"

	st, _ := store.Open(cxDir)
	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	impactDepth = 2
	err := runImpact(impactCmd, []string{"nonexistent.go"})
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

func TestRunImpact_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()
	cxDir := tmpDir + "/.cx"

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Chain: D -> C -> B -> A (we target A, depth 1 should only find B)
	for _, e := range []struct{ id, name string }{
		{"sa-fn-a-A", "FuncA"},
		{"sa-fn-b-B", "FuncB"},
		{"sa-fn-c-C", "FuncC"},
		{"sa-fn-d-D", "FuncD"},
	} {
		st.CreateEntity(&store.Entity{
			ID: e.id, Name: e.name, EntityType: "function",
			FilePath: "pkg/" + strings.ToLower(e.name) + ".go", LineStart: 1, LineEnd: intPtr(5),
			Signature: "func " + e.name + "()", Visibility: "public", Status: "active", Language: "go",
		})
	}

	st.CreateDependency(&store.Dependency{FromID: "sa-fn-b-B", ToID: "sa-fn-a-A", DepType: "calls"})
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-c-C", ToID: "sa-fn-b-B", DepType: "calls"})
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-d-D", ToID: "sa-fn-c-C", DepType: "calls"})

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	impactCmd.SetOut(&buf)

	impactDepth = 1
	outputFormat = "yaml"
	outputDensity = "medium"

	err = runImpact(impactCmd, []string{"sa-fn-a-A"})
	if err != nil {
		t.Fatalf("runImpact failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "FuncB") {
		t.Errorf("depth 1 should find FuncB, got:\n%s", out)
	}
	if strings.Contains(out, "FuncC") {
		t.Errorf("depth 1 should NOT find FuncC, got:\n%s", out)
	}
	if strings.Contains(out, "FuncD") {
		t.Errorf("depth 1 should NOT find FuncD, got:\n%s", out)
	}
}
