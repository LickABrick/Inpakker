package packager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
)

func TestFastInspectionMissingArtifactWithoutEvidence(t *testing.T) {
	ws, ref, _ := inspectionWorkspace(t)
	app := ws.Inspect(ref)
	cache, err := buildcache.Load(cacheDirectory(t, ws))
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(ref.Path, "output", "missing.intunewin")
	if err := cache.Set(app.Config.ID, "previous-build", ref.Path, []string{artifact}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(cacheDirectory(t, ws)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	i := (Service{Workspace: ws}).NewInspectionContext(ctx)
	got := i.InspectBuildStateForApp(ctx, app, false)
	if got.State != BuildStateNeedsBuild || got.Reason != "recorded build output is missing" {
		t.Fatalf("inspection = %#v", got)
	}
}

func TestInspectionRecoversCorruptEvidence(t *testing.T) {
	ws, ref, _ := inspectionWorkspace(t)
	ctx := context.Background()
	directory := cacheDirectory(t, ws)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, buildcache.EvidenceFileName), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	i := (Service{Workspace: ws}).NewInspectionContext(ctx)
	i.evidence.Schema = 0 // also exercise a zero-value fallback/constructor.
	app := ws.Inspect(ref)
	i.InspectBuildStateForApp(ctx, app, true)
	if err := i.SaveEvidence(ctx); err != nil {
		t.Fatal(err)
	}
	e, err := buildcache.LoadEvidence(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Apps) != 1 {
		t.Fatalf("recovered evidence = %#v", e)
	}
}
