package workspace

import (
	"github.com/LickABrick/inpakker/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeWorkspace(t *testing.T, name string) *Workspace {
	t.Helper()
	ws, err := CreateWorkspace(CreateWorkspaceOptions{Root: filepath.Join(t.TempDir(), name), Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return ws
}
func TestWorkspaceLifecycleAndResolution(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	t.Setenv("INPAKKER_WORKSPACE", "")
	outside := t.TempDir()
	if _, err := Resolve("", outside); err != ErrNoWorkspace {
		t.Fatal(err)
	}
	a := makeWorkspace(t, "ADS Groep")
	b := makeWorkspace(t, "Hencon")
	check := func(selector, cwd, id string) {
		t.Helper()
		ws, err := Resolve(selector, cwd)
		if err != nil || ws.Config.ID != id {
			t.Fatalf("resolve %q from %q: %v %#v", selector, cwd, err, ws)
		}
	}
	check("", outside, b.Config.ID)
	nested := filepath.Join(a.AppsDir(), "Browsers", "firefox", "source")
	os.MkdirAll(nested, 0755)
	check("", nested, a.Config.ID)
	t.Setenv("INPAKKER_WORKSPACE", "Hencon")
	check("", nested, b.Config.ID)
	check("ADS Groep", nested, a.Config.ID)
	t.Setenv("INPAKKER_WORKSPACE", "")
	if _, err := Use("ADS Groep"); err != nil {
		t.Fatal(err)
	}
	check("", outside, a.Config.ID)
	if _, err := Add(a.Root); err == nil {
		t.Fatal("duplicate add")
	}
	if err := Remove("ADS Groep"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.Root, ConfigFile)); err != nil {
		t.Fatal("remove deleted workspace", err)
	}
	check("", nested, a.Config.ID) // Unregistered discovery.
	if _, err := Add(a.Root); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(a.Root, moved); err != nil {
		t.Fatal(err)
	}
	if err := Relink(filepath.Base(a.Root), b.Root); err == nil {
		t.Fatal("wrong UUID accepted")
	}
	if err := Relink(filepath.Base(a.Root), moved); err != nil {
		t.Fatal(err)
	}
	check("ADS Groep", outside, a.Config.ID)
	if _, err := CreateWorkspace(CreateWorkspaceOptions{Root: t.TempDir(), Name: "ads groep"}); err == nil {
		t.Fatal("duplicate name")
	}
}
func TestCreateApplicationInheritanceGroupsAndSafety(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Customer Apps")
	ref, err := ws.Create(CreateOptions{Name: "Mozilla Firefox", Group: `Customer Apps\Legacy`, SetupFile: "Firefox Setup.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Relative != filepath.Join("Customer Apps", "Legacy", "mozilla-firefox") {
		t.Fatal(ref)
	}
	app := ws.Inspect(ref)
	if app.Config == nil || !config.ValidUUID(app.Config.ID) || app.Config.SourceDirectory != "" || app.Config.OutputDirectory != "" {
		t.Fatal(app)
	}
	if app.Effective.SourceDirectory != "source" {
		t.Fatal(app.Effective)
	}
	groups, err := ws.Groups()
	if err != nil || len(groups) != 2 {
		t.Fatal(groups, err)
	}
	os.MkdirAll(filepath.Join(ref.Path, "source", "not a group"), 0755)
	groups, _ = ws.Groups()
	if len(groups) != 2 {
		t.Fatal(groups)
	}
	if _, err := ws.Create(CreateOptions{Name: "Again", DirectoryName: "mozilla-firefox", Group: "Customer Apps/Legacy", SetupFile: "x"}); err == nil {
		t.Fatal("overwrite")
	}
	for _, options := range []CreateOptions{{Name: "bad", DirectoryName: "CON", SetupFile: "x"}, {Name: "bad", Group: "../escape", SetupFile: "x"}, {Name: "bad", SourceDirectory: "../escape", SetupFile: "x"}, {Name: "bad", SetupFile: ""}} {
		if _, err := ws.Create(options); err == nil {
			t.Fatal(options)
		}
	}
	found, err := ws.Find("Mozilla Firefox")
	if err != nil || found.Path != ref.Path {
		t.Fatal(found, err)
	}
	_, err = ws.Create(CreateOptions{Name: "Mozilla Firefox", DirectoryName: "another", SetupFile: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Find("Mozilla Firefox"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
	if Slug("Microsoft 365 Apps") != "microsoft-365-apps" || Slug("7-Zip") != "7-zip" {
		t.Fatal("slug")
	}
}

func TestResolverAmbiguousNamesAndMissingRegistry(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	t.Setenv("INPAKKER_WORKSPACE", "")
	a := makeWorkspace(t, "First")
	b := makeWorkspace(t, "Second")
	b.Config.Name = "first"
	if err := config.SaveWorkspace(filepath.Join(b.Root, ConfigFile), &b.Config); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("First", t.TempDir()); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
	if err := os.Rename(a.Root, a.Root+" moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("First", t.TempDir()); err == nil {
		t.Fatal("missing registration silently fell through")
	}
}
