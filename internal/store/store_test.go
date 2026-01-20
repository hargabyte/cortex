package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testStore creates a temporary store for testing.
func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "cx-store-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	store, err := Open(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("open store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestOpen(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-store-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")

	// Open should create the .cx directory
	store, err := Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Verify directory was created
	if _, err := os.Stat(cxDir); os.IsNotExist(err) {
		t.Error("expected .cx directory to be created")
	}

	// Verify Dolt database directory exists (now .cx/cortex/ instead of .cx/cortex.db)
	dbPath := filepath.Join(cxDir, "cortex")
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		t.Error("expected cortex directory to be created")
	} else if !info.IsDir() {
		t.Error("expected cortex to be a directory (Dolt repo)")
	}

	// Verify Path() returns correct path
	if store.Path() != dbPath {
		t.Errorf("expected path %s, got %s", dbPath, store.Path())
	}
}

func TestOpenDefault(t *testing.T) {
	// Skip this test if we can't change directories safely
	t.Skip("OpenDefault changes working directory, skipping in parallel test")
}

func TestStore_Close(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Close should succeed
	if err := store.Close(); err != nil {
		t.Errorf("close store: %v", err)
	}

	// Closing nil db should not panic
	store.db = nil
	if err := store.Close(); err != nil {
		t.Errorf("close nil db: %v", err)
	}
}

// Entity tests

func TestCreateEntity(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	lineEnd := 20
	entity := &Entity{
		ID:         "test-fn-abc123",
		Name:       "TestFunc",
		EntityType: "function",
		Kind:       "",
		FilePath:   "pkg/test.go",
		LineStart:  10,
		LineEnd:    &lineEnd,
		Signature:  "(x int) string",
		SigHash:    "a1b2c3d4",
		BodyHash:   "e5f6g7h8",
		Visibility: "pub",
	}

	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Verify default status
	got, err := store.GetEntity("test-fn-abc123")
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}

	if got.Status != "active" {
		t.Errorf("expected status 'active', got %q", got.Status)
	}
	if got.Name != "TestFunc" {
		t.Errorf("expected name 'TestFunc', got %q", got.Name)
	}
	if got.EntityType != "function" {
		t.Errorf("expected entity_type 'function', got %q", got.EntityType)
	}
	if got.LineEnd == nil || *got.LineEnd != 20 {
		t.Errorf("expected line_end 20, got %v", got.LineEnd)
	}
}

func TestCreateEntitiesBulk(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entities := []*Entity{
		{ID: "fn-1", Name: "Func1", EntityType: "function", FilePath: "a.go", LineStart: 1, Visibility: "pub"},
		{ID: "fn-2", Name: "Func2", EntityType: "function", FilePath: "a.go", LineStart: 10, Visibility: "pub"},
		{ID: "fn-3", Name: "Func3", EntityType: "function", FilePath: "b.go", LineStart: 1, Visibility: "priv"},
	}

	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities bulk: %v", err)
	}

	// Verify all entities exist
	for _, e := range entities {
		got, err := store.GetEntity(e.ID)
		if err != nil {
			t.Errorf("get entity %s: %v", e.ID, err)
			continue
		}
		if got.Name != e.Name {
			t.Errorf("expected name %s, got %s", e.Name, got.Name)
		}
	}

	// Empty slice should be no-op
	if err := store.CreateEntitiesBulk([]*Entity{}); err != nil {
		t.Errorf("create empty bulk: %v", err)
	}
}

func TestGetEntity_NotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	_, err := store.GetEntity("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestQueryEntities(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entities := []*Entity{
		{ID: "fn-1", Name: "CreateUser", EntityType: "function", FilePath: "pkg/user.go", LineStart: 1, Visibility: "pub", Status: "active"},
		{ID: "fn-2", Name: "DeleteUser", EntityType: "function", FilePath: "pkg/user.go", LineStart: 10, Visibility: "pub", Status: "active"},
		{ID: "tp-1", Name: "User", EntityType: "type", Kind: "struct", FilePath: "pkg/user.go", LineStart: 50, Visibility: "pub", Status: "active"},
		{ID: "fn-3", Name: "Helper", EntityType: "function", FilePath: "internal/util.go", LineStart: 1, Visibility: "priv", Status: "archived"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name   string
		filter EntityFilter
		want   int
	}{
		{
			name:   "all",
			filter: EntityFilter{},
			want:   4,
		},
		{
			name:   "by type function",
			filter: EntityFilter{EntityType: "function"},
			want:   3,
		},
		{
			name:   "by type type",
			filter: EntityFilter{EntityType: "type"},
			want:   1,
		},
		{
			name:   "by status active",
			filter: EntityFilter{Status: "active"},
			want:   3,
		},
		{
			name:   "by status archived",
			filter: EntityFilter{Status: "archived"},
			want:   1,
		},
		{
			name:   "by file path prefix",
			filter: EntityFilter{FilePath: "pkg/"},
			want:   3,
		},
		{
			name:   "by name contains User",
			filter: EntityFilter{Name: "User"},
			want:   3,
		},
		{
			name:   "with limit",
			filter: EntityFilter{Limit: 2},
			want:   2,
		},
		{
			name:   "with limit and offset",
			filter: EntityFilter{Limit: 2, Offset: 2},
			want:   2,
		},
		{
			name:   "combined filters",
			filter: EntityFilter{EntityType: "function", Status: "active", FilePath: "pkg/"},
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.QueryEntities(tt.filter)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("expected %d entities, got %d", tt.want, len(got))
			}
		})
	}
}

