package cmd

import (
	"github.com/spf13/cobra"
)

// checkCmd represents the "cx check" quality gate command.
// It dispatches to safe, guard, or test based on flags and args.
var checkCmd = &cobra.Command{
	Use:   "check [file-or-path]",
	Short: "Quality gate: safety checks, pre-commit guard, and test selection",
	Long: `Unified quality gate combining safety, guard, and test commands.

Modes:
  cx check <file>            Safety check on a file (same as: cx safe <file>)
  cx check                   Pre-commit guard on staged files (same as: cx guard)
  cx check --guard           Explicit guard mode
  cx check --guard --all     Guard all modified files
  cx check --test            Smart test selection (same as: cx test --diff)
  cx check --test --gaps     Coverage gap analysis
  cx check --coverage        Coverage summary

The default (no args, no flags) runs the pre-commit guard on staged files.

Examples:
  cx check src/auth/login.go        # Safety check before modifying file
  cx check                          # Pre-commit guard (staged files)
  cx check --guard --all            # Guard all modified files
  cx check --test                   # Which tests to run for changes?
  cx check --test --gaps            # Coverage gaps analysis
  cx check --test --run             # Run affected tests`,
	RunE: runCheck,
}

// check-specific flags
var (
	checkGuard    bool
	checkTest     bool
	checkCoverage bool
	checkDepth    int // shared depth flag (sets testDepth or safeDepth based on mode)
)

func init() {
	rootCmd.AddCommand(checkCmd)

	// Mode flags
	checkCmd.Flags().BoolVar(&checkGuard, "guard", false, "Run pre-commit guard checks")
	checkCmd.Flags().BoolVar(&checkTest, "test", false, "Run smart test selection")
	checkCmd.Flags().BoolVar(&checkCoverage, "coverage", false, "Show coverage summary")

	// Pass-through guard flags
	checkCmd.Flags().BoolVar(&guardAll, "all", false, "Check all modified files (with --guard)")
	checkCmd.Flags().BoolVar(&guardFailOnWarnings, "fail-on-warnings", false, "Exit with error on warnings (with --guard)")

	// Pass-through test flags
	checkCmd.Flags().BoolVar(&testShowGaps, "gaps", false, "Show coverage gaps (with --test)")
	checkCmd.Flags().BoolVar(&testKeystonesOnly, "keystones-only", false, "Only keystone gaps (with --test --gaps)")
	checkCmd.Flags().BoolVar(&testRun, "run", false, "Actually run selected tests (with --test)")
	checkCmd.Flags().BoolVar(&testOutputCommand, "output-command", false, "Output go test command (with --test)")
	checkCmd.Flags().IntVar(&testThreshold, "threshold", 75, "Coverage gap threshold % (with --test --gaps)")
	checkCmd.Flags().BoolVar(&testDiff, "diff", false, "Use git diff to find changed files (with --test)")
	checkCmd.Flags().StringVar(&testCommit, "commit", "", "Use specific commit for test selection (with --test)")

	// Pass-through safe flags
	checkCmd.Flags().BoolVar(&safeQuick, "quick", false, "Quick mode: impact only (with file target)")
	checkCmd.Flags().BoolVar(&safeDrift, "drift", false, "Check code staleness (with file target)")

	// Shared depth flag (applies to both test and safe modes)
	checkCmd.Flags().IntVar(&checkDepth, "depth", 0, "Analysis depth: indirect test hops (--test) or impact depth (file target)")
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Propagate shared depth flag to the appropriate target
	if checkDepth > 0 {
		testDepth = checkDepth
		safeDepth = checkDepth
	}

	// Explicit mode flags take priority
	if checkTest {
		// Default to --diff if no other test source specified
		if !testDiff && testCommit == "" && testFile == "" {
			testDiff = true
		}
		return runTest(testCmd, args)
	}
	if checkCoverage {
		testShowCoverage = true
		return runTest(testCmd, args)
	}
	if checkGuard {
		return runGuard(guardCmd, args)
	}

	// Auto-detect: args given → safe; no args → guard
	if len(args) > 0 {
		return runSafe(safeCmd, args)
	}

	// Default: pre-commit guard on staged files
	return runGuard(guardCmd, []string{})
}
