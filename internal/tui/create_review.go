package tui

import (
	"fmt"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) showCreateReview() {
	m.modal, m.modalTitle, m.form = ModalCreateReview, "Review application", nil
	m.modalViewport.GotoTop()
}

func (m Model) createPreview() string {
	draft := m.create
	root := filepath.Join(m.workspace.AppsDir(), pathutil.Native(draft.Group), draft.DirectoryName)
	source := filepath.Join(root, pathutil.Native(m.workspace.Config.SourceDirectory))
	output := filepath.Join(root, pathutil.Native(m.workspace.Config.OutputDirectory))
	setup := filepath.Join(source, pathutil.Native(draft.SetupFile))
	body := fmt.Sprintf("Name: %s\n\nApplication\n%s\n\nSource folder\n%s\n\nSetup file\n%s\n\nOutput folder (created on build)\n%s", draft.Name, root, source, setup, output)
	if draft.SetupFrom != "" {
		body += "\n\nCopy installer from\n" + draft.SetupFrom + "\n\nOnly this file will be copied. Add any companion files to the source folder before building."
	} else {
		body += "\n\nAdd the setup file and any companion files to the source folder before building."
	}
	return body
}

func (m *Model) prepareCreatePreview() {
	width := modalInnerWidth(m.width)
	body := m.createPreview()
	if m.modalErr != nil {
		body = "X Could not create: " + m.modalErr.Error() + "\n\n" + body
	}
	m.modalViewport.SetWidth(width)
	m.modalViewport.SetHeight(max(1, m.height-12))
	m.modalViewport.SetContent(ansi.Wrap(body, width, ""))
}

func (m Model) createReviewView() string {
	m.prepareCreatePreview()
	return m.dialog(m.modalTitle, m.modalViewport.View()+scrollPosition(m.modalViewport))
}

func (m Model) updateCreateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Escape, m.keys.Back):
			m.modalErr = nil
			return m, m.beginCreateForm()
		case key.Matches(pressed, m.keys.Cancel):
			m.closeModal()
			return m, nil
		case key.Matches(pressed, m.keys.Open):
			m.modal, m.modalTitle, m.modalBody, m.modalErr = ModalProgress, "Creating application", "Preparing application workspace…", nil
			return m, m.createCmd()
		}
	}
	m.prepareCreatePreview()
	var cmd tea.Cmd
	m.modalViewport, cmd = m.modalViewport.Update(msg)
	return m, cmd
}
