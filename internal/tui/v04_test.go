package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/LickABrick/inpakker/types"
	"github.com/charmbracelet/x/ansi"
)

func TestBrandingAboutAndFullHeightTable(t *testing.T) {
	m := newTestModel(t)
	top := ansi.Strip(strings.Split(m.shellView(), "\n")[0])
	if !strings.Contains(top, "📦 INPAKKER") || !strings.Contains(top, m.workspace.Config.Name) || strings.Contains(top, m.version) {
		t.Fatal(top)
	}
	updated, _ := m.Update(press("i"))
	m = updated.(Model)
	if !strings.Contains(m.aboutView(100), m.version) {
		t.Fatal("About lacks version")
	}
	if m.folderTarget() != "" {
		t.Fatal("About exposes folder")
	}
	m.accessible = true
	if strings.Contains(m.branding(), "📦") {
		t.Fatal("accessible branding")
	}
	m.resize(120, 50)
	if m.apps.table.Height() < 35 {
		t.Fatalf("table only uses %d rows", m.apps.table.Height())
	}
}
func TestRefreshSwitchDiscardsStaleInventory(t *testing.T) {
	m := newTestModel(t)
	m.refreshing = false
	oldID := m.workspace.Config.ID
	_ = m.refreshCmd("")
	oldGeneration := m.generation
	oldApps := append([]ApplicationView(nil), m.apps.all...)
	updated, _ := m.Update(press("i"))
	m = updated.(Model)
	if m.currentRoute().Kind != RouteAbout {
		t.Fatal("refresh blocked navigation")
	}
	ws, err := workspace.CreateWorkspace(workspace.CreateWorkspaceOptions{Root: t.TempDir(), Name: "Hencon"})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ = m.handleWorkspaceAction(workspaceActionMsg{ws: ws, action: "use"})
	m = updated.(Model)
	updated, _ = m.Update(inventoryMsg{workspaceID: oldID, generation: oldGeneration, apps: oldApps})
	m = updated.(Model)
	if m.workspace.Config.ID != ws.Config.ID || len(m.apps.all) != 0 {
		t.Fatal("stale inventory replaced new workspace")
	}
	m.routes = []Route{{Kind: RouteDiagnostics}}
	if m.folderTarget() != ws.Root {
		t.Fatal("stale folder target")
	}
	newest := m.generation
	updated, _ = m.Update(inventoryMsg{workspaceID: ws.Config.ID, generation: newest - 1, apps: oldApps})
	m = updated.(Model)
	if len(m.apps.all) != 0 {
		t.Fatal("older same-workspace refresh accepted")
	}
}
func TestFolderContextAndFriendlyErrors(t *testing.T) {
	m := newTestModel(t)
	calls := []string{}
	m.opener = pathopener.Func(func(path string) error { calls = append(calls, path); return nil })
	app := m.apps.all[0].App.Ref
	m.registrations = workspace.Registrations(&m.user)
	for _, test := range []struct {
		route Route
		want  string
	}{{Route{Kind: RouteApplications}, app.Path}, {Route{Kind: RouteApplication, AppID: app.Relative}, app.Path}, {Route{Kind: RouteDiagnostics}, m.workspace.Root}, {Route{Kind: RouteWorkspaces}, m.workspace.Root}, {Route{Kind: RouteAbout}, ""}, {Route{Kind: RouteSettings}, ""}} {
		m.routes = []Route{test.route}
		before := len(calls)
		updated, cmd := m.Update(press("o"))
		m = updated.(Model)
		if cmd != nil {
			updated, _ = m.Update(cmd())
			m = updated.(Model)
		}
		if test.want == "" {
			if len(calls) != before {
				t.Fatal("unexpected folder")
			}
		} else if len(calls) != before+1 || calls[len(calls)-1] != test.want {
			t.Fatal(calls, test.want)
		}
	}
	m.routes = []Route{{Kind: RouteApplications}}
	m.opener = pathopener.Func(func(string) error { return errors.New("Folder unavailable: directory no longer exists") })
	updated, cmd := m.Update(press("o"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.modalErr == nil || m.modal != ModalMessage {
		t.Fatal("folder failure lost")
	}
}
func advanceForm(t *testing.T, m *Model) {
	t.Helper()
	updated, _ := m.Update(huh.NextField())
	*m = updated.(Model)
}
func TestCreateFormSlugManualOverrideAndActualCreation(t *testing.T) {
	m := newTestModel(t)
	m.groups = []string{"Browsers", "Microsoft/Office"}
	m.beginCreate()
	updated, _ := m.Update(press("Mozilla Firefox"))
	m = updated.(Model)
	if m.create.DirectoryName != "mozilla-firefox" {
		t.Fatal(m.create)
	}
	advanceForm(t, &m)
	updated, _ = m.Update(press("-custom"))
	m = updated.(Model)
	manual := m.create.DirectoryName
	if !m.directoryEdited {
		t.Fatal("manual edit not recorded")
	}
	updated, _ = m.Update(huh.PrevField())
	m = updated.(Model)
	updated, _ = m.Update(press(" ESR"))
	m = updated.(Model)
	if m.create.DirectoryName != manual {
		t.Fatal("manual directory overwritten")
	}
	m.create.SetupFile = "Firefox Setup.exe"
	m.create.Group = "Microsoft/Office"
	// The creation command is also what the completed single-page form submits.
	msg := m.createCmd()()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.modalTitle != "Application created" {
		t.Fatal(m.modalTitle, m.modalErr)
	}
	selected, ok := m.apps.selected()
	if !ok || selected.App.Config.Name != "Mozilla Firefox ESR" {
		t.Fatal(selected)
	}
	data, err := os.ReadFile(filepath.Join(selected.App.Ref.Path, workspace.AppConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sourceDirectory") || strings.Contains(string(data), "outputDirectory") {
		t.Fatal("stored inherited defaults")
	}
}

type recordingPackager struct{ calls int }

func (r *recordingPackager) Run(_ context.Context, _ string, args []string, _, _ io.Writer) error {
	r.calls++
	for i := range args {
		if args[i] == "-o" {
			return os.WriteFile(filepath.Join(args[i+1], "example.intunewin"), []byte("package"), 0644)
		}
	}
	return errors.New("no output argument")
}
func TestBuildOptionsSubmittedValueReachesRunner(t *testing.T) {
	m := newTestModel(t)
	runner := &recordingPackager{}
	m.runner = runner
	refs := []workspace.AppRef{m.apps.all[0].App.Ref}
	service := packager.Service{Workspace: m.workspace, Runner: runner}
	if _, err := service.Build(context.Background(), refs, packager.BuildOptions{}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	state, _ := m.workspace.CacheDirectory()
	cachePath := filepath.Join(state, buildcache.FileName)
	for _, test := range []struct{ downs, calls int }{{0, 1}, {1, 2}, {2, 3}} {
		m.closeModal()
		m.beginBuildOptions()
		for i := 0; i < test.downs; i++ {
			updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			m = updated.(Model)
		}
		updated, _ := m.form.Update(huh.NextField())
		m.form = updated.(*huh.Form)
		expected := []string{"build", "force", "no-cache"}[test.downs]
		if mode := m.form.GetString("buildMode"); mode != expected {
			t.Fatalf("submitted %q, want %q", mode, expected)
		}
		before, _ := os.ReadFile(cachePath)
		updatedModel, cmd := m.completeForm(nil)
		m = updatedModel.(Model)
		if m.operation == nil {
			t.Fatal("build did not start", m.modalErr)
		}
		op := m.operation
		go func() {
			for range op.events {
			}
		}()
		batch := cmd().(tea.BatchMsg)
		done := batch[len(batch)-1]()
		updatedModel, _ = m.Update(done)
		m = updatedModel.(Model)
		if runner.calls != test.calls {
			t.Fatalf("mode %s made %d calls, want %d", expected, runner.calls, test.calls)
		}
		if m.operation != nil || m.modal == ModalProgress {
			t.Fatal("progress did not finish")
		}
		after, _ := os.ReadFile(cachePath)
		if expected == "no-cache" && !bytes.Equal(before, after) {
			t.Fatal("no-cache changed cache")
		}
	}
}
func TestProgressDisplaysCurrentFinalItem(t *testing.T) {
	m := newTestModel(t)
	op := m.startOperation(operationBuild, "Building", 4)
	defer op.cancel()
	op.current = activityEvent{current: 4, completed: 3, total: 4, label: "Firefox", phase: "packaging"}
	view := ansi.Strip(m.progressModalView())
	if !strings.Contains(view, "4 of 4") || !strings.Contains(view, "3 completed") {
		t.Fatal(view)
	}
}
func TestWorkspaceDefaultsAreCopied(t *testing.T) {
	m := newTestModel(t)
	before := m.workspace.Config.OutputDirectory
	if err := config.UpdateUser(func(user *types.UserConfig) error { user.WorkspaceDefaults.OutputDirectory = "packages"; return nil }); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(m.workspace.Root)
	if err != nil || ws.Config.OutputDirectory != before {
		t.Fatal(ws, err)
	}
}
