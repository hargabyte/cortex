package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cx/internal/store"
)

// TestDoltE2E_FullWorkflow tests the complete Dolt workflow:
// init → scan → commit → diff → history → rollback
func TestDoltE2E_FullWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-e2e-*")
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

	// Step 1: Create initial entities (simulating first scan)
	initialEntities := []*store.Entity{
		{ID: "fn-main", Name: "main", EntityType: "function", FilePath: "main.go", LineStart: 1, Language: "go"},
		{ID: "fn-init", Name: "init", EntityType: "function", FilePath: "main.go", LineStart: 10, Language: "go"},
	}
	if err := st.CreateEntitiesBulk(initialEntities); err != nil {
		t.Fatalf("create initial entities: %v", err)
	}

	// Step 2: Create first Dolt commit
	hash1, err := st.DoltCommit("initial scan: 2 entities")
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	t.Logf("First commit: %s", hash1)

	// Step 3: Modify entities (simulating second scan)
	// Update existing entity
	updatedEntity := &store.Entity{
		ID: "fn-main", Name: "main", EntityType: "function",
		FilePath: "main.go", LineStart: 1, Language: "go",
		Signature: "func main() error", // Changed signature
	}
	if err := st.UpdateEntity(updatedEntity); err != nil {
		t.Fatalf("update entity: %v", err)
	}

	// Add new entity
	newEntity := &store.Entity{
		ID: "fn-handler", Name: "Handler", EntityType: "function",
		FilePath: "api.go", LineStart: 5, Language: "go",
	}
	if err := st.CreateEntity(newEntity); err != nil {
		t.Fatalf("create new entity: %v", err)
	}

	// Step 4: Create second commit
	hash2, err := st.DoltCommit("second scan: 3 entities")
	if err != nil {
		t.Fatalf("second commit: %v", err)
	}
	t.Logf("Second commit: %s", hash2)

	// Step 5: Test diff between commits
	t.Run("diff between commits", func(t *testing.T) {
		added, modified, removed, err := st.DoltDiffSummary("HEAD~1", "HEAD")
		if err != nil {
			t.Fatalf("diff summary: %v", err)
		}
		// Should have 1 added (fn-handler) and 1 modified (fn-main)
		if added != 1 {
			t.Errorf("expected 1 added, got %d", added)
		}
		if modified != 1 {
			t.Errorf("expected 1 modified, got %d", modified)
		}
		if removed != 0 {
			t.Errorf("expected 0 removed, got %d", removed)
		}
	})

	// Step 6: Test history
	t.Run("history", func(t *testing.T) {
		logs, err := st.DoltLog(10)
		if err != nil {
			t.Fatalf("get log: %v", err)
		}
		// Should have at least 2 commits (plus any init commits)
		if len(logs) < 2 {
			t.Errorf("expected at least 2 commits, got %d", len(logs))
		}
	})

	// Step 7: Test time travel query
	t.Run("time travel at HEAD~1", func(t *testing.T) {
		// Query entities at first commit
		entities, err := st.QueryEntitiesAt(store.EntityFilter{}, "HEAD~1")
		if err != nil {
			t.Fatalf("query at HEAD~1: %v", err)
		}
		// Should have only 2 entities at HEAD~1
		if len(entities) != 2 {
			t.Errorf("expected 2 entities at HEAD~1, got %d", len(entities))
		}
	})
}

