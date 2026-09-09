package buildcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/types"
)

func TestFingerprintTracksInputsAndExcludesOutput(t *testing.T) {
	root := t.TempDir()
	appPath := filepath.Join(root, "app")
	sourcePath := filepath.Join(appPath, "source")
	outputPath := filepath.Join(sourcePath, "output")
	utilityPath := filepath.Join(root, "IntuneWinAppUtil.exe")
	writeFile(t, filepath.Join(sourcePath, "setup.ps1"), "first")
	writeFile(t, filepath.Join(outputPath, "old.intunewin"), "old output")
	writeFile(t, utilityPath, "utility")
	cfg := &types.AppConfig{Name: "app", DisplayName: "App", Source: "source", SetupFile: "setup.ps1", OutputDir: "source/output"}

	first, err := Fingerprint(appPath, cfg, outputPath, utilityPath)
	if err != nil {
		t.Fatalf("Fingerprint returned %v", err)
	}
	writeFile(t, filepath.Join(outputPath, "old.intunewin"), "changed output")
	second, err := Fingerprint(appPath, cfg, outputPath, utilityPath)
	if err != nil {
		t.Fatalf("Fingerprint after output change returned %v", err)
	}
	if first != second {
		t.Fatal("output change modified fingerprint")
	}
	writeFile(t, filepath.Join(sourcePath, "setup.ps1"), "second")
	third, err := Fingerprint(appPath, cfg, outputPath, utilityPath)
	if err != nil {
		t.Fatalf("Fingerprint after source change returned %v", err)
	}
	if second == third {
		t.Fatal("source change did not modify fingerprint")
	}
}

func TestCacheRequiresMatchingFingerprintAndArtifacts(t *testing.T) {
	root := t.TempDir()
	appPath := filepath.Join(root, "app")
	artifact := filepath.Join(appPath, "output", "app.intunewin")
	writeFile(t, artifact, "package")
	cache := empty()
	if err := cache.Set("app", "fingerprint", appPath, []string{artifact}); err != nil {
		t.Fatal(err)
	}
	if !cache.Current("app", "fingerprint", appPath) {
		t.Fatal("matching cache entry is not current")
	}
	if cache.Current("app", "changed", appPath) {
		t.Fatal("changed fingerprint is current")
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if cache.Current("app", "fingerprint", appPath) {
		t.Fatal("missing artifact is current")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	cache := empty()
	cache.Apps["app"] = Entry{Fingerprint: "abc", Artifacts: []string{"output/app.intunewin"}}
	if err := cache.Save(root); err != nil {
		t.Fatalf("Save returned %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}
	if loaded.Apps["app"].Fingerprint != "abc" {
		t.Fatalf("loaded cache = %#v", loaded)
	}
}

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
