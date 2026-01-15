package cache

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestCache(t *testing.T) (*Cache, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "cx-cache-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	cache, err := Open(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("open cache: %v", err)
	}

	cleanup := func() {
		cache.Close()
		os.RemoveAll(tmpDir)
	}

	return cache, cleanup
}

func TestCacheOpenClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-cache-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open cache
	cache, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}

	// Verify path
	expectedPath := filepath.Join(tmpDir, "cache.db")
	if cache.Path() != expectedPath {
		t.Errorf("path = %q, want %q", cache.Path(), expectedPath)
	}

	// Verify DB is accessible
	if cache.DB() == nil {
		t.Error("DB() returned nil")
	}

	// Close
	if err := cache.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	// Reopen should work
	cache2, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("reopen cache: %v", err)
	}
	defer cache2.Close()
}

func TestCacheClear(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	// Add some data
	now := time.Now()
	cache.SaveMetrics(&EntityMetrics{
		EntityID:   "test/entity",
		PageRank:   0.5,
		ComputedAt: now,
	})
	cache.SetFileScanned("test.go", "abc123")

	// Verify data exists
	stats, err := cache.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.MetricsCount != 1 || stats.FileIndexCount != 1 {
		t.Fatalf("expected 1 metric and 1 file, got %d and %d", stats.MetricsCount, stats.FileIndexCount)
	}

	// Clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}

	// Verify cleared
	stats, err = cache.GetStats()
	if err != nil {
		t.Fatalf("get stats after clear: %v", err)
	}
	if stats.MetricsCount != 0 || stats.FileIndexCount != 0 {
		t.Errorf("expected 0 metrics and 0 files, got %d and %d", stats.MetricsCount, stats.FileIndexCount)
	}
}

func TestMetricsSaveAndGet(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second) // SQLite stores seconds precision

	m := &EntityMetrics{
		EntityID:    "pkg/example/foo.go::Foo",
		PageRank:    0.75,
		InDegree:    5,
		OutDegree:   3,
		Betweenness: 0.42,
		ComputedAt:  now,
	}

	// Save
	if err := cache.SaveMetrics(m); err != nil {
		t.Fatalf("save metrics: %v", err)
	}

	// Get
	got, err := cache.GetMetrics(m.EntityID)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}

	// Verify
	if got.EntityID != m.EntityID {
		t.Errorf("EntityID = %q, want %q", got.EntityID, m.EntityID)
	}
	if got.PageRank != m.PageRank {
		t.Errorf("PageRank = %f, want %f", got.PageRank, m.PageRank)
	}
	if got.InDegree != m.InDegree {
		t.Errorf("InDegree = %d, want %d", got.InDegree, m.InDegree)
	}
	if got.OutDegree != m.OutDegree {
		t.Errorf("OutDegree = %d, want %d", got.OutDegree, m.OutDegree)
	}
	if got.Betweenness != m.Betweenness {
		t.Errorf("Betweenness = %f, want %f", got.Betweenness, m.Betweenness)
	}
}

func TestMetricsNotFound(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	_, err := cache.GetMetrics("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMetricsUpdate(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second)

	// Initial save
	m := &EntityMetrics{
		EntityID:    "test/entity",
		PageRank:    0.5,
		InDegree:    2,
		OutDegree:   1,
		Betweenness: 0.3,
		ComputedAt:  now,
	}
	cache.SaveMetrics(m)

	// Update
	m.PageRank = 0.8
	m.InDegree = 5
	cache.SaveMetrics(m)

	// Verify update
	got, _ := cache.GetMetrics(m.EntityID)
	if got.PageRank != 0.8 {
		t.Errorf("PageRank = %f, want 0.8", got.PageRank)
	}
	if got.InDegree != 5 {
		t.Errorf("InDegree = %d, want 5", got.InDegree)
	}
}

func TestMetricsTopByPageRank(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()

	// Add several entities with different PageRanks
	entities := []EntityMetrics{
		{EntityID: "low", PageRank: 0.1, ComputedAt: now},
		{EntityID: "high", PageRank: 0.9, ComputedAt: now},
		{EntityID: "medium", PageRank: 0.5, ComputedAt: now},
	}
	for _, e := range entities {
		cache.SaveMetrics(&e)
	}

	// Get top 2
	top, err := cache.GetTopByPageRank(2)
	if err != nil {
		t.Fatalf("get top by pagerank: %v", err)
	}

	if len(top) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top))
	}
	if top[0].EntityID != "high" {
		t.Errorf("first result = %q, want 'high'", top[0].EntityID)
	}
	if top[1].EntityID != "medium" {
		t.Errorf("second result = %q, want 'medium'", top[1].EntityID)
	}
}

