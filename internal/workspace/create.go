package workspace

import (
	"errors"
	"fmt"
	"io"
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
	// SetupFrom optionally copies one existing installer into the new source directory.
	SetupFrom       string
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
	if options.SetupFrom != "" && options.SetupFile == "" {
		options.SetupFile = filepath.Base(options.SetupFrom)
	}
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
	var installer *os.File
	if options.SetupFrom != "" {
		info, err := os.Stat(options.SetupFrom)
		if err != nil {
			return AppRef{}, fmt.Errorf("inspect setup file %q: %w", options.SetupFrom, err)
		}
		if !info.Mode().IsRegular() {
			return AppRef{}, errors.New("setup source must be a regular file")
		}
		installer, err = os.Open(options.SetupFrom)
		if err != nil {
			return AppRef{}, fmt.Errorf("open setup file %q: %w", options.SetupFrom, err)
		}
		defer installer.Close()
		info, err = installer.Stat()
		if err != nil {
			return AppRef{}, fmt.Errorf("inspect setup file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return AppRef{}, errors.New("setup source must be a regular file")
		}
	}
	// Track only directories created by this request; rollback removes empty directories only.
	var created []string
	var copied string
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
			if copied != "" {
				_ = os.Remove(copied)
			}
			for i := len(created) - 1; i >= 0; i-- {
				_ = os.Remove(created[i])
			}
		}
	}()
	if err := mkdir(filepath.Join(appDir, pathutil.Native(effective.SourceDirectory))); err != nil {
		return AppRef{}, err
	}
	if installer != nil {
		destination := filepath.Join(appDir, pathutil.Native(effective.SourceDirectory), pathutil.Native(options.SetupFile))
		if strings.EqualFold(destination, filepath.Join(appDir, AppConfigFile)) {
			return AppRef{}, errors.New("setup file must not replace the application configuration")
		}
		if err := pathutil.Within(appDir, destination); err != nil {
			return AppRef{}, err
		}
		if err := mkdir(filepath.Dir(destination)); err != nil {
			return AppRef{}, err
		}
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return AppRef{}, fmt.Errorf("create setup copy: %w", err)
		}
		copied = destination
		_, err = io.Copy(output, installer)
		if err == nil {
			err = output.Sync()
		}
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return AppRef{}, fmt.Errorf("copy setup file: %w", err)
		}
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
