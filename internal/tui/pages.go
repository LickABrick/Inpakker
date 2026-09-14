package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func scrollPosition(v viewport.Model) string {
	if v.TotalLineCount() <= v.Height() {
		return ""
	}
	if v.AtBottom() {
		return "\n100% · End"
	}
	return fmt.Sprintf("\n%d%% · ↓ more", int(v.ScrollPercent()*100))
}
func (m Model) backgroundActivity() string {
	var tasks []string
	if m.refreshing {
		tasks = append(tasks, "Refreshing applications")
	}
	if m.detecting {
		tasks = append(tasks, "Detecting tools")
	}
	if m.checkingUpdate {
		tasks = append(tasks, "Checking updates")
	}
	if len(tasks) > 1 {
		return fmt.Sprintf("%d background tasks", len(tasks))
	}
	if len(tasks) == 1 {
		return tasks[0]
	}
	return ""
}

// splitPreview keeps the left pane stable and bounds long external metadata.
func (m Model) splitPreview(list, detail string, width, height int) string {
	left := (width - 3) / 2
	right := width - left - 3
	detail = previewLines(detail, right, height)
	divider := strings.TrimSuffix(strings.Repeat(" │ \n", max(lipgloss.Height(list), lipgloss.Height(detail))), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(left).MaxWidth(left).Render(list), m.theme.HorizontalSeparator.Render(divider), lipgloss.NewStyle().Width(right).MaxWidth(right).Render(detail))
}
func previewLines(body string, width, height int) string {
	lines := strings.Split(ansi.Wrap(body, width, ""), "\n")
	if len(lines) > height {
		lines = append(lines[:max(0, height-1)], "… Enter for details")
	}
	return strings.Join(lines, "\n")
}
func (m Model) applicationPreview(app ApplicationView, width int) string {
	lines := []string{m.theme.PanelTitle.Render(app.App.Label()), "", keyValue("Status", buildLabel(app.Build.State), 11), keyValue("Validation", validationLabel(app.App), 11), "Path", app.App.Ref.Relative}
	if app.App.Config != nil {
		cfg := app.App.Config
		lines = append(lines, "", keyValue("Setup", cfg.SetupFile, 11), "Install command", metadataValue(cfg.InstallCommand), "Uninstall command", metadataValue(cfg.UninstallCommand))
	}
	if len(app.App.Packages) > 0 {
		lines = append(lines, "Package", app.App.Packages[0])
	}
	if app.App.Error != "" {
		lines = append(lines, "Validation issue", app.App.Error)
	}
	return ansi.Wrap(strings.Join(lines, "\n"), width, "")
}

