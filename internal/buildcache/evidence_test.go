package buildcache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/types"
)

func evidenceFixture(t *testing.T) (string, *types.AppConfig, string, string) {
	t.Helper()
	root := t.TempDir()
	app := filepath.Join(root, "app")
	writeFile(t, filepath.Join(app, "source", "setup.ps1"), "first")
	tool := filepath.Join(root, "utility.exe")
	writeFile(t, tool, "utility")
	return app, &types.AppConfig{Name: "app", SourceDirectory: "source", SetupFile: "setup.ps1", OutputDirectory: "source/output"}, filepath.Join(app, "source", "output"), tool
}

func TestMetadataEvidenceLifecycle(t *testing.T) {
	ctx := context.Background()
	app, cfg, output, tool := evidenceFixture(t)
	state := t.TempDir()
	e, err := LoadEvidence(ctx, state)
	if err != nil { t.Fatal(err) }
	metadata, err := MetadataDigest(ctx, app, cfg, output, tool)
	if err != nil { t.Fatal(err) }
	if metadata.Files != 1 { t.Fatalf("files = %d", metadata.Files) }
	if _, ok := e.Lookup("app", metadata); ok { t.Fatal("empty evidence matched") }
	fingerprint, err := FingerprintContext(ctx, app, cfg, output, tool)
	if err != nil { t.Fatal(err) }
	if !e.Record("app", metadata, metadata, fingerprint) { t.Fatal("record failed") }
	if err := e.Save(ctx, state); err != nil { t.Fatal(err) }
	loaded, err := LoadEvidence(ctx, state)
	if err != nil { t.Fatal(err) }
	if got, ok := loaded.Lookup("app", metadata); !ok || got != fingerprint { t.Fatalf("lookup = %q, %v", got, ok) }
	if _, err := os.Stat(filepath.Join(state, FileName)); !errors.Is(err, os.ErrNotExist) { t.Fatal("evidence touched build cache") }
	writeFile(t, filepath.Join(output, "app.intunewin"), "output")
	writeFile(t, filepath.Join(app, "source", ".git", "index"), "ignored")
	unchanged, err := MetadataDigest(ctx, app, cfg, output, tool)
	if err != nil { t.Fatal(err) }
	if unchanged != metadata { t.Fatal("excluded files changed metadata") }
	writeFile(t, filepath.Join(app, "source", "new.txt"), "new")
	changed, err := MetadataDigest(ctx, app, cfg, output, tool)
	if err != nil { t.Fatal(err) }
	if _, ok := loaded.Lookup("app", changed); ok { t.Fatal("changed metadata matched") }
	if loaded.Record("app", metadata, changed, fingerprint) { t.Fatal("racy snapshot recorded") }
	if _, ok := loaded.Lookup("app", metadata); ok { t.Fatal("old evidence was not invalidated") }
}

