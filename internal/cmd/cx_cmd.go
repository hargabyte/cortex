package cmd

import (
	"github.com/spf13/cobra"
)

// cx_cmd.go implements the bare "cx" auto-detect dispatcher.
// It routes to the appropriate existing command based on flags and argument type.

// cx (bare) flags — registered on rootCmd in registerCxFlags()
var (
	cxMap   bool
	cxTrace string // --trace <entity>
	cxBlame string // --blame <entity>
)

// registerCxFlags adds the bare-cx dispatcher flags to rootCmd.
// Called from root.go init() — safe because it only touches rootCmd flags.
func registerCxFlags() {
	rootCmd.Flags().BoolVar(&cxMap, "map", false, "Show project skeleton (same as: cx map)")
	rootCmd.Flags().StringVar(&cxTrace, "trace", "", "Trace call chains for entity (same as: cx trace)")
	rootCmd.Flags().StringVar(&cxBlame, "blame", "", "Show entity commit history (same as: cx blame)")

	// Context flags duplicated on rootCmd for "cx --smart", "cx --diff", etc.
	rootCmd.Flags().StringVar(&contextSmart, "smart", "", "Smart context assembly for a task description")
	rootCmd.Flags().BoolVar(&contextDiff, "diff", false, "Context for uncommitted changes")
	rootCmd.Flags().BoolVar(&contextStaged, "staged", false, "Context for staged changes only")
	rootCmd.Flags().StringVar(&contextFor, "for", "", "Context for specific file/entity/directory")
	rootCmd.Flags().BoolVar(&contextFull, "full", false, "Extended session recovery with keystones and map")

	// Context pass-through flags (most commonly used with --smart and --diff)
	rootCmd.Flags().IntVar(&contextMaxTokens, "budget", 4000, "Token budget for context assembly")
	rootCmd.Flags().IntVar(&contextDepth, "depth", 2, "Max hops from entry points (for --smart)")
	rootCmd.Flags().IntVar(&contextHops, "hops", 1, "Graph expansion depth")
	rootCmd.Flags().BoolVar(&contextWithCoverage, "with-coverage", false, "Include test coverage data")
	rootCmd.Flags().StringVar(&contextCommitRange, "commit-range", "", "Context for commit range (e.g., HEAD~3)")
}

// hideOldCommands marks legacy commands as hidden from help output.
// Called via cobra.OnInitialize (runs after all init()s, before command execution).
func hideOldCommands() {
	hiddenCmds := []*cobra.Command{
		blameCmd, branchCmd, catchupCmd, contextCmd, coverageCmd,
		daemonControlCmd, dbCmd, deadCmd, diffCmd, doctorCmd,
		guardCmd, guideCmd, helpAgentsCmd, historyCmd, impactCmd,
		linkCmd, mapCmd, reportCmd, renderCmd, resetCmd, rollbackCmd,
		safeCmd, serveCmd, showCmd, sqlCmd, staleCmd, tagCmd,
		testCmd, traceCmd,
	}
	for _, cmd := range hiddenCmds {
		cmd.Hidden = true
	}
}

// runCx is the root command's RunE. It auto-detects what to do based on flags and args.
func runCx(cmd *cobra.Command, args []string) error {
	// Flag-based dispatch (highest priority)
	if cxMap {
		return runMap(mapCmd, args)
	}
	if cxTrace != "" {
		// Default to callers mode for bare cx --trace (most common use)
		traceCallers = true
		return runTrace(traceCmd, []string{cxTrace})
	}
	if cxBlame != "" {
		return runBlame(blameCmd, []string{cxBlame})
	}

	// Context-mode flags (--smart, --diff, --staged) — delegate to context
	if contextSmart != "" || contextDiff || contextStaged || contextCommitRange != "" || contextFor != "" || contextFull {
		return runContext(contextCmd, args)
	}

	// No args → session recovery
	if len(args) == 0 {
		return runContext(contextCmd, []string{})
	}

	// Arg-based auto-detect
	target := args[0]

	// File path → safety check
	if isFilePath(target) {
		return runSafe(safeCmd, args)
	}

	// Entity ID or name → show details
	return runShow(showCmd, args)
}
