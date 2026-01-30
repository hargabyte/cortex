package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/store"
	"gopkg.in/yaml.v3"
)

// TestTagsExportImport tests the export and import functionality for tags
func TestTagsExportImport(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-tag-export-*")
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

	// Create test entities
	entities := []*store.Entity{
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
	t.Run("GetAllTagsWithEntity", func(t *testing.T) {
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
			}
		}
		if !foundLoginAuth {
			t.Error("did not find expected tag fn-login/auth")
		}
	})

	// Test export format
	t.Run("export format", func(t *testing.T) {
		tags, err := st.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}

		export := TagExport{
			Tags: make([]ExportedTag, len(tags)),
		}
		for i, tag := range tags {
			export.Tags[i] = ExportedTag{
				EntityID:   tag.EntityID,
				EntityName: tag.EntityName,
				Tag:        tag.Tag,
				Note:       tag.Note,
			}
		}

		data, err := yaml.Marshal(&export)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		// Verify it can be parsed back
		var parsed TagExport
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if len(parsed.Tags) != 4 {
			t.Errorf("expected 4 tags after round-trip, got %d", len(parsed.Tags))
		}
	})

	// Test import with skip existing
	t.Run("import skip existing", func(t *testing.T) {
		// Create YAML with some new and some existing tags
		importData := TagExport{
			Tags: []ExportedTag{
				{EntityID: "fn-login", Tag: "auth", Note: "Updated note"},     // exists
				{EntityID: "fn-login", Tag: "important", Note: "New tag"},     // new
				{EntityID: "tp-user", Tag: "core", Note: "Updated core note"}, // exists
				{EntityID: "tp-user", Tag: "model", Note: "Model tag"},        // new
			},
		}

		data, err := yaml.Marshal(&importData)
		if err != nil {
			t.Fatalf("marshal import data: %v", err)
		}

		// Write to temp file
		importFile := filepath.Join(tmpDir, "import-test.yaml")
		if err := os.WriteFile(importFile, data, 0644); err != nil {
			t.Fatalf("write import file: %v", err)
		}

		// Simulate import (skip existing - default)
		imported, skipped := 0, 0
		for _, tag := range importData.Tags {
			existingTags, err := st.GetTags(tag.EntityID)
			if err != nil {
				t.Fatalf("get tags: %v", err)
			}

			tagExists := false
			for _, et := range existingTags {
				if et.Tag == tag.Tag {
					tagExists = true
					break
				}
			}

			if tagExists {
				skipped++
			} else {
				if err := st.AddTagWithNote(tag.EntityID, tag.Tag, "import", tag.Note); err != nil {
					t.Fatalf("add tag: %v", err)
				}
				imported++
			}
		}

		if imported != 2 {
			t.Errorf("expected 2 imported, got %d", imported)
		}
		if skipped != 2 {
			t.Errorf("expected 2 skipped, got %d", skipped)
		}

		// Verify total tags now
		allTags, err := st.GetAllTagsWithEntity()
		if err != nil {
			t.Fatalf("get all tags: %v", err)
		}
		if len(allTags) != 6 { // 4 original + 2 new
			t.Errorf("expected 6 total tags, got %d", len(allTags))
		}
	})

	// Test default export location
	t.Run("default export location", func(t *testing.T) {
		expectedPath := filepath.Join(cxDir, "tags.yaml")

		// Just verify the path construction
		if expectedPath != filepath.Join(tmpDir, ".cx", "tags.yaml") {
			t.Errorf("unexpected default path: %s", expectedPath)
		}
	})
}

// TestTagExportYAMLFormat verifies the YAML output format
func TestTagExportYAMLFormat(t *testing.T) {
	export := TagExport{
		Tags: []ExportedTag{
			{EntityID: "fn-main", EntityName: "main", Tag: "important", Note: "Entry point"},
			{EntityID: "fn-init", EntityName: "init", Tag: "startup"},
		},
	}

	data, err := yaml.Marshal(&export)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify it contains expected structure
	if !bytes.Contains(data, []byte("tags:")) {
		t.Error("expected 'tags:' in output")
	}
	if !bytes.Contains(data, []byte("entity_id:")) {
		t.Error("expected 'entity_id:' in output")
	}
	if !bytes.Contains(data, []byte("entity_name:")) {
		t.Error("expected 'entity_name:' in output")
	}
	if !bytes.Contains(data, []byte("note:")) {
		t.Error("expected 'note:' in output")
	}

	// Verify round-trip
	var parsed TagExport
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(parsed.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(parsed.Tags))
	}

	if parsed.Tags[0].EntityID != "fn-main" {
		t.Errorf("expected entity_id 'fn-main', got %q", parsed.Tags[0].EntityID)
	}
	if parsed.Tags[0].Note != "Entry point" {
		t.Errorf("expected note 'Entry point', got %q", parsed.Tags[0].Note)
	}

	// Second tag should have empty note (omitempty)
	if parsed.Tags[1].Note != "" {
		t.Errorf("expected empty note for second tag, got %q", parsed.Tags[1].Note)
	}
}
