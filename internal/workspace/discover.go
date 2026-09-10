package workspace

import (
	"fmt"
	"github.com/LickABrick/inpakker/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
)

const AppConfigFile = "inpakker.app.json"

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
	if _, err := os.Stat(appsRoot); err == nil {
		if err := pathutil.Within(w.Root, appsRoot); err != nil {
			return selection, err
		}
	}
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
			if current != filepath.Clean(appsRoot) && isFile(filepath.Join(current, "inpakker.app.json")) {
				add(current)
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return Selection{}, fmt.Errorf("scan applications directory: %w", err)
		}
	} else {
		inventory, err := w.Discover(nil, true)
		if err != nil {
			return selection, err
		}
		for _, name := range names {
			candidate, err := w.targetPath(name)
			if err != nil {
				selection.Skipped = append(selection.Skipped, name)
				continue
			}
			exact := false
			for _, ref := range inventory.Apps {
				if strings.EqualFold(ref.Path, candidate) {
					add(ref.Path)
					exact = true
					break
				}
			}
			if exact {
				continue
			}
			var matches []AppRef
			for _, ref := range inventory.Apps {
				cfg, err := config.LoadAppConfig(filepath.Join(ref.Path, AppConfigFile))
				if err == nil && strings.EqualFold(cfg.Name, name) {
					matches = append(matches, ref)
				}
			}
			if len(matches) > 1 {
				var targets []string
				for _, ref := range matches {
					targets = append(targets, ref.Relative)
				}
				return Selection{}, fmt.Errorf("application %q is ambiguous: %s", name, strings.Join(targets, ", "))
			}
			if len(matches) == 1 {
				add(matches[0].Path)
				continue
			}
			for _, ref := range inventory.Apps {
				rel, err := filepath.Rel(candidate, ref.Path)
				if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					add(ref.Path)
					exact = true
				}
			}
			if !exact {
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
	if !pathutil.ValidRelative(name) || filepath.Clean(name) == "." {
		return "", fmt.Errorf("target %q must remain within the applications directory", name)
	}
	return filepath.Join(w.AppsDir(), filepath.Clean(pathutil.Native(name))), nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Groups discovers directories once per inventory load, stopping at application roots.
func (w *Workspace) Groups() ([]string, error) {
	groups := []string{}
	err := filepath.WalkDir(w.AppsDir(), func(path string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) && path == w.AppsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == ".git" || isFile(filepath.Join(path, AppConfigFile)) {
			return filepath.SkipDir
		}
		if path != w.AppsDir() {
			groups = append(groups, w.Relative(path))
		}
		return nil
	})
	sort.Slice(groups, func(i, j int) bool { return naturalLess(groups[i], groups[j]) })
	return groups, err
}
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			i, j := 0, 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			an, bn := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}
