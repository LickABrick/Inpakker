package tui

import (
	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
)

func (m *Model) beginPicker() tea.Cmd {
	m.picker = filepicker.New()
	m.picker.KeyMap.Back.SetHelp("←", "parent directory")
	m.picker.KeyMap.Open.SetHelp("→", "directory")
	m.picker.KeyMap.Up = m.keys.Up
	m.picker.KeyMap.Down = m.keys.Down
	m.picker.KeyMap.PageUp = m.keys.PageUp
	m.picker.KeyMap.PageDown = m.keys.PageDown
	m.picker.CurrentDirectory = m.root
	if m.pathInput != nil {
		path, _ := m.pathInput.GetValue().(string)
		if m.modal == ModalNewApplication {
			path = m.create.SetupFrom
		}
		if filepath.IsAbs(path) {
			m.picker.CurrentDirectory = path
		}
	}
	// Start at the nearest existing directory for a new path or a filename.
	for {
		info, err := os.Stat(m.picker.CurrentDirectory)
		if err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(m.picker.CurrentDirectory)
		if parent == m.picker.CurrentDirectory {
			break
		}
		m.picker.CurrentDirectory = parent
	}
	m.picker.DirAllowed = m.modal == ModalWorkspaceForm
	m.picker.FileAllowed = !m.picker.DirAllowed
	if m.modal == ModalTool {
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
		if m.modal == ModalNewApplication {
			m.create.SetupFrom, m.create.SetupFile = path, filepath.Base(path)
			m.pathInput.Value(&m.create.SetupFile)
		} else if m.modal == ModalWorkspaceForm && m.workspaceAction == "create" {
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

func (m Model) pickerTitle() string {
	switch m.modal {
	case ModalWorkspaceForm:
		return "Choose workspace folder"
	case ModalNewApplication:
		return "Choose installer"
	case ModalTool:
		if m.toolID == "decoder" {
			return "Choose package decoder"
		}
		return "Choose Content Prep Tool"
	}
	return "Choose directory"
}
