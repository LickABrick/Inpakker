package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/workspace"
)

// benchmarkInventory uses the same isolated workspace/tool fixture as the TUI
// tests. Every app has one real source file and a successful local build record.
// Fixture construction and evidence priming are never included in timings.
func benchmarkInventory(b *testing.B, count int) (*workspace.Workspace, int64) {
	b.Helper()
	ws := testWorkspace(b)
	data := make([]byte, 64*1024)
	for i := 1; i < count; i++ {
		ref, err := ws.Create(workspace.CreateOptions{Name: fmt.Sprintf("app-%04d", i), SetupFile: "install.ps1"})
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ref.Path, "source", "install.ps1"), data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	ctx := context.Background()
	apps, _, err := ws.ListWithGroups(ctx)
	if err != nil {
		b.Fatal(err)
	}
	directory, err := ws.CacheDirectory()
	if err != nil {
		b.Fatal(err)
	}
	cache, err := buildcache.Load(directory)
	if err != nil {
		b.Fatal(err)
	}
	var sourceBytes int64
	for _, app := range apps {
		if app.Status != "valid" {
			b.Fatalf("invalid fixture: %s", app.Error)
		}
		output, err := ws.OutputDir(app.Ref, app.Config)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.MkdirAll(output, 0o700); err != nil {
			b.Fatal(err)
		}
		artifact := filepath.Join(output, "example.intunewin")
		if err := os.WriteFile(artifact, []byte("fixture package"), 0o600); err != nil {
			b.Fatal(err)
		}
		fingerprint, err := buildcache.FingerprintContext(ctx, app.Ref.Path, &app.Effective.AppConfig, output, ws.User.Tools.ContentPrepTool.Path)
		if err != nil {
			b.Fatal(err)
		}
		if err := cache.Set(app.Config.ID, fingerprint, app.Ref.Path, []string{artifact}); err != nil {
			b.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(app.Ref.Path, "source", "install.ps1"))
		if err != nil {
			b.Fatal(err)
		}
		sourceBytes += info.Size()
	}
	if err := cache.Save(directory); err != nil {
		b.Fatal(err)
	}
	return ws, sourceBytes
}

func benchmarkInventorySnapshot(b *testing.B, ws *workspace.Workspace, deep bool) []ApplicationView {
	b.Helper()
	ctx := context.Background()
	apps, _, err := ws.ListWithGroups(ctx)
	if err != nil {
		b.Fatal(err)
	}
	inspection := (packager.Service{Workspace: ws}).NewInspectionContext(ctx)
	views := make([]ApplicationView, 0, len(apps))
	for _, app := range apps {
		state := inspection.InspectBuildStateForApp(ctx, app, deep)
		if state.State == packager.BuildStateUnavailable || state.State == packager.BuildStateUnknown {
			b.Fatalf("unexpected inspection: %#v", state)
		}
		views = append(views, ApplicationView{App: app, Group: filepath.Dir(app.Ref.Relative), Build: state})
	}
	return views
}

func BenchmarkInventorySnapshots(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("apps-%d", count), func(b *testing.B) {
			for _, mode := range []string{"cold", "warm", "one-changed", "authoritative"} {
				b.Run(mode, func(b *testing.B) {
					ws, sourceBytes := benchmarkInventory(b, count)
					ctx := context.Background()
					if mode != "cold" {
						apps, _, err := ws.ListWithGroups(ctx)
						if err != nil {
							b.Fatal(err)
						}
						inspection := (packager.Service{Workspace: ws}).NewInspectionContext(ctx)
						for _, app := range apps {
							if state := inspection.InspectBuildStateForApp(ctx, app, true); state.State != packager.BuildStateCurrent {
								b.Fatalf("prime evidence: %#v", state)
							}
						}
						if err := inspection.SaveEvidence(ctx); err != nil {
							b.Fatal(err)
						}
						if mode == "one-changed" {
							path := filepath.Join(apps[0].Ref.Path, "source", "install.ps1")
							file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
							if err != nil {
								b.Fatal(err)
							}
							_, writeErr := file.Write([]byte("changed"))
							closeErr := file.Close()
							if writeErr != nil || closeErr != nil {
								b.Fatalf("change fixture: write %v, close %v", writeErr, closeErr)
							}
							sourceBytes += int64(len("changed"))
						}
					}
					deep := mode == "authoritative"
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						benchmarkInventorySnapshot(b, ws, deep)
					}
					b.StopTimer()
					// These are logical workload metrics, NOT OS read calls or I/O
					// instrumentation. Sizes come from the actual fixture files.
					// Fast inspection only reads source metadata. Authoritative
					// inspection hashes every source file once; tool/config/cache
					// reads are deliberately excluded from these source-only metrics.
					b.ReportMetric(float64(sourceBytes), "logical-source-bytes/op")
					readBytes, contentReads := int64(0), 0
					if deep {
						readBytes, contentReads = sourceBytes, count
					}
					b.ReportMetric(float64(readBytes), "logical-source-read-bytes/op")
					b.ReportMetric(float64(contentReads), "logical-source-content-reads/op")
				})
			}
		})
	}
}

func BenchmarkInventoryTableRender(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("apps-%d", count), func(b *testing.B) {
			ws, _ := benchmarkInventory(b, count)
			views := benchmarkInventorySnapshot(b, ws, false)
			for _, width := range []int{60, 90, 120} {
				b.Run(fmt.Sprintf("width-%d", width), func(b *testing.B) {
					for _, rebuild := range []bool{false, true} {
						b.Run(fmt.Sprintf("rebuild-%t", rebuild), func(b *testing.B) {
							page := newApplicationsPage(NewTheme(true))
							page.resize(width, 28)
							page.refresh(views, "")
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								if rebuild {
									page.rebuildTable()
								}
								if page.table.View() == "" {
									b.Fatal("empty table")
								}
							}
						})
					}
				})
			}
		})
	}
}
