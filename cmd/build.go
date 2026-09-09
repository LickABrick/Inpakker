package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
)

type processRunner interface {
	Run(context.Context, string, []string, io.Writer, io.Writer) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func newBuildCmd(runner processRunner) *cobra.Command {
	var all bool

	command := &cobra.Command{
		Use:   "build [app-name|group-name...]",
		Short: "Package one or more applications",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return errors.New("--all cannot be combined with named targets")
			}
			if !all && len(args) == 0 {
				return errors.New("provide an application or group name, or use --all")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, runner, args, all)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Build every application in the workspace")
	return command
}

func runBuild(cmd *cobra.Command, runner processRunner, args []string, all bool) error {
	globalCfg, err := config.LoadGlobalConfig("inpakker.config.json")
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	if err := config.ValidateGlobal(globalCfg); err != nil {
		return fmt.Errorf("validate global config: %w", err)
	}

	appsRoot := globalCfg.AppsDir
	if appsRoot == "" {
		appsRoot = "apps"
	}
	defaultOutputDir := globalCfg.DefaultOutputDir
	if defaultOutputDir == "" {
		defaultOutputDir = "output"
	}

	targets, skipped, err := discoverTargets(appsRoot, args, all)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if skipped > 0 {
			return fmt.Errorf("no valid applications found (%d %s skipped)", skipped, plural(skipped, "target", "targets"))
		}
		return errors.New("no valid applications found")
	}

	intuneUtilPath := globalCfg.IntuneWinAppUtil
	if intuneUtilPath == "" {
		intuneUtilPath = globalCfg.IntuneWinAppUtilPath
	}
	if strings.TrimSpace(intuneUtilPath) == "" {
		return errors.New("intunewinapputil is not configured")
	}
	if info, statErr := os.Stat(intuneUtilPath); statErr != nil {
		return fmt.Errorf("access intunewinapputil %q: %w", intuneUtilPath, statErr)
	} else if info.IsDir() {
		return fmt.Errorf("intunewinapputil %q is a directory", intuneUtilPath)
	}

	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	console.start("Building", len(targets))

	succeeded, failed := 0, 0
	for _, appPath := range targets {
		appCfg, loadErr := config.LoadAppConfig(filepath.Join(appPath, "app.config.json"))
		if loadErr != nil {
			failed++
			console.failureDetail(filepath.Base(appPath), fmt.Errorf("load config: %w", loadErr))
			continue
		}
		if validationErr := config.ValidateApp(appCfg); validationErr != nil {
			failed++
			console.failureDetail(appLabel(appPath, appCfg.Name), validationErr)
			continue
		}

		sourcePath := filepath.Join(appPath, appCfg.Source)
		if validationErr := validateBuildInput(sourcePath, appCfg.SetupFile); validationErr != nil {
			failed++
			console.failureDetail(appLabel(appPath, appCfg.Name), validationErr)
			continue
		}

		outputDir := appCfg.OutputDir
		if outputDir == "" {
			outputDir = defaultOutputDir
		}
		if !safeRelativePath(outputDir) {
			failed++
			console.failureDetail(appLabel(appPath, appCfg.Name), errors.New("output directory must remain within the app directory"))
			continue
		}
		outputPath := filepath.Join(appPath, outputDir)
		if mkdirErr := os.MkdirAll(outputPath, 0o755); mkdirErr != nil {
			failed++
			console.failureDetail(appLabel(appPath, appCfg.Name), fmt.Errorf("create output directory: %w", mkdirErr))
			continue
		}

		var stdout, stderr io.Writer
		if !globalCfg.MuteIntuneWinAppUtil {
			stdout = cmd.OutOrStdout()
			stderr = cmd.ErrOrStderr()
		}
		runErr := runner.Run(cmd.Context(), intuneUtilPath, []string{
			"-c", sourcePath,
			"-s", appCfg.SetupFile,
			"-o", outputPath,
			"-q",
		}, stdout, stderr)
		if runErr != nil {
			failed++
			console.failureDetail(appLabel(appPath, appCfg.Name), runErr)
			continue
		}
		succeeded++
	}

	console.summary("Build finished", succeeded, failed, skipped)
	if failed > 0 {
		return reportedError{err: fmt.Errorf("%d %s failed", failed, plural(failed, "application", "applications"))}
	}
	return nil
}

func discoverTargets(appsRoot string, names []string, all bool) ([]string, int, error) {
	seen := make(map[string]struct{})
	var targets []string
	add := func(path string) {
		clean := filepath.Clean(path)
		if _, exists := seen[clean]; !exists {
			seen[clean] = struct{}{}
			targets = append(targets, clean)
		}
	}

	if all {
		err := filepath.Walk(appsRoot, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				if info.Name() == ".git" {
					return filepath.SkipDir
				}
				if path != filepath.Clean(appsRoot) && isFile(filepath.Join(path, "app.config.json")) {
					add(path)
					return filepath.SkipDir
				}
			}
			return nil
		})
		if err != nil {
			return nil, 0, fmt.Errorf("scan applications directory: %w", err)
		}
	} else {
		skipped := 0
		for _, name := range names {
			candidate, pathErr := pathWithin(appsRoot, name)
			if pathErr != nil {
				skipped++
				continue
			}
			info, statErr := os.Stat(candidate)
			if statErr != nil || !info.IsDir() {
				skipped++
				continue
			}
			if isFile(filepath.Join(candidate, "app.config.json")) {
				add(candidate)
				continue
			}

			entries, readErr := os.ReadDir(candidate)
			if readErr != nil {
				skipped++
				continue
			}
			found := false
			for _, entry := range entries {
				if entry.IsDir() && isFile(filepath.Join(candidate, entry.Name(), "app.config.json")) {
					add(filepath.Join(candidate, entry.Name()))
					found = true
				}
			}
			if !found {
				skipped++
			}
		}
		sort.Strings(targets)
		return targets, skipped, nil
	}

	sort.Strings(targets)
	return targets, 0, nil
}

func validateBuildInput(sourcePath, setupFile string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("access source directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("source path is not a directory")
	}
	setupInfo, err := os.Stat(filepath.Join(sourcePath, setupFile))
	if err != nil {
		return fmt.Errorf("access setup file: %w", err)
	}
	if setupInfo.IsDir() {
		return errors.New("setup file is a directory")
	}
	return nil
}

func pathWithin(root, name string) (string, error) {
	if !safeRelativePath(name) || filepath.Clean(name) == "." {
		return "", fmt.Errorf("target %q must be a relative path within the applications directory", name)
	}
	return filepath.Join(root, filepath.Clean(name)), nil
}

func safeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func appLabel(path, configuredName string) string {
	if strings.TrimSpace(configuredName) != "" {
		return configuredName
	}
	return filepath.Base(path)
}

func init() {
	rootCmd.AddCommand(newBuildCmd(execRunner{}))
}