func workspaceStatus(v workspace.RegistrationView) string {
	state := "✓ Ready"
	if v.Status != "ready" {
		state = "! Unavailable"
	}
	if v.Active {
		state = "● Active · " + state
	}
	return state
}
func (m Model) workspacesView(width, height int) string {
	lines := []string{m.theme.PageTitle.Render("Workspaces"), ""}
	if m.workspaceSearching || m.workspaceSearch.Value() != "" {
		lines = append(lines, "Filter: "+m.workspaceSearch.View())
	}
	views := m.filteredWorkspaces()
	wide := width >= 92
	rowWidth := width
	if wide {
		rowWidth = (width - 3) / 2
	}
	available := max(1, height-len(lines)-1)
	rowHeight := 2
	start, end := listWindow(m.workspaceCursor, len(views), max(1, available/rowHeight))
	var rows []string
	for i := start; i < end; i++ {
		v := views[i]
		prefix := "  "
		if i == m.workspaceCursor {
			prefix = "› "
		}
		rows = append(rows, padBetween(prefix+v.Name, workspaceStatus(v), rowWidth), m.theme.TextMuted.Render(ansi.Truncate("  "+v.Path, rowWidth, "…")))
	}
	if len(views) == 0 {
		if m.workspaceSearch.Value() != "" {
			rows = append(rows, "No matching workspaces. Esc clears the filter.")
		} else {
			rows = append(rows, "No registered workspaces.", "Create with n or use : Add workspace.")
		}
	}
	body := strings.Join(rows, "\n")
	if wide {
		detail := "Select a workspace"
		if v, ok := m.highlightedWorkspace(); ok {
			detail = m.theme.PanelTitle.Render(v.Name) + "\n\n" + workspaceStatus(v) + "\n\nPath\n" + v.Path
			if v.Error != "" {
				detail += "\n\n" + v.Error + "\nUse Actions → Relink."
			}
			if v.Active && m.workspace != nil {
				detail += fmt.Sprintf("\n\nApplications  %d", len(m.apps.all))
			}
		}
		body = m.splitPreview(body, detail, width, available)
	}
	lines = append(lines, body)
	if len(views) > 0 {
		lines = append(lines, fmt.Sprintf("%d/%d workspaces", m.workspaceCursor+1, len(views)))
	}
	return strings.Join(lines, "\n")
}
func settingSection(row settingRow) string {
	if row.scope == "workspace" {
		return "CURRENT WORKSPACE"
	}
	if row.scope == "tool" {
		return "EXTERNAL TOOLS"
	}
	if strings.HasPrefix(row.key, "preferences.") {
		return "PREFERENCES"
	}
	return "NEW WORKSPACE DEFAULTS"
}
func (m Model) toolState(id, path string) string {
	for _, s := range m.toolStatuses {
		if s.ID == id && (s.Config.Path == path || (s.Valid && s.Config.Path == "")) {
			if s.Valid {
				return "✓ Ready"
			}
			if s.Config.Path != "" {
				return "! Configured path unavailable"
			}
			if s.Candidate != "" {
				return "• Candidate detected"
			}
			return "○ Not configured"
		}
	}
	if path != "" {
		return "Checking"
	}
	return "○ Not configured"
}
func (m Model) settingsView(width, height int) string {
	rows := m.settingsRows()
	var lines []string
	selectedLine := 0
	section := ""
	for i, row := range rows {
		next := settingSection(row)
		if next != section {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.theme.PanelTitle.Render(next))
			section = next
		}
		prefix := "  "
		if i == m.settingCursor {
			prefix = "› "
			selectedLine = len(lines)
		}
		value := row.value
		if row.key == "preferences.showToolOutput" {
			value = "Off"
			if row.value == "true" {
				value = "On"
			}
		}
		if row.scope == "tool" {
			value = m.toolState(row.key, row.value)
		}
		lines = append(lines, ansi.Truncate(prefix+row.label+"  "+value, width, "…"))
	}
	header := m.theme.PageTitle.Render("Settings") + "\n\n"
	filterHeight := 0
	if m.settingsFilter.visible() {
		header += m.settingsFilter.input.View() + "\n\n"
		filterHeight = 2
	}
	if len(rows) == 0 {
		return header + "No matching settings. Esc clears the filter."
	}
	visible := max(1, height-3-filterHeight)
	start := max(0, selectedLine-visible+1)
	// Retain section context when scrolling into a section.
	if start > 0 {
		lines[start] = ansi.Truncate(m.theme.TextMuted.Render(settingSection(rows[m.settingCursor]))+" · "+lines[start], width, "…")
	}
	return header + strings.Join(lines[start:min(len(lines), start+visible)], "\n") + fmt.Sprintf("\n%d/%d settings", m.settingCursor+1, len(rows))
}
func diagnosticStatus(c workspace.Check) string {
	switch c.Status {
	case workspace.CheckFailure:
		return "X Failed"
	case workspace.CheckWarning:
		return "! Warning"
	}
	return "✓ Ready"
}
func diagnosticDetail(c workspace.Check) string {
	body := c.Name + "\n\n" + diagnosticStatus(c) + "\n\n" + c.Detail
	if c.Status != workspace.CheckOK {
		switch {
		case strings.Contains(c.Name, "Tool") || strings.Contains(c.Name, "decoder"):
			body += "\n\nOpen Settings → External tools to configure or install the tool."
		case strings.Contains(c.Name, "Application"):
			body += "\n\nOpen Applications and validate the affected application to inspect its configuration."
		default:
			body += "\n\nReview the workspace path and configuration, then refresh diagnostics."
		}
	}
	return body
}
func (m Model) diagnosticsView(width int) string {
	checks := m.filteredDiagnostics()
	header := m.theme.PageTitle.Render("Workspace diagnostics") + "\n\n"
	filterHeight := 0
	if m.diagnosticsFilter.visible() {
		header += m.diagnosticsFilter.input.View() + "\n\n"
		filterHeight = 2
	}
	height := max(1, m.height-9-filterHeight)
	start, end := listWindow(m.diagnosticCursor, len(checks), height)
	var lines []string
	rowWidth := width
	if width >= 92 {
		rowWidth = (width - 3) / 2
	}
	for i := start; i < end; i++ {
		prefix := "  "
		if i == m.diagnosticCursor {
			prefix = "› "
		}
		c := checks[i]
		lines = append(lines, padBetween(prefix+c.Name, diagnosticStatus(c), rowWidth))
	}
	body := strings.Join(lines, "\n")
	if len(checks) == 0 {
		body = "Diagnostics are loading. Refresh to try again."
		if m.diagnosticsFilter.input.Value() != "" {
			body = "No matching checks. Esc clears the filter."
		}
	} else if width >= 92 {
		body = m.splitPreview(body, diagnosticDetail(checks[min(m.diagnosticCursor, len(checks)-1)]), width, height)
	}
	return header + body + fmt.Sprintf("\n%d/%d checks", min(len(checks), m.diagnosticCursor+1), len(checks))
}
func (m Model) updateDiagnostics(p tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	checks := m.filteredDiagnostics()
	switch {
	case key.Matches(p, m.keys.Up):
		m.diagnosticCursor = max(0, m.diagnosticCursor-1)
	case key.Matches(p, m.keys.Down):
		m.diagnosticCursor = min(max(0, len(checks)-1), m.diagnosticCursor+1)
	case key.Matches(p, m.keys.PageUp):
		m.diagnosticCursor = max(0, m.diagnosticCursor-max(1, m.height-10))
	case key.Matches(p, m.keys.PageDown):
		m.diagnosticCursor = min(max(0, len(checks)-1), m.diagnosticCursor+max(1, m.height-10))
	case key.Matches(p, m.keys.Open):
		if len(checks) > 0 {
			m.showMessage("Diagnostic details", diagnosticDetail(checks[min(m.diagnosticCursor, len(checks)-1)]))
		}
	case key.Matches(p, m.keys.Refresh):
		return m, m.refreshCmd("")
	default:
		return m.updateGlobalKey(p)
	}
	return m, nil
}

func metadataValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func (m Model) filteredDiagnostics() []workspace.Check {
	candidates := make([]string, len(m.diagnostics))
	for i, c := range m.diagnostics {
		candidates[i] = c.Name + " " + c.Detail + " " + diagnosticStatus(c)
	}
	out := make([]workspace.Check, 0, len(candidates))
	for _, i := range matchIndices(m.diagnosticsFilter.input.Value(), candidates) {
		out = append(out, m.diagnostics[i])
	}
	return out
}
