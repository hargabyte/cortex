package cmd

import (
	"github.com/spf13/cobra"
)

// checkCmd is deprecated - use safeCmd instead
// This file maintains backwards compatibility by delegating to the safe command
var checkCmd = &cobra.Command{
	Use:   "check [file-or-entity]",
	Short: "[DEPRECATED] Use 'cx safe' instead - Pre-flight safety check",
	Long: `DEPRECATED: This command has been renamed to 'cx safe'.

Please use 'cx safe' instead. All flags and functionality remain the same.

Examples:
  cx safe src/auth/jwt.go              # Full safety assessment
  cx safe src/auth/jwt.go --quick      # Just blast radius
  cx safe --coverage                   # Coverage gaps report
  cx safe --drift                      # Staleness check`,
	Args:   cobra.MaximumNArgs(1),
	Hidden: true, // Hide from help, but keep functional
	RunE:   runCheckDeprecated,
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Copy all flags from safeCmd so they work with the deprecated check command
	// General flags
	checkCmd.Flags().IntVar(&safeDepth, "depth", 3, "Transitive impact depth")
	checkCmd.Flags().BoolVar(&safeCreateTask, "create-task", false, "Create a beads task for safety findings")

	// Mode flags
	checkCmd.Flags().BoolVar(&safeQuick, "quick", false, "Quick mode: just blast radius/impact analysis")
	checkCmd.Flags().BoolVar(&safeCoverage, "coverage", false, "Coverage mode: show coverage gaps")
	checkCmd.Flags().BoolVar(&safeDrift, "drift", false, "Drift mode: check staleness")
	checkCmd.Flags().BoolVar(&safeChanges, "changes", false, "Changes mode: what changed since scan")

	// Coverage-specific flags
	checkCmd.Flags().BoolVar(&safeKeystonesOnly, "keystones-only", false, "Only show keystones with gaps (--coverage mode)")
	checkCmd.Flags().IntVar(&safeThreshold, "threshold", 75, "Coverage threshold percentage (--coverage mode)")

	// Drift-specific flags
	checkCmd.Flags().BoolVar(&safeStrict, "strict", false, "Exit non-zero on any drift (--drift mode, for CI)")
	checkCmd.Flags().BoolVar(&safeFix, "fix", false, "Update hashes for drifted entities (--drift mode)")
	checkCmd.Flags().BoolVar(&safeDryRun, "dry-run", false, "Show what --fix would do without making changes")

	// Diff-specific flags
	checkCmd.Flags().StringVar(&safeFile, "file", "", "Show changes for specific file/directory only (--changes mode)")
	checkCmd.Flags().BoolVar(&safeDetailed, "detailed", false, "Show hash changes for modified entities (--changes mode)")
	checkCmd.Flags().BoolVar(&safeSemantic, "semantic", false, "Show semantic analysis (--changes mode)")

	// Impact-specific flags
	checkCmd.Flags().Float64Var(&safeImpactThreshold, "impact-threshold", 0, "Min importance threshold for impact analysis")
}

func runCheckDeprecated(cmd *cobra.Command, args []string) error {
	// Emit deprecation warning
	emitDeprecationWarning(DeprecationInfo{
		OldCommand: "cx check",
		NewCommand: "cx safe",
	})

	// Delegate to the safe command
	return runSafe(cmd, args)
}
