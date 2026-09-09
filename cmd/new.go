package cmd

import (
	"fmt"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var options workspace.CreateOptions
	var noInput bool
	command := &cobra.Command{
		Use:   "new [app-name]",
		Short: "Create a new application configuration",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				options.Name = args[0]
			}
			if interactive(cmd) && !noInput {
				ws, err := workspace.Open(".")
				if err != nil {
					return err
				}
				if options.DisplayName == "" {
					options.DisplayName = options.Name
				}
				if options.OutputDir == "" {
					options.OutputDir = ws.DefaultOutputDir()
				}
				if err := promptNew(cmd, &options); err != nil {
					return err
				}
			} else if options.Name == "" {
				return asUsage(fmt.Errorf("application name is required"))
			}
			return runNew(cmd, options)
		},
	}
	command.Flags().StringVar(&options.DisplayName, "display-name", "", "Application display name")
	command.Flags().StringVar(&options.Group, "group", "", "Group path under the applications directory")
	command.Flags().StringVar(&options.Source, "source", "source", "Source directory within the application")
	command.Flags().StringVar(&options.SetupFile, "setup-file", "", "Setup filename within the source directory")
	command.Flags().StringVar(&options.OutputDir, "output-dir", "", "Output directory within the application")
	command.Flags().BoolVar(&noInput, "no-input", false, "Disable interactive prompts")
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
