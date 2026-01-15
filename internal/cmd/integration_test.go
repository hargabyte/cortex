package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/store"
)

// TestFullWorkflow tests the complete scan -> find -> show -> rank workflow
func TestFullWorkflow(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-integration-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cx directory and store
	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Create test entities (simulating scan results)
	entities := []*store.Entity{
		{ID: "fn-main", Name: "main", EntityType: "function", FilePath: "main.go", LineStart: 1, Visibility: "pub"},
		{ID: "fn-init", Name: "init", EntityType: "function", FilePath: "main.go", LineStart: 10, Visibility: "priv"},
		{ID: "fn-handler", Name: "Handler", EntityType: "function", FilePath: "pkg/api/handler.go", LineStart: 5, Visibility: "pub"},
		{ID: "tp-user", Name: "User", EntityType: "type", Kind: "struct", FilePath: "pkg/models/user.go", LineStart: 1, Visibility: "pub"},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}

	// Create dependencies
	deps := []*store.Dependency{
		{FromID: "fn-main", ToID: "fn-init", DepType: "calls"},
		{FromID: "fn-main", ToID: "fn-handler", DepType: "calls"},
		{FromID: "fn-handler", ToID: "tp-user", DepType: "uses_type"},
	}
	if err := st.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("create deps: %v", err)
	}

	// Test: Query entities
	t.Run("find entities", func(t *testing.T) {
		results, err := st.QueryEntities(store.EntityFilter{Name: "main"})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'main', got %d", len(results))
		}
	})

	t.Run("find by type", func(t *testing.T) {
		results, err := st.QueryEntities(store.EntityFilter{EntityType: "function"})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 functions, got %d", len(results))
		}
	})

	// Test: Get entity details
	t.Run("show entity", func(t *testing.T) {
		entity, err := st.GetEntity("fn-main")
		if err != nil {
			t.Fatalf("get entity: %v", err)
		}
		if entity.Name != "main" {
			t.Errorf("expected name 'main', got %q", entity.Name)
		}
	})

	// Test: Get dependencies
	t.Run("show dependencies", func(t *testing.T) {
		depsFrom, err := st.GetDependenciesFrom("fn-main")
		if err != nil {
			t.Fatalf("get deps from: %v", err)
		}
		if len(depsFrom) != 2 {
			t.Errorf("expected 2 deps from fn-main, got %d", len(depsFrom))
		}

		depsTo, err := st.GetDependenciesTo("tp-user")
		if err != nil {
			t.Fatalf("get deps to: %v", err)
		}
		if len(depsTo) != 1 {
			t.Errorf("expected 1 dep to tp-user, got %d", len(depsTo))
		}
	})

	// Test: Count operations
	t.Run("count entities", func(t *testing.T) {
		count, err := st.CountEntities(store.EntityFilter{})
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 4 {
			t.Errorf("expected 4 entities, got %d", count)
		}
	})

	t.Run("count dependencies", func(t *testing.T) {
		count, err := st.CountDependencies()
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 deps, got %d", count)
		}
	})
}

// TestIncrementalScan tests the file index for incremental scanning
func TestIncrementalScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-incremental-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Initial scan
	files := []*store.FileIndex{
		{FilePath: "main.go", ScanHash: "hash1"},
		{FilePath: "pkg/api.go", ScanHash: "hash2"},
	}
	if err := st.SetFilesScannedBulk(files); err != nil {
		t.Fatalf("set files: %v", err)
	}

	// Check for changes
	t.Run("no changes", func(t *testing.T) {
		changed, err := st.IsFileChanged("main.go", "hash1")
		if err != nil {
			t.Fatalf("check changed: %v", err)
		}
		if changed {
			t.Error("expected no change for same hash")
		}
	})

	t.Run("file changed", func(t *testing.T) {
		changed, err := st.IsFileChanged("main.go", "newhash")
		if err != nil {
			t.Fatalf("check changed: %v", err)
		}
		if !changed {
			t.Error("expected change for different hash")
		}
	})

	t.Run("new file", func(t *testing.T) {
		changed, err := st.IsFileChanged("newfile.go", "anyhash")
		if err != nil {
			t.Fatalf("check changed: %v", err)
		}
		if !changed {
			t.Error("expected change for new file")
		}
	})

	// Prune stale entries
	t.Run("prune stale", func(t *testing.T) {
		validPaths := map[string]bool{
			"main.go": true, // Keep this
			// pkg/api.go is gone
		}
		pruned, err := st.PruneStaleEntries(validPaths)
		if err != nil {
			t.Fatalf("prune: %v", err)
		}
		if pruned != 1 {
			t.Errorf("expected 1 pruned, got %d", pruned)
		}
	})
}

