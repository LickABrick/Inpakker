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
	"github.com/LickABrick/inpakker/internal/pathutil"
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
	BuildStateCurrent           BuildState = "current"
	BuildStateNeedsBuild        BuildState = "needs_build"
	BuildStateNotBuilt          BuildState = "not_built"
	BuildStateUnavailable       BuildState = "unavailable"
	BuildStateUnknown           BuildState = "unknown"
	BuildStateNeedsVerification BuildState = "needs_verification"
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
	// Diagnostics receives output even when ShowToolOutput is disabled.
	// Callers should use a bounded, concurrency-safe writer.
	Diagnostics io.Writer
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
		return errors.New("Content Prep Tool is not configured; use 'inpakker tools detect' or 'tools install content-prep'")
	}
	return workspace.EnsureFile(path, "Content Prep Tool")
}

func (s Service) Build(ctx context.Context, refs []workspace.AppRef, options BuildOptions, stdout, stderr io.Writer) ([]Result, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	cacheDirectory, err := s.Workspace.CacheDirectory()
	if err != nil {
		return nil, err
	}
	var cache *buildcache.Cache
	if !options.NoCache {
		cache, err = buildcache.Load(cacheDirectory)
		if err != nil {
			return nil, err
		}
	}
	var evidence *buildcache.InspectionEvidence
	if cache != nil {
		evidence, _ = buildcache.LoadEvidence(ctx, cacheDirectory)
	}
	results := make([]Result, 0, len(refs))
	for index, ref := range refs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		app := s.Workspace.InspectContext(ctx, ref)
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
		fingerprint := ""
		var before buildcache.Metadata
		if evidence != nil {
			before, _ = buildcache.MetadataDigest(ctx, ref.Path, &app.Effective.AppConfig, outputPath, s.utilityPath())
		}
		if cache != nil {
			fingerprint, err = buildcache.FingerprintContext(ctx, ref.Path, &app.Effective.AppConfig, outputPath, s.utilityPath())
		}
		if err != nil {
			result.Status, result.Err = StatusFailed, err
			results = append(results, result)
			continue
		}
		cacheKey := app.Config.ID
		if !options.Force && cache != nil && cache.Current(cacheKey, fingerprint, ref.Path) {
			result.Status, result.Reason = StatusCurrent, "inputs and output are unchanged"
			result.Artifacts = append([]string(nil), app.Packages...)
			s.recordBuildEvidence(ctx, evidence, cacheDirectory, app, outputPath, before, fingerprint)
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
		if s.Workspace.User.Preferences.ShowToolOutput {
			processOut, processErr = stdout, stderr
		}
		if options.Diagnostics != nil {
			processOut = diagnosticWriter(options.Diagnostics, processOut)
			processErr = diagnosticWriter(options.Diagnostics, processErr)
		}
		err = s.Runner.Run(ctx, s.utilityPath(), []string{
			"-c", filepath.Join(ref.Path, pathutil.Native(app.Effective.SourceDirectory)),
			"-s", app.Config.SetupFile,
			"-o", outputPath,
			"-q",
		}, processOut, processErr)
		if err != nil {
			result.Status, result.Err = StatusFailed, fmt.Errorf("package application: %w", err)
			results = append(results, result)
			if ctx.Err() != nil {
				return results, ctx.Err()
			}
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
			if err := cache.Save(cacheDirectory); err != nil {
				return results, err
			}
			s.recordBuildEvidence(ctx, evidence, cacheDirectory, app, outputPath, before, fingerprint)
		}
	}
	return results, nil
}

func diagnosticWriter(diagnostics, visible io.Writer) io.Writer {
	if visible == nil {
		return diagnostics
	}
	return io.MultiWriter(diagnostics, visible)
}

// InspectBuildState calculates the same fingerprint and cache state used by a
// build, without creating directories, invoking the packaging utility,
// or writing the cache.
func (s Service) InspectBuildState(ref workspace.AppRef) BuildInspection {
	if s.Workspace == nil {
		return BuildInspection{State: BuildStateUnknown, Reason: "workspace is unavailable"}
	}
	return s.InspectAppBuildState(s.Workspace.Inspect(ref))
}

// InspectAppBuildState reuses a loaded application for authoritative, read-only
// inspection. Use an InspectionContext to share cache reads across an inventory.
func (s Service) InspectAppBuildState(app workspace.App) BuildInspection {
	inspection := s.NewInspectionContext(context.Background())
	return inspection.InspectBuildStateForApp(context.Background(), app, true)
}

func (s Service) inspectAppBuildState(ctx context.Context, app workspace.App, inspection *InspectionContext, deep bool) BuildInspection {
	ref := app.Ref
	if s.Workspace == nil {
		return BuildInspection{State: BuildStateUnknown, Reason: "workspace is unavailable"}
	}
	if err := ctx.Err(); err != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
	}
	if app.Status != "valid" || app.Config == nil || app.Effective == nil {
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
	if inspection.toolErr != nil {
		return BuildInspection{State: BuildStateUnavailable, Reason: inspection.toolErr.Error()}
	}
	outputPath, err := s.Workspace.OutputDir(ref, app.Config)
	if err != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
	}
	if inspection.cacheErr != nil {
		return BuildInspection{State: BuildStateUnknown, Reason: inspection.cacheErr.Error()}
	}
	cache := inspection.cache
	key := app.Config.ID
	before, metadataErr := buildcache.MetadataDigest(ctx, ref.Path, &app.Effective.AppConfig, outputPath, utility)
	var fingerprint string
	if deep {
		fingerprint, err = buildcache.FingerprintContext(ctx, ref.Path, &app.Effective.AppConfig, outputPath, utility)
		if err != nil {
			return BuildInspection{State: BuildStateUnknown, Reason: err.Error()}
		}
		if metadataErr == nil {
			after, afterErr := buildcache.MetadataDigest(ctx, ref.Path, &app.Effective.AppConfig, outputPath, utility)
			if afterErr == nil {
				inspection.evidence.Record(key, before, after, fingerprint)
			}
		}
	} else {
		if metadataErr != nil {
			return BuildInspection{State: BuildStateUnknown, Reason: metadataErr.Error()}
		}
		var ok bool
		fingerprint, ok = inspection.evidence.Lookup(key, before)
		if !ok {
			if _, built := cache.Get(key); !built {
				return BuildInspection{State: BuildStateNotBuilt, Reason: "no successful cached build"}
			}
			return BuildInspection{State: BuildStateNeedsVerification, Reason: "input metadata changed or deep inspection required"}
		}
	}
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

func (s Service) utilityPath() string { return s.Workspace.User.Tools.ContentPrepTool.Path }

func emit(options BuildOptions, event Event) {
	if options.OnProgress != nil {
		options.OnProgress(event)
	}
}
