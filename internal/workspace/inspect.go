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
	Ref       AppRef                    `json:"ref"`
	Config    *types.AppConfig          `json:"config,omitempty"`
	Effective *types.EffectiveAppConfig `json:"effective,omitempty"`
	Status    string                    `json:"status"`
	Error     string                    `json:"error,omitempty"`
	Packages  []string                  `json:"packages,omitempty"`
}

func (a App) Label() string {
	if a.Config != nil && strings.TrimSpace(a.Config.Name) != "" {
		return a.Config.Name
	}
	return filepath.Base(a.Ref.Path)
}

func (w *Workspace) Inspect(ref AppRef) App {
	app := App{Ref: ref, Status: "valid"}
	cfg, err := config.LoadAppConfig(filepath.Join(ref.Path, "inpakker.app.json"))
	if err != nil {
		app.Status, app.Error = "invalid", fmt.Sprintf("load config: %v", err)
		return app
	}
	app.Config = cfg
	if err := config.ValidateApp(cfg); err != nil {
		app.Status, app.Error = "invalid", err.Error()
		return app
	}
	effective := w.Effective(*cfg)
	app.Effective = &effective
	if err := ValidateInput(ref.Path, &effective.AppConfig); err != nil {
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
	if err := pathutil.Within(appPath, filepath.Join(appPath, pathutil.Native(cfg.SourceDirectory))); err != nil {
		return err
	}
	sourcePath := filepath.Join(appPath, pathutil.Native(cfg.SourceDirectory))
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("access source directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("source path is not a directory")
	}
	if err := pathutil.Within(sourcePath, filepath.Join(sourcePath, pathutil.Native(cfg.SetupFile))); err != nil {
		return err
	}
	setupInfo, err := os.Stat(filepath.Join(sourcePath, pathutil.Native(cfg.SetupFile)))
	if err != nil {
		return fmt.Errorf("access setup file: %w", err)
	}
	if setupInfo.IsDir() {
		return errors.New("setup file is a directory")
	}
	return filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source entry %q is a symbolic link; use regular source files", path)
		}
		return nil
	})
}

func (w *Workspace) OutputDir(ref AppRef, cfg *types.AppConfig) (string, error) {
	effective := w.Effective(*cfg)
	if !pathutil.ValidRelative(effective.OutputDirectory) {
		return "", errors.New("output directory must remain within the application directory")
	}
	target := filepath.Join(ref.Path, pathutil.Native(effective.OutputDirectory))
	if err := pathutil.Within(ref.Path, target); err != nil {
		return "", err
	}
	return target, nil
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
