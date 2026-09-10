package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/sahilm/fuzzy"
)

type ApplicationView struct {
	App   workspace.App
	Group string
	Build packager.BuildInspection
}

type applicationsPage struct {
	table     table.Model
	search    textinput.Model
	theme     Theme
	all       []ApplicationView
	filtered  []ApplicationView
	searching bool
	width     int
	height    int
}

func newApplicationsPage(theme Theme) applicationsPage {
	tableKeys := table.DefaultKeyMap()
	tableKeys.PageUp = binding([]string{"pgup"}, "pgup", "page up")
	tableKeys.PageDown = binding([]string{"pgdown"}, "pgdn", "page down")
	tableKeys.HalfPageUp = key.NewBinding(key.WithDisabled())
	tableKeys.HalfPageDown = key.NewBinding(key.WithDisabled())
	styles := table.DefaultStyles()
	styles.Header, styles.Cell, styles.Selected = theme.TableHeader, theme.TableCell, theme.TableSelected
	t := table.New(table.WithFocused(true), table.WithKeyMap(tableKeys), table.WithStyles(styles))
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "name, group, path, or state"
	search.CharLimit = 120
	setSearchTheme(&search, theme)
	return applicationsPage{table: t, search: search, theme: theme}
}

func inspectApplications(ws *workspace.Workspace) ([]ApplicationView, error) {
	apps, err := ws.List()
	if err != nil {
		return nil, err
	}
	service := packager.Service{Workspace: ws}
	views := make([]ApplicationView, 0, len(apps))
	for _, app := range apps {
		group := filepath.Dir(app.Ref.Relative)
		if group == "." {
			group = "—"
		}
		views = append(views, ApplicationView{App: app, Group: group, Build: service.InspectBuildState(app.Ref)})
	}
	return views, nil
}

func (p *applicationsPage) setTheme(theme Theme) {
	p.theme = theme
	setSearchTheme(&p.search, theme)
	styles := table.DefaultStyles()
	styles.Header, styles.Cell, styles.Selected = theme.TableHeader, theme.TableCell, theme.TableSelected
	p.table.SetStyles(styles)
	p.rebuildTable()
}

func setSearchTheme(input *textinput.Model, theme Theme) {
	styles := input.Styles()
	styles.Focused.Prompt = theme.Brand
	styles.Focused.Text = theme.Text
	styles.Focused.Placeholder = theme.TextMuted
	styles.Focused.Suggestion = theme.TextMuted
	styles.Blurred = styles.Focused
	styles.Cursor.Color = theme.BrandColor
	input.SetStyles(styles)
}

func (p *applicationsPage) resize(width, height int) {
	p.width, p.height = max(1, width), max(3, height)
	p.search.SetWidth(max(10, width-10))
	p.table.SetWidth(width)
	p.rebuildTable()
}

func (p *applicationsPage) refresh(apps []ApplicationView, preferredID string) {
	if preferredID == "" {
		if selected, ok := p.selected(); ok {
			preferredID = selected.App.Ref.Relative
		}
	}
	p.all = append([]ApplicationView(nil), apps...)
	p.applyFilter(preferredID)
}

func (p *applicationsPage) applyFilter(preferredID string) {
	query := strings.TrimSpace(p.search.Value())
	if query == "" {
		p.filtered = append([]ApplicationView(nil), p.all...)
	} else {
		candidates := make([]string, len(p.all))
		for i, app := range p.all {
			candidates[i] = searchText(app)
		}
		matches := fuzzy.Find(query, candidates)
		p.filtered = make([]ApplicationView, 0, len(matches))
		for _, match := range matches {
			p.filtered = append(p.filtered, p.all[match.Index])
		}
	}
	p.rebuildTable()
	if preferredID != "" {
		for i, app := range p.filtered {
			if app.App.Ref.Relative == preferredID {
				p.table.SetCursor(i)
				break
			}
		}
	}
}