func TestEntityLanguageField(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test default language is "go"
	entity := &Entity{
		ID:         "fn-lang-default",
		Name:       "DefaultLang",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetEntity("fn-lang-default")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Language != "go" {
		t.Errorf("expected default language 'go', got %q", got.Language)
	}

	// Test explicit language setting
	entityTS := &Entity{
		ID:         "fn-lang-ts",
		Name:       "TypeScriptFunc",
		EntityType: "function",
		FilePath:   "test.ts",
		LineStart:  1,
		Visibility: "pub",
		Language:   "typescript",
	}
	if err := store.CreateEntity(entityTS); err != nil {
		t.Fatalf("create ts: %v", err)
	}

	gotTS, err := store.GetEntity("fn-lang-ts")
	if err != nil {
		t.Fatalf("get ts: %v", err)
	}
	if gotTS.Language != "typescript" {
		t.Errorf("expected language 'typescript', got %q", gotTS.Language)
	}
}

func TestQueryEntitiesByLanguage(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create entities with different languages
	entities := []*Entity{
		{ID: "fn-go-1", Name: "GoFunc1", EntityType: "function", FilePath: "a.go", LineStart: 1, Language: "go", Visibility: "pub"},
		{ID: "fn-go-2", Name: "GoFunc2", EntityType: "function", FilePath: "b.go", LineStart: 1, Language: "go", Visibility: "pub"},
		{ID: "fn-ts-1", Name: "TSFunc1", EntityType: "function", FilePath: "a.ts", LineStart: 1, Language: "typescript", Visibility: "pub"},
		{ID: "fn-py-1", Name: "PyFunc1", EntityType: "function", FilePath: "a.py", LineStart: 1, Language: "python", Visibility: "pub"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name   string
		filter EntityFilter
		want   int
	}{
		{
			name:   "filter by go",
			filter: EntityFilter{Language: "go"},
			want:   2,
		},
		{
			name:   "filter by typescript",
			filter: EntityFilter{Language: "typescript"},
			want:   1,
		},
		{
			name:   "filter by python",
			filter: EntityFilter{Language: "python"},
			want:   1,
		},
		{
			name:   "filter by nonexistent language",
			filter: EntityFilter{Language: "rust"},
			want:   0,
		},
		{
			name:   "combined type and language",
			filter: EntityFilter{EntityType: "function", Language: "go"},
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.QueryEntities(tt.filter)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("expected %d entities, got %d", tt.want, len(got))
			}
		})
	}
}

