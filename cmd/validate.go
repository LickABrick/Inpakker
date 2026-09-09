package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate every application in the workspace",
		Args:  cobra.NoArgs,
		RunE:  runValidate,
	}
}

func runValidate(cmd *cobra.Command, _ []string) error {
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

	targets, _, err := discoverTargets(appsRoot, nil, true)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no applications found")
	}

	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	console.start("Validating", len(targets))
	valid, invalid := 0, 0
	for _, appPath := range targets {
		appCfg, loadErr := config.LoadAppConfig(filepath.Join(appPath, "app.config.json"))
		if loadErr != nil {
			invalid++
			console.failureDetail(filepath.Base(appPath), fmt.Errorf("load config: %w", loadErr))
			continue
		}
		if validationErr := config.ValidateApp(appCfg); validationErr != nil {
			invalid++
			console.failureDetail(appLabel(appPath, appCfg.Name), validationErr)
			continue
		}
		if validationErr := validateBuildInput(filepath.Join(appPath, appCfg.Source), appCfg.SetupFile); validationErr != nil {
			invalid++
			console.failureDetail(appLabel(appPath, appCfg.Name), validationErr)
			continue
		}
		valid++
	}

	console.summary("Validation finished", valid, invalid, 0)
	if invalid > 0 {
		return reportedError{err: fmt.Errorf("%d invalid %s", invalid, plural(invalid, "application", "applications"))}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newValidateCmd())
}