func (p *applicationsPage) rebuildTable() {
	// Bubbles renders immediately from SetColumns and SetRows. Clear the old
	// shape first so a responsive column change cannot index a stale row.
	cursor := p.table.Cursor()
	p.table.SetRows(nil)
	p.table.SetColumns(applicationColumns(p.width))
	rows := make([]table.Row, 0, len(p.filtered))
	for _, app := range p.filtered {
		rows = append(rows, p.styledApplicationRow(app))
	}
	p.table.SetRows(rows)
	p.table.SetHeight(min(max(3, len(rows)+1), max(3, p.height-4)))
	if len(rows) > 0 {
		p.table.SetCursor(max(0, cursor))
	}
}

func (p applicationsPage) styledApplicationRow(app ApplicationView) table.Row {
	row := applicationRow(app, p.width)
	validation := p.theme.StatusSuccess.Render(validationLabel(app.App))
	if app.App.Status != "valid" {
		validation = p.theme.StatusError.Render(validationLabel(app.App))
	}
	build := buildLabel(app.Build.State)
	switch app.Build.State {
	case packager.BuildStateCurrent:
		build = p.theme.StatusSuccess.Render(build)
	case packager.BuildStateNeedsBuild, packager.BuildStateUnknown:
		build = p.theme.StatusWarning.Render(build)
	case packager.BuildStateUnavailable:
		build = p.theme.StatusMuted.Render(build)
	default:
		build = p.theme.TextMuted.Render(build)
	}
	switch {
	case p.width >= 110:
		row[2], row[3] = validation, build
	case p.width >= 80:
		if app.App.Status != "valid" {
			row[2] = validation
		} else {
			row[2] = build
		}
	case app.App.Status != "valid":
		row[1] = validation
	default:
		row[1] = build
	}
	return row
}

func applicationColumns(width int) []table.Column {
	switch {
	case width >= 110:
		usable := max(50, width-14)
		return []table.Column{{Title: "APPLICATION", Width: max(20, usable-50)}, {Title: "GROUP", Width: 16}, {Title: "VALIDATION", Width: 12}, {Title: "BUILD", Width: 15}, {Title: "PACKAGE", Width: 7}}
	case width >= 80:
		usable := max(38, width-12)
		return []table.Column{{Title: "APPLICATION", Width: max(18, usable-38)}, {Title: "GROUP", Width: 14}, {Title: "STATE", Width: 17}, {Title: "PACKAGE", Width: 7}}
	default:
		usable := max(35, width-8)
		return []table.Column{{Title: "APPLICATION", Width: max(18, usable-17)}, {Title: "STATE", Width: 17}}
	}
}

func applicationRow(app ApplicationView, width int) table.Row {
	name := app.App.Label()
	validation := validationLabel(app.App)
	build := buildLabel(app.Build.State)
	packages := packageCount(len(app.App.Packages))
	switch {
	case width >= 110:
		return table.Row{name, app.Group, validation, build, packages}
	case width >= 80:
		state := build
		if app.App.Status != "valid" {
			state = validation
		}
		return table.Row{name, app.Group, state, packages}
	default:
		state := build
		if app.App.Status != "valid" {
			state = validation
		}
		return table.Row{name, state}
	}
}

func (p applicationsPage) selected() (ApplicationView, bool) {
	index := p.table.Cursor()
	if index < 0 || index >= len(p.filtered) {
		return ApplicationView{}, false
	}
	return p.filtered[index], true
}

func searchText(app ApplicationView) string {
	display := ""
	if app.App.Config != nil {
		display = app.App.Config.DisplayName
	}
	return strings.Join([]string{app.App.Label(), display, app.Group, app.App.Ref.Relative, app.App.Status, string(app.Build.State), validationLabel(app.App), buildLabel(app.Build.State)}, " ")
}

func validationLabel(app workspace.App) string {
	if app.Status == "valid" {
		return "✓ Valid"
	}
	return "X Invalid"
}

func buildLabel(state packager.BuildState) string {
	switch state {
	case packager.BuildStateCurrent:
		return "✓ Current"
	case packager.BuildStateNeedsBuild:
		return "• Needs build"
	case packager.BuildStateNotBuilt:
		return "○ Not built"
	case packager.BuildStateUnavailable:
		return "— Unavailable"
	default:
		return "! Unknown"
	}
}

func packageCount(count int) string {
	if count == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", count)
}
