package mcp

import (
	"sort"
	"testing"
)

func TestGetToolSchemas(t *testing.T) {
	// Verify the schema registry has all 8 tools
	expectedTools := []string{
		"cx_diff", "cx_impact", "cx_context", "cx_show",
		"cx_find", "cx_gaps", "cx_safe", "cx_map",
	}

	for _, name := range expectedTools {
		schema, ok := toolSchemaRegistry[name]
		if !ok {
			t.Errorf("toolSchemaRegistry missing tool: %s", name)
			continue
		}
		if schema.Name != name {
			t.Errorf("schema name mismatch: got %q, want %q", schema.Name, name)
		}
		if schema.Description == "" {
			t.Errorf("tool %s has empty description", name)
		}
	}

	if len(toolSchemaRegistry) != len(expectedTools) {
		t.Errorf("toolSchemaRegistry has %d tools, want %d", len(toolSchemaRegistry), len(expectedTools))
	}
}

func TestToolSchemaParameters(t *testing.T) {
	// Verify required parameters are marked correctly
	tests := []struct {
		tool          string
		requiredParam string
	}{
		{"cx_impact", "target"},
		{"cx_show", "name"},
		{"cx_find", "pattern"},
		{"cx_safe", "target"},
	}

	for _, tt := range tests {
		schema, ok := toolSchemaRegistry[tt.tool]
		if !ok {
			t.Fatalf("missing tool: %s", tt.tool)
		}

		found := false
		for _, p := range schema.Parameters {
			if p.Name == tt.requiredParam {
				found = true
				if !p.Required {
					t.Errorf("tool %s param %s should be required", tt.tool, tt.requiredParam)
				}
			}
		}
		if !found {
			t.Errorf("tool %s missing parameter %s", tt.tool, tt.requiredParam)
		}
	}
}

func TestToolSchemaNoRequiredParams(t *testing.T) {
	// These tools have no required params
	noRequired := []string{"cx_diff", "cx_gaps", "cx_context", "cx_map"}

	for _, name := range noRequired {
		schema := toolSchemaRegistry[name]
		for _, p := range schema.Parameters {
			if p.Required {
				t.Errorf("tool %s param %s is marked required but should not be", name, p.Name)
			}
		}
	}
}

func TestAllToolsMatchesRegistry(t *testing.T) {
	// AllTools should match the schema registry
	registryNames := make([]string, 0, len(toolSchemaRegistry))
	for name := range toolSchemaRegistry {
		registryNames = append(registryNames, name)
	}
	sort.Strings(registryNames)

	allToolsCopy := make([]string, len(AllTools))
	copy(allToolsCopy, AllTools)
	sort.Strings(allToolsCopy)

	if len(registryNames) != len(allToolsCopy) {
		t.Errorf("schema registry has %d tools, AllTools has %d", len(registryNames), len(allToolsCopy))
	}

	for i, name := range registryNames {
		if i >= len(allToolsCopy) {
			t.Errorf("AllTools missing: %s", name)
			continue
		}
		if name != allToolsCopy[i] {
			t.Errorf("mismatch at index %d: registry=%s, AllTools=%s", i, name, allToolsCopy[i])
		}
	}
}
