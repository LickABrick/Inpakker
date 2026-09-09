package cmd

import (
	"fmt"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var options workspace.CreateOptions
	command := &cobra.Command{
		Use:   "new [app-name]",
		Short: "Create a new application configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return runNew(cmd, options)
		},
	}
	command.Flags().StringVar(&options.DisplayName, "display-name", "", "Application display name")
	command.Flags().StringVar(&options.Group, "group", "", "Group path under the applications directory")
	command.Flags().StringVar(&options.Source, "source", "source", "Source directory within the application")
	command.Flags().StringVar(&options.SetupFile, "setup-file", "", "Setup filename within the source directory")
	command.Flags().StringVar(&options.OutputDir, "output-dir", "", "Output directory within the application")
	return command
}

func runNew(cmd *cobra.Command, options workspace.CreateOptions) error {
	ws, err := workspace.Open(".")
	if err != nil {
		return err
	}
	ref, err := ws.Create(options)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", ref.Relative)
	return nil
}

func init() {
	rootCmd.AddCommand(newNewCmd())
}
