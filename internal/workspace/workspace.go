package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

const ConfigFile = "inpakker.workspace.json"

type Workspace struct {
	Root   string
	Config types.WorkspaceConfig
	User   types.UserConfig
}

func Open(root string) (*Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	cfg, err := config.LoadWorkspace(filepath.Join(absRoot, ConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workspace unavailable; use 'inpakker workspace create' or 'workspace add': %w", err)
		}
		return nil, fmt.Errorf("load workspace configuration: %w", err)
	}
	if err := config.ValidateWorkspace(cfg); err != nil {
		return nil, fmt.Errorf("validate workspace configuration: %w", err)
	}
	user, err := config.LoadUser()
	if err != nil {
		return nil, err
	}
	return &Workspace{Root: absRoot, Config: *cfg, User: *user}, nil
}

func (w *Workspace) AppsDir() string {
	dir := w.Config.ApplicationsDirectory
	if dir == "" {
		dir = "apps"
	}
	return filepath.Join(w.Root, pathutil.Native(dir))
}

func (w *Workspace) DefaultOutputDir() string {
	if w.Config.OutputDirectory != "" {
		return w.Config.OutputDirectory
	}
	return "output"
}

func (w *Workspace) Relative(path string) string {
	relative, err := filepath.Rel(w.AppsDir(), path)
	if err != nil {
		return filepath.Base(path)
	}
	return relative
}

func EnsureFile(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("access %s %q: %w", label, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s %q is a directory", label, path)
	}
	return nil
}

func (w *Workspace) Effective(app types.AppConfig) types.EffectiveAppConfig {
	return config.EffectiveApp(w.Config, app)
}
func (w *Workspace) CacheDirectory() (string, error) {
	if !config.ValidUUID(w.Config.ID) {
		return "", fmt.Errorf("workspace id must be a UUID")
	}
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "state", "workspaces", w.Config.ID), nil
}
