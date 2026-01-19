package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback [ref]",
	Short: "Reset the Dolt database to a previous state",
	Long: `Reset the Cortex Dolt database to a previous commit or ref.

This command uses Dolt's reset functionality to restore the database
to an earlier point in time. By default, it resets to the previous
commit (HEAD~1).

Reset modes:
  soft    Move HEAD only, keep working set changes (default)
  hard    Reset HEAD and discard all working set changes (--hard)

WARNING: --hard will permanently discard uncommitted changes.

Arguments:
  ref     The commit, branch, or ref to reset to (default: HEAD~1)
          Supports: commit hashes, branch names, HEAD~N, tags

Examples:
  cx rollback                    # Reset to HEAD~1 (soft)
  cx rollback HEAD~3             # Reset to 3 commits ago
  cx rollback main               # Reset to main branch
  cx rollback abc1234            # Reset to specific commit
  cx rollback --hard             # Hard reset to HEAD~1
  cx rollback HEAD~5 --hard      # Hard reset to 5 commits ago
  cx rollback --hard --yes       # Skip confirmation prompt`,
	RunE: runRollback,
}

var (
	rollbackHard bool
	rollbackYes  bool
)

func init() {
	rootCmd.AddCommand(rollbackCmd)

	rollbackCmd.Flags().BoolVar(&rollbackHard, "hard", false, "Hard reset - discard all uncommitted changes")
	rollbackCmd.Flags().BoolVarP(&rollbackYes, "yes", "y", false, "Skip confirmation prompt for destructive operations")
}

func runRollback(cmd *cobra.Command, args []string) error {
	// Default to HEAD~1
	ref := "HEAD~1"
	if len(args) > 0 {
		ref = args[0]
	}

	// Validate ref
	if !store.IsValidRef(ref) {
		return fmt.Errorf("invalid ref: %s", ref)
	}

	// Find config directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	// Open store
	st, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	db := st.DB()

	// For hard reset, require confirmation
	if rollbackHard && !rollbackYes {
		// Show what will be lost
		fmt.Fprintln(cmd.OutOrStdout(), "WARNING: Hard reset will permanently discard all uncommitted changes.")
		fmt.Fprintf(cmd.OutOrStdout(), "Resetting to: %s\n", ref)

		// Check for uncommitted changes
		hasChanges, err := hasUncommittedChanges(st)
		if err == nil && hasChanges {
			fmt.Fprintln(cmd.OutOrStdout(), "You have uncommitted changes that will be lost.")
		}

		fmt.Fprint(cmd.OutOrStdout(), "\nAre you sure? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Rollback cancelled.")
			return nil
		}
	}

	// Get the commit info before reset for display
	var beforeHash string
	err = db.QueryRow("SELECT COMMIT_HASH FROM dolt_log LIMIT 1").Scan(&beforeHash)
	if err != nil {
		beforeHash = "unknown"
	}

	// Perform the reset
	var resetErr error
	if rollbackHard {
		_, resetErr = db.Exec("CALL dolt_reset('--hard', ?)", ref)
	} else {
		_, resetErr = db.Exec("CALL dolt_reset(?)", ref)
	}

	if resetErr != nil {
		if strings.Contains(resetErr.Error(), "cannot resolve") ||
			strings.Contains(resetErr.Error(), "not found") ||
			strings.Contains(resetErr.Error(), "unknown ref") {
			return fmt.Errorf("ref '%s' not found in history", ref)
		}
		return fmt.Errorf("rollback failed: %w", resetErr)
	}

	// Get the commit info after reset
	var afterHash string
	err = db.QueryRow("SELECT COMMIT_HASH FROM dolt_log LIMIT 1").Scan(&afterHash)
	if err != nil {
		afterHash = "unknown"
	}

	// Output result
	mode := "soft"
	if rollbackHard {
		mode = "hard"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Reset (%s) to %s\n", mode, ref)
	if len(beforeHash) > 7 {
		beforeHash = beforeHash[:7]
	}
	if len(afterHash) > 7 {
		afterHash = afterHash[:7]
	}
	fmt.Fprintf(cmd.OutOrStdout(), "HEAD: %s -> %s\n", beforeHash, afterHash)

	return nil
}

// hasUncommittedChanges checks if there are uncommitted changes in the working set
func hasUncommittedChanges(st *store.Store) (bool, error) {
	db := st.DB()

	// Query dolt_status for any changes
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM dolt_status").Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