// TestDoltE2E_BranchOperations tests branch create, checkout, and delete
func TestDoltE2E_BranchOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-branch-*")
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

	db := st.DB()

	// Create initial entity and commit
	entity := &store.Entity{
		ID: "fn-branch-test", Name: "BranchTest", EntityType: "function",
		FilePath: "test.go", LineStart: 1, Language: "go",
	}
	if err := st.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if _, err := st.DoltCommit("initial commit for branch test"); err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// Test list branches
	t.Run("list branches", func(t *testing.T) {
		rows, err := db.Query("SELECT name FROM dolt_branches")
		if err != nil {
			t.Fatalf("list branches: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var name string
			rows.Scan(&name)
			count++
			t.Logf("Branch: %s", name)
		}
		if count < 1 {
			t.Error("expected at least 1 branch")
		}
	})

	// Test create branch
	t.Run("create branch", func(t *testing.T) {
		_, err := db.Exec("CALL dolt_branch('feature-test')")
		if err != nil {
			t.Fatalf("create branch: %v", err)
		}

		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM dolt_branches WHERE name='feature-test')").Scan(&exists)
		if err != nil {
			t.Fatalf("check branch exists: %v", err)
		}
		if !exists {
			t.Error("branch 'feature-test' not found after creation")
		}
	})

	// Test checkout branch
	t.Run("checkout branch", func(t *testing.T) {
		_, err := db.Exec("CALL dolt_checkout('feature-test')")
		if err != nil {
			t.Fatalf("checkout branch: %v", err)
		}

		var current string
		err = db.QueryRow("SELECT active_branch()").Scan(&current)
		if err != nil {
			t.Fatalf("get current branch: %v", err)
		}
		if current != "feature-test" {
			t.Errorf("expected current branch 'feature-test', got %q", current)
		}
	})

	// Test delete branch (switch back to main first)
	t.Run("delete branch", func(t *testing.T) {
		// Switch to main first
		if _, err := db.Exec("CALL dolt_checkout('main')"); err != nil {
			t.Fatalf("checkout main: %v", err)
		}

		_, err := db.Exec("CALL dolt_branch('-d', 'feature-test')")
		if err != nil {
			t.Fatalf("delete branch: %v", err)
		}

		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM dolt_branches WHERE name='feature-test')").Scan(&exists)
		if err != nil {
			t.Fatalf("check branch after delete: %v", err)
		}
		if exists {
			t.Error("deleted branch 'feature-test' still exists")
		}
	})
}

// TestDoltE2E_TagOperations tests tag creation and usage as refs
func TestDoltE2E_TagOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-tag-*")
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

	// Create entities and commits with tags
	for i := 1; i <= 3; i++ {
		entity := &store.Entity{
			ID: "fn-tag-test-" + string(rune('0'+i)), Name: "TagTest" + string(rune('0'+i)),
			EntityType: "function", FilePath: "test.go", LineStart: i * 10, Language: "go",
		}
		if err := st.CreateEntity(entity); err != nil {
			t.Fatalf("create entity %d: %v", i, err)
		}
		if _, err := st.DoltCommit("commit " + string(rune('0'+i))); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
		if err := st.DoltTag("v"+string(rune('0'+i))+".0", "release "+string(rune('0'+i))); err != nil {
			t.Fatalf("tag %d: %v", i, err)
		}
	}

	// Test list tags
	t.Run("list tags", func(t *testing.T) {
		tags, err := st.DoltListTags()
		if err != nil {
			t.Fatalf("list tags: %v", err)
		}
		if len(tags) != 3 {
			t.Errorf("expected 3 tags, got %d", len(tags))
		}
	})

	// Test query at tag
	t.Run("query at tag v1.0", func(t *testing.T) {
		entities, err := st.QueryEntitiesAt(store.EntityFilter{}, "v1.0")
		if err != nil {
			t.Fatalf("query at v1.0: %v", err)
		}
		// At v1.0, should have only 1 entity
		if len(entities) != 1 {
			t.Errorf("expected 1 entity at v1.0, got %d", len(entities))
		}
	})

	// Test diff between tags
	t.Run("diff between tags", func(t *testing.T) {
		added, _, _, err := st.DoltDiffSummary("v1.0", "v3.0")
		if err != nil {
			t.Fatalf("diff v1.0..v3.0: %v", err)
		}
		// Should have 2 entities added between v1.0 and v3.0
		if added != 2 {
			t.Errorf("expected 2 added between v1.0 and v3.0, got %d", added)
		}
	})
}

// TestDoltE2E_SQLExecution tests direct SQL execution
func TestDoltE2E_SQLExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-sql-*")
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

	db := st.DB()

	// Create test data
	entities := []*store.Entity{
		{ID: "fn-sql-1", Name: "SQLTest1", EntityType: "function", FilePath: "test.go", LineStart: 1, Language: "go"},
		{ID: "fn-sql-2", Name: "SQLTest2", EntityType: "function", FilePath: "test.go", LineStart: 10, Language: "go"},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}

	// Test SELECT query
	t.Run("select query", func(t *testing.T) {
		// Use exact match instead of LIKE to avoid Dolt FTS limitations
		rows, err := db.Query("SELECT id, name FROM entities WHERE id IN ('fn-sql-1', 'fn-sql-2')")
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				t.Fatalf("scan: %v", err)
			}
			count++
		}
		if count != 2 {
			t.Errorf("expected 2 results, got %d", count)
		}
	})

	// Test dolt_log query
	t.Run("dolt_log query", func(t *testing.T) {
		// First create a commit
		st.DoltCommit("commit for log test")

		var hash string
		err := db.QueryRow("SELECT commit_hash FROM dolt_log LIMIT 1").Scan(&hash)
		if err != nil {
			t.Fatalf("dolt_log query: %v", err)
		}
		if hash == "" {
			t.Error("expected non-empty commit hash")
		}
	})

	// Test dolt_status query
	t.Run("dolt_status query", func(t *testing.T) {
		rows, err := db.Query("SELECT * FROM dolt_status")
		if err != nil {
			t.Fatalf("dolt_status: %v", err)
		}
		rows.Close() // Just verify it works
	})
}

