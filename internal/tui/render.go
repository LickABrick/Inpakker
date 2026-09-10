package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() tea.View {
	var content string
	if m.width > 0 && m.height > 0 && (m.width < minimumWidth || m.height < minimumHeight) {
		content = m.tooSmallView()
	} else {
		content = m.shellView()
		if m.modal != ModalNone || m.operation != nil {
			content = m.overlay(content, m.modalView())
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.ReportFocus = true
	return view
}

func (m Model) tooSmallView() string {
	width := max(20, m.width)
	header := m.theme.TopBar.Width(width).Render(padBetween(m.theme.Brand.Render(m.branding()), "", width-2))
	body := fmt.Sprintf("\n%s\n\nInpakker needs at least %d×%d characters for this view.\n\nCurrent size: %d×%d",
		m.theme.PageTitle.Render("Terminal too small"), minimumWidth, minimumHeight, m.width, m.height)
	return header + "\n" + lipgloss.NewStyle().Padding(0, 2).Render(body)
}

func (m Model) shellView() string {
	width := max(minimumWidth, m.width)
	inner := max(20, width-4)
	brand := m.theme.Brand.Render(m.branding())
	right := ""
	if m.workspace != nil {
		right = m.theme.Text.Render(m.workspace.Config.Name)
	}
	top := m.theme.TopBar.Width(width).Render(padBetween(brand, right, width-2))
	workspaceLabel := "No workspace selected"
	if m.workspace != nil {
		workspaceLabel = m.workspace.Root
	}
	workspaceRight := ""
	if width >= 74 && m.workspace != nil {
		workspaceRight = fmt.Sprintf("%d %s", len(m.apps.all), pluralWord(len(m.apps.all), "application", "applications"))
	}
	if m.refreshing {
		workspaceRight = m.spinner.View() + " Refreshing…  " + workspaceRight
	}
	if m.detecting {
		workspaceRight = m.spinner.View() + " Detecting tools…  " + workspaceRight
	}
	if m.checkingUpdate {
		workspaceRight = m.spinner.View() + " Checking for updates…  " + workspaceRight
	}
	if m.updateResult.Available {
		workspaceRight = "↑ Update v" + m.updateResult.LatestVersion + " available · i About"
	}
	contextLine := m.theme.WorkspaceBar.Width(width).Render(padBetween(ansi.Truncate(workspaceLabel, max(15, width-lipgloss.Width(workspaceRight)-6), "…"), workspaceRight, width-2))
	separator := m.theme.HorizontalSeparator.Render(strings.Repeat("─", width))
	contentWidth := max(20, inner-4)
	page := lipgloss.NewStyle().Width(inner).Height(max(5, m.height-6)).Padding(0, 2).Render(m.pageView(contentWidth, max(5, m.height-6)))
	footer := lipgloss.NewStyle().Width(width).Padding(0, 1).Render(m.footerView(width - 2))
	return strings.Join([]string{top, contextLine, separator, page, separator, footer}, "\n")
}

func (m Model) pageView(width, height int) string {
	switch m.currentRoute().Kind {
	case RouteAbout:
		return m.aboutView(width)
	case RouteWorkspaces:
		return m.workspacesView(width, height)
	case RouteSettings:
		return m.settingsView(width, height)
	case RouteApplication:
		viewport := m.detailViewport
		viewport.SetWidth(width)
		viewport.SetHeight(height)
		viewport.SetContent(m.applicationView(width))
		return viewport.View()
	case RouteDiagnostics:
		return m.diagnosticsView(width)
	case RouteValidationResults, RouteBuildResults, RouteUnpackResults:
		return m.resultsView(width, height)
	case RouteLogs:
		return m.logsView()
	default:
		return m.applicationsView(width)
	}
}

func (m Model) applicationsView(width int) string {
	if m.workspace == nil {
		return lipgloss.Place(width, max(6, m.height-10), lipgloss.Center, lipgloss.Center,
			m.theme.PageTitle.Render("No workspace selected")+"\n\nCreate a new packaging workspace or add an existing one.\n\n[ n ] Create workspace\n[ a ] Add existing workspace")
	}
	var output strings.Builder
	output.WriteString(m.theme.PageTitle.Render("Applications"))
	output.WriteString("\n\n")
	if m.apps.searching {
		output.WriteString(m.apps.search.View())
		output.WriteString("\n\n")
	}
	if len(m.apps.all) == 0 {
		empty := m.theme.PageTitle.Render("No applications yet") + "\n\n" +
			m.theme.TextMuted.Render("Create your first Win32 application package\nand add its source files to the workspace.") + "\n\n" +
			m.theme.Brand.Render("[ n ] Create application")
		output.WriteString(lipgloss.Place(width, 10, lipgloss.Center, lipgloss.Center, empty))
		return output.String()
	}
	if len(m.apps.filtered) == 0 {
		output.WriteString(m.theme.StatusWarning.Render(fmt.Sprintf("No applications match %q", m.apps.search.Value())))
		return output.String()
	}
	output.WriteString(m.apps.table.View())
	output.WriteString("\n")
	if m.apps.searching {
		output.WriteString(m.theme.TextMuted.Render(fmt.Sprintf("%d %s", len(m.apps.filtered), pluralWord(len(m.apps.filtered), "match", "matches"))))
	} else {
		output.WriteString(m.theme.TextMuted.Render(fmt.Sprintf("%d of %d applications", m.apps.table.Cursor()+1, len(m.apps.filtered))))
	}
	return output.String()
}

func (m Model) applicationView(width int) string {
	app, ok := m.selectedApplication()
	if !ok {
		return m.theme.StatusError.Render("X Application is no longer available. Press Backspace to return.")
	}
	status := m.styleValidation(app.App)
	display := app.App.Label()
	if app.App.Config != nil && strings.TrimSpace(app.App.Config.Name) != "" {
		display = app.App.Config.Name
	}
	heading := m.theme.Breadcrumb.Render("Applications / "+app.App.Label()) + "\n\n" +
		m.theme.PageTitle.Render(app.App.Label()) + "\n" + padBetween(m.theme.TextMuted.Render(display), status, width)
	if app.App.Status != "valid" {
		issue := m.theme.PanelTitle.Render("Validation issue") + "\n" + m.theme.StatusError.Render("X "+app.App.Error)
		return heading + "\n\n" + m.theme.Panel.Width(max(20, width-2)).Render(ansi.Wrap(issue, max(20, width-8), ""))
	}
	build := m.styleBuild(app.Build.State)
	packageLines := []string{
		m.theme.PanelTitle.Render("Package state"),
		keyValue("Build", build, 16),
		keyValue("Packages", packageCount(len(app.App.Packages)), 16),
	}
	if len(app.App.Packages) > 0 {
		packageLines = append(packageLines, keyValue("Output", ansi.Truncate(app.App.Packages[0], max(18, width/2), "…"), 16))
	}
	packagePanel := m.theme.Panel.Render(strings.Join(packageLines, "\n"))
	cfg := app.App.Config
	configLines := []string{
		m.theme.PanelTitle.Render("Configuration"),
		keyValue("Directory", app.App.Ref.Relative, 18),
		keyValue("Group", app.Group, 18),
		keyValue("Source directory", inheritedLabel(app.App.Effective.SourceDirectory, app.App.Effective.SourceInherited), 18),
		keyValue("Setup file", cfg.SetupFile, 18),
		keyValue("Output directory", inheritedLabel(app.App.Effective.OutputDirectory, app.App.Effective.OutputInherited), 18),
	}
	configPanel := m.theme.Panel.Render(strings.Join(configLines, "\n"))
	panels := packagePanel + "\n\n" + configPanel
	if width >= 100 {
		panelWidth := (width - 4) / 2
		panels = lipgloss.JoinHorizontal(lipgloss.Top,
			m.theme.Panel.Width(panelWidth).Render(strings.Join(packageLines, "\n")), "  ",
			m.theme.Panel.Width(panelWidth).Render(strings.Join(configLines, "\n")))
	}
	if len(app.App.Packages) > 0 {
		var packages []string
		for _, item := range app.App.Packages {
			packages = append(packages, filepath.Base(item)+"\n"+m.theme.TextMuted.Render(ansi.Wrap(item, width, "")))
		}
		panels += "\n\n" + m.theme.PageTitle.Render("Package") + "\n\n" + strings.Join(packages, "\n\n")
	}
	if strings.TrimSpace(m.workspace.User.Tools.Decoder.Path) == "" {
		panels += "\n\n" + m.theme.StatusMuted.Render("— Unpack unavailable: decoder is not configured")
	}
	return heading + "\n\n" + panels
}

func (m Model) diagnosticsView(width int) string {
	failures, warnings := 0, 0
	for _, check := range m.diagnostics {
		if check.Status == workspace.CheckFailure {
			failures++
		} else if check.Status == workspace.CheckWarning {
			warnings++
		}
	}
	summary := m.theme.StatusSuccess.Render("✓ Healthy")
	if failures > 0 {
		summary = m.theme.StatusError.Render("X Problems found")
	} else if warnings > 0 {
		summary = m.theme.StatusWarning.Render("! Attention required")
	}
	var lines []string
	lines = append(lines, m.theme.Breadcrumb.Render("Workspace / Diagnostics"), "", padBetween(m.theme.PageTitle.Render("Workspace health"), summary, width), "")
	nameWidth := min(28, max(18, width/3))
	statusWidth := 13
	detailWidth := max(15, width-nameWidth-statusWidth-4)
	lines = append(lines, m.theme.TableHeader.Render(fmt.Sprintf("%-*s %-*s %s", nameWidth, "CHECK", statusWidth, "STATUS", "DETAILS")))
	for _, check := range m.diagnostics {
		status := "✓ OK"
		statusStyle := m.theme.StatusSuccess
		if check.Status == workspace.CheckWarning {
			status = "! Warning"
			statusStyle = m.theme.StatusWarning
		} else if check.Status == workspace.CheckFailure {
			status = "X Failed"
			statusStyle = m.theme.StatusError
		}
		status = statusStyle.Render(fmt.Sprintf("%-*s", statusWidth, status))
		lines = append(lines, fmt.Sprintf("%-*s %s %s", nameWidth, ansi.Truncate(check.Name, nameWidth, "…"), status, ansi.Truncate(check.Detail, detailWidth, "…")))
	}
	if warnings > 0 {
		lines = append(lines, "", m.theme.StatusWarning.Render("! Optional capabilities may be unavailable; review the warning details above."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) resultsView(width, height int) string {
	lines := []string{m.theme.Breadcrumb.Render(resultBreadcrumb(m.currentRoute().Kind)), "", m.theme.PageTitle.Render(m.resultTitle), ""}
	visible := max(1, height-7)
	start := 0
	if m.resultCursor >= visible {
		start = m.resultCursor - visible + 1
	}
	end := min(len(m.resultRows), start+visible)
	nameWidth := min(28, max(16, width/3))
	statusWidth := 16
	detailWidth := max(12, width-nameWidth-statusWidth-5)
	lines = append(lines, m.theme.TableHeader.Render(fmt.Sprintf("  %-*s %-*s %s", nameWidth, "APPLICATION", statusWidth, "STATUS", "DETAILS")))
	for index := start; index < end; index++ {
		row := m.resultRows[index]
		prefix := "  "
		if index == m.resultCursor {
			prefix = "› "
		}
		line := fmt.Sprintf("%s%-*s %-*s %s", prefix, nameWidth, ansi.Truncate(row.Name, nameWidth, "…"), statusWidth, row.Status, ansi.Truncate(row.Detail, detailWidth, "…"))
		if index == m.resultCursor {
			line = m.theme.TableSelected.Render(line)
		}
		lines = append(lines, line)
	}
	if len(m.resultRows) == 0 {
		lines = append(lines, m.theme.TextMuted.Render("No result rows to display."))
	} else {
		lines = append(lines, "", m.theme.TextMuted.Render(fmt.Sprintf("%d of %d", m.resultCursor+1, len(m.resultRows))))
	}
	return strings.Join(lines, "\n")
}

func (m Model) logsView() string {
	return m.theme.Breadcrumb.Render("Applications / Build log") + "\n\n" + m.theme.PageTitle.Render(m.logTitle) + "\n\n" + m.logViewport.View()
}

func (m Model) footerView(width int) string {
	m.help.SetWidth(width)
	bindings := helpBindings{}
	if m.operation != nil {
		bindings.short = []key.Binding{m.keys.Cancel}
		return m.help.View(bindings)
	}
	if m.modal != ModalNone {
		description := "close"
		if m.form != nil {
			description = "cancel"
		}
		bindings.short = []key.Binding{binding(m.keys.Escape.Keys(), "esc", description)}
		if m.modal == ModalHelp {
			bindings.short = []key.Binding{m.keys.Up, m.keys.Down, binding(m.keys.Escape.Keys(), "esc", "close")}
		}
		return m.help.View(bindings)
	}
	if m.apps.searching && m.currentRoute().Kind == RouteApplications {
		bindings.short = []key.Binding{m.keys.Up, m.keys.Down, m.keys.Open, binding(m.keys.Escape.Keys(), "esc", "clear search")}
		return m.help.View(bindings)
	}
	switch m.currentRoute().Kind {
	case RouteAbout:
		bindings.short = []key.Binding{binding([]string{"c"}, "c", "check updates"), binding([]string{"u"}, "u", "install update"), m.keys.Back}
	case RouteWorkspaces:
		bindings.short = []key.Binding{m.keys.Open, m.keys.NewApp, binding([]string{"a"}, "a", "add"), binding([]string{"r"}, "r", "relink"), binding([]string{"x"}, "x", "remove"), m.keys.Search, m.keys.Back}
	case RouteSettings:
		bindings.short = []key.Binding{binding([]string{"enter"}, "enter", "edit"), binding([]string{"d"}, "d", "detect"), binding([]string{"I"}, "I", "install tool"), binding([]string{"a"}, "a", "use detected"), binding([]string{"x"}, "x", "clear"), m.keys.Back}
	case RouteApplications:
		if width < 80 {
			bindings.short = []key.Binding{m.keys.Open, m.keys.Search, m.keys.NewApp, m.keys.Help, m.keys.Quit}
		} else {
			bindings.short = []key.Binding{m.keys.Up, m.keys.Down, m.keys.Open, m.keys.Search, m.keys.NewApp, m.keys.Build, m.keys.Help, m.keys.Quit}
		}
	case RouteApplication:
		build, unpack := m.keys.Build, m.keys.Unpack
		if app, ok := m.selectedApplication(); ok {
			if app.App.Status != "valid" {
				build = binding(build.Keys(), "b", "build unavailable")
			}
			if m.workspace == nil || strings.TrimSpace(m.workspace.User.Tools.Decoder.Path) == "" || len(app.App.Packages) == 0 {
				unpack = binding(unpack.Keys(), "u", "unpack unavailable")
			}
		}
		bindings.short = []key.Binding{build, m.keys.Validate, unpack, m.keys.Back, m.keys.Help}
	case RouteDiagnostics:
		bindings.short = []key.Binding{m.keys.Refresh, m.keys.Back, m.keys.Help}
	case RouteLogs:
		bindings.short = []key.Binding{m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown, m.keys.Back}
	default:
		bindings.short = []key.Binding{m.keys.Up, m.keys.Down, m.keys.Open, m.keys.Back, m.keys.Help}
	}
	if m.workspace == nil && m.currentRoute().Kind == RouteApplications {
		bindings.short = []key.Binding{binding([]string{"n"}, "n", "create"), binding([]string{"a"}, "a", "add"), m.keys.Workspaces, m.keys.Settings, m.keys.About, m.keys.Help}
	}
	if m.folderTarget() != "" {
		bindings.short = append([]key.Binding{m.keys.Folder}, bindings.short...)
	}
	return m.help.View(bindings)
}

func (m Model) modalView() string {
	if m.operation != nil {
		return m.progressModalView()
	}
	if m.picking {
		return m.dialog("Choose path", m.picker.View()+"\n↑↓ select · → enter directory · enter choose · esc back to form")
	}
	var body string
	switch m.modal {
	case ModalHelp:
		viewport := m.helpViewport
		viewport.SetWidth(modalInnerWidth(m.width))
		viewport.SetHeight(max(4, min(14, m.height-10)))
		viewport.SetContent(m.helpContent())
		body = viewport.View()
	case ModalNewApplication, ModalWorkspaceForm, ModalBuildOptions, ModalPackageSelect, ModalUpdate, ModalSetting, ModalTool:
		if m.form != nil {
			body = m.form.View()
		}
	default:
		if m.modalErr != nil {
			body = m.theme.StatusError.Render("X " + friendlyError(m.modalErr))
			if detail := errorDetail(m.modalErr); detail != "" {
				body += "\n\n" + m.theme.TextMuted.Render("Details\n"+ansi.Wrap(detail, modalInnerWidth(m.width), ""))
			}
		} else {
			body = ansi.Wrap(m.modalBody, modalInnerWidth(m.width), "")
		}
		body += "\n\n" + m.theme.Help.Render("enter/esc close")
		if strings.TrimSpace(m.logText) != "" && m.modalErr != nil {
			body += "   " + m.theme.Help.Render("l view tool output")
		}
	}
	return m.dialog(m.modalTitle, body)
}

func (m Model) helpContent() string {
	helper := m.help
	helper.ShowAll = true
	helper.SetWidth(modalInnerWidth(m.width))
	return helper.View(m.fullHelp())
}

func (m Model) progressModalView() string {
	current := m.operation.current
	total := max(1, current.total)
	percent := float64(current.completed) / float64(total)
	elapsed := time.Since(m.operation.started).Round(time.Second)
	body := m.progress.ViewAs(percent) + fmt.Sprintf("  %d of %d", current.current, current.total) + "\n\n" +
		m.spinner.View() + " " + titleCase(current.phase)
	if current.label != "" {
		body += "\n" + m.theme.TextMuted.Render(current.label)
	}
	body += fmt.Sprintf("\n%d completed", current.completed) + "\n\n" + m.theme.TextMuted.Render(elapsed.String()+" elapsed") + "\n\n" + m.theme.Help.Render("ctrl+c cancel")
	return m.dialog(m.operation.title, body)
}

func (m Model) dialog(title, body string) string {
	width := modalContentWidth(m.width)
	content := m.theme.ModalTitle.Render(title) + "\n\n" + body
	return m.theme.Modal.Width(width).MaxWidth(width).Render(content)
}

func (m Model) overlay(base, dialog string) string {
	width, height := max(minimumWidth, m.width), max(minimumHeight, m.height)
	base = lipgloss.NewStyle().Width(width).Height(height).Render(base)
	x := max(1, (width-lipgloss.Width(dialog))/2)
	y := max(1, (height-lipgloss.Height(dialog))/2)
	return lipgloss.NewCompositor(lipgloss.NewLayer(base), lipgloss.NewLayer(dialog).X(x).Y(y).Z(1)).Render()
}

func (m Model) fullHelp() helpBindings {
	if m.width < 100 {
		return helpBindings{full: [][]key.Binding{
			{m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown, m.keys.Open, m.keys.Back, m.keys.Escape},
			{m.keys.Search, m.keys.NewApp, m.keys.Build, m.keys.BuildOptions, m.keys.Validate, m.keys.Unpack, m.keys.BuildAll, m.keys.ValidateAll, m.keys.UnpackAll, m.keys.Diagnostics, m.keys.Refresh, m.keys.Workspaces, m.keys.Settings, m.keys.About, m.keys.Folder, m.keys.Help, m.keys.Quit},
		}}
	}
	return helpBindings{full: [][]key.Binding{
		{m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown, m.keys.Open, m.keys.Back, m.keys.Escape},
		{m.keys.Search, m.keys.NewApp, m.keys.Build, m.keys.BuildOptions, m.keys.Validate, m.keys.Unpack},
		{m.keys.BuildAll, m.keys.ValidateAll, m.keys.UnpackAll, m.keys.Diagnostics, m.keys.Refresh},
		{m.keys.Workspaces, m.keys.Settings, m.keys.About, m.keys.Folder, m.keys.Help, m.keys.Quit},
	}}
}

func modalContentWidth(terminalWidth int) int { return min(68, max(38, terminalWidth-10)) }

func modalInnerWidth(terminalWidth int) int { return max(32, modalContentWidth(terminalWidth)-6) }

func formContentHeight(terminalHeight int) int { return max(6, terminalHeight-10) }

func padBetween(left, right string, width int) string {
	if right == "" {
		return ansi.Truncate(left, width, "…")
	}
	left = ansi.Truncate(left, max(1, width-lipgloss.Width(right)-1), "…")
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right
}

func keyValue(label, value string, width int) string {
	return fmt.Sprintf("%-*s %s", width, label, value)
}

func displayVersion(version string) string {
	if version == "" || version == "dev" {
		return "dev"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func (m Model) styleValidation(app workspace.App) string {
	if app.Status == "valid" {
		return m.theme.StatusSuccess.Render("✓ Valid")
	}
	return m.theme.StatusError.Render("X Invalid")
}

func (m Model) styleBuild(state packager.BuildState) string {
	label := buildLabel(state)
	switch state {
	case packager.BuildStateCurrent:
		return m.theme.StatusSuccess.Render(label)
	case packager.BuildStateNeedsBuild:
		return m.theme.StatusWarning.Render(label)
	case packager.BuildStateUnavailable:
		return m.theme.StatusMuted.Render(label)
	case packager.BuildStateUnknown:
		return m.theme.StatusWarning.Render(label)
	default:
		return m.theme.TextMuted.Render(label)
	}
}

func pluralWord(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

func resultBreadcrumb(kind RouteKind) string {
	switch kind {
	case RouteValidationResults:
		return "Workspace / Validation"
	case RouteBuildResults:
		return "Workspace / Build results"
	case RouteUnpackResults:
		return "Workspace / Unpack results"
	default:
		return "Workspace"
	}
}

func titleCase(value string) string {
	if value == "" {
		return "Working"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func friendlyError(err error) string {
	text := err.Error()
	if index := strings.Index(text, ":"); index > 0 {
		return text[:index]
	}
	return text
}

func errorDetail(err error) string {
	text := err.Error()
	if index := strings.Index(text, ":"); index > 0 && index+1 < len(text) {
		return strings.TrimSpace(text[index+1:])
	}
	return ""
}

func (m Model) branding() string {
	if m.accessible {
		return "INPAKKER"
	}
	return "📦 INPAKKER"
}
func (m Model) aboutView(width int) string {
	state := "— Not checked"
	if m.version == "dev" || m.version == "" {
		state = "— Update installation unavailable for development builds"
	} else if m.updateError != nil {
		state = "! " + m.updateError.Error()
	} else if m.updateResult.Available {
		state = "↑ " + displayVersion(m.updateResult.LatestVersion) + " is available"
	} else if !m.updateResult.CheckedAt.IsZero() {
		state = "✓ You're up to date"
	}
	checked := "Never"
	if !m.updateResult.CheckedAt.IsZero() {
		checked = m.updateResult.CheckedAt.Local().Format("2006-01-02 15:04")
	}
	return ansi.Wrap("About Inpakker\n\n"+m.branding()+"\nPackage Win32 applications for Microsoft Intune.\n\nVersion\n"+displayVersion(m.version)+"\n\nUpdate\n"+state+"\nLast checked: "+checked+"\n\nProject\nLickABrick/Inpakker · github.com/LickABrick/Inpakker\n\nLicense\nMIT", width, "")
}
func (m Model) workspacesView(width, height int) string {
	lines := []string{m.theme.PageTitle.Render("Workspaces"), ""}
	if m.workspaceSearching {
		lines = append(lines, m.workspaceSearch.View(), "")
	}
	views := m.filteredWorkspaces()
	visible := max(1, height-len(lines)-3)
	start := max(0, m.workspaceCursor-visible+1)
	for i := start; i < min(len(views), start+visible); i++ {
		view := views[i]
		status := "✓ Ready"
		if view.Status == "unavailable" {
			status = "! Path unavailable"
		}
		if view.Status == "invalid" {
			status = "X Invalid workspace"
		}
		prefix := "  "
		if i == m.workspaceCursor {
			prefix = "› "
		}
		line := prefix + view.Name + " · " + view.Path + " · " + status
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	if len(views) == 0 {
		lines = append(lines, "No registered workspaces. Create a workspace or add an existing one.")
	}
	lines = append(lines, "", "n Create workspace · a Add existing workspace")
	return strings.Join(lines, "\n")
}
func (m Model) settingsView(width, height int) string {
	lines := []string{m.theme.PageTitle.Render("Settings"), ""}
	rows := m.settingsRows()
	visible := max(1, height-5)
	start := max(0, m.settingCursor-visible+1)
	for i := start; i < min(len(rows), start+visible); i++ {
		row := rows[i]
		prefix := "  "
		if i == m.settingCursor {
			prefix = "› "
		}
		value := row.value
		if value == "" {
			value = "! Not configured"
		}
		scope := "Global"
		if row.scope == "workspace" {
			scope = "Workspace"
		}
		if row.scope == "tool" {
			scope = "External tools"
		}
		lines = append(lines, ansi.Truncate(prefix+scope+" · "+row.label+"   "+value, width, "…"))
	}
	return strings.Join(lines, "\n")
}
