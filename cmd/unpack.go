package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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

func newUnpackCmd(runner process.Runner) *cobra.Command {
	var all, force bool
	var destination string
	command := &cobra.Command{
		Use:   "unpack [app-name|group-name|package.intunewin...]",
		Short: "Unpack applications using an external decoder",
		Args: func(cmd *cobra.Command, args []string) error {
			return validateTargetArgs(args, all)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnpack(cmd, runner, args, all, destination, force)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Unpack every application package in the workspace")
	command.Flags().BoolVar(&force, "force", false, "Replace an existing destination directory")
	command.Flags().StringVar(&destination, "destination", "", "Destination for a single package")
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
	console.startCount("Unpacking", len(targets)+len(failures), "package", "packages")
	for _, failure := range failures {
		console.failureDetail(failure.label, failure.err)
	}
	succeeded, failed := 0, len(failures)
	var onlyDestination string
	for _, target := range targets {
		result := service.Unpack(cmd.Context(), target.path, destination, force, nil, nil)
		if result.Err != nil {
			failed++
			console.failureDetail(target.label, result.Err)
			continue
		}
		succeeded++
		onlyDestination = result.Destination
	}
	console.summary("Unpack finished", succeeded, failed, len(skipped))
	if succeeded == 1 && failed == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Output: %s\n", onlyDestination)
	}
	if failed > 0 {
		return reportedError{err: fmt.Errorf("%d %s failed", failed, plural(failed, "package", "packages"))}
	}
	return nil
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