func TestMetricsKeystones(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()

	entities := []EntityMetrics{
		{EntityID: "low", PageRank: 0.1, ComputedAt: now},
		{EntityID: "high", PageRank: 0.9, ComputedAt: now},
		{EntityID: "medium", PageRank: 0.5, ComputedAt: now},
	}
	for _, e := range entities {
		cache.SaveMetrics(&e)
	}

	// Get keystones with threshold 0.5
	keystones, err := cache.GetKeystones(0.5)
	if err != nil {
		t.Fatalf("get keystones: %v", err)
	}

	if len(keystones) != 2 {
		t.Fatalf("expected 2 keystones, got %d", len(keystones))
	}
}

func TestMetricsBottlenecks(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()

	entities := []EntityMetrics{
		{EntityID: "low", Betweenness: 0.1, ComputedAt: now},
		{EntityID: "high", Betweenness: 0.8, ComputedAt: now},
		{EntityID: "medium", Betweenness: 0.5, ComputedAt: now},
	}
	for _, e := range entities {
		cache.SaveMetrics(&e)
	}

	// Get bottlenecks with threshold 0.4
	bottlenecks, err := cache.GetBottlenecks(0.4)
	if err != nil {
		t.Fatalf("get bottlenecks: %v", err)
	}

	if len(bottlenecks) != 2 {
		t.Fatalf("expected 2 bottlenecks, got %d", len(bottlenecks))
	}
}

func TestMetricsBulkSave(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()

	// Create 100 metrics
	var metrics []EntityMetrics
	for i := 0; i < 100; i++ {
		metrics = append(metrics, EntityMetrics{
			EntityID:    "entity-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			PageRank:    float64(i) / 100.0,
			InDegree:    i % 10,
			OutDegree:   i % 5,
			Betweenness: float64(i%50) / 50.0,
			ComputedAt:  now,
		})
	}

	// Bulk save
	if err := cache.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("bulk save: %v", err)
	}

	// Verify count
	stats, err := cache.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.MetricsCount != 100 {
		t.Errorf("expected 100 metrics, got %d", stats.MetricsCount)
	}
}

func TestFileIndex(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	path := "pkg/foo/bar.go"
	hash := "sha256:abc123def456"

	// Set scanned
	if err := cache.SetFileScanned(path, hash); err != nil {
		t.Fatalf("set file scanned: %v", err)
	}

	// Get hash
	got, err := cache.GetFileHash(path)
	if err != nil {
		t.Fatalf("get file hash: %v", err)
	}
	if got != hash {
		t.Errorf("hash = %q, want %q", got, hash)
	}

	// Get entry
	entry, err := cache.GetFileEntry(path)
	if err != nil {
		t.Fatalf("get file entry: %v", err)
	}
	if entry.FilePath != path {
		t.Errorf("FilePath = %q, want %q", entry.FilePath, path)
	}
	if entry.ScanHash != hash {
		t.Errorf("ScanHash = %q, want %q", entry.ScanHash, hash)
	}
	if entry.ScannedAt.IsZero() {
		t.Error("ScannedAt is zero")
	}
}

func TestFileIndexNotFound(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	_, err := cache.GetFileHash("nonexistent.go")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestIsFileChanged(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	path := "test.go"
	hash1 := "hash1"
	hash2 := "hash2"

	// New file should be "changed"
	changed, err := cache.IsFileChanged(path, hash1)
	if err != nil {
		t.Fatalf("is file changed (new): %v", err)
	}
	if !changed {
		t.Error("new file should be reported as changed")
	}

	// Set scanned
	cache.SetFileScanned(path, hash1)

	// Same hash should not be changed
	changed, err = cache.IsFileChanged(path, hash1)
	if err != nil {
		t.Fatalf("is file changed (same): %v", err)
	}
	if changed {
		t.Error("same hash should not be reported as changed")
	}

	// Different hash should be changed
	changed, err = cache.IsFileChanged(path, hash2)
	if err != nil {
		t.Fatalf("is file changed (different): %v", err)
	}
	if !changed {
		t.Error("different hash should be reported as changed")
	}
}

func TestFileIndexBulkSave(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	entries := []FileEntry{
		{FilePath: "a.go", ScanHash: "hash_a"},
		{FilePath: "b.go", ScanHash: "hash_b"},
		{FilePath: "c.go", ScanHash: "hash_c"},
	}

	if err := cache.SetBulkFilesScanned(entries); err != nil {
		t.Fatalf("bulk save: %v", err)
	}

	stats, err := cache.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.FileIndexCount != 3 {
		t.Errorf("expected 3 file entries, got %d", stats.FileIndexCount)
	}
}

func TestGetChangedFiles(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	// Set initial state
	cache.SetFileScanned("a.go", "hash_a")
	cache.SetFileScanned("b.go", "hash_b")

	// Check for changes
	fileHashes := map[string]string{
		"a.go": "hash_a",     // unchanged
		"b.go": "hash_b_new", // changed
		"c.go": "hash_c",     // new
	}

	changed, err := cache.GetChangedFiles(fileHashes)
	if err != nil {
		t.Fatalf("get changed files: %v", err)
	}

	// Should have b.go and c.go as changed
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed files, got %d", len(changed))
	}

	changedSet := make(map[string]bool)
	for _, p := range changed {
		changedSet[p] = true
	}
	if !changedSet["b.go"] || !changedSet["c.go"] {
		t.Errorf("expected b.go and c.go to be changed, got %v", changed)
	}
}

