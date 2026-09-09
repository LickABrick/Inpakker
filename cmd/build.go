package cmd

import (
	"errors"
	"fmt"

	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newBuildCmd(runner process.Runner) *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "build [app-name|group-name...]",
		Short: "Package one or more applications",
		Args: func(cmd *cobra.Command, args []string) error {
			return validateTargetArgs(args, all)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, runner, args, all)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Build every application in the workspace")
	return command
}

func runBuild(cmd *cobra.Command, runner process.Runner, args []string, all bool) error {
	ws, err := workspace.Open(".")
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

	service := packager.Service{Workspace: ws, Runner: runner}
	if err := service.Validate(); err != nil {
		return err
	}
	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	console.start("Building", len(selection.Apps))
	results := service.Build(cmd.Context(), selection.Apps, cmd.OutOrStdout(), cmd.ErrOrStderr())
	succeeded, failed := 0, 0
	for _, result := range results {
		if result.Err != nil {
			failed++
			console.failureDetail(result.App.Label(), result.Err)
		} else {
			succeeded++
		}
	}
	console.summary("Build finished", succeeded, failed, len(selection.Skipped))
	if failed > 0 {
		return reportedError{err: fmt.Errorf("%d %s failed", failed, plural(failed, "application", "applications"))}
	}
	return nil
}

func validateTargetArgs(args []string, all bool) error {
	if all && len(args) > 0 {
		return errors.New("--all cannot be combined with named targets")
	}
	if !all && len(args) == 0 {
		return errors.New("provide an application or group name, or use --all")
	}
	return nil
}

func noApplicationsError(skipped []string) error {
	if len(skipped) > 0 {
		return fmt.Errorf("no valid applications found (%d %s skipped)", len(skipped), plural(len(skipped), "target", "targets"))
	}
	return errors.New("no valid applications found")
}

func init() {
	rootCmd.AddCommand(newBuildCmd(process.ExecRunner{}))
}
