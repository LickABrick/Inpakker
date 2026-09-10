package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Check workspace configuration and dependencies",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := resolveWorkspace(cmd)
			var checks []workspace.Check
			if err != nil {
				checks = []workspace.Check{{Name: "Workspace", Status: workspace.CheckFailure, Detail: err.Error()}}
			} else {
				checks = workspace.Diagnose(ws.Root)
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(checks); err != nil {
					return err
				}
			} else {
				console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
				for _, check := range checks {
					switch check.Status {
					case workspace.CheckOK:
						console.successDetail(check.Name, check.Detail)
					case workspace.CheckWarning:
						console.warningDetail(check.Name, check.Detail)
					case workspace.CheckFailure:
						console.failureDetail(check.Name, fmt.Errorf("%s", check.Detail))
					}
				}
			}
			failures := 0
			for _, check := range checks {
				if check.Status == workspace.CheckFailure {
					failures++
				}
			}
			if failures > 0 {
				return reportedError{err: fmt.Errorf("%d workspace checks failed", failures)}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable JSON")
	return command
}

func init() {
	rootCmd.AddCommand(newDoctorCmd())
}
