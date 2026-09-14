package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/toolmanager"
	"github.com/LickABrick/inpakker/types"
)

// Inspect the saved paths after a change without accepting discovered candidates.
// The snapshot prevents an older inspection replacing newer configuration state.
type toolInspectionMsg struct {
	root     string
	tools    types.Tools
	statuses []toolmanager.Status
}

func (m Model) inspectToolStatusCmd() tea.Cmd {
	user, root, ctx := m.user, m.root, m.ctx
	return func() tea.Msg {
		statuses, _ := (toolmanager.Service{WorkspaceRoot: root}).Detect(ctx, &user)
		return toolInspectionMsg{root: root, tools: user.Tools, statuses: statuses}
	}
}
