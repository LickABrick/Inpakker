package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/types"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new [app-name]",
		Short: "Create a new application configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args[0])
		},
	}
}

func runNew(cmd *cobra.Command, appName string) error {
	if !validAppName(appName) {
		return fmt.Errorf("app name %q must be a single directory name", appName)
	}

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
	appDir := filepath.Join(appsRoot, appName)
	configPath := filepath.Join(appDir, "app.config.json")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("application %q already exists", appName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check existing application: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(appDir, "source"), 0o755); err != nil {
		return fmt.Errorf("create application directory: %w", err)
	}

	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create app config: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(types.AppConfig{
		Name:        appName,
		DisplayName: appName,
		Source:      "source",
		OutputDir:   "output",
	})
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(configPath)
		return fmt.Errorf("write app config: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(configPath)
		return fmt.Errorf("close app config: %w", closeErr)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", configPath)
	return nil
}

func validAppName(name string) bool {
	if name == "" ||
		name != strings.TrimSpace(name) ||
		name != strings.TrimRight(name, ". ") ||
		name == "." || name == ".." ||
		filepath.Base(filepath.Clean(name)) != name ||
		strings.ContainsAny(name, `<>:"/\|?*`) {
		return false
	}

	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
		return false
	}
	return true
}

func init() {
	rootCmd.AddCommand(newNewCmd())
}
