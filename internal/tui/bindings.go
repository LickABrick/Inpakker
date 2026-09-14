package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// ContextBinding connects the effective shortcut, help and palette entry to
// the existing page action. Commands with no name are navigation controls.
type ContextBinding struct {
	Key     key.Binding
	Command string
}

func (m Model) pageBindings() []ContextBinding {
	var out []ContextBinding
	add := func(k key.Binding, name string) { out = append(out, ContextBinding{k, name}) }
	custom := func(k key.Binding, label, name string) { h := k.Help(); k.SetHelp(h.Key, label); add(k, name) }
	route := m.currentRoute().Kind
	switch route {
	case RouteApplications, RouteApplication:
		if m.workspace == nil {
			custom(m.keys.NewApp, "create workspace", "Create workspace")
			custom(m.keys.Actions, "add workspace", "Add workspace")
			break
		}
		if route == RouteApplications {
			add(m.keys.Open, "")
			add(m.keys.Search, "")
			add(m.keys.NewApp, "New application")
			add(m.keys.Up, "")
			add(m.keys.Down, "")
			add(m.keys.PageUp, "")
			add(m.keys.PageDown, "")
			if len(m.apps.all) > 0 {
				add(m.keys.BuildAll, "Build all applications")
				add(m.keys.ValidateAll, "Validate all applications")
				add(m.keys.UnpackAll, "Unpack all applications")
			}
		} else {
			add(m.keys.Up, "")
			add(m.keys.Down, "")
			add(m.keys.PageUp, "")
			add(m.keys.PageDown, "")
		}
		if _, ok := m.selectedApplication(); ok {
			add(m.keys.Actions, "Application actions")
			add(m.keys.Build, "Build selected application")
			add(m.keys.BuildOptions, "Build options")
			add(m.keys.Validate, "Validate selected application")
			add(m.keys.Unpack, "Unpack selected application")
		}
		add(m.keys.Refresh, "Refresh applications")
		add(m.keys.Diagnostics, "Workspace diagnostics")
	case RouteWorkspaces:
		add(m.keys.Open, "Switch workspace")
		add(m.keys.Search, "")
		add(m.keys.Up, "")
		add(m.keys.Down, "")
		add(m.keys.PageUp, "")
		add(m.keys.PageDown, "")
		custom(m.keys.NewApp, "create workspace", "Create workspace")
		custom(m.keys.AddWorkspace, "add workspace", "Add workspace")
		if view, ok := m.highlightedWorkspace(); ok {
			add(m.keys.Actions, "Workspace actions")
			custom(m.keys.RelinkWorkspace, "relink", "Relink workspace")
			custom(m.keys.RemoveWorkspace, "remove registration", "Remove workspace registration")
			if view.Config != nil {
				custom(m.keys.RenameWorkspace, "rename", "Rename workspace")
			}
		}
	case RouteSettings:
		add(m.keys.Search, "")
		custom(m.keys.Open, "edit / set up tool", "Edit selected setting")
		add(m.keys.Up, "")
		add(m.keys.Down, "")
		custom(m.keys.DetectTools, "detect tools", "Detect tools")
		rows := m.settingsRows()
		if m.settingCursor < len(rows) && rows[m.settingCursor].scope == "tool" {
			custom(m.keys.Actions, "actions", "Tool actions")
			custom(m.keys.DownloadTool, "download", "Download selected tool")
			custom(m.keys.ClearTool, "clear configuration", "Clear selected tool")
		}
	case RouteAbout:
		add(m.keys.Up, "")
		add(m.keys.Down, "")
		add(m.keys.PageUp, "")
		add(m.keys.PageDown, "")
		if !m.checkingUpdate {
			custom(m.keys.CheckUpdates, "check updates", "Check for updates")
		}
		if m.updateResult.Available && m.version != "dev" && m.version != "" {
			custom(m.keys.InstallUpdate, "install update", "Install update")
		}
	case RouteDiagnostics:
		add(m.keys.Search, "")
		add(m.keys.Up, "")
		add(m.keys.Down, "")
		add(m.keys.PageUp, "")
		add(m.keys.PageDown, "")
		custom(m.keys.Open, "details", "Open diagnostic details")
		add(m.keys.Refresh, "Refresh workspace diagnostics")
	}
	if m.folderTarget() != "" {
		add(m.keys.Folder, "Open folder")
	}
	custom(m.keys.Palette, "commands", "")
	custom(m.keys.Applications, "applications", "Applications")
	add(m.keys.Workspaces, "Workspaces")
	add(m.keys.Settings, "Settings")
	add(m.keys.About, "About")
	add(m.keys.Help, "")
	add(m.keys.Escape, "")
	add(m.keys.Back, "")
	if m.resultWorkspaceMatches(m.lastResult) {
		add(m.keys.LastResult, "Last operation result")
	}
	add(m.keys.Quit, "")
	return out
}

