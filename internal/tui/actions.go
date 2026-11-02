package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type menuAction struct{ label, id string }

func (m *Model) beginActions() {
	m.actions = nil
	m.actionReturn = m.modal
	m.actionAppID = ""
	m.actionWorkspaceID = ""
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
		if m.resultAppID() != "" {
			m.actions = append(m.actions, menuAction{"Open application details", "details"})
		}
	} else if m.currentRoute().Kind == RouteWorkspaces {
		view, ok := m.highlightedWorkspace()
		if !ok {
			return
		}
		m.actionWorkspaceID = view.ID
		if view.Status == "ready" {
			m.actions = append(m.actions, menuAction{"Switch to workspace", "workspace-enter"}, menuAction{"Open folder", "workspace-o"})
		}
		if view.Config != nil {
			m.actions = append(m.actions, menuAction{"Rename", "workspace-e"})
		}
		m.actions = append(m.actions, menuAction{"Relink", "workspace-r"}, menuAction{"Remove registration", "workspace-x"})
	} else if app, ok := m.selectedApplication(); ok {
		m.actionAppID = app.App.Ref.Relative
		m.actions = []menuAction{{"Build", "build"}, {"Build options / rebuild", "build-options"}, {"Validate", "validate"}, {"Unpack", "unpack"}, {"Open application folder", "folder"}, {"Workspace diagnostics", "diagnostics"}}
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
		if m.actionCursor < 0 || m.actionCursor >= len(m.actions) {
			return m, nil
		}
		id := m.actions[m.actionCursor].id
		if strings.HasPrefix(id, "workspace-") {
			views := m.filteredWorkspaces()
			found := false
			for i, v := range views {
				if v.ID == m.actionWorkspaceID {
					m.workspaceCursor = i
					found = true
					break
				}
			}
			if !found {
				m.showMessage("Workspace unavailable", "The selected registration changed. Reopen Actions.")
				return m, nil
			}
			m.modal = m.actionReturn
			return m.ExecuteAction(ActionID(id))
		}
		if m.actionAppID != "" {
			found := false
			for i, app := range m.apps.filtered {
				if app.App.Ref.Relative == m.actionAppID {
					m.apps.table.SetCursor(i)
					found = true
					break
				}
			}
			if m.currentRoute().Kind == RouteApplication {
				found = m.currentRoute().AppID == m.actionAppID
			}
			if !found {
				m.showMessage("Application unavailable", "The selected application changed. Reopen Actions.")
				return m, nil
			}
		}

		m.modal = m.actionReturn
		switch id {
		case "build", "build-options", "validate", "unpack", "folder", "diagnostics":
			if m.actionAvailability(ActionID(id)) != "" {
				m.modal = ModalActions
				return m, nil
			}
			return m.ExecuteAction(ActionID(id))

		case "output":
			return m, m.openResultFolder()
		case "retry":
			return m.retryFailed()
		case "details":
			id := m.resultAppID()
			if id == "" {
				return m, nil
			}
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
		label := action.label
		if reason := m.actionAvailability(ActionID(action.id)); reason != "" {
			label += " · disabled: " + reason
		}
		line := "  " + label
		if i == m.actionCursor {
			line = m.theme.StatusActive.Render("› " + label)
		}
		lines = append(lines, line)
	}
	// Keep every action reachable at the minimum terminal size.
	height := max(1, m.height-12)
	start := max(0, m.actionCursor-height+1)
	end := min(len(lines), start+height)
	body := strings.Join(lines[start:end], "\n")
	title := "Actions"
	if m.actionAppID != "" {
		if app, ok := m.selectedApplication(); ok {
			title += " · " + app.App.Label()
		}
	}
	if m.actionWorkspaceID != "" {
		if v, ok := m.highlightedWorkspace(); ok {
			title += " · " + v.Name
		}
	}
	return m.dialog(title, body)
}

func (m Model) actionAvailability(action ActionID) string {
	if m.currentRoute().Kind != RouteApplications && m.currentRoute().Kind != RouteApplication {
		return ""
	}
	all := action == "build-all" || action == "unpack-all"
	unpack := action == "unpack" || action == "unpack-all"
	if action != "build" && action != "build-all" && action != "build-options" && !unpack {
		return ""
	}
	app, ok := m.selectedApplication()
	if !ok && !all {
		return "No application selected"
	}
	if m.workspace == nil {
		return "No workspace selected"
	}
	if !all && !unpack && app.App.Status != "valid" {
		return "Application configuration invalid"
	}
	id, path, name := "content-prep", m.workspace.User.Tools.ContentPrepTool.Path, "Content Prep Tool"
	if unpack {
		id, path, name = "decoder", m.workspace.User.Tools.Decoder.Path, "Decoder"
	}
	if path == "" {
		return name + " not configured"
	}
	for _, status := range m.toolStatuses {
		if status.ID == id && status.Config.Path == path && !status.Valid {
			return name + " unavailable"
		}
	}
	if !all && unpack && len(app.App.Packages) == 0 {
		return "No package exists"
	}
	return ""
}

// Cached availability is presentation only. The operation service still validates
// before execution, including paths that changed since tool detection.
func (m Model) actionReason(k string) string {
	switch k {
	case m.keys.Build.Keys()[0]:
		return m.actionAvailability("build")
	case m.keys.BuildAll.Keys()[0]:
		return m.actionAvailability("build-all")
	case m.keys.BuildOptions.Keys()[0]:
		return m.actionAvailability("build-options")
	case m.keys.Unpack.Keys()[0]:
		return m.actionAvailability("unpack")
	case m.keys.UnpackAll.Keys()[0]:
		return m.actionAvailability("unpack-all")
	}
	return ""
}
