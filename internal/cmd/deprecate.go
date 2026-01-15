// Package cmd contains all CLI commands for cx.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// DeprecationInfo holds information about a deprecated command
type DeprecationInfo struct {
	OldCommand string
	NewCommand string
	NewFlags   string // Optional: specific flags to use with new command
	Message    string // Optional: custom message
}

// deprecationWarningsDisabled returns true if warnings should be suppressed
func deprecationWarningsDisabled() bool {
	return os.Getenv("CX_NO_DEPRECATION_WARNINGS") == "1"
}

// emitDeprecationWarning prints a deprecation warning to stderr
// It respects CX_NO_DEPRECATION_WARNINGS=1 environment variable
func emitDeprecationWarning(info DeprecationInfo) {
	if deprecationWarningsDisabled() {
		return
	}

	if info.Message != "" {
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", info.Message)
		return
	}

	newCmd := info.NewCommand
	if info.NewFlags != "" {
		newCmd = fmt.Sprintf("%s %s", info.NewCommand, info.NewFlags)
	}

	fmt.Fprintf(os.Stderr, "⚠️  '%s' is deprecated, use '%s' instead\n", info.OldCommand, newCmd)
}

// DeprecateCommand marks a command as deprecated and wraps its execution
// to emit a warning before running the original command logic.
// The command will still be hidden from help but remain functional.
func DeprecateCommand(cmd *cobra.Command, info DeprecationInfo) {
	// Store the original run functions
	originalPreRun := cmd.PreRun
	originalPreRunE := cmd.PreRunE

	// Wrap with deprecation warning
	if originalPreRunE != nil {
		cmd.PreRunE = func(c *cobra.Command, args []string) error {
			emitDeprecationWarning(info)
			return originalPreRunE(c, args)
		}
	} else if originalPreRun != nil {
		cmd.PreRun = func(c *cobra.Command, args []string) {
			emitDeprecationWarning(info)
			originalPreRun(c, args)
		}
	} else {
		// No existing PreRun, add one
		cmd.PreRun = func(c *cobra.Command, args []string) {
			emitDeprecationWarning(info)
		}
	}

	// Hide from help but keep functional
	cmd.Hidden = true

	// Add deprecation note to command's short description (for --help)
	cmd.Short = fmt.Sprintf("[DEPRECATED] %s (use '%s' instead)", cmd.Short, info.NewCommand)
}

// CreateDeprecatedAlias creates a new command that is an alias for another command
// The alias will emit a deprecation warning and then call the target command's Run function
func CreateDeprecatedAlias(oldName, newName string, targetCmd *cobra.Command, flagMapping map[string]string) *cobra.Command {
	info := DeprecationInfo{
		OldCommand: fmt.Sprintf("cx %s", oldName),
		NewCommand: fmt.Sprintf("cx %s", newName),
	}

	aliasCmd := &cobra.Command{
		Use:    targetCmd.Use,
		Short:  fmt.Sprintf("[DEPRECATED] Use 'cx %s' instead", newName),
		Hidden: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			emitDeprecationWarning(info)
		},
		Run:  targetCmd.Run,
		RunE: targetCmd.RunE,
	}

	// Copy flags from target command
	targetCmd.Flags().VisitAll(func(f *pflag.Flag) {
		aliasCmd.Flags().AddFlag(f)
	})

	return aliasCmd
}
