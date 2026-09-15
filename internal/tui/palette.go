package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
	"strings"
	"unicode/utf8"
)

// matchIndices is shared by inventories and the command palette.
func matchIndices(query string, candidates []string) []int {
	if strings.TrimSpace(query) == "" {
		ids := make([]int, len(candidates))
		for i := range ids {
			ids[i] = i
		}
		return ids
	}
	matches := fuzzy.Find(strings.TrimSpace(query), candidates)
	ids := make([]int, len(matches))
	for i, m := range matches {
		ids[i] = m.Index
	}
	return ids
}

func (m *Model) beginPalette() tea.Cmd {
	m.paletteCommands = nil
	m.paletteRoutes = map[string]RouteKind{}
	origin := m.currentRoute().Kind
	for _, route := range []RouteKind{origin, RouteApplications, RouteWorkspaces, RouteAbout} {
		context := *m
		context.routes = []Route{{Kind: route}}
		if route == origin {
			context.routes = m.routes
		}
		for _, b := range context.pageBindings() {
			if b.Command == "" {
				continue
			}
			if _, exists := m.paletteRoutes[b.Command]; exists {
				continue
			}
			if route != origin {
				switch b.Command {
				case "New application", "Refresh applications", "Build all applications", "Validate all applications", "Workspace diagnostics", "Create workspace", "Add workspace", "Check for updates", "Install update":
				default:
					continue
				}
			}
			if context.actionReason(b.Key.Keys()[0]) != "" {
				continue
			}
			if b.Command == "Switch workspace" {
				v, ok := m.highlightedWorkspace()
				if !ok || v.Status != "ready" {
					continue
				}
			}
			m.paletteCommands = append(m.paletteCommands, b)
			m.paletteRoutes[b.Command] = route
		}
	}
	if _, exists := m.paletteRoutes["Add workspace"]; !exists {
		m.paletteCommands = append(m.paletteCommands, ContextBinding{Command: "Add workspace"})
		m.paletteRoutes["Add workspace"] = RouteWorkspaces
	}
	m.paletteRegistryID = ""
	if v, ok := m.highlightedWorkspace(); ok {
		m.paletteRegistryID = v.ID
	}
	m.paletteAppID = ""
	m.paletteAppUUID = ""
	m.paletteWorkspaceID = ""
	if app, ok := m.selectedApplication(); ok {
		m.paletteAppID = app.App.Ref.Relative
		if app.App.Config != nil {
			m.paletteAppUUID = app.App.Config.ID
		}
	}
	if m.workspace != nil {
		m.paletteWorkspaceID = m.workspace.Config.ID
	}

	m.paletteSearch = textinput.New()
	m.paletteSearch.Prompt = "Commands: "
	m.paletteSearch.Placeholder = "Type to find an action"
	m.paletteSearch.SetWidth(modalInnerWidth(m.width) - 11)
	setSearchTheme(&m.paletteSearch, m.theme)
	m.paletteCursor = 0
	m.modal = ModalPalette
	return m.paletteSearch.Focus()
}
func (m Model) filteredCommands() []ContextBinding {
	names := make([]string, len(m.paletteCommands))
	for i, b := range m.paletteCommands {
		names[i] = b.Command
	}
	var out []ContextBinding
	for _, i := range matchIndices(m.paletteSearch.Value(), names) {
		out = append(out, m.paletteCommands[i])
	}
	return out
}
func (m Model) updatePalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.paletteCursor = min(max(0, m.paletteCursor), max(0, len(m.filteredCommands())-1))
	if p, ok := msg.(tea.KeyPressMsg); ok {
		switch p.Code {
		case tea.KeyEscape:
			m.closeModal()
			return m, nil
		case tea.KeyUp:
			m.paletteCursor = max(0, m.paletteCursor-1)
			return m, nil
		case tea.KeyDown:
			m.paletteCursor = min(max(0, len(m.filteredCommands())-1), m.paletteCursor+1)
			return m, nil
		case tea.KeyEnter:
			commands := m.filteredCommands()
			if len(commands) == 0 {
				return m, nil
			}
			b := commands[min(m.paletteCursor, len(commands)-1)]
			if b.Command == "Add workspace" {
				m.closeModal()
				return m, m.beginWorkspacePath("add", "")
			}
			k := b.Key.Keys()[0]
			if m.paletteRoutes[b.Command] == m.currentRoute().Kind && (m.currentRoute().Kind == RouteApplications || m.currentRoute().Kind == RouteApplication) && (k == "b" || k == "B" || k == "v" || k == "u" || (k == "a" && b.Command == "Application actions") || k == "o") {
				found := false
				if m.workspace != nil && m.workspace.Config.ID == m.paletteWorkspaceID {
					for i, app := range m.apps.filtered {
						if app.App.Ref.Relative == m.paletteAppID && (app.App.Config == nil || app.App.Config.ID == m.paletteAppUUID) {
							m.apps.table.SetCursor(i)
							found = true
							break
						}
					}
				}
				if !found {
					m.showMessage("Application unavailable", "The selected application changed. Reopen Commands.")
					return m, nil
				}
			}
			target := m.paletteRoutes[b.Command]
			if target == RouteWorkspaces && (b.Command == "Switch workspace" || b.Command == "Workspace actions" || b.Command == "Relink workspace" || b.Command == "Remove workspace registration" || b.Command == "Rename workspace" || b.Command == "Open folder") {
				found := false
				for i, v := range m.filteredWorkspaces() {
					if v.ID == m.paletteRegistryID {
						m.workspaceCursor = i
						found = b.Command != "Switch workspace" || v.Status == "ready"
						break
					}
				}
				if !found {
					m.showMessage("Workspace unavailable", "The selected registration changed. Reopen Commands.")
					return m, nil
				}
			}

			originalRoutes := append([]Route{}, m.routes...)
			originalKind := m.currentRoute().Kind
			m.closeModal()
			if target != m.currentRoute().Kind {
				m.pushRoute(Route{Kind: target})
			}

			p := tea.KeyPressMsg{Text: k}
			p.Code, _ = utf8.DecodeRuneInString(k)
			if strings.HasPrefix(k, "ctrl+") {
				p.Text = ""
				p.Code, _ = utf8.DecodeRuneInString(strings.TrimPrefix(k, "ctrl+"))
				p.Mod = tea.ModCtrl
			}
			if k == "enter" {
				p = tea.KeyPressMsg{Code: tea.KeyEnter}
			}
			updated, cmd := m.dispatchPageKey(p)
			result := updated.(Model)
			if target != originalKind {
				if result.currentRoute().Kind == target {
					result.routes = originalRoutes
				} else if result.currentRoute().Kind == RouteDiagnostics {
					result.routes = append(originalRoutes, Route{Kind: RouteDiagnostics})
				}
			}
			return result, cmd
		}
	}
	var cmd tea.Cmd
	before := m.paletteSearch.Value()
	m.paletteSearch, cmd = m.paletteSearch.Update(msg)
	if before != m.paletteSearch.Value() {
		m.paletteCursor = 0
	}
	m.paletteCursor = min(max(0, m.paletteCursor), max(0, len(m.filteredCommands())-1))
	return m, cmd
}
func (m Model) paletteView() string {
	lines := []string{m.paletteSearch.View(), ""}
	commands := m.filteredCommands()
	start, end := listWindow(m.paletteCursor, len(commands), max(1, m.height-13))
	for i := start; i < end; i++ {
		prefix := "  "
		if i == m.paletteCursor {
			prefix = "› "
		}
		lines = append(lines, ansi.Truncate(prefix+commands[i].Command, modalInnerWidth(m.width), "…"))
	}
	if len(commands) == 0 {
		lines = append(lines, "No matching commands. Change the query.")
	} else {
		lines = append(lines, fmt.Sprintf("%d/%d", m.paletteCursor+1, len(commands)))
	}
	return m.dialog("Commands", strings.Join(lines, "\n"))
}

func listWindow(cursor, count, height int) (int, int) {
	start := max(0, cursor-max(1, height)+1)
	return start, min(count, start+max(1, height))
}
