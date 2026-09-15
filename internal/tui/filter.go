package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// A retained query and keyboard focus are independent state. Every page renders
// visible() while the query still affects its inventory.
type listFilter struct {
	input   textinput.Model
	editing bool
}

func (f listFilter) visible() bool { return f.editing || f.input.Value() != "" }
func (m *Model) otherFilter() *listFilter {
	switch m.currentRoute().Kind {
	case RouteSettings:
		return &m.settingsFilter
	case RouteDiagnostics:
		return &m.diagnosticsFilter
	}
	return nil
}
func (m *Model) focusOtherFilter() tea.Cmd {
	f := m.otherFilter()
	if f == nil {
		return nil
	}
	query := f.input.Value()
	f.input = textinput.New()
	f.input.Prompt = "Filter: "
	f.input.SetValue(query)
	f.input.SetWidth(max(10, m.width-18))
	setSearchTheme(&f.input, m.theme)
	f.editing = true
	return f.input.Focus()
}
func (m Model) updateOtherFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.otherFilter()
	if p, ok := msg.(tea.KeyPressMsg); ok {
		switch p.Code {
		case tea.KeyEscape:
			f.input.SetValue("")
			f.input.Blur()
			f.editing = false
			m.settingCursor = 0
			m.diagnosticCursor = 0
			return m, nil
		case tea.KeyEnter:
			f.input.Blur()
			f.editing = false
			return m.dispatchPageKey(p)
		case tea.KeyUp, tea.KeyDown:
			if m.currentRoute().Kind == RouteSettings {
				return m.updateSettings(p)
			}
			return m.updateDiagnostics(p)
		}
	}
	var cmd tea.Cmd
	before := f.input.Value()
	f.input, cmd = f.input.Update(msg)
	if before != f.input.Value() {
		if m.currentRoute().Kind == RouteSettings {
			m.settingCursor = 0
		} else {
			m.diagnosticCursor = 0
		}
	}
	return m, cmd
}
