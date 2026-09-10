package packager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type Service struct {
	Workspace *workspace.Workspace
	Runner    process.Runner
}

type ResultStatus string

const (
	StatusBuilt   ResultStatus = "built"
	StatusCurrent ResultStatus = "current"
	StatusFailed  ResultStatus = "failed"
)

type Result struct {
	App       workspace.App
	Status    ResultStatus
	Reason    string
	Artifacts []string
	Err       error
}

type BuildState string

const (
	BuildStateCurrent     BuildState = "current"
	BuildStateNeedsBuild  BuildState = "needs_build"
	BuildStateNotBuilt    BuildState = "not_built"
	BuildStateUnavailable BuildState = "unavailable"
	BuildStateUnknown     BuildState = "unknown"
)

// BuildInspection describes the current packaging state without mutating the
// workspace or build cache.
type BuildInspection struct {
	State     BuildState
	Reason    string
	Artifacts []string
}

type Event struct {
	Index int
	Total int
	App   string
	Phase string
}

type BuildOptions struct {
	Force      bool
	NoCache    bool
	OnProgress func(Event)
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

func (s Service) Build(ctx context.Context, refs []workspace.AppRef, options BuildOptions, stdout, stderr io.Writer) ([]Result, error) {
	var cache *buildcache.Cache
	var err error
	if !options.NoCache {
		cache, err = buildcache.Load(s.Workspace.Root)
		if err != nil {
			return nil, err
		}
	}
	results := make([]Result, 0, len(refs))
	for index, ref := range refs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		app := s.Workspace.Inspect(ref)
		result := Result{App: app}
		emit(options, Event{Index: index + 1, Total: len(refs), App: app.Label(), Phase: "checking"})
		if app.Status != "valid" {
			result.Status, result.Err = StatusFailed, errors.New(app.Error)
			results = append(results, result)
			continue
		}
		outputPath, err := s.Workspace.OutputDir(ref, app.Config)
		if err != nil {
			result.Status, result.Err = StatusFailed, err
			results = append(results, result)
			continue
		}
		fingerprint, err := buildcache.Fingerprint(ref.Path, app.Config, outputPath, s.utilityPath())
		if err != nil {
			result.Status, result.Err = StatusFailed, err
			results = append(results, result)
			continue
		}
		cacheKey := filepath.ToSlash(ref.Relative)
		if !options.Force && cache != nil && cache.Current(cacheKey, fingerprint, ref.Path) {
			result.Status, result.Reason = StatusCurrent, "inputs and output are unchanged"
			result.Artifacts = append([]string(nil), app.Packages...)
			results = append(results, result)
			continue
		}
		if err := os.MkdirAll(outputPath, 0o755); err != nil {
			result.Status, result.Err = StatusFailed, fmt.Errorf("create output directory: %w", err)
			results = append(results, result)
			continue
		}
		emit(options, Event{Index: index + 1, Total: len(refs), App: app.Label(), Phase: "packaging"})
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
		if err != nil {
			result.Status, result.Err = StatusFailed, fmt.Errorf("package application: %w", err)
			results = append(results, result)
			continue
		}
		artifacts, err := s.Workspace.Packages(ref, app.Config)
		if err != nil {
			result.Status, result.Err = StatusFailed, fmt.Errorf("inspect build output: %w", err)
			results = append(results, result)
			continue
		}
		if len(artifacts) == 0 {
			result.Status, result.Err = StatusFailed, errors.New("packaging utility completed without producing an .intunewin file")
			results = append(results, result)
			continue
		}
		result.Status, result.Artifacts = StatusBuilt, artifacts
		results = append(results, result)
		if cache != nil {
			if err := cache.Set(cacheKey, fingerprint, ref.Path, artifacts); err != nil {
				return results, err
			}
			if err := cache.Save(s.Workspace.Root); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

// InspectBuildState calculates the same fingerprint and cache state used by a
// smart build, without creating directories, invoking the packaging utility,
// or writing the cache.
func (s Service) InspectBuildState(ref workspace.AppRef) BuildInspection {
	if s.Workspace == nil {
		return BuildInspection{State: BuildStateUnknown, Reason: "workspace is unavailable"}
	}
	app := s.Workspace.Inspect(ref)
	if app.Status != "valid" || app.Config == nil {
		reason := app.Error
		if reason == "" {
			reason = "application configuration is invalid"
		}
		return BuildInspection{State: BuildStateUnavailable, Reason: reason}
	}
	utility := s.utilityPath()
	if strings.TrimSpace(utility) == "" {
		return BuildInspection{State: BuildStateUnavailable, Reason: "packaging utility is not configured"}
	}
	if err := workspace.EnsureFile(utility, "intunewinapputil"); err != nil {
		return BuildInspection{State: BuildStateUnavailable, Reason: err.Error()}
	}
	outputPath, err := s.Workspace.OutputDir(ref, app.Config)
	if err != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
	}
	fingerprint, err := buildcache.Fingerprint(ref.Path, app.Config, outputPath, utility)
	if err != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
	}
	cache, err := buildcache.Load(s.Workspace.Root)
	if err != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
	}
	key := filepath.ToSlash(ref.Relative)
	entry, ok := cache.Get(key)
	if !ok {
		return BuildInspection{State: BuildStateNotBuilt, Reason: "no successful cached build"}
	}
	artifacts := make([]string, 0, len(entry.Artifacts))
	for _, artifact := range entry.Artifacts {
		artifacts = append(artifacts, filepath.Join(ref.Path, filepath.FromSlash(artifact)))
	}
	if cache.Current(key, fingerprint, ref.Path) {
		return BuildInspection{State: BuildStateCurrent, Reason: "inputs and output are unchanged", Artifacts: artifacts}
	}
	reason := "recorded build output is missing"
	if entry.Fingerprint != fingerprint {
		reason = "application inputs or packaging utility changed"
	}
	return BuildInspection{State: BuildStateNeedsBuild, Reason: reason, Artifacts: artifacts}
}

func (s Service) utilityPath() string {
	if s.Workspace.Config.IntuneWinAppUtil != "" {
		return s.Workspace.Config.IntuneWinAppUtil
	}
	return s.Workspace.Config.IntuneWinAppUtilPath
}

func emit(options BuildOptions, event Event) {
	if options.OnProgress != nil {
		options.OnProgress(event)
	}
}
