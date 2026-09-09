package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	var jsonOutput, noInput bool
	command := &cobra.Command{
		Use:   "show [app-name]",
		Short: "Show application details",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Open(".")
			if err != nil {
				return err
			}
			if len(args) == 0 {
				if jsonOutput || noInput || !interactive(cmd) {
					return asUsage(fmt.Errorf("application name is required"))
				}
				args, err = promptApplications(cmd, ws, "Application to show", false, false)
				if err != nil {
					return err
				}
			}
			ref, err := ws.Find(args[0])
			if err != nil {
				return err
			}
			app := ws.Inspect(ref)
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(app)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Application: %s\n", app.Label())
			fmt.Fprintf(cmd.OutOrStdout(), "Path:        %s\n", app.Ref.Relative)
			fmt.Fprintf(cmd.OutOrStdout(), "Status:      %s\n", app.Status)
			if app.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Issue:       %s\n", app.Error)
			}
			if app.Config != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Display:     %s\n", app.Config.DisplayName)
				fmt.Fprintf(cmd.OutOrStdout(), "Source:      %s\n", app.Config.Source)
				fmt.Fprintf(cmd.OutOrStdout(), "Setup:       %s\n", app.Config.SetupFile)
				fmt.Fprintf(cmd.OutOrStdout(), "Output:      %s\n", app.Config.OutputDir)
			}
			if len(app.Packages) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Packages:    %s\n", strings.Join(app.Packages, ", "))
			}
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable JSON")
	command.Flags().BoolVar(&noInput, "no-input", false, "Disable interactive target selection")
	return command
}

func init() {
	rootCmd.AddCommand(newShowCmd())
}
