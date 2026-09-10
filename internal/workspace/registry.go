package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

var ErrNoWorkspace = errors.New("no workspace selected; use 'inpakker workspace create' or 'workspace add'")

type RegistrationView struct {
	types.WorkspaceRegistration
	Name   string                 `json:"name"`
	Status string                 `json:"status"`
	Error  string                 `json:"error,omitempty"`
	Active bool                   `json:"active"`
	Config *types.WorkspaceConfig `json:"config,omitempty"`
}

type CreateWorkspaceOptions struct {
	Root                  string
	Name                  string
	ApplicationsDirectory string
	SourceDirectory       string
	OutputDirectory       string
}

func Registrations(user *types.UserConfig) []RegistrationView {
	result := make([]RegistrationView, 0, len(user.Workspaces))
	for _, reg := range user.Workspaces {
		view := RegistrationView{WorkspaceRegistration: reg, Name: filepath.Base(reg.Path), Status: "ready", Active: reg.ID == user.ActiveWorkspaceID}
		cfg, err := config.LoadWorkspace(filepath.Join(reg.Path, ConfigFile))
		if err != nil {
			view.Status, view.Error = "invalid", err.Error()
			if _, pathErr := os.Stat(reg.Path); pathErr != nil {
				view.Status = "unavailable"
			}
		} else if cfg.ID != reg.ID {
			view.Status, view.Error = "invalid", "workspace UUID differs; this appears to be another workspace"
		} else {
			view.Name, view.Config = cfg.Name, cfg
		}
		result = append(result, view)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Active != b.Active {
			return a.Active
		}
		if !a.LastOpenedAt.Equal(b.LastOpenedAt) {
			return a.LastOpenedAt.After(b.LastOpenedAt)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result
}

func Lookup(user *types.UserConfig, selector string) (RegistrationView, error) {
	var found []RegistrationView
	for _, view := range Registrations(user) {
		if strings.EqualFold(view.Name, selector) || view.ID == selector || sameLocation(view.Path, selector) {
			found = append(found, view)
		}
	}
	if len(found) == 0 {
		return RegistrationView{}, fmt.Errorf("workspace %q is not registered", selector)
	}
	if len(found) > 1 {
		var paths []string
		for _, v := range found {
			paths = append(paths, v.Path)
		}
		return RegistrationView{}, fmt.Errorf("workspace %q is ambiguous: %s; use a path", selector, strings.Join(paths, ", "))
	}
	return found[0], nil
}
func sameLocation(a, b string) bool {
	abs, err := filepath.Abs(b)
	return err == nil && strings.EqualFold(filepath.Clean(a), filepath.Clean(abs))
}

func checkRegistration(user *types.UserConfig, ws *Workspace) error {
	for _, view := range Registrations(user) {
		if view.ID == ws.Config.ID {
			if sameLocation(view.Path, ws.Root) {
				return fmt.Errorf("workspace already added at %q", view.Path)
			}
			return fmt.Errorf("workspace is registered at %q; use workspace relink", view.Path)
		}
		if sameLocation(view.Path, ws.Root) || strings.EqualFold(view.Name, ws.Config.Name) {
			return fmt.Errorf("workspace name or path is already registered: %s", view.Name)
		}
	}
	return nil
}

func CreateWorkspace(options CreateWorkspaceOptions) (*Workspace, error) {
	user, err := config.EnsureUser()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Name) == "" {
		return nil, errors.New("workspace name is required")
	}
	if options.Root == "" {
		return nil, errors.New("workspace location is required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, err
	}
	d := user.WorkspaceDefaults
	if options.ApplicationsDirectory != "" {
		d.ApplicationsDirectory = options.ApplicationsDirectory
	}
	if options.SourceDirectory != "" {
		d.SourceDirectory = options.SourceDirectory
	}
	if options.OutputDirectory != "" {
		d.OutputDirectory = options.OutputDirectory
	}
	cfg := types.WorkspaceConfig{SchemaVersion: 1, ID: config.NewUUID(), Name: strings.TrimSpace(options.Name), ApplicationsDirectory: d.ApplicationsDirectory, SourceDirectory: d.SourceDirectory, OutputDirectory: d.OutputDirectory}
	if err := config.ValidateWorkspace(&cfg); err != nil {
		return nil, err
	}
	ws := &Workspace{Root: root, Config: cfg, User: *user}
	if err := checkRegistration(user, ws); err != nil {
		return nil, err
	}
	configPath := filepath.Join(root, ConfigFile)
	if _, err := os.Lstat(configPath); !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("workspace config already exists or is inaccessible: %s", configPath)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	if err := pathutil.Within(root, ws.AppsDir()); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(ws.AppsDir(), 0755); err != nil {
		return nil, err
	}
	if err := config.SaveWorkspace(configPath, &cfg); err != nil {
		return nil, err
	}
	if err := config.UpdateUser(func(current *types.UserConfig) error {
		if err := checkRegistration(current, ws); err != nil {
			return err
		}
		current.Workspaces = append(current.Workspaces, types.WorkspaceRegistration{ID: cfg.ID, Path: root, LastOpenedAt: time.Now().UTC()})
		current.ActiveWorkspaceID = cfg.ID
		return nil
	}); err != nil {
		return nil, fmt.Errorf("workspace created; registration failed (use workspace add): %w", err)
	}
	return Open(root)
}

func Add(root string) (*Workspace, error) {
	ws, err := Open(root)
	if err != nil {
		return nil, err
	}
	err = config.UpdateUser(func(user *types.UserConfig) error {
		if err := checkRegistration(user, ws); err != nil {
			return err
		}
		user.Workspaces = append(user.Workspaces, types.WorkspaceRegistration{ID: ws.Config.ID, Path: ws.Root})
		return nil
	})
	return ws, err
}
func Use(selector string) (*Workspace, error) {
	var ws *Workspace
	err := config.UpdateUser(func(user *types.UserConfig) error {
		view, err := Lookup(user, selector)
		if err != nil {
			return err
		}
		if view.Status != "ready" {
			return fmt.Errorf("workspace %s: %s; use workspace relink", view.Name, view.Error)
		}
		ws, err = Open(view.Path)
		if err != nil {
			return err
		}
		for i := range user.Workspaces {
			if user.Workspaces[i].ID == view.ID {
				user.Workspaces[i].LastOpenedAt = time.Now().UTC()
			}
		}
		user.ActiveWorkspaceID = view.ID
		return nil
	})
	return ws, err
}
func Remove(selector string) error {
	return config.UpdateUser(func(user *types.UserConfig) error {
		view, err := Lookup(user, selector)
		if err != nil {
			return err
		}
		for i, reg := range user.Workspaces {
			if reg.ID == view.ID {
				user.Workspaces = append(user.Workspaces[:i], user.Workspaces[i+1:]...)
				break
			}
		}
		if user.ActiveWorkspaceID == view.ID {
			user.ActiveWorkspaceID = ""
		}
		return nil
	})
}
func Relink(selector, root string) error {
	ws, err := Open(root)
	if err != nil {
		return err
	}
	return config.UpdateUser(func(user *types.UserConfig) error {
		view, err := Lookup(user, selector)
		if err != nil {
			return err
		}
		if view.ID != ws.Config.ID {
			return errors.New("UUID differs: this appears to be another workspace")
		}
		for i := range user.Workspaces {
			if user.Workspaces[i].ID == view.ID {
				user.Workspaces[i].Path = ws.Root
			}
		}
		return nil
	})
}

// Resolve is shared by every workspace-dependent entry point. Explicit failures never fall through.
func Resolve(selector, cwd string) (*Workspace, error) {
	user, err := config.LoadUser()
	if err != nil {
		return nil, err
	}
	if selector == "" {
		selector = os.Getenv("INPAKKER_WORKSPACE")
	}
	if selector != "" {
		if info, e := os.Stat(selector); e == nil && info.IsDir() {
			return Open(selector)
		}
		view, e := Lookup(user, selector)
		if e != nil {
			return nil, e
		}
		if view.Status != "ready" {
			return nil, fmt.Errorf("workspace %s: %s; use workspace relink", view.Name, view.Error)
		}
		return Open(view.Path)
	}
	root, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	for {
		_, err := os.Lstat(filepath.Join(root, ConfigFile))
		if err == nil {
			return Open(root)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	if user.ActiveWorkspaceID != "" {
		view, err := Lookup(user, user.ActiveWorkspaceID)
		if err != nil {
			return nil, err
		}
		if view.Status != "ready" {
			return nil, fmt.Errorf("active workspace %s: %s; use workspace relink", view.Name, view.Error)
		}
		return Open(view.Path)
	}
	return nil, ErrNoWorkspace
}

func (w *Workspace) SetSetting(key, value string) error {
	cfg := w.Config
	if err := config.SetWorkspaceValue(&cfg, key, value); err != nil {
		return err
	}
	user, err := config.LoadUser()
	if err != nil {
		return err
	}
	if key == "name" {
		for _, view := range Registrations(user) {
			if view.ID != cfg.ID && strings.EqualFold(view.Name, cfg.Name) {
				return errors.New("workspace name is already registered")
			}
		}
	}
	return config.SaveWorkspace(filepath.Join(w.Root, ConfigFile), &cfg)
}