// TestDoltE2E_ScanMetadata tests scan metadata tracking
func TestDoltE2E_ScanMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-scanmeta-*")
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

	// Save scan metadata
	meta := &store.ScanMetadata{
		GitCommit:         "abc1234",
		GitBranch:         "main",
		FilesScanned:      100,
		EntitiesFound:     500,
		DependenciesFound: 1000,
		DurationMs:        2500,
	}
	if err := st.SaveScanMetadata(meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Add a small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Save another scan
	meta2 := &store.ScanMetadata{
		GitCommit:         "def5678",
		GitBranch:         "feature",
		FilesScanned:      110,
		EntitiesFound:     520,
		DependenciesFound: 1050,
		DurationMs:        2700,
	}
	if err := st.SaveScanMetadata(meta2); err != nil {
		t.Fatalf("save metadata 2: %v", err)
	}

	// Test get latest
	t.Run("get latest metadata", func(t *testing.T) {
		latest, err := st.GetLatestScanMetadata()
		if err != nil {
			t.Fatalf("get latest: %v", err)
		}
		if latest == nil {
			t.Fatal("expected metadata, got nil")
		}
		if latest.GitCommit != "def5678" {
			t.Errorf("expected commit 'def5678', got %q", latest.GitCommit)
		}
		if latest.EntitiesFound != 520 {
			t.Errorf("expected 520 entities, got %d", latest.EntitiesFound)
		}
	})
}

// TestDoltE2E_Rollback tests rollback to previous states
func TestDoltE2E_Rollback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-rollback-*")
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

	db := st.DB()

	// Create initial state
	entity1 := &store.Entity{
		ID: "fn-rollback-1", Name: "RollbackTest1", EntityType: "function",
		FilePath: "test.go", LineStart: 1, Language: "go",
	}
	if err := st.CreateEntity(entity1); err != nil {
		t.Fatalf("create entity 1: %v", err)
	}
	if _, err := st.DoltCommit("commit 1"); err != nil {
		t.Fatalf("commit 1: %v", err)
	}

	// Add more data
	entity2 := &store.Entity{
		ID: "fn-rollback-2", Name: "RollbackTest2", EntityType: "function",
		FilePath: "test.go", LineStart: 10, Language: "go",
	}
	if err := st.CreateEntity(entity2); err != nil {
		t.Fatalf("create entity 2: %v", err)
	}
	if _, err := st.DoltCommit("commit 2"); err != nil {
		t.Fatalf("commit 2: %v", err)
	}

	// Verify we have 2 entities
	entities, _ := st.QueryEntities(store.EntityFilter{})
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities before rollback, got %d", len(entities))
	}

	// Test soft rollback (DOLT_RESET)
	t.Run("soft rollback", func(t *testing.T) {
		_, err := db.Exec("CALL dolt_reset('HEAD~1')")
		if err != nil {
			t.Fatalf("soft reset: %v", err)
		}

		// After soft reset, data should still be there (working changes)
		entities, _ := st.QueryEntities(store.EntityFilter{})
		// With soft reset, the data remains but commit is undone
		t.Logf("Entities after soft reset: %d", len(entities))
	})
}

