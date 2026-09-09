package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/LickABrick/inpakker/internal/pathutil"
)

type AppRef struct {
	Path     string `json:"path"`
	Relative string `json:"relativePath"`
}

type Selection struct {
	Apps    []AppRef
	Skipped []string
}

func (w *Workspace) Discover(names []string, all bool) (Selection, error) {
	seen := make(map[string]struct{})
	selection := Selection{}
	add := func(appPath string) {
		clean := filepath.Clean(appPath)
		if _, exists := seen[clean]; exists {
			return
		}
		seen[clean] = struct{}{}
		selection.Apps = append(selection.Apps, AppRef{Path: clean, Relative: w.Relative(clean)})
	}

	appsRoot := w.AppsDir()
	if all {
		if _, err := os.Stat(appsRoot); os.IsNotExist(err) {
			return selection, nil
		}
		err := filepath.Walk(appsRoot, func(current string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !info.IsDir() {
				return nil
			}
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			if current != filepath.Clean(appsRoot) && isFile(filepath.Join(current, "app.config.json")) {
				add(current)
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return Selection{}, fmt.Errorf("scan applications directory: %w", err)
		}
	} else {
		for _, name := range names {
			candidate, err := w.targetPath(name)
			if err != nil {
				selection.Skipped = append(selection.Skipped, name)
				continue
			}
			info, err := os.Stat(candidate)
			if err != nil || !info.IsDir() {
				selection.Skipped = append(selection.Skipped, name)
				continue
			}
			if isFile(filepath.Join(candidate, "app.config.json")) {
				add(candidate)
				continue
			}
			entries, err := os.ReadDir(candidate)
			if err != nil {
				selection.Skipped = append(selection.Skipped, name)
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
				selection.Skipped = append(selection.Skipped, name)
			}
		}
	}

	sort.Slice(selection.Apps, func(i, j int) bool {
		return selection.Apps[i].Relative < selection.Apps[j].Relative
	})
	sort.Strings(selection.Skipped)
	return selection, nil
}

func (w *Workspace) Find(name string) (AppRef, error) {
	selection, err := w.Discover([]string{name}, false)
	if err != nil {
		return AppRef{}, err
	}
	if len(selection.Apps) == 0 {
		return AppRef{}, fmt.Errorf("application %q not found", name)
	}
	if len(selection.Apps) > 1 {
		return AppRef{}, fmt.Errorf("%q is a group; specify an application", name)
	}
	return selection.Apps[0], nil
}

func (w *Workspace) targetPath(name string) (string, error) {
	if !pathutil.IsSafeRelative(name) || filepath.Clean(name) == "." {
		return "", fmt.Errorf("target %q must remain within the applications directory", name)
	}
	return filepath.Join(w.AppsDir(), filepath.Clean(name)), nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