func TestPruneStaleEntries(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	// Add files
	cache.SetFileScanned("keep.go", "hash1")
	cache.SetFileScanned("delete.go", "hash2")
	cache.SetFileScanned("also_delete.go", "hash3")

	// Prune
	validPaths := map[string]bool{
		"keep.go": true,
	}
	pruned, err := cache.PruneStaleEntries(validPaths)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}

	if pruned != 2 {
		t.Errorf("expected 2 pruned, got %d", pruned)
	}

	// Verify
	stats, _ := cache.GetStats()
	if stats.FileIndexCount != 1 {
		t.Errorf("expected 1 file entry remaining, got %d", stats.FileIndexCount)
	}

	// Verify the right one was kept
	_, err = cache.GetFileHash("keep.go")
	if err != nil {
		t.Error("keep.go should still exist")
	}
}

func TestClearMetrics(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()
	cache.SaveMetrics(&EntityMetrics{EntityID: "test", ComputedAt: now})
	cache.SetFileScanned("test.go", "hash")

	// Clear only metrics
	if err := cache.ClearMetrics(); err != nil {
		t.Fatalf("clear metrics: %v", err)
	}

	stats, _ := cache.GetStats()
	if stats.MetricsCount != 0 {
		t.Errorf("metrics should be 0, got %d", stats.MetricsCount)
	}
	if stats.FileIndexCount != 1 {
		t.Errorf("file index should still be 1, got %d", stats.FileIndexCount)
	}
}

func TestClearFileIndex(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()
	cache.SaveMetrics(&EntityMetrics{EntityID: "test", ComputedAt: now})
	cache.SetFileScanned("test.go", "hash")

	// Clear only file index
	if err := cache.ClearFileIndex(); err != nil {
		t.Fatalf("clear file index: %v", err)
	}

	stats, _ := cache.GetStats()
	if stats.MetricsCount != 1 {
		t.Errorf("metrics should still be 1, got %d", stats.MetricsCount)
	}
	if stats.FileIndexCount != 0 {
		t.Errorf("file index should be 0, got %d", stats.FileIndexCount)
	}
}

func TestDeleteMetrics(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()
	cache.SaveMetrics(&EntityMetrics{EntityID: "keep", ComputedAt: now})
	cache.SaveMetrics(&EntityMetrics{EntityID: "delete", ComputedAt: now})

	// Delete one
	if err := cache.DeleteMetrics("delete"); err != nil {
		t.Fatalf("delete metrics: %v", err)
	}

	// Verify
	_, err := cache.GetMetrics("keep")
	if err != nil {
		t.Error("'keep' should still exist")
	}
	_, err = cache.GetMetrics("delete")
	if err != sql.ErrNoRows {
		t.Error("'delete' should not exist")
	}
}

func TestGetAllMetrics(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()
	cache.SaveMetrics(&EntityMetrics{EntityID: "a", PageRank: 0.3, ComputedAt: now})
	cache.SaveMetrics(&EntityMetrics{EntityID: "b", PageRank: 0.7, ComputedAt: now})
	cache.SaveMetrics(&EntityMetrics{EntityID: "c", PageRank: 0.5, ComputedAt: now})

	all, err := cache.GetAllMetrics()
	if err != nil {
		t.Fatalf("get all metrics: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(all))
	}

	// Should be sorted by pagerank descending
	if all[0].EntityID != "b" {
		t.Errorf("first should be 'b' (highest pagerank), got %q", all[0].EntityID)
	}
}
