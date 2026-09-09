package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LickABrick/inpakker/internal/cliui"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newBuildCmd(runner process.Runner) *cobra.Command {
	var all, force, noCache, noInput bool
	command := &cobra.Command{
		Use:   "build [app-name|group-name...]",
		Short: "Package one or more applications",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return asUsage(errors.New("--all cannot be combined with named targets"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				if !interactive(cmd) || noInput {
					return asUsage(errors.New("provide an application or group name, or use --all"))
				}
				ws, err := workspace.Open(".")
				if err != nil {
					return err
				}
				args, err = promptApplications(cmd, ws, "Applications to build", true, false)
				if err != nil {
					return err
				}
			}
			return runBuild(cmd, runner, args, all, packager.BuildOptions{Force: force, NoCache: noCache})
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Build every application in the workspace")
	command.Flags().BoolVar(&force, "force", false, "Build even when inputs and output are unchanged")
	command.Flags().BoolVar(&noCache, "no-cache", false, "Build without reading or writing the build cache")
	command.Flags().BoolVar(&noInput, "no-input", false, "Disable interactive target selection")
	return command
}

func runBuild(cmd *cobra.Command, runner process.Runner, args []string, all bool, options packager.BuildOptions) error {
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
	var results []packager.Result
	var buildErr error
	if interactive(cmd) {
		var toolOutput bytes.Buffer
		value, progressErr := cliui.Run(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr(), "Building applications", len(selection.Apps), func(ctx context.Context, emit func(cliui.Event)) (any, error) {
			options.OnProgress = func(event packager.Event) {
				emit(cliui.Event{Current: event.Index - 1, Total: event.Total, Label: event.App, Phase: event.Phase})
			}
			return service.Build(ctx, selection.Apps, options, &toolOutput, &toolOutput)
		})
		if progressErr != nil && value == nil {
			return progressErr
		}
		results, _ = value.([]packager.Result)
		buildErr = progressErr
		if strings.TrimSpace(toolOutput.String()) != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Tool output:\n%s\n", strings.TrimSpace(toolOutput.String()))
		}
	} else {
		console.start("Building", len(selection.Apps))
		results, buildErr = service.Build(cmd.Context(), selection.Apps, options, cmd.OutOrStdout(), cmd.ErrOrStderr())
	}
	built, current, failed := 0, 0, 0
	for _, result := range results {
		if result.Err != nil {
			failed++
			console.failureDetail(result.App.Label(), result.Err)
		} else if result.Status == packager.StatusCurrent {
			current++
		} else {
			built++
			console.successDetail(result.App.Label(), "built")
		}
	}
	console.buildSummary(built, current, failed, len(selection.Skipped))
	if buildErr != nil {
		return buildErr
	}
	if failed > 0 {
		return reportedError{err: fmt.Errorf("%d %s failed", failed, plural(failed, "application", "applications"))}
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