func (m Model) effectiveBindings() []ContextBinding {
	if m.operation != nil {
		return []ContextBinding{{Key: m.keys.Cancel}}
	}
	if m.modal == ModalProgress {
		return nil
	}
	if m.modal == ModalNone {
		if f := m.otherFilter(); f != nil && f.editing {
			return []ContextBinding{{Key: binding([]string{"up", "down"}, "↑/↓", "select")}, {Key: m.keys.Open}, {Key: binding(m.keys.Escape.Keys(), "esc", "clear filter")}}
		}
		if (m.currentRoute().Kind == RouteApplications && m.apps.searching) || (m.currentRoute().Kind == RouteWorkspaces && m.workspaceSearching) {
			return []ContextBinding{{Key: binding([]string{"up", "down"}, "↑/↓", "select")}, {Key: m.keys.Open}, {Key: binding([]string{"esc"}, "esc", "clear filter")}}
		}
		return m.pageBindings()
	}
	keys := []key.Binding{m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown, m.keys.Open, binding([]string{"esc", "backspace"}, "esc", "close")}
	if m.form != nil {
		keys = append([]key.Binding{}, m.form.GetFocusedField().KeyBinds()...)
		if m.formKeys != nil {
			keys = append(keys, m.formKeys.Quit)
		}
		if m.pathInput != nil {
			keys = append(keys, binding([]string{"f2"}, "f2", "browse"))
		}
	}

	if m.picking {
		k := m.picker.KeyMap
		keys = []key.Binding{k.Up, k.Down, k.Open, k.Select, k.Back, k.PageUp, k.PageDown, binding(m.keys.Escape.Keys(), "esc", "back to form")}
	}
	if m.modal == ModalCreateReview {
		keys = append(keys, m.keys.Cancel)
		keys[4] = binding([]string{"enter"}, "enter", "create")
		keys[5] = binding([]string{"esc", "backspace"}, "esc", "edit")
	}
	if m.form == nil && !m.picking && m.modal != ModalPalette {
		keys = append(keys, m.keys.Help)
	}
	if m.modal == ModalPalette {
		keys = []key.Binding{binding([]string{"up", "down"}, "↑/↓", "select"), m.keys.Open, binding(m.keys.Escape.Keys(), "esc", "close")}
	}
	if m.modal == ModalActions {
		keys = []key.Binding{m.keys.Up, m.keys.Down, m.keys.Open, binding(m.keys.Escape.Keys(), "esc", "back"), m.keys.Help, m.keys.Cancel}
	}
	if m.modal == ModalMessage || m.modal == ModalResults {
		if m.modal == ModalMessage && m.workspace != nil {
			keys = append(keys, m.keys.Diagnostics)
		}
		if m.displayedResult != nil {
			keys = append(keys, m.keys.Actions)
		}
		if m.modalHasLog {
			keys = append(keys, m.keys.Logs)
		}
		if m.resultFolder() != "" {
			keys = append(keys, m.keys.Folder)
		}
		if m.displayedResult != nil && len(m.displayedResult.retryIDs) > 0 {
			keys = append(keys, binding([]string{"r"}, "r", "retry failed"))
		}
	}
	out := make([]ContextBinding, len(keys))
	for i, k := range keys {
		out[i] = ContextBinding{Key: k}
	}
	return out
}

func (m Model) dispatchPageKey(pressed tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	for _, b := range m.pageBindings() {
		if !key.Matches(pressed, b.Key) {
			continue
		}
		if f := m.otherFilter(); f != nil {
			if key.Matches(pressed, m.keys.Search) {
				return m, m.focusOtherFilter()
			}
			if key.Matches(pressed, m.keys.Escape) && f.visible() {
				return m.updateOtherFilter(pressed)
			}
		}
		switch {
		case key.Matches(pressed, m.keys.Palette):
			return m, m.beginPalette()
		case key.Matches(pressed, m.keys.Applications):
			m.pushRoute(Route{Kind: RouteApplications})
			return m, nil
		}
		switch m.currentRoute().Kind {
		case RouteWorkspaces:
			return m.updateWorkspaces(pressed)
		case RouteSettings:
			return m.updateSettings(pressed)
		case RouteDiagnostics:
			return m.updateDiagnostics(pressed)
		case RouteApplications, RouteApplication, RouteAbout:
			if key.Matches(pressed, m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown) {
				return m.updatePage(pressed)
			}
		}
		return m.updateGlobalKey(pressed)
	}
	return m, nil
}

func (m Model) bindingHelp(bindings []ContextBinding) string {
	var lines []string
	for _, b := range bindings {
		if !b.Key.Enabled() {
			continue
		}
		h := b.Key.Help()
		lines = append(lines, ansi.Wrap(h.Key+"  "+h.Desc, modalInnerWidth(m.width), ""))
	}
	return strings.Join(lines, "\n")
}
