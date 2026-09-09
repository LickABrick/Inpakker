package cmd

import (
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/process"
	uiterm "github.com/LickABrick/inpakker/internal/tui"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newTUICmd(runner process.Runner) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive workspace interface",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd, runner)
		},
	}
}

func runTUI(cmd *cobra.Command, runner process.Runner) error {
	ws, err := workspace.Open(".")
	if errors.Is(err, os.ErrNotExist) && interactive(cmd) {
		options := workspace.SetupOptions{Root: ".", AppsDir: "apps", OutputDir: "output", CreateExample: true}
		if promptErr := promptSetup(cmd, &options); promptErr != nil {
			return promptErr
		}
		result, setupErr := workspace.Initialize(options)
		if setupErr != nil {
			return setupErr
		}
		ws, err = result.Workspace, nil
	}
	if err != nil {
		return err
	}
	model, err := uiterm.New(cmd.Context(), ws, runner)
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()))
	_, err = program.Run()
	return err
}

func init() {
	rootCmd.AddCommand(newTUICmd(process.ExecRunner{}))
}
