// Package pathopener provides shell-free, asynchronous directory opening.
package pathopener

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type PathOpener interface{ OpenDirectory(string) error }
type Func func(string) error

func (f Func) OpenDirectory(path string) error { return f(path) }

type Explorer struct{ Start func(string, ...string) error }

func (e Explorer) OpenDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("Folder unavailable: no directory selected")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("Folder unavailable: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("Folder unavailable: %s: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("Folder unavailable: %s is not a directory", abs)
	}
	start := e.Start
	if start == nil {
		if runtime.GOOS != "windows" {
			return errors.New("Open folder requires an interactive Windows desktop")
		}
		start = func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			if err := cmd.Start(); err != nil {
				return err
			}
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}
	if err := start("explorer.exe", abs); err != nil {
		return fmt.Errorf("Could not open folder: Windows Explorer could not be started: %w", err)
	}
	return nil
}
