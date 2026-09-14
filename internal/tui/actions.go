package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/charmbracelet/x/ansi"
)

type menuAction struct{ label, id string }

func (m *Model) beginActions() {
	m.actions = nil
	m.actionReturn = m.modal
	if m.displayedResult != nil {
		if m.modalHasLog {
			m.actions = append(m.actions, menuAction{"View tool output", "log"})
		}
		if m.resultFolder() != "" {
			m.actions = append(m.actions, menuAction{"Open output folder", "output"})
		}
		if len(m.displayedResult.retryIDs) > 0 {
			m.actions = append(m.actions, menuAction{"Retry failed applications", "retry"})
		}
		if len(m.resultRows) > m.resultCursor && m.resultRows[m.resultCursor].AppID != "" {
			m.actions = append(m.actions, menuAction{"Open application details", "details"})
		}
	} else if _, ok := m.selectedApplication(); ok {
		m.actions = []menuAction{{"Build", "build"}, {"Build options / rebuild", "build-options"}, {"Validate", "validate"}, {"Unpack", "unpack"}, {"Open application folder", "folder"}, {"Diagnostics", "diagnostics"}}
	} else {
		return
	}
	m.actions = append(m.actions, menuAction{"Back", "back"})
	m.actionCursor = 0
	m.modal = ModalActions
}

func (m Model) updateActions(msg tea.Msg) (tea.Model, tea.Cmd) {
	pressed, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(pressed, m.keys.Escape, m.keys.Back, m.keys.Cancel):
		m.modal = m.actionReturn
	case key.Matches(pressed, m.keys.Up):
		m.actionCursor = max(0, m.actionCursor-1)
	case key.Matches(pressed, m.keys.Down):
		m.actionCursor = min(len(m.actions)-1, m.actionCursor+1)
	case key.Matches(pressed, m.keys.Open):
		id := m.actions[m.actionCursor].id
		m.modal = m.actionReturn
		switch id {
		case "build":
			return m.startBuild(false, packager.BuildOptions{})
		case "build-options":
			return m, m.beginBuildOptions()
		case "validate":
			return m.startValidate(false)
		case "unpack":
			return m.startUnpack(false, "")
		case "folder":
			return m, m.openFolderCmd()
		case "diagnostics":
			m.openDiagnostics()
		case "output":
			return m, m.openResultFolder()
		case "retry":
			return m.retryFailed()
		case "details":
			id := m.resultRows[m.resultCursor].AppID
			m.closeModal()
			m.pushRoute(Route{Kind: RouteApplication, AppID: id})
		case "log":
			m.logReturnModal, m.modal = m.modal, ModalLogs
			m.prepareLogViewport()
			m.logViewport.GotoTop()
		}
	}
	return m, nil
}

func (m Model) actionsView() string {
	lines := make([]string, 0, len(m.actions))
	for i, action := range m.actions {
		line := "  " + action.label
		if i == m.actionCursor {
			line = m.theme.StatusActive.Render("› " + action.label)
		}
		lines = append(lines, line)
	}
	// Keep every action reachable at the minimum terminal size.
	height := max(1, m.height-12)
	start := max(0, m.actionCursor-height+1)
	end := min(len(lines), start+height)
	body := strings.Join(lines[start:end], "\n")
	return m.dialog("Actions", body+"\n\n"+m.theme.Help.Render(ansi.Wrap("↑↓ choose · enter select · esc back", modalInnerWidth(m.width), "")))
}
