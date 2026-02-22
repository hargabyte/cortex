package cmd

import (
	"github.com/spf13/cobra"
)

// adminCmd is the parent for administrative commands.
// It provides real subcommands that delegate to the original (hidden) command RunE functions.
// The original commands remain on rootCmd (hidden) for backwards compatibility.
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands (db, tags, branches, daemon, etc.)",
	Long: `Administrative and power-user commands for cx database management.

These commands are used less frequently during normal agent workflows.
For everyday use, prefer: cx, cx find, cx check, cx scan.

Subcommands:
  db          Database management (info, compact, doctor, export)
  tag         Entity tag management (add, remove, list, find)
  sql         Execute SQL directly
  doctor      Health check
  blame       Entity commit history
  daemon      Daemon control (start, stop, status)
  diff        Show entity changes between commits
  history     Show Dolt commit history
  stale       Find unchanged entities
  catchup     Show changes since a ref
  branch      List/manage branches
  reset       Reset database
  rollback    Rollback to previous state
  guide       Codebase documentation
  report      Generate reports
  serve       Start MCP server
  impact      Analyze blast radius
  coverage    Import/analyze coverage data

Examples:
  cx admin db info              # Database statistics
  cx admin tag list             # List all tags
  cx admin tag add Foo important  # Tag an entity
  cx admin doctor               # Health check
  cx admin sql "SELECT ..."     # Direct SQL query
  cx admin blame Execute        # Entity commit history`,
}

// ── db subgroup ──────────────────────────────────────────────

var adminDbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Database management: info, compact, doctor, export.`,
}

var adminDbInfoCmd = &cobra.Command{
	Use: "info", Short: "Show database statistics",
	RunE: runDbInfo,
}

var adminDbCompactCmd = &cobra.Command{
	Use: "compact", Short: "Compact the database",
	RunE: runDbCompact,
}

var adminDbDoctorCmd = &cobra.Command{
	Use: "doctor", Short: "Check database health",
	RunE: runDoctor,
}

// ── tag subgroup ─────────────────────────────────────────────

var adminTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage entity tags",
	Long:  `Tag management: add, remove, list, find.`,
	RunE:  runTagShortcut,
}

var adminTagAddCmd = &cobra.Command{
	Use: "add <entity> <tags...>", Short: "Add tags to an entity",
	RunE: runTagAdd,
}

var adminTagRemoveCmd = &cobra.Command{
	Use: "remove <entity> <tag>", Short: "Remove a tag from an entity",
	RunE: runTagRemove,
}

var adminTagListCmd = &cobra.Command{
	Use: "list [entity]", Short: "List tags for an entity or all tags",
	RunE: runTagsList,
}

var adminTagFindCmd = &cobra.Command{
	Use: "find <tag>", Short: "Find entities by tag",
	RunE: runTagFind,
}

// ── direct wrappers ──────────────────────────────────────────

var adminDoctorCmd = &cobra.Command{
	Use: "doctor", Short: "Check database health",
	RunE: runDoctor,
}

var adminSqlCmd = &cobra.Command{
	Use: "sql <query>", Short: "Execute SQL directly",
	Args: cobra.ExactArgs(1),
	RunE: runSQL,
}

var adminBlameCmd = &cobra.Command{
	Use: "blame <entity>", Short: "Show entity commit history",
	Args: cobra.ExactArgs(1),
	RunE: runBlame,
}

var adminDiffCmd = &cobra.Command{
	Use: "diff", Short: "Show entity changes between commits",
	RunE: runDiff,
}

var adminHistoryCmd = &cobra.Command{
	Use: "history", Short: "Show Dolt commit history",
	RunE: runHistory,
}

var adminStaleCmd = &cobra.Command{
	Use: "stale", Short: "Find unchanged entities",
	RunE: runStale,
}

var adminCatchupCmd = &cobra.Command{
	Use: "catchup", Short: "Show changes since a ref",
	RunE: runCatchup,
}

var adminBranchCmd = &cobra.Command{
	Use: "branch", Short: "List/manage branches",
	RunE: runBranch,
}

var adminResetCmd = &cobra.Command{
	Use: "reset", Short: "Reset database",
	RunE: runReset,
}

var adminRollbackCmd = &cobra.Command{
	Use: "rollback", Short: "Rollback to previous state",
	RunE: runRollback,
}

var adminImpactCmd = &cobra.Command{
	Use: "impact [file-or-entity]", Short: "Analyze blast radius",
	RunE: runImpact,
}

func init() {
	rootCmd.AddCommand(adminCmd)

	// db subgroup
	adminCmd.AddCommand(adminDbCmd)
	adminDbCmd.AddCommand(adminDbInfoCmd)
	adminDbCmd.AddCommand(adminDbCompactCmd)
	adminDbCmd.AddCommand(adminDbDoctorCmd)

	// tag subgroup
	adminCmd.AddCommand(adminTagCmd)
	adminTagCmd.AddCommand(adminTagAddCmd)
	adminTagCmd.AddCommand(adminTagRemoveCmd)
	adminTagCmd.AddCommand(adminTagListCmd)
	adminTagCmd.AddCommand(adminTagFindCmd)

	// direct wrappers
	adminCmd.AddCommand(adminDoctorCmd)
	adminCmd.AddCommand(adminSqlCmd)
	adminCmd.AddCommand(adminBlameCmd)
	adminCmd.AddCommand(adminDiffCmd)
	adminCmd.AddCommand(adminHistoryCmd)
	adminCmd.AddCommand(adminStaleCmd)
	adminCmd.AddCommand(adminCatchupCmd)
	adminCmd.AddCommand(adminBranchCmd)
	adminCmd.AddCommand(adminResetCmd)
	adminCmd.AddCommand(adminRollbackCmd)
	adminCmd.AddCommand(adminImpactCmd)
}
