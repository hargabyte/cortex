package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cx/internal/store"
)

func setupDeadTierTestStore(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	now := time.Now()

	// Tier 1: private, zero callers
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-dead-private", Name: "deadPrivate", EntityType: "function",
		FilePath: "pkg/a.go", LineStart: 1, Visibility: "private", Status: "active", Language: "go",
		CreatedAt: now, UpdatedAt: now,
	})

	// Tier 2: exported, zero internal callers
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-dead-export", Name: "DeadExport", EntityType: "function",
		FilePath: "pkg/b.go", LineStart: 1, Visibility: "public", Status: "active", Language: "go",
		CreatedAt: now, UpdatedAt: now,
	})

	// Tier 3: called ONLY by deadPrivate (which is dead)
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-suspicious", Name: "suspicious", EntityType: "function",
		FilePath: "pkg/c.go", LineStart: 1, Visibility: "private", Status: "active", Language: "go",
		CreatedAt: now, UpdatedAt: now,
	})

	// Alive: has a non-dead caller
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-alive", Name: "alive", EntityType: "function",
		FilePath: "pkg/d.go", LineStart: 1, Visibility: "private", Status: "active", Language: "go",
		CreatedAt: now, UpdatedAt: now,
	})
	st.CreateEntity(&store.Entity{
		ID: "sa-fn-caller", Name: "caller", EntityType: "function",
		FilePath: "pkg/e.go", LineStart: 1, Visibility: "public", Status: "active", Language: "go",
		CreatedAt: now, UpdatedAt: now,
	})

	// Dependencies: deadPrivate -> suspicious, caller -> alive
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-dead-private", ToID: "sa-fn-suspicious", DepType: "calls"})
	st.CreateDependency(&store.Dependency{FromID: "sa-fn-caller", ToID: "sa-fn-alive", DepType: "calls"})

	// Metrics
	st.SaveBulkMetrics([]*store.Metrics{
		{EntityID: "sa-fn-dead-private", InDegree: 0, OutDegree: 1, ComputedAt: now},
		{EntityID: "sa-fn-dead-export", InDegree: 0, OutDegree: 0, ComputedAt: now},
		{EntityID: "sa-fn-suspicious", InDegree: 1, OutDegree: 0, ComputedAt: now},
		{EntityID: "sa-fn-alive", InDegree: 1, OutDegree: 0, ComputedAt: now},
		{EntityID: "sa-fn-caller", InDegree: 0, OutDegree: 1, ComputedAt: now},
	})

	st.Close()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	return tmpDir, func() { os.Chdir(origDir) }
}

func TestDeadTier1_OnlyDefinite(t *testing.T) {
	_, cleanup := setupDeadTierTestStore(t)
	defer cleanup()

	deadTier = 1
	deadIncludeExports = false
	deadChains = false
	deadByFile = false
	deadCreateTask = false
	deadTypeFilter = ""

	var buf bytes.Buffer
	deadCmd.SetOut(&buf)

	err := runDead(deadCmd, []string{})
	if err != nil {
		t.Fatalf("runDead failed: %v", err)
	}

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("deadPrivate")) {
		t.Error("tier 1 should find deadPrivate")
	}
	if bytes.Contains(buf.Bytes(), []byte("DeadExport")) {
		t.Error("tier 1 should NOT find DeadExport")
	}
	if bytes.Contains(buf.Bytes(), []byte("suspicious")) {
		t.Error("tier 1 should NOT find suspicious")
	}
	_ = out
}

func TestDeadTier2_IncludesProbable(t *testing.T) {
	_, cleanup := setupDeadTierTestStore(t)
	defer cleanup()

	deadTier = 2
	deadIncludeExports = false
	deadChains = false
	deadByFile = false
	deadCreateTask = false
	deadTypeFilter = ""

	var buf bytes.Buffer
	deadCmd.SetOut(&buf)

	err := runDead(deadCmd, []string{})
	if err != nil {
		t.Fatalf("runDead failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("deadPrivate")) {
		t.Error("tier 2 should find deadPrivate (tier 1)")
	}
	if !bytes.Contains(buf.Bytes(), []byte("DeadExport")) {
		t.Error("tier 2 should find DeadExport (tier 2)")
	}
	if bytes.Contains(buf.Bytes(), []byte("suspicious")) {
		t.Error("tier 2 should NOT find suspicious (tier 3)")
	}
}

func TestDeadTier3_IncludesSuspicious(t *testing.T) {
	_, cleanup := setupDeadTierTestStore(t)
	defer cleanup()

	deadTier = 3
	deadIncludeExports = false
	deadChains = false
	deadByFile = false
	deadCreateTask = false
	deadTypeFilter = ""

	var buf bytes.Buffer
	deadCmd.SetOut(&buf)

	err := runDead(deadCmd, []string{})
	if err != nil {
		t.Fatalf("runDead failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("deadPrivate")) {
		t.Error("tier 3 should find deadPrivate")
	}
	if !bytes.Contains(buf.Bytes(), []byte("DeadExport")) {
		t.Error("tier 3 should find DeadExport")
	}
	if !bytes.Contains(buf.Bytes(), []byte("suspicious")) {
		t.Error("tier 3 should find suspicious")
	}
	if !bytes.Contains(buf.Bytes(), []byte("All callers are dead")) {
		t.Error("suspicious entity should have 'All callers are dead' reason")
	}
	// alive IS found at tier 3 because its only caller (caller) is dead at tier 2,
	// making alive suspicious via transitive dead chain detection
	if !bytes.Contains(buf.Bytes(), []byte("alive")) {
		t.Error("tier 3 should find alive (its only caller is dead at tier 2)")
	}
}

func TestDeadChains(t *testing.T) {
	_, cleanup := setupDeadTierTestStore(t)
	defer cleanup()

	deadTier = 3
	deadIncludeExports = false
	deadChains = true
	deadByFile = false
	deadCreateTask = false
	deadTypeFilter = ""

	var buf bytes.Buffer
	deadCmd.SetOut(&buf)

	err := runDead(deadCmd, []string{})
	if err != nil {
		t.Fatalf("runDead failed: %v", err)
	}

	out := buf.String()
	// deadPrivate -> suspicious should be in the same chain
	if !bytes.Contains(buf.Bytes(), []byte("chain:")) {
		t.Errorf("expected chain grouping in output, got:\n%s", out)
	}
}
