package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/pathopener"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func TestSemanticActionIgnoresShortcutRebinding(t *testing.T) {
	m := newTestModel(t)
	m.keys.Validate.SetKeys("ctrl+v")
	m.beginActions()
	cmd := chooseMenuAction(t, &m, "validate")
	if cmd == nil || m.operation == nil || len(m.operation.targets) != 1 {
		t.Fatal("semantic menu action did not start validation")
	}
	m.operation.cancel()
}

func TestPaletteSemanticActionIgnoresShortcutRebinding(t *testing.T) {
	m := newTestModel(t)
	m.keys.Validate.SetKeys("ctrl+v")
	m.beginPalette()
	m.paletteSearch.SetValue("Validate selected application")
	updated, cmd := m.updatePalette(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.operation == nil || len(m.operation.targets) != 1 {
		t.Fatal("palette did not execute semantic validation action")
	}
	m.operation.cancel()
}

func TestCreateDraftReviewsOnlyExternalCopy(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "external-copy"}[external], func(t *testing.T) {
			m := newTestModel(t)
			m.create = &workspace.CreateOptions{Name: "Draft", DirectoryName: "draft", SetupFile: "setup.exe"}
			if external {
				m.create.SetupFrom = "external-installer.exe"
			}
			m.beginCreateForm()
			updated, cmd := m.completeCreateDraft(nil)
			m = updated.(Model)
			if external {
				if m.modal != ModalCreateReview || cmd != nil {
					t.Fatal("external copy bypassed review")
				}
			} else if m.modal != ModalProgress || cmd == nil {
				t.Fatal("ordinary create did not proceed directly")
			}
		})
	}
}

func TestPaletteOffersBuildStateVerificationWithoutShortcut(t *testing.T) {
	m := newTestModel(t)
	m.beginPalette()
	found := false
	for _, command := range m.paletteCommands {
		if command.Command == "Verify all build states" {
			found = true
			if len(command.Key.Keys()) != 0 || actionForCommand(command.Command) != "verify-build-states" {
				t.Fatal("verification must be a semantic palette-only command")
			}
		}
	}
	if !found {
		t.Fatal("verification command missing")
	}
	m.workspace = nil
	m.beginPalette()
	for _, command := range m.paletteCommands {
		if command.Command == "Verify all build states" {
			t.Fatal("verification offered without a workspace")
		}
	}
}

func TestPreferenceSaveDoesNotRefreshInventory(t *testing.T) {
	m := newTestModel(t)
	generation, refreshing := m.generation, m.refreshing
	updated, _ := m.handleSettings(settingsMsg{user: &m.user})
	m = updated.(Model)
	if m.generation != generation || m.refreshing != refreshing {
		t.Fatal("preference-only settings change invalidated application inventory")
	}
}

func TestApplicationActionsAreScopedAndPackageAware(t *testing.T) {
	m := newTestModel(t)
	m.apps.all[0].App.Packages = nil
	m.apps.applyFilter("")
	m.beginActions()
	for _, action := range m.actions {
		if action.id == "diagnostics" || action.id == "unpack" || action.id == "package-folder" {
			t.Fatalf("irrelevant application action: %s", action.id)
		}
	}
	packagePath := filepath.Join(t.TempDir(), "output", "app.intunewin")
	m.apps.all[0].App.Packages = []string{packagePath}
	m.apps.applyFilter("")
	m.modal = ModalNone
	m.beginActions()
	opened := ""
	m.opener = pathopener.Func(func(path string) error { opened = path; return nil })
	cmd := chooseMenuAction(t, &m, "package-folder")
	if cmd == nil {
		t.Fatal("package folder action did not return opener command")
	}
	cmd()
	if opened != filepath.Dir(packagePath) {
		t.Fatalf("opened %q, want package directory", opened)
	}
	m.workspace.User.Tools.Decoder.Path = ""
	m.beginActions()
	chooseMenuAction(t, &m, "unpack")
	if m.modal != ModalTool || m.toolID != "decoder" {
		t.Fatal("missing decoder did not offer recovery")
	}
}

func TestBulkCommandLabelsCountUnfilteredInventory(t *testing.T) {
	m := newTestModel(t)
	addTestApplication(t, &m)
	count := len(m.apps.all)
	m.apps.search.SetValue("no matching application")
	m.apps.applyFilter("")
	found := 0
	for _, b := range m.pageBindings() {
		action := actionForCommand(b.Command)
		for _, item := range []struct{ action ActionID; label string }{
			{"build-all", "Build all applications"},
			{"validate-all", "Validate all applications"},
			{"unpack-all", "Unpack all applications"},
		} {
			if action == item.action {
				found++
				if b.Command != fmt.Sprintf("%s (%d)", item.label, count) {
					t.Fatalf("bulk command count excludes hidden applications: %q", b.Command)
				}
			}
		}
	}
	if found != 3 {
		t.Fatalf("found %d bulk commands, want 3", found)
	}
}

func TestToolPathSettingRefreshesFastInventory(t *testing.T) {
	m := newTestModel(t)
	user := m.user
	user.Tools.ContentPrepTool.Path = filepath.Join(t.TempDir(), "new-tool.exe")
	generation := m.generation
	updated, cmd := m.handleSettings(settingsMsg{user: &user})
	m = updated.(Model)
	if cmd == nil || m.generation != generation+1 || !m.refreshing {
		t.Fatal("tool path change did not invalidate fast build state inventory")
	}
}
