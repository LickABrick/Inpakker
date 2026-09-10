package tui

import (
	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
)

func (m *Model) beginPicker() tea.Cmd {
	m.picker = filepicker.New()
	m.picker.CurrentDirectory = m.root
	m.picker.DirAllowed = m.modal == ModalWorkspaceForm
	m.picker.FileAllowed = !m.picker.DirAllowed
	if m.picker.FileAllowed {
		m.picker.AllowedTypes = []string{".exe"}
	}
	m.picker.SetHeight(max(4, m.height-12))
	m.picking = true
	return m.picker.Init()
}
func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if pressed, ok := msg.(tea.KeyPressMsg); ok && pressed.Code == tea.KeyEscape {
		m.picking = false
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if selected, path := m.picker.DidSelectFile(msg); selected {
		if m.modal == ModalWorkspaceForm && m.workspaceAction == "create" {
			m.workspaceDraft.Root = path
			m.pathInput.Value(&m.workspaceDraft.Root)
		} else {
			m.formPath = path
			m.pathInput.Value(&path)
		}
		m.picking = false
	}
	return m, cmd
}
