package packager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type Service struct {
	Workspace *workspace.Workspace
	Runner    process.Runner
}

type Result struct {
	App workspace.App
	Err error
}

func (s Service) Validate() error {
	if s.Workspace == nil {
		return errors.New("workspace is required")
	}
	if s.Runner == nil {
		return errors.New("process runner is required")
	}
	path := s.utilityPath()
	if strings.TrimSpace(path) == "" {
		return errors.New("intunewinapputil is not configured")
	}
	return workspace.EnsureFile(path, "intunewinapputil")
}

func (s Service) Build(ctx context.Context, refs []workspace.AppRef, stdout, stderr io.Writer) []Result {
	results := make([]Result, 0, len(refs))
	for _, ref := range refs {
		app := s.Workspace.Inspect(ref)
		result := Result{App: app}
		if app.Status != "valid" {
			result.Err = errors.New(app.Error)
			results = append(results, result)
			continue
		}
		outputPath, err := s.Workspace.OutputDir(ref, app.Config)
		if err == nil {
			err = os.MkdirAll(outputPath, 0o755)
		}
		if err == nil {
			var processOut, processErr io.Writer
			if !s.Workspace.Config.MuteIntuneWinAppUtil {
				processOut, processErr = stdout, stderr
			}
			err = s.Runner.Run(ctx, s.utilityPath(), []string{
				"-c", filepath.Join(ref.Path, app.Config.Source),
				"-s", app.Config.SetupFile,
				"-o", outputPath,
				"-q",
			}, processOut, processErr)
		}
		if err != nil {
			result.Err = fmt.Errorf("package application: %w", err)
		}
		results = append(results, result)
	}
	return results
}

func (s Service) utilityPath() string {
	if s.Workspace.Config.IntuneWinAppUtil != "" {
		return s.Workspace.Config.IntuneWinAppUtil
	}
	return s.Workspace.Config.IntuneWinAppUtilPath
}
