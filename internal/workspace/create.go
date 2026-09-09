package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

type CreateOptions struct {
	Name        string
	Group       string
	DisplayName string
	Source      string
	SetupFile   string
	OutputDir   string
}

func (w *Workspace) Create(options CreateOptions) (AppRef, error) {
	if !ValidAppName(options.Name) {
		return AppRef{}, fmt.Errorf("app name %q must be a valid Windows directory name", options.Name)
	}
	if !ValidGroupName(options.Group) {
		return AppRef{}, fmt.Errorf("group %q must be a relative path of valid Windows directory names", options.Group)
	}
	if options.DisplayName == "" {
		options.DisplayName = options.Name
	}
	if options.Source == "" {
		options.Source = "source"
	}
	if options.OutputDir == "" {
		options.OutputDir = w.DefaultOutputDir()
	}
	for name, value := range map[string]string{
		"source": options.Source, "setup file": options.SetupFile, "output directory": options.OutputDir,
	} {
		if value != "" && !pathutil.IsSafeRelative(value) {
			return AppRef{}, fmt.Errorf("%s must remain within the application directory", name)
		}
	}

	appDir := filepath.Join(w.AppsDir(), options.Group, options.Name)
	configPath := filepath.Join(appDir, "app.config.json")
	if _, err := os.Stat(configPath); err == nil {
		return AppRef{}, fmt.Errorf("application %q already exists", options.Name)
	} else if !os.IsNotExist(err) {
		return AppRef{}, fmt.Errorf("check existing application: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(appDir, options.Source), 0o755); err != nil {
		return AppRef{}, fmt.Errorf("create application directory: %w", err)
	}

	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return AppRef{}, fmt.Errorf("create app config: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(types.AppConfig{
		Name:        options.Name,
		DisplayName: options.DisplayName,
		Source:      options.Source,
		SetupFile:   options.SetupFile,
		OutputDir:   options.OutputDir,
	})
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(configPath)
		return AppRef{}, fmt.Errorf("write app config: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(configPath)
		return AppRef{}, fmt.Errorf("close app config: %w", closeErr)
	}
	return AppRef{Path: appDir, Relative: w.Relative(appDir)}, nil
}

func ValidGroupName(group string) bool {
	if group == "" {
		return true
	}
	if !pathutil.IsSafeRelative(group) || filepath.Clean(group) == "." {
		return false
	}
	for _, part := range strings.FieldsFunc(group, func(r rune) bool { return r == '/' || r == '\\' }) {
		if !ValidAppName(part) {
			return false
		}
	}
	return true
}

func ValidAppName(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || name != strings.TrimRight(name, ". ") ||
		name == "." || name == ".." || filepath.Base(filepath.Clean(name)) != name ||
		strings.ContainsAny(name, `<>:"/\|?*`) {
		return false
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	return !(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9')
}
