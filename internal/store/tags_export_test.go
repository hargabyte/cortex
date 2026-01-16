package store

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetAllTagsWithEntity tests the GetAllTagsWithEntity function
func TestGetAllTagsWithEntity(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-tags-export-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Create test entities
	entities := []*Entity{
		{ID: "fn-login", Name: "LoginUser", EntityType: "function", FilePath: "auth/login.go", LineStart: 10, Visibility: "pub", Language: "go", Status: "active"},
		{ID: "fn-logout", Name: "LogoutUser", EntityType: "function", FilePath: "auth/logout.go", LineStart: 5, Visibility: "pub", Language: "go", Status: "active"},
		{ID: "tp-user", Name: "User", EntityType: "type", Kind: "struct", FilePath: "models/user.go", LineStart: 1, Visibility: "pub", Language: "go", Status: "active"},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}

	// Add some tags
	if err := st.AddTagWithNote("fn-login", "auth", "cli", "Authentication related"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := st.AddTagWithNote("fn-login", "security", "cli", "Security sensitive"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := st.AddTagWithNote("fn-logout", "auth", "cli", ""); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := st.AddTagWithNote("tp-user", "core", "cli", "Core model"); err != nil {
		t.Fatalf("add tag: %v", err)
	}

	// Test GetAllTagsWithEntity
	t.Run("returns all tags with entity names", func(t *testing.T) {
		tags, err := st.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}

		if len(tags) != 4 {
			t.Errorf("expected 4 tags, got %d", len(tags))
		}

		// Check that entity names are included
		foundLoginAuth := false
		for _, tag := range tags {
			if tag.EntityID == "fn-login" && tag.Tag == "auth" {
				foundLoginAuth = true
				if tag.EntityName != "LoginUser" {
					t.Errorf("expected entity name 'LoginUser', got %q", tag.EntityName)
				}
				if tag.Note != "Authentication related" {
					t.Errorf("expected note 'Authentication related', got %q", tag.Note)
				}
				if tag.CreatedBy != "cli" {
					t.Errorf("expected created_by 'cli', got %q", tag.CreatedBy)
				}
			}
		}
		if !foundLoginAuth {
			t.Error("did not find expected tag fn-login/auth")
		}
	})

	t.Run("orders by entity_id and tag", func(t *testing.T) {
		tags, err := st.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}

		// Expected order: fn-login/auth, fn-login/security, fn-logout/auth, tp-user/core
		expected := []struct {
			entityID string
			tag      string
		}{
			{"fn-login", "auth"},
			{"fn-login", "security"},
			{"fn-logout", "auth"},
			{"tp-user", "core"},
		}

		for i, exp := range expected {
			if tags[i].EntityID != exp.entityID || tags[i].Tag != exp.tag {
				t.Errorf("at index %d: expected %s/%s, got %s/%s",
					i, exp.entityID, exp.tag, tags[i].EntityID, tags[i].Tag)
			}
		}
	})

	t.Run("handles orphaned tags gracefully", func(t *testing.T) {
		// Add a tag to a non-existent entity (simulating orphaned data)
		if _, err := st.DB().Exec(`INSERT INTO entity_tags (entity_id, tag, created_at, created_by, note) VALUES (?, ?, ?, ?, ?)`,
			"fn-deleted", "orphan", "2024-01-01T00:00:00Z", "cli", ""); err != nil {
			t.Fatalf("insert orphan tag: %v", err)
		}

		tags, err := st.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}

		// Should now have 5 tags
		if len(tags) != 5 {
			t.Errorf("expected 5 tags, got %d", len(tags))
		}

		// Find the orphaned tag - entity_name should be empty string
		foundOrphan := false
		for _, tag := range tags {
			if tag.EntityID == "fn-deleted" {
				foundOrphan = true
				if tag.EntityName != "" {
					t.Errorf("expected empty entity name for orphan, got %q", tag.EntityName)
				}
			}
		}
		if !foundOrphan {
			t.Error("did not find orphaned tag")
		}
	})

	t.Run("returns empty slice for no tags", func(t *testing.T) {
		// Create a new empty store
		emptyDir := filepath.Join(tmpDir, "empty")
		emptySt, err := Open(emptyDir)
		if err != nil {
			t.Fatalf("open empty store: %v", err)
		}
		defer emptySt.Close()

		tags, err := emptySt.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}

		if tags != nil && len(tags) != 0 {
			t.Errorf("expected nil or empty slice, got %d tags", len(tags))
		}
	})
}
