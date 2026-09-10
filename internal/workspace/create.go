package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

type CreateOptions struct {
	Name            string
	DirectoryName   string
	Group           string
	SourceDirectory string
	SetupFile       string
	OutputDirectory string
}

func Slug(name string) string {
	var out strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return out.String()
}

func (w *Workspace) Create(options CreateOptions) (AppRef, error) {
	if strings.TrimSpace(options.Name) == "" {
		return AppRef{}, errors.New("application name is required")
	}
	if options.DirectoryName == "" {
		options.DirectoryName = Slug(options.Name)
	}
	if !ValidAppName(options.DirectoryName) {
		return AppRef{}, errors.New("directory name must be a valid Windows directory name")
	}
	if !ValidGroupName(options.Group) {
		return AppRef{}, errors.New("group must be a safe relative directory path")
	}
	cfg := types.AppConfig{SchemaVersion: 1, ID: config.NewUUID(), Name: strings.TrimSpace(options.Name), SetupFile: options.SetupFile, SourceDirectory: options.SourceDirectory, OutputDirectory: options.OutputDirectory}
	if err := config.ValidateApp(&cfg); err != nil {
		return AppRef{}, err
	}
	effective := w.Effective(cfg)
	if !pathutil.ValidRelative(effective.SourceDirectory) || !pathutil.ValidRelative(effective.OutputDirectory) {
		return AppRef{}, errors.New("workspace source/output directories are invalid")
	}
	appDir := filepath.Join(w.AppsDir(), pathutil.Native(options.Group), options.DirectoryName)
	if err := pathutil.Within(w.Root, appDir); err != nil {
		return AppRef{}, err
	}
	// Never create nested applications inside another application's source or output.
	for parent := filepath.Dir(appDir); parent != w.AppsDir() && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
		if isFile(filepath.Join(parent, AppConfigFile)) {
			return AppRef{}, errors.New("an application cannot be created inside another application")
		}
	}
	if _, err := os.Lstat(appDir); !errors.Is(err, os.ErrNotExist) {
		return AppRef{}, fmt.Errorf("application directory already exists or is inaccessible: %s", appDir)
	}
	// Track only directories created by this request; rollback removes empty directories only.
	var created []string
	mkdir := func(target string) error {
		var missing []string
		for p := target; ; p = filepath.Dir(p) {
			if _, err := os.Stat(p); err == nil {
				break
			} else if !os.IsNotExist(err) {
				return err
			}
			missing = append(missing, p)
		}
		for i := len(missing) - 1; i >= 0; i-- {
			if err := os.Mkdir(missing[i], 0755); err != nil {
				return err
			}
			created = append(created, missing[i])
		}
		return nil
	}
	success := false
	defer func() {
		if !success {
			for i := len(created) - 1; i >= 0; i-- {
				_ = os.Remove(created[i])
			}
		}
	}()
	if err := mkdir(filepath.Join(appDir, pathutil.Native(effective.SourceDirectory))); err != nil {
		return AppRef{}, err
	}
	if err := config.SaveApp(filepath.Join(appDir, AppConfigFile), &cfg); err != nil {
		return AppRef{}, err
	}
	success = true
	return AppRef{Path: appDir, Relative: w.Relative(appDir)}, nil
}
func ValidAppName(name string) bool { return pathutil.ValidName(name) }
func ValidGroupName(group string) bool {
	return group == "" || (group != "." && pathutil.ValidRelative(group))
}
