package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "validate [app-name|group-name...]",
		Short: "Validate applications in the workspace",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return asUsage(fmt.Errorf("--all cannot be combined with named targets"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args, all || len(args) == 0)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Validate every application in the workspace")
	return command
}

func runValidate(cmd *cobra.Command, args []string, all bool) error {
	ws, err := resolveWorkspace(cmd)
	if err != nil {
		return err
	}
	selection, err := ws.Discover(args, all)
	if err != nil {
		return err
	}
	if len(selection.Apps) == 0 {
		return noApplicationsError(selection.Skipped)
	}

	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	console.start("Validating", len(selection.Apps))
	valid, invalid := 0, 0
	for _, ref := range selection.Apps {
		app := ws.Inspect(ref)
		if app.Status == "valid" {
			valid++
			continue
		}
		invalid++
		console.failureDetail(app.Label(), fmt.Errorf("%s", app.Error))
	}
	console.summary("Validation finished", valid, invalid, len(selection.Skipped))
	if invalid > 0 {
		return reportedError{err: fmt.Errorf("%d invalid %s", invalid, plural(invalid, "application", "applications"))}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newValidateCmd())
}
