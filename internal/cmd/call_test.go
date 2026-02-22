package cmd

import (
	"testing"
)

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"find", "cx_find"},
		{"cx_find", "cx_find"},
		{"show", "cx_show"},
		{"cx_show", "cx_show"},
		{"context", "cx_context"},
		{"diff", "cx_diff"},
		{"impact", "cx_impact"},
		{"gaps", "cx_gaps"},
		{"safe", "cx_safe"},
		{"map", "cx_map"},
		{"nonexistent", "cx_nonexistent"},
	}

	for _, tt := range tests {
		got := normalizeToolName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCallCmdRequiresToolOrFlag(t *testing.T) {
	// runCall with no args and no flags should error
	err := runCall(callCmd, []string{})
	if err == nil {
		t.Error("runCall with no args should return error")
	}
}