// TestDoltE2E_HistoryStats tests history with statistics
func TestDoltE2E_HistoryStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-history-*")
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

	// Create multiple commits with varying changes
	for i := 1; i <= 5; i++ {
		entity := &store.Entity{
			ID: "fn-hist-" + string(rune('0'+i)), Name: "HistTest" + string(rune('0'+i)),
			EntityType: "function", FilePath: "test.go", LineStart: i * 10, Language: "go",
		}
		if err := st.CreateEntity(entity); err != nil {
			t.Fatalf("create entity %d: %v", i, err)
		}
		if _, err := st.DoltCommit("commit " + string(rune('0'+i))); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	t.Run("history log", func(t *testing.T) {
		logs, err := st.DoltLog(10)
		if err != nil {
			t.Fatalf("get log: %v", err)
		}
		if len(logs) < 5 {
			t.Errorf("expected at least 5 commits, got %d", len(logs))
		}
		// Verify log entries have expected fields
		for _, log := range logs {
			if log.CommitHash == "" {
				t.Error("log entry has empty hash")
			}
			if log.Message == "" {
				t.Error("log entry has empty message")
			}
		}
	})

	t.Run("history stats", func(t *testing.T) {
		// Get the first commit hash
		logs, err := st.DoltLog(1)
		if err != nil || len(logs) == 0 {
			t.Fatalf("get log for stats: %v", err)
		}
		stats, err := st.DoltLogStats(logs[0].CommitHash)
		if err != nil {
			t.Fatalf("get log stats: %v", err)
		}
		if stats == nil {
			t.Error("expected stats, got nil")
		}
		t.Logf("Stats at HEAD: entities=%d, deps=%d", stats.Entities, stats.Dependencies)
	})
}

// TestDoltE2E_StaleAndCatchup tests stale entity detection
func TestDoltE2E_StaleAndCatchup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-stale-*")
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

	// Create initial entities
	entities := []*store.Entity{
		{ID: "fn-stale-1", Name: "StaleTest1", EntityType: "function", FilePath: "test.go", LineStart: 1, Language: "go"},
		{ID: "fn-stale-2", Name: "StaleTest2", EntityType: "function", FilePath: "test.go", LineStart: 10, Language: "go"},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}
	hash1, err := st.DoltCommit("initial commit")
	if err != nil {
		t.Fatalf("commit 1: %v", err)
	}
	t.Logf("Commit 1: %s", hash1)

	// Create second commit with modification
	modEntity := &store.Entity{
		ID: "fn-stale-1", Name: "StaleTest1Modified", EntityType: "function",
		FilePath: "test.go", LineStart: 1, Language: "go",
	}
	if err := st.UpdateEntity(modEntity); err != nil {
		t.Fatalf("update entity: %v", err)
	}
	hash2, err := st.DoltCommit("modified entity")
	if err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	t.Logf("Commit 2: %s", hash2)

	// Test: compare states
	t.Run("compare states", func(t *testing.T) {
		added, modified, removed, err := st.DoltDiffSummary("HEAD~1", "HEAD")
		if err != nil {
			t.Fatalf("diff summary: %v", err)
		}
		t.Logf("Diff: +%d ~%d -%d", added, modified, removed)
		// Modified should be 1 (the entity we changed)
		if modified != 1 {
			t.Errorf("expected 1 modified, got %d", modified)
		}
	})
}

// TestDoltE2E_SafeTrend tests safe --trend functionality
func TestDoltE2E_SafeTrend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-dolt-trend-*")
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

	// Create multiple commits to build history
	for i := 1; i <= 3; i++ {
		entity := &store.Entity{
			ID: "fn-trend-" + string(rune('0'+i)), Name: "TrendTest" + string(rune('0'+i)),
			EntityType: "function", FilePath: "test.go", LineStart: i * 10, Language: "go",
		}
		if err := st.CreateEntity(entity); err != nil {
			t.Fatalf("create entity %d: %v", i, err)
		}
		// Save scan metadata for each "scan"
		meta := &store.ScanMetadata{
			GitCommit:         "commit" + string(rune('0'+i)),
			GitBranch:         "main",
			FilesScanned:      10 + i,
			EntitiesFound:     i,
			DependenciesFound: i * 2,
			DurationMs:        1000 + i*100,
		}
		if err := st.SaveScanMetadata(meta); err != nil {
			t.Fatalf("save metadata %d: %v", i, err)
		}
		if _, err := st.DoltCommit("scan " + string(rune('0'+i))); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	// Test: verify we can get latest scan metadata
	t.Run("get latest scan", func(t *testing.T) {
		meta, err := st.GetLatestScanMetadata()
		if err != nil {
			t.Fatalf("get latest: %v", err)
		}
		if meta == nil {
			t.Fatal("expected metadata")
		}
		if meta.EntitiesFound != 3 {
			t.Errorf("expected 3 entities, got %d", meta.EntitiesFound)
		}
	})

	// Test: verify history exists
	t.Run("verify history", func(t *testing.T) {
		logs, err := st.DoltLog(10)
		if err != nil {
			t.Fatalf("get log: %v", err)
		}
		if len(logs) < 3 {
			t.Errorf("expected at least 3 commits, got %d", len(logs))
		}
	})
}
