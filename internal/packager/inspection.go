package packager

import (
	"context"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/workspace"
)

// InspectionContext shares local cache/evidence reads and utility validation for
// one inventory operation. It is not safe for concurrent use. Discard it after
// the operation so later refreshes observe fresh local state.
type InspectionContext struct {
	service   Service
	cache     *buildcache.Cache
	cacheErr  error
	toolErr   error
	evidence  *buildcache.InspectionEvidence
	directory string
}

func (s Service) NewInspectionContext(ctx context.Context) *InspectionContext {
	i := &InspectionContext{service: s, evidence: &buildcache.InspectionEvidence{Schema: 1, Apps: make(map[string]buildcache.EvidenceEntry)}}
	if s.Workspace == nil {
		return i
	}
	if err := ctx.Err(); err != nil {
		i.cacheErr = err
		return i
	}
	i.toolErr = workspace.EnsureFile(s.utilityPath(), "Content Prep Tool")
	i.directory, i.cacheErr = s.Workspace.CacheDirectory()
	if i.cacheErr == nil {
		i.cache, i.cacheErr = buildcache.Load(i.directory)
		// Evidence is disposable. Missing/corrupt evidence requires deep inspection,
		// but must not hide successful cache records or break authoritative reads.
		if evidence, err := buildcache.LoadEvidence(ctx, i.directory); err == nil {
			i.evidence = evidence
		}
	}
	return i
}

// InspectBuildStateForApp never reloads application configuration. Fast mode is
// a metadata-only hint and returns unknown when no matching deep evidence exists.
// Deep mode always hashes source and tool contents; neither mode writes files.
func (i *InspectionContext) InspectBuildStateForApp(ctx context.Context, app workspace.App, deep bool) BuildInspection {
	return i.service.inspectAppBuildState(ctx, app, i, deep)
}

// SaveEvidence explicitly persists deep-inspection hints, never build records.
func (i *InspectionContext) SaveEvidence(ctx context.Context) error {
	if i.cacheErr != nil {
		return i.cacheErr
	}
	if i.directory == "" {
		return nil
	}
	return i.evidence.Save(ctx, i.directory)
}

// recordBuildEvidence is best effort: disposable hints must never turn a
// successful build into a failure. Failed/no-cache builds never reach this path.
func (s Service) recordBuildEvidence(ctx context.Context, evidence *buildcache.InspectionEvidence, directory string, app workspace.App, output string, before buildcache.Metadata, fingerprint string) {
	if evidence == nil {
		return
	}
	after, err := buildcache.MetadataDigest(ctx, app.Ref.Path, &app.Effective.AppConfig, output, s.utilityPath())
	if err != nil {
		return
	}
	evidence.Record(app.Config.ID, before, after, fingerprint)
	_ = evidence.Save(ctx, directory)
}

// InspectBuildStateForApp is a convenience for a single already loaded app.
func (s Service) InspectBuildStateForApp(ctx context.Context, app workspace.App, deep bool) BuildInspection {
	return s.NewInspectionContext(ctx).InspectBuildStateForApp(ctx, app, deep)
}
