package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
)

// A session-local snapshot keeps result actions tied to the operation's workspace,
// rather than whichever application or workspace is selected later.
type operationResult struct {
	kind                 operationKind
	workspaceID, root    string
	modal                ModalKind
	title, body, summary string
	err                  error
	rows                 []resultRow
	logTitle, log        string
	options              packager.BuildOptions
	packagePath          string
	retryIDs             []string
	targetUUIDs          map[string]string
}

func (m Model) rememberResult(updated tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	n := updated.(Model)
	if m.operation == nil || n.operation != nil {
		return n, cmd
	}
	op := m.operation
	r := &operationResult{kind: op.kind, modal: n.modal, title: n.modalTitle, body: n.modalBody, err: n.modalErr,
		summary: n.resultTitle, rows: append([]resultRow(nil), n.resultRows...), options: op.buildOptions, packagePath: op.packagePath}
	if m.workspace != nil && op.kind != operationTool && op.kind != operationUpdate {
		r.workspaceID, r.root = m.workspace.Config.ID, m.workspace.Root
	}
	r.targetUUIDs = make(map[string]string)
	for _, target := range op.targets {
		if target.App.Config != nil {
			r.targetUUIDs[target.App.Ref.Relative] = target.App.Config.ID
		}
	}
	if n.modalHasLog {
		r.logTitle, r.log = n.logTitle, n.logText
	}
	seen := make(map[string]bool)
	for _, row := range r.rows {
		if row.Failed && row.AppID != "" && !seen[row.AppID] {
			r.retryIDs = append(r.retryIDs, row.AppID)
			seen[row.AppID] = true
		}
	}
	n.lastResult, n.displayedResult = r, r
	return n, cmd
}

func (m Model) resultWorkspaceMatches(r *operationResult) bool {
	return r != nil && (r.workspaceID == "" || (m.workspace != nil && m.workspace.Config.ID == r.workspaceID && m.workspace.Root == r.root))
}

func (m *Model) reopenResult() {
	r := m.lastResult
	if !m.resultWorkspaceMatches(r) {
		m.showMessage("No result available", "Run an operation in this workspace to view its result here.")
		return
	}
	m.closeModal()
	m.modal, m.modalTitle, m.modalBody, m.modalErr = r.modal, r.title, r.body, r.err
	m.resultRows, m.resultTitle, m.resultCursor = append([]resultRow(nil), r.rows...), r.summary, 0
	m.setLog(r.logTitle, r.log)
	m.modalHasLog = r.log != ""
	m.modalViewport.GotoTop()
	m.displayedResult = r
}

func (m Model) resultFolder() string {
	if !m.resultWorkspaceMatches(m.displayedResult) || len(m.resultRows) == 0 {
		return ""
	}
	index := m.resultCursor
	if index < 0 || index >= len(m.resultRows) {
		return ""
	}
	return m.resultRows[index].OutputDir
}

func (m Model) openResultFolder() tea.Cmd {
	path, opener := m.resultFolder(), m.opener
	if path == "" || opener == nil {
		return nil
	}
	return func() tea.Msg { return folderMsg{err: opener.OpenDirectory(path)} }
}

func (m Model) retryFailed() (tea.Model, tea.Cmd) {
	r := m.displayedResult
	if !m.resultWorkspaceMatches(r) {
		return m, nil
	}
	targets := make([]ApplicationView, 0, len(r.retryIDs))
	for _, id := range r.retryIDs {
		found := false
		for _, app := range m.apps.all {
			matches := app.App.Ref.Relative == id
			if uuid := r.targetUUIDs[id]; uuid != "" {
				matches = app.App.Config != nil && app.App.Config.ID == uuid
			}
			if matches {
				targets = append(targets, app)
				found = true
				break
			}
		}
		if !found {
			m.showError("Retry unavailable", fmt.Errorf("application %s is no longer in this workspace; refresh first", id))
			return m, nil
		}
	}
	if len(targets) == 0 {
		return m, nil
	}
	m.closeModal()
	switch r.kind {
	case operationBuild:
		return m.buildTargets(targets, true, r.options)
	case operationValidate:
		return m.validateTargets(targets, true)
	case operationUnpack:
		return m.unpackTargets(targets, r.packagePath == "", r.packagePath)
	}
	return m, nil
}

// resultAppID resolves against the current inventory before exposing navigation.
func (m Model) resultAppID() string {
	if m.modal == ModalMessage && m.displayedResult == nil {
		return ""
	}
	if m.resultCursor < 0 || m.resultCursor >= len(m.resultRows) {
		return ""
	}
	if m.displayedResult != nil && !m.resultWorkspaceMatches(m.displayedResult) {
		return ""
	}
	id := m.resultRows[m.resultCursor].AppID
	for _, app := range m.apps.all {
		if app.App.Ref.Relative != id {
			continue
		}
		if m.displayedResult != nil {
			if uuid := m.displayedResult.targetUUIDs[id]; uuid != "" && (app.App.Config == nil || app.App.Config.ID != uuid) {
				return ""
			}
		}
		return id
	}
	return ""
}
