package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/cliui"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/unpacker"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

type unpackTarget struct {
	label string
	path  string
}

type unpackFailure struct {
	label string
	err   error
}

type unpackExecution struct {
	succeeded       int
	failures        []unpackFailure
	onlyDestination string
}

func newUnpackCmd(runner process.Runner) *cobra.Command {
	var all, force, noInput bool
	var destination string
	command := &cobra.Command{
		Use:   "unpack [app-name|group-name|package.intunewin...]",
		Short: "Unpack applications using an external decoder",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return asUsage(errors.New("--all cannot be combined with named targets"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				if noInput || !interactive(cmd) {
					return asUsage(errors.New("provide an application, group, or package path, or use --all"))
				}
				ws, err := workspace.Open(".")
				if err != nil {
					return err
				}
				args, err = promptApplications(cmd, ws, "Application to unpack", false, true)
				if err != nil {
					return err
				}
			}
			return runUnpack(cmd, runner, args, all, destination, force)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Unpack every application package in the workspace")
	command.Flags().BoolVar(&force, "force", false, "Replace an existing destination directory")
	command.Flags().StringVar(&destination, "destination", "", "Destination for a single package")
	command.Flags().BoolVar(&noInput, "no-input", false, "Disable interactive target selection")
	return command
}

func runUnpack(cmd *cobra.Command, runner process.Runner, args []string, all bool, destination string, force bool) error {
	ws, err := workspace.Open(".")
	if err != nil {
		return err
	}
	service := unpacker.Service{DecoderPath: ws.Config.DecoderPath, Runner: runner}
	if err := service.Validate(); err != nil {
		return err
	}

	targets, failures, skipped, err := resolveUnpackTargets(ws, args, all)
	if err != nil {
		return err
	}
	if destination != "" && len(targets)+len(failures) != 1 {
		return errors.New("--destination requires exactly one package target")
	}
	if len(targets) == 0 && len(failures) == 0 {
		return noApplicationsError(skipped)
	}

	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	var execution unpackExecution
	if interactive(cmd) {
		value, progressErr := cliui.Run(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr(), "Unpacking applications", len(targets)+len(failures), func(ctx context.Context, emit func(cliui.Event)) (any, error) {
			return executeUnpack(ctx, service, targets, failures, destination, force, emit), nil
		})
		if progressErr != nil {
			return progressErr
		}
		execution, _ = value.(unpackExecution)
	} else {
		console.startCount("Unpacking", len(targets)+len(failures), "package", "packages")
		execution = executeUnpack(cmd.Context(), service, targets, failures, destination, force, nil)
	}
	for _, failure := range execution.failures {
		console.failureDetail(failure.label, failure.err)
	}
	failed := len(execution.failures)
	console.summary("Unpack finished", execution.succeeded, failed, len(skipped))
	if execution.succeeded == 1 && failed == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Output: %s\n", execution.onlyDestination)
	}
	if failed > 0 {
		return reportedError{err: fmt.Errorf("%d %s failed", failed, plural(failed, "package", "packages"))}
	}
	return nil
}

func executeUnpack(ctx context.Context, service unpacker.Service, targets []unpackTarget, initialFailures []unpackFailure, destination string, force bool, emit func(cliui.Event)) unpackExecution {
	execution := unpackExecution{failures: append([]unpackFailure(nil), initialFailures...)}
	total := len(targets) + len(initialFailures)
	completed := len(initialFailures)
	for _, target := range targets {
		if emit != nil {
			emit(cliui.Event{Current: completed, Total: total, Label: target.label, Phase: "decoding"})
		}
		result := service.Unpack(ctx, target.path, destination, force, nil, nil)
		completed++
		if result.Err != nil {
			execution.failures = append(execution.failures, unpackFailure{label: target.label, err: result.Err})
			continue
		}
		execution.succeeded++
		execution.onlyDestination = result.Destination
	}
	return execution
}

func resolveUnpackTargets(ws *workspace.Workspace, args []string, all bool) ([]unpackTarget, []unpackFailure, []string, error) {
	var targets []unpackTarget
	var failures []unpackFailure
	var names []string
	for _, arg := range args {
		if strings.EqualFold(filepath.Ext(arg), ".intunewin") {
			targets = append(targets, unpackTarget{label: filepath.Base(arg), path: arg})
		} else {
			names = append(names, arg)
		}
	}
	selection, err := ws.Discover(names, all)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, ref := range selection.Apps {
		cfg, loadErr := config.LoadAppConfig(filepath.Join(ref.Path, "app.config.json"))
		if loadErr != nil {
			failures = append(failures, unpackFailure{label: filepath.Base(ref.Path), err: fmt.Errorf("load config: %w", loadErr)})
			continue
		}
		if validationErr := config.ValidateApp(cfg); validationErr != nil {
			failures = append(failures, unpackFailure{label: appName(ref, cfg.Name), err: validationErr})
			continue
		}
		packages, packageErr := ws.Packages(ref, cfg)
		if packageErr != nil {
			failures = append(failures, unpackFailure{label: appName(ref, cfg.Name), err: packageErr})
			continue
		}
		if len(packages) == 0 {
			failures = append(failures, unpackFailure{label: appName(ref, cfg.Name), err: errors.New("no .intunewin package found")})
			continue
		}
		if len(packages) > 1 {
			failures = append(failures, unpackFailure{label: appName(ref, cfg.Name), err: errors.New("multiple .intunewin packages found; specify a package path")})
			continue
		}
		targets = append(targets, unpackTarget{label: appName(ref, cfg.Name), path: packages[0]})
	}
	return targets, failures, selection.Skipped, nil
}

func appName(ref workspace.AppRef, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	return filepath.Base(ref.Path)
}

func init() {
	rootCmd.AddCommand(newUnpackCmd(process.ExecRunner{}))
}
