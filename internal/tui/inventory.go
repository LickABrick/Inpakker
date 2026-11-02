package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

type ApplicationView struct {
	App   workspace.App
	Group string
	Build packager.BuildInspection
}

type applicationsPage struct {
	table       table.Model
	search      textinput.Model
	theme       Theme
	all         []ApplicationView
	filtered    []ApplicationView
	searching   bool
	searchCache []string
	nameCounts  map[string]int
	positions   map[string]int
	width       int
	height      int
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
	search.Prompt = "Filter: "
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
	inspection := (packager.Service{Workspace: ws}).NewInspectionContext(context.Background())
	views := make([]ApplicationView, 0, len(apps))
	for _, app := range apps {
		group := filepath.Dir(app.Ref.Relative)
		if group == "." {
			group = "—"
		}
		views = append(views, ApplicationView{App: app, Group: group, Build: inspection.InspectBuildStateForApp(context.Background(), app, false)})
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
	// Build immutable search and identity indexes once per inventory snapshot,
	// not once per filter keystroke or once per rendered row.
	p.searchCache = make([]string, len(p.all))
	p.nameCounts = make(map[string]int, len(p.all))
	p.positions = make(map[string]int, len(p.all))
	for i, app := range p.all {
		p.searchCache[i] = searchText(app)
		p.nameCounts[app.App.Label()]++
		p.positions[app.App.Ref.Relative] = i + 1
	}
	p.applyFilter(preferredID)
}

func (p *applicationsPage) applyFilter(preferredID string) {
	if preferredID == "" {
		if app, ok := p.selected(); ok {
			preferredID = app.App.Ref.Relative
		}
	}
	query := strings.TrimSpace(p.search.Value())
	if query == "" {
		p.filtered = append([]ApplicationView(nil), p.all...)
	} else {
		matches := matchIndices(query, p.searchCache)
		p.filtered = make([]ApplicationView, 0, len(matches))
		for _, match := range matches {
			p.filtered = append(p.filtered, p.all[match])
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
	filterHeight := 0
	if p.searching || p.search.Value() != "" {
		filterHeight = 2
	}
	p.table.SetHeight(max(3, p.height-3-filterHeight))
	if len(rows) > 0 {
		p.table.SetCursor(min(len(rows)-1, max(0, cursor)))
	}
}

func (p applicationsPage) styledApplicationRow(app ApplicationView) table.Row {
	row := applicationRow(app, p.width)
	if p.width < 80 {
		if p.nameCounts[app.App.Label()] > 1 {
			prefix := fmt.Sprintf("%d · ", p.positions[app.App.Ref.Relative])
			path := app.App.Ref.Relative
			available := max(1, applicationColumns(p.width)[0].Width-ansi.StringWidth(prefix))
			if ansi.StringWidth(path) > available {
				path = ansi.TruncateLeft(path, ansi.StringWidth(path)-available+1, "…")
			}
			row[0] = prefix + path
		}
	}

	status := buildLabel(app.Build.State)
	style := p.theme.TextMuted
	switch app.Build.State {
	case packager.BuildStateCurrent:
		style = p.theme.StatusSuccess
	case packager.BuildStateNeedsBuild, packager.BuildStateUnknown:
		style = p.theme.StatusWarning
	case packager.BuildStateUnavailable:
		style = p.theme.StatusMuted
	}
	if app.App.Status != "valid" {
		status = validationLabel(app.App)
		style = p.theme.StatusError
	}
	row[len(row)-1] = style.Render(status)
	return row
}

func applicationColumns(width int) []table.Column {
	if width >= 80 {
		return []table.Column{{Title: "APPLICATION", Width: max(1, width-39)}, {Title: "GROUP", Width: 14}, {Title: "STATUS", Width: 17}}
	}
	return []table.Column{{Title: "APPLICATION", Width: max(1, width-25)}, {Title: "STATUS", Width: 17}}
}

func applicationRow(app ApplicationView, width int) table.Row {
	status := buildLabel(app.Build.State)
	if app.App.Status != "valid" {
		status = validationLabel(app.App)
	}
	if width >= 80 {
		return table.Row{app.App.Label(), app.Group, status}
	}
	return table.Row{app.App.Label(), status}
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
		display = app.App.Config.Name
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
		return "✓ Up to date"
	case packager.BuildStateNeedsVerification:
		return "! Verify state"
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