func TestCountEntitiesByLanguage(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entities := []*Entity{
		{ID: "fn-go-1", Name: "GoFunc1", EntityType: "function", FilePath: "a.go", LineStart: 1, Language: "go", Visibility: "pub"},
		{ID: "fn-go-2", Name: "GoFunc2", EntityType: "function", FilePath: "b.go", LineStart: 1, Language: "go", Visibility: "pub"},
		{ID: "fn-ts-1", Name: "TSFunc1", EntityType: "function", FilePath: "a.ts", LineStart: 1, Language: "typescript", Visibility: "pub"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup: %v", err)
	}

	count, err := store.CountEntities(EntityFilter{Language: "go"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 go entities, got %d", count)
	}

	count, err = store.CountEntities(EntityFilter{Language: "typescript"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 typescript entity, got %d", count)
	}
}

func TestUpdateEntityLanguage(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entity := &Entity{
		ID:         "fn-lang-update",
		Name:       "LangUpdate",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Language:   "go",
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update language
	if err := store.UpdateEntity(&Entity{ID: "fn-lang-update", Language: "typescript"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetEntity("fn-lang-update")
	if got.Language != "typescript" {
		t.Errorf("expected language 'typescript', got %q", got.Language)
	}
}

func TestUpdateEntity(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entity := &Entity{
		ID:         "fn-update",
		Name:       "Original",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update name
	if err := store.UpdateEntity(&Entity{ID: "fn-update", Name: "Updated"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetEntity("fn-update")
	if got.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", got.Name)
	}

	// Update nonexistent should return error
	err := store.UpdateEntity(&Entity{ID: "nonexistent", Name: "Test"})
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for nonexistent entity, got %v", err)
	}

	// Empty ID should error
	err = store.UpdateEntity(&Entity{ID: "", Name: "Test"})
	if err == nil {
		t.Error("expected error for empty ID")
	}

	// No updates should be no-op
	if err := store.UpdateEntity(&Entity{ID: "fn-update"}); err != nil {
		t.Errorf("empty update should succeed: %v", err)
	}
}

func TestDeleteEntity(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entity := &Entity{
		ID:         "fn-delete",
		Name:       "ToDelete",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.DeleteEntity("fn-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := store.GetEntity("fn-delete")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}

	// Delete nonexistent should return error
	err = store.DeleteEntity("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for nonexistent, got %v", err)
	}
}

func TestArchiveEntity(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entity := &Entity{
		ID:         "fn-archive",
		Name:       "ToArchive",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.ArchiveEntity("fn-archive"); err != nil {
		t.Fatalf("archive: %v", err)
	}

	got, _ := store.GetEntity("fn-archive")
	if got.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", got.Status)
	}

	// Archive nonexistent should return error
	err := store.ArchiveEntity("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for nonexistent, got %v", err)
	}
}

func TestCountEntities(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entities := []*Entity{
		{ID: "fn-1", Name: "Func1", EntityType: "function", FilePath: "a.go", LineStart: 1, Visibility: "pub"},
		{ID: "fn-2", Name: "Func2", EntityType: "function", FilePath: "a.go", LineStart: 10, Visibility: "pub"},
		{ID: "tp-1", Name: "Type1", EntityType: "type", FilePath: "a.go", LineStart: 20, Visibility: "pub"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup: %v", err)
	}

	count, err := store.CountEntities(EntityFilter{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	count, err = store.CountEntities(EntityFilter{EntityType: "function"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 functions, got %d", count)
	}
}

func TestDeleteEntitiesByFile(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entities := []*Entity{
		{ID: "fn-1", Name: "Func1", EntityType: "function", FilePath: "a.go", LineStart: 1, Visibility: "pub"},
		{ID: "fn-2", Name: "Func2", EntityType: "function", FilePath: "a.go", LineStart: 10, Visibility: "pub"},
		{ID: "fn-3", Name: "Func3", EntityType: "function", FilePath: "b.go", LineStart: 1, Visibility: "pub"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.DeleteEntitiesByFile("a.go"); err != nil {
		t.Fatalf("delete by file: %v", err)
	}

	// Only fn-3 should remain
	remaining, _ := store.QueryEntities(EntityFilter{})
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
	if len(remaining) > 0 && remaining[0].ID != "fn-3" {
		t.Errorf("expected fn-3 to remain, got %s", remaining[0].ID)
	}

	// Delete from nonexistent file should not error
	if err := store.DeleteEntitiesByFile("nonexistent.go"); err != nil {
		t.Errorf("expected no error for nonexistent file: %v", err)
	}
}

// Dependency tests

func TestCreateDependency(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	dep := &Dependency{
		FromID:  "fn-caller",
		ToID:    "fn-callee",
		DepType: "calls",
	}

	if err := store.CreateDependency(dep); err != nil {
		t.Fatalf("create dependency: %v", err)
	}

	// Verify it exists
	deps, err := store.GetDependenciesFrom("fn-caller")
	if err != nil {
		t.Fatalf("get deps: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].ToID != "fn-callee" {
		t.Errorf("expected to_id 'fn-callee', got %q", deps[0].ToID)
	}

	// Duplicate should replace (not error)
	if err := store.CreateDependency(dep); err != nil {
		t.Errorf("duplicate should replace: %v", err)
	}
}

func TestCreateDependenciesBulk(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	deps := []*Dependency{
		{FromID: "fn-1", ToID: "fn-2", DepType: "calls"},
		{FromID: "fn-1", ToID: "fn-3", DepType: "calls"},
		{FromID: "fn-2", ToID: "fn-3", DepType: "uses_type"},
	}

	if err := store.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("create bulk: %v", err)
	}

	count, _ := store.CountDependencies()
	if count != 3 {
		t.Errorf("expected 3 deps, got %d", count)
	}

	// Empty slice should be no-op
	if err := store.CreateDependenciesBulk([]*Dependency{}); err != nil {
		t.Errorf("empty bulk: %v", err)
	}
}

func TestGetDependencies(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	deps := []*Dependency{
		{FromID: "fn-1", ToID: "fn-2", DepType: "calls"},
		{FromID: "fn-1", ToID: "fn-3", DepType: "uses_type"},
		{FromID: "fn-2", ToID: "fn-3", DepType: "calls"},
	}
	if err := store.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// From fn-1
	got, err := store.GetDependenciesFrom("fn-1")
	if err != nil {
		t.Fatalf("get from: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 deps from fn-1, got %d", len(got))
	}

	// To fn-3
	got, err = store.GetDependenciesTo("fn-3")
	if err != nil {
		t.Fatalf("get to: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 deps to fn-3, got %d", len(got))
	}

	// By type
	got, err = store.GetDependencies(DependencyFilter{DepType: "calls"})
	if err != nil {
		t.Fatalf("get by type: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 'calls' deps, got %d", len(got))
	}

	// Get all
	got, err = store.GetAllDependencies()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 total deps, got %d", len(got))
	}
}

func TestDeleteDependency(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	deps := []*Dependency{
		{FromID: "fn-1", ToID: "fn-2", DepType: "calls"},
		{FromID: "fn-1", ToID: "fn-3", DepType: "calls"},
	}
	if err := store.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.DeleteDependency("fn-1", "fn-2", "calls"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, _ := store.GetDependenciesFrom("fn-1")
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestDeleteDependenciesFrom(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	deps := []*Dependency{
		{FromID: "fn-1", ToID: "fn-2", DepType: "calls"},
		{FromID: "fn-1", ToID: "fn-3", DepType: "calls"},
		{FromID: "fn-2", ToID: "fn-3", DepType: "calls"},
	}
	if err := store.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.DeleteDependenciesFrom("fn-1"); err != nil {
		t.Fatalf("delete from: %v", err)
	}

	count, _ := store.CountDependencies()
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestDeleteDependenciesByFile(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create entities first
	entities := []*Entity{
		{ID: "fn-1", Name: "Func1", EntityType: "function", FilePath: "a.go", LineStart: 1, Visibility: "pub"},
		{ID: "fn-2", Name: "Func2", EntityType: "function", FilePath: "b.go", LineStart: 1, Visibility: "pub"},
	}
	if err := store.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("setup entities: %v", err)
	}

	deps := []*Dependency{
		{FromID: "fn-1", ToID: "fn-2", DepType: "calls"},
		{FromID: "fn-2", ToID: "fn-1", DepType: "calls"},
	}
	if err := store.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("setup deps: %v", err)
	}

	if err := store.DeleteDependenciesByFile("a.go"); err != nil {
		t.Fatalf("delete by file: %v", err)
	}

	// Only dep from fn-2 should remain
	remaining, _ := store.GetAllDependencies()
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
	if len(remaining) > 0 && remaining[0].FromID != "fn-2" {
		t.Errorf("expected fn-2 dep to remain")
	}
}

// Links tests

func TestCreateLink(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	link := &EntityLink{
		EntityID:       "fn-1",
		ExternalSystem: "beads",
		ExternalID:     "bd-abc123",
		LinkType:       "implements",
	}

	if err := store.CreateLink(link); err != nil {
		t.Fatalf("create link: %v", err)
	}

	// Verify
	links, err := store.GetLinks("fn-1")
	if err != nil {
		t.Fatalf("get links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].ExternalID != "bd-abc123" {
		t.Errorf("expected external_id 'bd-abc123', got %q", links[0].ExternalID)
	}

	// Default link type
	link2 := &EntityLink{
		EntityID:       "fn-2",
		ExternalSystem: "github",
		ExternalID:     "issue-456",
	}
	if err := store.CreateLink(link2); err != nil {
		t.Fatalf("create link2: %v", err)
	}
	links, _ = store.GetLinks("fn-2")
	if len(links) > 0 && links[0].LinkType != "related" {
		t.Errorf("expected default link_type 'related', got %q", links[0].LinkType)
	}
}

func TestGetLinksByExternalID(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	links := []*EntityLink{
		{EntityID: "fn-1", ExternalSystem: "beads", ExternalID: "bd-abc", LinkType: "implements"},
		{EntityID: "fn-2", ExternalSystem: "beads", ExternalID: "bd-abc", LinkType: "related"},
		{EntityID: "fn-3", ExternalSystem: "beads", ExternalID: "bd-xyz", LinkType: "related"},
	}
	for _, link := range links {
		if err := store.CreateLink(link); err != nil {
			t.Fatalf("create link: %v", err)
		}
	}

	got, err := store.GetLinksByExternalID("beads", "bd-abc")
	if err != nil {
		t.Fatalf("get by external id: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 links to bd-abc, got %d", len(got))
	}
}

func TestGetLinksBySystem(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	links := []*EntityLink{
		{EntityID: "fn-1", ExternalSystem: "beads", ExternalID: "bd-1"},
		{EntityID: "fn-2", ExternalSystem: "beads", ExternalID: "bd-2"},
		{EntityID: "fn-3", ExternalSystem: "github", ExternalID: "issue-1"},
	}
	for _, link := range links {
		if err := store.CreateLink(link); err != nil {
			t.Fatalf("create link: %v", err)
		}
	}

	got, err := store.GetLinksBySystem("beads")
	if err != nil {
		t.Fatalf("get by system: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 beads links, got %d", len(got))
	}
}

func TestDeleteLink(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	link := &EntityLink{
		EntityID:       "fn-1",
		ExternalSystem: "beads",
		ExternalID:     "bd-abc",
	}
	if err := store.CreateLink(link); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.DeleteLink("fn-1", "beads", "bd-abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	links, _ := store.GetLinks("fn-1")
	if len(links) != 0 {
		t.Errorf("expected 0 links after delete, got %d", len(links))
	}
}

func TestDeleteLinksForEntity(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	links := []*EntityLink{
		{EntityID: "fn-1", ExternalSystem: "beads", ExternalID: "bd-1"},
		{EntityID: "fn-1", ExternalSystem: "github", ExternalID: "issue-1"},
	}
	for _, link := range links {
		if err := store.CreateLink(link); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if err := store.DeleteLinksForEntity("fn-1"); err != nil {
		t.Fatalf("delete for entity: %v", err)
	}

	remaining, _ := store.GetLinks("fn-1")
	if len(remaining) != 0 {
		t.Errorf("expected 0 links, got %d", len(remaining))
	}
}

func TestCountLinks(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	links := []*EntityLink{
		{EntityID: "fn-1", ExternalSystem: "beads", ExternalID: "bd-1"},
		{EntityID: "fn-2", ExternalSystem: "beads", ExternalID: "bd-2"},
	}
	for _, link := range links {
		if err := store.CreateLink(link); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	count, err := store.CountLinks()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// FileIndex tests

func TestSetFileScanned(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	if err := store.SetFileScanned("test.go", "abc123"); err != nil {
		t.Fatalf("set file scanned: %v", err)
	}

	hash, err := store.GetFileHash("test.go")
	if err != nil {
		t.Fatalf("get hash: %v", err)
	}
	if hash != "abc123" {
		t.Errorf("expected hash 'abc123', got %q", hash)
	}

	// Update should replace
	if err := store.SetFileScanned("test.go", "xyz789"); err != nil {
		t.Fatalf("update file scanned: %v", err)
	}
	hash, _ = store.GetFileHash("test.go")
	if hash != "xyz789" {
		t.Errorf("expected updated hash 'xyz789', got %q", hash)
	}
}

func TestSetFilesScannedBulk(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entries := []*FileIndex{
		{FilePath: "a.go", ScanHash: "hash-a"},
		{FilePath: "b.go", ScanHash: "hash-b"},
		{FilePath: "c.go", ScanHash: "hash-c"},
	}

	if err := store.SetFilesScannedBulk(entries); err != nil {
		t.Fatalf("bulk set: %v", err)
	}

	count, _ := store.CountFileIndex()
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	// Empty should be no-op
	if err := store.SetFilesScannedBulk([]*FileIndex{}); err != nil {
		t.Errorf("empty bulk: %v", err)
	}
}

func TestGetFileHash_NotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	_, err := store.GetFileHash("nonexistent.go")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetFileEntry(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	if err := store.SetFileScanned("test.go", "abc123"); err != nil {
		t.Fatalf("set: %v", err)
	}

	entry, err := store.GetFileEntry("test.go")
	if err != nil {
		t.Fatalf("get entry: %v", err)
	}
	if entry.ScanHash != "abc123" {
		t.Errorf("expected hash 'abc123', got %q", entry.ScanHash)
	}
	if entry.ScannedAt.IsZero() {
		t.Error("expected non-zero scanned_at")
	}
}

func TestIsFileChanged(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Never scanned = changed
	changed, err := store.IsFileChanged("new.go", "hash123")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !changed {
		t.Error("expected new file to be marked as changed")
	}

	// Scan it
	if err := store.SetFileScanned("new.go", "hash123"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Same hash = not changed
	changed, _ = store.IsFileChanged("new.go", "hash123")
	if changed {
		t.Error("expected same hash to not be changed")
	}

	// Different hash = changed
	changed, _ = store.IsFileChanged("new.go", "different")
	if !changed {
		t.Error("expected different hash to be changed")
	}
}

func TestGetAllFileEntries(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entries := []*FileIndex{
		{FilePath: "a.go", ScanHash: "hash-a"},
		{FilePath: "b.go", ScanHash: "hash-b"},
	}
	if err := store.SetFilesScannedBulk(entries); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetAllFileEntries()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestDeleteFileEntry(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	if err := store.SetFileScanned("test.go", "abc"); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := store.DeleteFileEntry("test.go"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := store.GetFileHash("test.go")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestGetChangedFiles(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Scan some files
	entries := []*FileIndex{
		{FilePath: "a.go", ScanHash: "hash-a"},
		{FilePath: "b.go", ScanHash: "hash-b"},
	}
	if err := store.SetFilesScannedBulk(entries); err != nil {
		t.Fatalf("setup: %v", err)
	}

	fileHashes := map[string]string{
		"a.go": "hash-a",    // unchanged
		"b.go": "different", // changed
		"c.go": "new-hash",  // new
	}

	changed, err := store.GetChangedFiles(fileHashes)
	if err != nil {
		t.Fatalf("get changed: %v", err)
	}
	if len(changed) != 2 {
		t.Errorf("expected 2 changed (b.go, c.go), got %d", len(changed))
	}
}

func TestPruneStaleEntries(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entries := []*FileIndex{
		{FilePath: "keep.go", ScanHash: "hash-keep"},
		{FilePath: "delete.go", ScanHash: "hash-delete"},
	}
	if err := store.SetFilesScannedBulk(entries); err != nil {
		t.Fatalf("setup: %v", err)
	}

	validPaths := map[string]bool{
		"keep.go": true,
	}

	pruned, err := store.PruneStaleEntries(validPaths)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	count, _ := store.CountFileIndex()
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestClearFileIndex(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	entries := []*FileIndex{
		{FilePath: "a.go", ScanHash: "hash-a"},
		{FilePath: "b.go", ScanHash: "hash-b"},
	}
	if err := store.SetFilesScannedBulk(entries); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.ClearFileIndex(); err != nil {
		t.Fatalf("clear: %v", err)
	}

	count, _ := store.CountFileIndex()
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

// Metrics tests

func TestSaveMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	m := &Metrics{
		EntityID:    "fn-test",
		PageRank:    0.85,
		InDegree:    5,
		OutDegree:   3,
		Betweenness: 0.25,
		ComputedAt:  time.Now().UTC(),
	}

	if err := store.SaveMetrics(m); err != nil {
		t.Fatalf("save metrics: %v", err)
	}

	got, err := store.GetMetrics("fn-test")
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	if got.PageRank != 0.85 {
		t.Errorf("expected pagerank 0.85, got %f", got.PageRank)
	}
	if got.InDegree != 5 {
		t.Errorf("expected in_degree 5, got %d", got.InDegree)
	}
}

func TestGetMetrics_NotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	_, err := store.GetMetrics("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetAllMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", PageRank: 0.5, InDegree: 1, OutDegree: 1, Betweenness: 0.1, ComputedAt: time.Now()},
		{EntityID: "fn-2", PageRank: 0.9, InDegree: 5, OutDegree: 2, Betweenness: 0.5, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetAllMetrics()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
	// Should be sorted by pagerank desc
	if got[0].PageRank < got[1].PageRank {
		t.Error("expected descending pagerank order")
	}
}

func TestGetTopByPageRank(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", PageRank: 0.3, ComputedAt: time.Now()},
		{EntityID: "fn-2", PageRank: 0.9, ComputedAt: time.Now()},
		{EntityID: "fn-3", PageRank: 0.6, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetTopByPageRank(2)
	if err != nil {
		t.Fatalf("get top: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].EntityID != "fn-2" {
		t.Errorf("expected fn-2 first, got %s", got[0].EntityID)
	}
	if got[1].EntityID != "fn-3" {
		t.Errorf("expected fn-3 second, got %s", got[1].EntityID)
	}
}

func TestGetTopByBetweenness(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", Betweenness: 0.1, ComputedAt: time.Now()},
		{EntityID: "fn-2", Betweenness: 0.9, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetTopByBetweenness(1)
	if err != nil {
		t.Fatalf("get top: %v", err)
	}
	if len(got) != 1 || got[0].EntityID != "fn-2" {
		t.Errorf("expected fn-2, got %v", got)
	}
}

func TestGetTopByInDegree(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", InDegree: 10, ComputedAt: time.Now()},
		{EntityID: "fn-2", InDegree: 2, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetTopByInDegree(1)
	if err != nil {
		t.Fatalf("get top: %v", err)
	}
	if len(got) != 1 || got[0].InDegree != 10 {
		t.Errorf("expected in_degree 10, got %v", got)
	}
}

func TestGetTopByOutDegree(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", OutDegree: 1, ComputedAt: time.Now()},
		{EntityID: "fn-2", OutDegree: 20, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetTopByOutDegree(1)
	if err != nil {
		t.Fatalf("get top: %v", err)
	}
	if len(got) != 1 || got[0].OutDegree != 20 {
		t.Errorf("expected out_degree 20, got %v", got)
	}
}

func TestGetKeystones(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-low", PageRank: 0.2, ComputedAt: time.Now()},
		{EntityID: "fn-high", PageRank: 0.8, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetKeystones(0.5)
	if err != nil {
		t.Fatalf("get keystones: %v", err)
	}
	if len(got) != 1 || got[0].EntityID != "fn-high" {
		t.Errorf("expected only fn-high, got %v", got)
	}
}

func TestGetBottlenecks(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-low", Betweenness: 0.1, ComputedAt: time.Now()},
		{EntityID: "fn-high", Betweenness: 0.7, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetBottlenecks(0.5)
	if err != nil {
		t.Fatalf("get bottlenecks: %v", err)
	}
	if len(got) != 1 || got[0].EntityID != "fn-high" {
		t.Errorf("expected only fn-high, got %v", got)
	}
}

func TestGetHighlyConnected(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-low", InDegree: 2, ComputedAt: time.Now()},
		{EntityID: "fn-high", InDegree: 15, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := store.GetHighlyConnected(10)
	if err != nil {
		t.Fatalf("get highly connected: %v", err)
	}
	if len(got) != 1 || got[0].EntityID != "fn-high" {
		t.Errorf("expected only fn-high, got %v", got)
	}
}

func TestSaveBulkMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", PageRank: 0.5, ComputedAt: time.Now()},
		{EntityID: "fn-2", PageRank: 0.9, ComputedAt: time.Now()},
		{EntityID: "fn-3", PageRank: 0.3, ComputedAt: time.Now()},
	}

	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("save bulk: %v", err)
	}

	count, _ := store.CountMetrics()
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	// Empty should be no-op
	if err := store.SaveBulkMetrics([]*Metrics{}); err != nil {
		t.Errorf("empty bulk: %v", err)
	}
}

func TestDeleteMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	m := &Metrics{EntityID: "fn-test", PageRank: 0.5, ComputedAt: time.Now()}
	if err := store.SaveMetrics(m); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.DeleteMetrics("fn-test"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := store.GetMetrics("fn-test")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestClearMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	metrics := []*Metrics{
		{EntityID: "fn-1", PageRank: 0.5, ComputedAt: time.Now()},
		{EntityID: "fn-2", PageRank: 0.9, ComputedAt: time.Now()},
	}
	if err := store.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := store.ClearMetrics(); err != nil {
		t.Fatalf("clear: %v", err)
	}

	count, _ := store.CountMetrics()
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestCountMetrics(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	count, err := store.CountMetrics()
	if err != nil {
		t.Fatalf("count empty: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	m := &Metrics{EntityID: "fn-test", PageRank: 0.5, ComputedAt: time.Now()}
	if err := store.SaveMetrics(m); err != nil {
		t.Fatalf("save: %v", err)
	}

	count, _ = store.CountMetrics()
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestSaveScanMetadata(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	meta := &ScanMetadata{
		GitCommit:         "abc1234",
		GitBranch:         "main",
		FilesScanned:      10,
		EntitiesFound:     50,
		DependenciesFound: 100,
		DurationMs:        1500,
	}

	if err := store.SaveScanMetadata(meta); err != nil {
		t.Fatalf("save scan metadata: %v", err)
	}

	// Verify the metadata was saved by querying directly
	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM scan_metadata").Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	// Verify the values
	var gitCommit, gitBranch string
	var files, entities, deps, duration int
	err = store.db.QueryRow(`
		SELECT git_commit, git_branch, files_scanned, entities_found, dependencies_found, scan_duration_ms
		FROM scan_metadata ORDER BY id DESC LIMIT 1
	`).Scan(&gitCommit, &gitBranch, &files, &entities, &deps, &duration)
	if err != nil {
		t.Fatalf("query values: %v", err)
	}

	if gitCommit != "abc1234" {
		t.Errorf("expected git_commit 'abc1234', got %q", gitCommit)
	}
	if gitBranch != "main" {
		t.Errorf("expected git_branch 'main', got %q", gitBranch)
	}
	if files != 10 {
		t.Errorf("expected files_scanned 10, got %d", files)
	}
	if entities != 50 {
		t.Errorf("expected entities_found 50, got %d", entities)
	}
	if deps != 100 {
		t.Errorf("expected dependencies_found 100, got %d", deps)
	}
	if duration != 1500 {
		t.Errorf("expected scan_duration_ms 1500, got %d", duration)
	}
}

func TestDoltCommit(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create some data to commit
	entity := &Entity{
		ID:         "fn-dolt-commit-test",
		Name:       "TestFunc",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Language:   "go",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Test Dolt commit
	hash, err := store.DoltCommit("test commit message")
	if err != nil {
		t.Fatalf("dolt commit: %v", err)
	}

	// Hash may be empty if dolt_log query fails, but commit should succeed
	t.Logf("Dolt commit hash: %q", hash)

	// Verify the commit was created by checking dolt_log
	var commitHash, commitMessage string
	err = store.db.QueryRow("SELECT commit_hash, message FROM dolt_log LIMIT 1").Scan(&commitHash, &commitMessage)
	if err != nil {
		t.Fatalf("query dolt_log: %v", err)
	}

	if commitMessage != "test commit message" {
		t.Errorf("expected message 'test commit message', got %q", commitMessage)
	}
	if commitHash == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestDoltTag(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create some data and commit first
	entity := &Entity{
		ID:         "fn-dolt-tag-test",
		Name:       "TestFuncTag",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  1,
		Language:   "go",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Create a commit first
	_, err := store.DoltCommit("commit before tag")
	if err != nil {
		t.Fatalf("dolt commit: %v", err)
	}

	// Test creating a tag with message
	err = store.DoltTag("v1.0", "release version 1.0")
	if err != nil {
		t.Fatalf("dolt tag with message: %v", err)
	}

	// Test creating a tag without message
	err = store.DoltTag("v1.1", "")
	if err != nil {
		t.Fatalf("dolt tag without message: %v", err)
	}

	// Verify tags exist by listing them
	tags, err := store.DoltListTags()
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}

	// Check that both tags are present
	foundV10 := false
	foundV11 := false
	for _, tag := range tags {
		if tag == "v1.0" {
			foundV10 = true
		}
		if tag == "v1.1" {
			foundV11 = true
		}
	}

	if !foundV10 {
		t.Error("expected tag v1.0 to exist")
	}
	if !foundV11 {
		t.Error("expected tag v1.1 to exist")
	}

	// Test that we can use the tag as a ref for time travel
	entityAt, err := store.GetEntityAt("fn-dolt-tag-test", "v1.0")
	if err != nil {
		t.Fatalf("get entity at tag: %v", err)
	}
	if entityAt == nil {
		t.Error("expected to find entity at tag v1.0")
	}
	if entityAt != nil && entityAt.Name != "TestFuncTag" {
		t.Errorf("expected name TestFuncTag, got %s", entityAt.Name)
	}
}

// Tests for AS OF query methods (time travel)

func TestGetEntityAt_InvalidRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test with invalid ref (SQL injection attempt)
	_, err := store.GetEntityAt("test-id", "'; DROP TABLE --")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

func TestQueryEntitiesAt_InvalidRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test with invalid ref
	_, err := store.QueryEntitiesAt(EntityFilter{Status: "active"}, "foo bar")
	if err == nil {
		t.Error("expected error for invalid ref with space")
	}
}

func TestGetDependenciesAt_InvalidRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test with invalid ref
	_, err := store.GetDependenciesAt(DependencyFilter{FromID: "test"}, "\"injection")
	if err == nil {
		t.Error("expected error for invalid ref with quote")
	}
}

func TestQueryEntitiesAt_ValidRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create an entity
	entity := &Entity{
		ID:         "test-fn-at-1",
		Name:       "TestFuncAt",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  10,
		Language:   "go",
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Commit the entity
	hash, err := store.DoltCommit("add test entity for AS OF test")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if hash == "" {
		t.Skip("Dolt commit did not return hash, skipping AS OF test")
	}

	// Query with AS OF using the commit hash
	entities, err := store.QueryEntitiesAt(EntityFilter{Status: "active"}, hash)
	if err != nil {
		t.Fatalf("QueryEntitiesAt: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("expected 1 entity at commit %s, got %d", hash, len(entities))
	}
}

func TestGetEntityAt_ValidRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create an entity
	entity := &Entity{
		ID:         "test-fn-getat-1",
		Name:       "TestFuncGetAt",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  20,
		Language:   "go",
		Visibility: "pub",
	}
	if err := store.CreateEntity(entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	// Commit the entity
	hash, err := store.DoltCommit("add test entity for GetEntityAt test")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if hash == "" {
		t.Skip("Dolt commit did not return hash, skipping AS OF test")
	}

	// Get entity at commit
	got, err := store.GetEntityAt("test-fn-getat-1", hash)
	if err != nil {
		t.Fatalf("GetEntityAt: %v", err)
	}

	if got.Name != "TestFuncGetAt" {
		t.Errorf("expected name TestFuncGetAt, got %s", got.Name)
	}
}

func TestResolveRef(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test HEAD refs pass through unchanged
	t.Run("HEAD passthrough", func(t *testing.T) {
		ref, err := store.ResolveRef("HEAD")
		if err != nil {
			t.Fatalf("ResolveRef(HEAD): %v", err)
		}
		if ref != "HEAD" {
			t.Errorf("expected HEAD, got %s", ref)
		}
	})

	t.Run("HEAD~1 passthrough", func(t *testing.T) {
		ref, err := store.ResolveRef("HEAD~1")
		if err != nil {
			t.Fatalf("ResolveRef(HEAD~1): %v", err)
		}
		if ref != "HEAD~1" {
			t.Errorf("expected HEAD~1, got %s", ref)
		}
	})

	// Test full hashes pass through unchanged
	t.Run("full hash passthrough", func(t *testing.T) {
		fullHash := "abcdefghijklmnopqrstuvwxyz123456" // 32 chars
		ref, err := store.ResolveRef(fullHash)
		if err != nil {
			t.Fatalf("ResolveRef(full hash): %v", err)
		}
		if ref != fullHash {
			t.Errorf("expected %s, got %s", fullHash, ref)
		}
	})

	// Test branch-like refs (with /) pass through unchanged
	t.Run("branch passthrough", func(t *testing.T) {
		ref, err := store.ResolveRef("feature/test")
		if err != nil {
			t.Fatalf("ResolveRef(feature/test): %v", err)
		}
		if ref != "feature/test" {
			t.Errorf("expected feature/test, got %s", ref)
		}
	})

	// Test empty ref returns error
	t.Run("empty ref error", func(t *testing.T) {
		_, err := store.ResolveRef("")
		if err == nil {
			t.Error("expected error for empty ref")
		}
	})

	// Test short hash resolution (requires actual commits)
	t.Run("short hash resolution", func(t *testing.T) {
		// Create and commit an entity to get a real hash
		entity := &Entity{
			ID:         "test-resolve-1",
			Name:       "TestResolve",
			EntityType: "function",
			FilePath:   "test.go",
			LineStart:  1,
			Language:   "go",
			Visibility: "pub",
		}
		if err := store.CreateEntity(entity); err != nil {
			t.Fatalf("create entity: %v", err)
		}

		hash, err := store.DoltCommit("test commit for resolve")
		if err != nil {
			t.Fatalf("commit: %v", err)
		}
		if hash == "" || len(hash) < 10 {
			t.Skip("Dolt commit did not return usable hash")
		}

		// Try resolving with a 7-char prefix
		shortHash := hash[:7]
		resolved, err := store.ResolveRef(shortHash)
		if err != nil {
			t.Fatalf("ResolveRef(%s): %v", shortHash, err)
		}

		// Should resolve to the full hash
		if resolved != hash {
			t.Errorf("expected %s, got %s", hash, resolved)
		}
	})
}