// TestEntityLinks tests linking entities to external systems
func TestEntityLinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-links-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Create entity
	entity := &store.Entity{
		ID: "fn-test", Name: "Test", EntityType: "function",
		FilePath: "test.go", LineStart: 1, Visibility: "pub",
	}
	if err := st.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Create links
	links := []*store.EntityLink{
		{EntityID: "fn-test", ExternalSystem: "beads", ExternalID: "bd-123", LinkType: "implements"},
		{EntityID: "fn-test", ExternalSystem: "github", ExternalID: "issue-456", LinkType: "fixes"},
	}
	for _, link := range links {
		if err := st.CreateLink(link); err != nil {
			t.Fatalf("create link: %v", err)
		}
	}

	t.Run("get links for entity", func(t *testing.T) {
		got, err := st.GetLinks("fn-test")
		if err != nil {
			t.Fatalf("get links: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 links, got %d", len(got))
		}
	})

	t.Run("find by external ID", func(t *testing.T) {
		got, err := st.GetLinksByExternalID("beads", "bd-123")
		if err != nil {
			t.Fatalf("get by external: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 link, got %d", len(got))
		}
		if got[0].EntityID != "fn-test" {
			t.Errorf("expected entity_id 'fn-test', got %q", got[0].EntityID)
		}
	})

	t.Run("delete link", func(t *testing.T) {
		if err := st.DeleteLink("fn-test", "github", "issue-456"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		got, _ := st.GetLinks("fn-test")
		if len(got) != 1 {
			t.Errorf("expected 1 link after delete, got %d", len(got))
		}
	})
}

// TestMetricsCache tests metrics storage and retrieval
func TestMetricsCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-metrics-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Create test metrics
	metrics := []*store.Metrics{
		{EntityID: "fn-1", PageRank: 0.9, InDegree: 10, OutDegree: 2, Betweenness: 0.8},
		{EntityID: "fn-2", PageRank: 0.5, InDegree: 5, OutDegree: 5, Betweenness: 0.3},
		{EntityID: "fn-3", PageRank: 0.1, InDegree: 1, OutDegree: 10, Betweenness: 0.1},
	}
	if err := st.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("save metrics: %v", err)
	}

	t.Run("top by pagerank", func(t *testing.T) {
		top, err := st.GetTopByPageRank(2)
		if err != nil {
			t.Fatalf("get top: %v", err)
		}
		if len(top) != 2 {
			t.Fatalf("expected 2, got %d", len(top))
		}
		if top[0].EntityID != "fn-1" {
			t.Errorf("expected fn-1 first, got %s", top[0].EntityID)
		}
	})

	t.Run("keystones", func(t *testing.T) {
		keystones, err := st.GetKeystones(0.4)
		if err != nil {
			t.Fatalf("get keystones: %v", err)
		}
		if len(keystones) != 2 {
			t.Errorf("expected 2 keystones (pagerank >= 0.4), got %d", len(keystones))
		}
	})

	t.Run("bottlenecks", func(t *testing.T) {
		bottlenecks, err := st.GetBottlenecks(0.5)
		if err != nil {
			t.Fatalf("get bottlenecks: %v", err)
		}
		if len(bottlenecks) != 1 {
			t.Errorf("expected 1 bottleneck (betweenness >= 0.5), got %d", len(bottlenecks))
		}
	})
}

// TestDatabasePersistence tests that data persists across store reopens
func TestDatabasePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-persist-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")

	// Create and populate store
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	entity := &store.Entity{
		ID: "fn-persist", Name: "Persist", EntityType: "function",
		FilePath: "test.go", LineStart: 1, Visibility: "pub",
	}
	if err := st.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	st.Close()

	// Reopen and verify data persists
	st2, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer st2.Close()

	got, err := st2.GetEntity("fn-persist")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Name != "Persist" {
		t.Errorf("expected name 'Persist', got %q", got.Name)
	}
}
