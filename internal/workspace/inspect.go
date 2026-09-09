package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

type App struct {
	Ref      AppRef           `json:"ref"`
	Config   *types.AppConfig `json:"config,omitempty"`
	Status   string           `json:"status"`
	Error    string           `json:"error,omitempty"`
	Packages []string         `json:"packages,omitempty"`
}

func (a App) Label() string {
	if a.Config != nil && strings.TrimSpace(a.Config.Name) != "" {
		return a.Config.Name
	}
	return filepath.Base(a.Ref.Path)
}

func (w *Workspace) Inspect(ref AppRef) App {
	app := App{Ref: ref, Status: "valid"}
	cfg, err := config.LoadAppConfig(filepath.Join(ref.Path, "app.config.json"))
	if err != nil {
		app.Status, app.Error = "invalid", fmt.Sprintf("load config: %v", err)
		return app
	}
	app.Config = cfg
	if err := config.ValidateApp(cfg); err != nil {
		app.Status, app.Error = "invalid", err.Error()
		return app
	}
	if err := ValidateInput(ref.Path, cfg); err != nil {
		app.Status, app.Error = "invalid", err.Error()
	}
	packages, packageErr := w.Packages(ref, cfg)
	if packageErr == nil {
		app.Packages = packages
	}
	return app
}

func (w *Workspace) List() ([]App, error) {
	selection, err := w.Discover(nil, true)
	if err != nil {
		return nil, err
	}
	apps := make([]App, 0, len(selection.Apps))
	for _, ref := range selection.Apps {
		apps = append(apps, w.Inspect(ref))
	}
	return apps, nil
}

func ValidateInput(appPath string, cfg *types.AppConfig) error {
	if err := config.ValidateApp(cfg); err != nil {
		return err
	}
	sourcePath := filepath.Join(appPath, cfg.Source)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("access source directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("source path is not a directory")
	}
	setupInfo, err := os.Stat(filepath.Join(sourcePath, cfg.SetupFile))
	if err != nil {
		return fmt.Errorf("access setup file: %w", err)
	}
	if setupInfo.IsDir() {
		return errors.New("setup file is a directory")
	}
	return nil
}

func (w *Workspace) OutputDir(ref AppRef, cfg *types.AppConfig) (string, error) {
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = w.DefaultOutputDir()
	}
	if !pathutil.IsSafeRelative(outputDir) {
		return "", errors.New("output directory must remain within the app directory")
	}
	return filepath.Join(ref.Path, outputDir), nil
}

func (w *Workspace) Packages(ref AppRef, cfg *types.AppConfig) ([]string, error) {
	outputDir, err := w.OutputDir(ref, cfg)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var packages []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".intunewin") {
			packages = append(packages, filepath.Join(outputDir, entry.Name()))
		}
	}
	sort.Strings(packages)
	return packages, nil
}