func TestAuthoritativeFingerprintDetectsSameSizeAndMtimeEdits(t *testing.T) {
	for _, target := range []string{"source", "utility"} {
		t.Run(target, func(t *testing.T) {
			ctx := context.Background()
			app, cfg, output, tool := evidenceFixture(t)
			path := filepath.Join(app, "source", "setup.ps1")
			value := "other"
			if target == "utility" { path, value = tool, "changed" }
			info, err := os.Stat(path)
			if err != nil { t.Fatal(err) }
			before, err := MetadataDigest(ctx, app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			first, err := Fingerprint(app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			writeFile(t, path, value)
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil { t.Fatal(err) }
			after, err := MetadataDigest(ctx, app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			if before != after { t.Fatal("fixture did not preserve metadata") }
			second, err := FingerprintContext(ctx, app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			if first == second { t.Fatal("authoritative fingerprint trusted metadata instead of contents") }
		})
	}
}

func TestMetadataTracksInputs(t *testing.T) {
	for _, change := range []string{"config", "utility", "remove", "rename", "output", "mtime"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			app, cfg, output, tool := evidenceFixture(t)
			before, err := MetadataDigest(ctx, app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			path := filepath.Join(app, "source", "setup.ps1")
			switch change {
			case "config": cfg.Name = "renamed"
			case "utility": writeFile(t, tool, "larger utility")
			case "remove": err = os.Remove(path)
			case "rename": err = os.Rename(path, filepath.Join(app, "source", "renamed.ps1"))
			case "output": output = filepath.Join(app, "different-output")
			case "mtime":
				info, statErr := os.Stat(path)
				if statErr != nil { t.Fatal(statErr) }
				err = os.Chtimes(path, info.ModTime(), info.ModTime().Add(1000000000))
			}
			if err != nil { t.Fatal(err) }
			after, err := MetadataDigest(ctx, app, cfg, output, tool)
			if err != nil { t.Fatal(err) }
			if before.Digest == after.Digest { t.Fatal("input change did not change digest") }
		})
	}
}

func TestMetadataRejectsUnsafeSource(t *testing.T) {
	for _, kind := range []string{"traversal", "root-link", "file-link", "directory-link", "reserved-name"} {
		t.Run(kind, func(t *testing.T) {
			app, cfg, output, tool := evidenceFixture(t)
			switch kind {
			case "traversal": cfg.SourceDirectory = "../outside"
			case "root-link":
				cfg.SourceDirectory = "linked"
				if err := os.Symlink(filepath.Join(app, "source"), filepath.Join(app, "linked")); err != nil { t.Skip(err) }
			case "file-link", "directory-link":
				target := tool
				if kind == "directory-link" { target = filepath.Dir(app) }
				if err := os.Symlink(target, filepath.Join(app, "source", "linked")); err != nil { t.Skip(err) }
			case "reserved-name": writeFile(t, filepath.Join(app, "source", "CON"), "unsafe")
			}
			if _, err := MetadataDigest(context.Background(), app, cfg, output, tool); err == nil { t.Fatal("unsafe input accepted") }
		})
	}
}

func TestEvidenceRejectsInvalidPersistence(t *testing.T) {
	for _, data := range []string{"{", `{"schemaVersion":2,"apps":{}}`, `{"schemaVersion":1}`} {
		t.Run(data, func(t *testing.T) {
			state := t.TempDir()
			writeFile(t, filepath.Join(state, EvidenceFileName), data)
			if _, err := LoadEvidence(context.Background(), state); err == nil { t.Fatal("invalid evidence accepted") }
		})
	}
}

func TestEvidenceCancellationDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app, cfg, output, tool := evidenceFixture(t)
	if _, err := MetadataDigest(ctx, app, cfg, output, tool); !errors.Is(err, context.Canceled) { t.Fatalf("metadata: %v", err) }
	if _, err := FingerprintContext(ctx, app, cfg, output, tool); !errors.Is(err, context.Canceled) { t.Fatalf("fingerprint: %v", err) }
	state := t.TempDir()
	if _, err := LoadEvidence(ctx, state); !errors.Is(err, context.Canceled) { t.Fatalf("load: %v", err) }
	e := &InspectionEvidence{Schema: evidenceSchema, Apps: map[string]EvidenceEntry{}}
	if err := e.Save(ctx, state); !errors.Is(err, context.Canceled) { t.Fatalf("save: %v", err) }
	if _, err := os.Stat(filepath.Join(state, EvidenceFileName)); !errors.Is(err, os.ErrNotExist) { t.Fatal("canceled save wrote evidence") }
}

func TestContextReaderStopsDuringHashing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := contextReader{ctx: ctx, reader: strings.NewReader("contents")}
	if n, err := r.Read(make([]byte, 1)); n != 1 || err != nil { t.Fatalf("first read = %d, %v", n, err) }
	cancel()
	if n, err := r.Read(make([]byte, 1)); n != 0 || !errors.Is(err, context.Canceled) { t.Fatalf("canceled read = %d, %v", n, err) }
}

func BenchmarkInspection(b *testing.B) {
	for _, files := range []int{10, 1000} {
		b.Run(fmt.Sprintf("files-%d", files), func(b *testing.B) {
			app := b.TempDir()
			source := filepath.Join(app, "source")
			if err := os.Mkdir(source, 0700); err != nil { b.Fatal(err) }
			const size = 64 * 1024
			data := make([]byte, size)
			for i := 0; i < files; i++ {
				if err := os.WriteFile(filepath.Join(source, fmt.Sprintf("file-%04d", i)), data, 0600); err != nil { b.Fatal(err) }
			}
			tool := filepath.Join(app, "utility")
			if err := os.WriteFile(tool, data, 0600); err != nil { b.Fatal(err) }
			cfg := &types.AppConfig{SourceDirectory: "source"}
			output := filepath.Join(app, "output")
			ctx := context.Background()
			b.Run("metadata", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(0, "content-bytes/op")
				for i := 0; i < b.N; i++ {
					metadata, err := MetadataDigest(ctx, app, cfg, output, tool)
					if err != nil { b.Fatal(err) }
					b.ReportMetric(float64(metadata.Files), "source-files/op")
				}
			})
			b.Run("authoritative", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64((files+1)*size), "content-bytes/op")
				b.ReportMetric(float64(files), "source-files/op")
				for i := 0; i < b.N; i++ {
					if _, err := FingerprintContext(ctx, app, cfg, output, tool); err != nil { b.Fatal(err) }
				}
			})
		})
	}
}
