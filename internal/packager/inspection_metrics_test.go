package packager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
)

func TestFastInspectionDoesNotHashContents(t *testing.T) {
	ws, ref, utility := inspectionWorkspace(t)
	payload := make([]byte, 4*1024*1024)
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "payload.bin"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	app := ws.Inspect(ref)
	output, err := ws.OutputDir(ref, app.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(output, "browser.intunewin")
	if err := os.WriteFile(artifact, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := buildcache.Fingerprint(ref.Path, &app.Effective.AppConfig, output, utility)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := buildcache.Load(cacheDirectory(t, ws))
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(app.Config.ID, fingerprint, ref.Path, []string{artifact}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(cacheDirectory(t, ws)); err != nil {
		t.Fatal(err)
	}
	service := Service{Workspace: ws}
	checkFast := func(t *testing.T, want BuildState) {
		t.Helper()
		metrics := &buildcache.Metrics{}
		ctx := buildcache.WithMetrics(context.Background(), metrics)
		inspection := service.NewInspectionContext(ctx)
		if got := inspection.InspectBuildStateForApp(ctx, app, false); got.State != want {
			t.Fatalf("fast state: %#v, want %s", got, want)
		}
		if metrics.SourceBytes.Load() != 0 || metrics.SourceHashes.Load() != 0 || metrics.ToolBytes.Load() != 0 || metrics.ToolHashes.Load() != 0 {
			t.Fatalf("fast inspection read contents: source %d bytes/%d hashes, tool %d bytes/%d hashes", metrics.SourceBytes.Load(), metrics.SourceHashes.Load(), metrics.ToolBytes.Load(), metrics.ToolHashes.Load())
		}
		if metrics.SourceFileInfos.Load() == 0 {
			t.Fatal("fast metadata work was not observed")
		}
	}
	t.Run("cold", func(t *testing.T) { checkFast(t, BuildStateNeedsVerification) })
	metrics := &buildcache.Metrics{}
	ctx := buildcache.WithMetrics(context.Background(), metrics)
	inspection := service.NewInspectionContext(ctx)
	if got := inspection.InspectBuildStateForApp(ctx, app, true); got.State != BuildStateCurrent {
		t.Fatalf("authoritative state: %#v", got)
	}
	// Assert exact fixture byte counts so missing instrumentation cannot make
	// the zero-read regression pass vacuously.
	if got, want := metrics.SourceBytes.Load(), int64(len(payload)+len("installer-v1")); got != want {
		t.Fatalf("source hash bytes = %d, want %d", got, want)
	}
	if metrics.SourceHashes.Load() != 2 || metrics.ToolHashes.Load() != 1 || metrics.ToolBytes.Load() != int64(len("utility-v1")) {
		t.Fatalf("authoritative counters: source hashes %d, tool hashes %d, tool bytes %d", metrics.SourceHashes.Load(), metrics.ToolHashes.Load(), metrics.ToolBytes.Load())
	}
	if err := inspection.SaveEvidence(ctx); err != nil {
		t.Fatal(err)
	}
	t.Run("warm", func(t *testing.T) { checkFast(t, BuildStateCurrent) })
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "payload.bin"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("changed", func(t *testing.T) { checkFast(t, BuildStateNeedsVerification) })
}
