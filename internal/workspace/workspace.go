package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/types"
)

const ConfigFile = "inpakker.config.json"

type Workspace struct {
	Root   string
	Config types.GlobalConfig
}

func Open(root string) (*Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	cfg, err := config.LoadGlobalConfig(filepath.Join(absRoot, ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}
	if err := config.ValidateGlobal(cfg); err != nil {
		return nil, fmt.Errorf("validate global config: %w", err)
	}
	return &Workspace{Root: absRoot, Config: *cfg}, nil
}

func (w *Workspace) AppsDir() string {
	dir := w.Config.AppsDir
	if dir == "" {
		dir = "apps"
	}
	return filepath.Join(w.Root, dir)
}

func (w *Workspace) DefaultOutputDir() string {
	if w.Config.DefaultOutputDir != "" {
		return w.Config.DefaultOutputDir
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
