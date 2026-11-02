package tui

import (
	"testing"

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
