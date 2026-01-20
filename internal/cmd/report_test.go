package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReportCmd_Structure(t *testing.T) {
	// Verify report command exists and has expected subcommands
	if reportCmd == nil {
		t.Fatal("reportCmd is nil")
	}

	expectedSubcmds := []string{"overview", "feature", "changes", "health"}
	subcmds := reportCmd.Commands()

	if len(subcmds) != len(expectedSubcmds) {
		t.Errorf("expected %d subcommands, got %d", len(expectedSubcmds), len(subcmds))
	}

	subcmdNames := make(map[string]bool)
	for _, cmd := range subcmds {
		subcmdNames[cmd.Name()] = true
	}

	for _, expected := range expectedSubcmds {
		if !subcmdNames[expected] {
			t.Errorf("missing expected subcommand: %s", expected)
		}
	}
}

func TestReportCmd_Flags(t *testing.T) {
	// Check that --data flag exists
	dataFlag := reportCmd.PersistentFlags().Lookup("data")
	if dataFlag == nil {
		t.Error("missing --data flag")
	}

	// Check that -o/--output flag exists
	outputFlag := reportCmd.PersistentFlags().Lookup("output")
	if outputFlag == nil {
		t.Error("missing --output flag")
	}
	if outputFlag.Shorthand != "o" {
		t.Errorf("expected --output shorthand to be 'o', got '%s'", outputFlag.Shorthand)
	}
}

func TestReportChangesCmd_Flags(t *testing.T) {
	// Check that --since flag exists and is required
	sinceFlag := reportChangesCmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Error("missing --since flag on changes command")
	}

	// Check that --until flag exists
	untilFlag := reportChangesCmd.Flags().Lookup("until")
	if untilFlag == nil {
		t.Error("missing --until flag on changes command")
	}
	if untilFlag.DefValue != "HEAD" {
		t.Errorf("expected --until default to be 'HEAD', got '%s'", untilFlag.DefValue)
	}
}

func TestReportFeatureCmd_Args(t *testing.T) {
	// Feature command should require exactly 1 argument
	cmd := &cobra.Command{}
	*cmd = *reportFeatureCmd

	// Test with no args - should fail
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("expected error with no args, got nil")
	}

	// Test with 1 arg - should succeed
	err = cmd.Args(cmd, []string{"authentication"})
	if err != nil {
		t.Errorf("expected no error with 1 arg, got %v", err)
	}

	// Test with 2 args - should fail
	err = cmd.Args(cmd, []string{"auth", "extra"})
	if err == nil {
		t.Error("expected error with 2 args, got nil")
	}
}

func TestReportCmd_RequiresDataFlag(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*cobra.Command, []string) error
		args []string
	}{
		{"overview", runReportOverview, []string{}},
		{"feature", runReportFeature, []string{"auth"}},
		{"changes", func(cmd *cobra.Command, args []string) error {
			changesSince = "HEAD~10"
			return runReportChanges(cmd, args)
		}, []string{}},
		{"health", runReportHealth, []string{}},
	}

	// Ensure --data flag is false
	reportData = false

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(nil, tt.args)
			if err == nil {
				t.Error("expected error when --data flag not set")
			}
			if !strings.Contains(err.Error(), "--data flag is required") {
				t.Errorf("expected error about --data flag, got: %v", err)
			}
		})
	}
}

func TestReportCmd_HelpOutput(t *testing.T) {
	// Test the Long description directly instead of executing
	help := reportCmd.Long

	// Verify key content is present
	expectedPhrases := []string{
		"Generate structured",
		"overview",
		"feature",
		"changes",
		"health",
		"--data",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(help, phrase) {
			t.Errorf("Long description missing expected phrase: %s", phrase)
		}
	}

	// Also check Short description
	if !strings.Contains(reportCmd.Short, "report") {
		t.Error("Short description should mention 'report'")
	}
}
