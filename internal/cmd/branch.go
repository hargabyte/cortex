package cmd

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command
var branchCmd = &cobra.Command{
	Use:   "branch [name]",
	Short: "List, create, or delete Dolt branches",
	Long: `Manage Dolt branches in the Cortex database.

Without arguments, lists all branches and shows the current branch.
With a name argument, creates a new branch.

Branch operations:
  cx branch              List all branches
  cx branch <name>       Create a new branch
  cx branch -d <name>    Delete a branch
  cx branch -c <name>    Switch to a branch (checkout)
  cx branch --from <ref> Create branch from specific ref

The current branch is marked with an asterisk (*).

Examples:
  cx branch                      # List all branches
  cx branch feature/new-parser   # Create new branch
  cx branch -c main              # Switch to main branch
  cx branch -d old-feature       # Delete branch
  cx branch hotfix --from HEAD~5 # Create branch from 5 commits ago`,
	RunE: runBranch,
}

var (
	branchDelete   bool
	branchCheckout bool
	branchFrom     string
)

func init() {
	rootCmd.AddCommand(branchCmd)

	branchCmd.Flags().BoolVarP(&branchDelete, "delete", "d", false, "Delete the specified branch")
	branchCmd.Flags().BoolVarP(&branchCheckout, "checkout", "c", false, "Switch to the specified branch")
	branchCmd.Flags().StringVar(&branchFrom, "from", "", "Create branch from specific ref (commit, branch, tag)")
}

// BranchInfo represents a branch in the output
type BranchInfo struct {
	Name    string `yaml:"name" json:"name"`
	Hash    string `yaml:"hash" json:"hash"`
	Current bool   `yaml:"current" json:"current"`
}

// BranchOutput is the full output structure
type BranchOutput struct {
	Branches []BranchInfo `yaml:"branches" json:"branches"`
	Current  string       `yaml:"current" json:"current"`
}

func runBranch(cmd *cobra.Command, args []string) error {
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

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Determine operation
	if len(args) == 0 {
		// List branches
		return listBranches(cmd, st, format)
	}

	branchName := args[0]

	if branchDelete {
		return deleteBranch(cmd, st, branchName)
	}

	if branchCheckout {
		return checkoutBranch(cmd, st, branchName)
	}

	// Create new branch
	return createBranch(cmd, st, branchName, branchFrom)
}

func listBranches(cmd *cobra.Command, st *store.Store, format output.Format) error {
	db := st.DB()

	// Get current branch first
	var currentBranch string
	err := db.QueryRow("SELECT active_branch()").Scan(&currentBranch)
	if err != nil {
		currentBranch = "main" // default
	}

	// Query all branches
	rows, err := db.Query("SELECT name, hash FROM dolt_branches ORDER BY name")
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}
	defer rows.Close()

	branchOutput := BranchOutput{
		Branches: make([]BranchInfo, 0),
		Current:  currentBranch,
	}

	for rows.Next() {
		var info BranchInfo
		err := rows.Scan(&info.Name, &info.Hash)
		if err != nil {
			return fmt.Errorf("scan branch: %w", err)
		}
		info.Current = info.Name == currentBranch
		// Shorten hash
		if len(info.Hash) > 7 {
			info.Hash = info.Hash[:7]
		}
		branchOutput.Branches = append(branchOutput.Branches, info)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate branches: %w", err)
	}

	// For YAML format (default), use custom tabular output
	if format == output.FormatYAML {
		out := cmd.OutOrStdout()
		for _, b := range branchOutput.Branches {
			marker := "  "
			if b.Current {
				marker = "* "
			}
			fmt.Fprintf(out, "%s%s (%s)\n", marker, b.Name, b.Hash)
		}
		return nil
	}

	// YAML/JSON output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("get formatter: %w", err)
	}
	return formatter.FormatToWriter(cmd.OutOrStdout(), branchOutput, output.DensityMedium)
}

func createBranch(cmd *cobra.Command, st *store.Store, name string, fromRef string) error {
	db := st.DB()

	// Validate branch name
	if !isValidBranchName(name) {
		return fmt.Errorf("invalid branch name: %s", name)
	}

	var err error
	if fromRef != "" {
		// Validate ref
		if !store.IsValidRef(fromRef) {
			return fmt.Errorf("invalid ref: %s", fromRef)
		}
		_, err = db.Exec("CALL dolt_branch(?, ?)", name, fromRef)
	} else {
		_, err = db.Exec("CALL dolt_branch(?)", name)
	}

	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("branch '%s' already exists", name)
		}
		return fmt.Errorf("create branch: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created branch '%s'\n", name)
	return nil
}

func deleteBranch(cmd *cobra.Command, st *store.Store, name string) error {
	db := st.DB()

	// Check if trying to delete current branch
	var currentBranch string
	err := db.QueryRow("SELECT active_branch()").Scan(&currentBranch)
	if err == nil && currentBranch == name {
		return fmt.Errorf("cannot delete the current branch '%s'", name)
	}

	// Validate branch name
	if !isValidBranchName(name) {
		return fmt.Errorf("invalid branch name: %s", name)
	}

	_, err = db.Exec("CALL dolt_branch('-d', ?)", name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("branch '%s' not found", name)
		}
		return fmt.Errorf("delete branch: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch '%s'\n", name)
	return nil
}

func checkoutBranch(cmd *cobra.Command, st *store.Store, name string) error {
	db := st.DB()

	// Validate branch name
	if !isValidBranchName(name) {
		return fmt.Errorf("invalid branch name: %s", name)
	}

	_, err := db.Exec("CALL dolt_checkout(?)", name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("branch '%s' not found", name)
		}
		return fmt.Errorf("checkout branch: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Switched to branch '%s'\n", name)
	return nil
}

// isValidBranchName checks if a branch name is valid.
// Similar to git branch names: alphanumeric, /, -, _, but no .., no starting/ending with /, etc.
func isValidBranchName(name string) bool {
	if name == "" || len(name) > 250 {
		return false
	}
	// Disallow dangerous patterns
	if strings.Contains(name, "..") ||
		strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") ||
		strings.HasPrefix(name, "-") {
		return false
	}
	// Only allow safe characters
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '/' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
