package tui

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
)

// ActionID identifies an operation independently of its shortcut or visible label.
type ActionID string

// actionForCommand bridges the existing help/palette descriptions to semantic actions.
// Keyboard dispatch can use the same mapping rather than manufacturing key events.
func actionForCommand(command string) ActionID {
	// Bulk labels carry the complete inventory count, not just filtered rows.
	if base, _, ok := strings.Cut(command, " ("); ok {
		switch base {
		case "Build all applications", "Validate all applications", "Unpack all applications":
			command = base
		}
	}
	switch command {
	case "Verify all build states":
		return "verify-build-states"
	case "Build selected application":
		return "build"
	case "Build all applications":
		return "build-all"
	case "Build options":
		return "build-options"
	case "Validate selected application":
		return "validate"
	case "Validate all applications":
		return "validate-all"
	case "Unpack selected application":
		return "unpack"
	case "Unpack all applications":
		return "unpack-all"
	case "Application actions", "Workspace actions":
		return "actions"
	case "New application":
		return "create-app"
	case "Create workspace":
		return "workspace-create"
	case "Add workspace":
		return "workspace-add"
	case "Switch workspace":
		return "workspace-enter"
	case "Relink workspace":
		return "workspace-r"
	case "Remove workspace registration":
		return "workspace-x"
	case "Rename workspace":
		return "workspace-e"
	case "Refresh applications":
		return "refresh"
	case "Workspace diagnostics":
		return "diagnostics"
	case "Refresh workspace diagnostics":
		return "refresh"
	case "Open diagnostic details":
		return "diagnostic-details"
	case "Open folder":
		return "folder"
	case "Applications":
		return "applications"
	case "Workspaces":
		return "workspaces"
	case "Settings":
		return "settings"
	case "About":
		return "about"
	case "Last operation result":
		return "last-result"
	case "Check for updates":
		return "check-update"
	case "Install update":
		return "install-update"
	case "Edit selected setting":
		return "setting-edit"
	case "Detect tools":
		return "tools-detect"
	case "Tool actions":
		return "tool-actions"
	case "Download selected tool":
		return "tool-download"
	case "Clear selected tool":
		return "tool-clear"
	}
	return ""
}

// ExecuteAction is shared by keyboard, contextual actions and command palette.
// Callers resolve and verify their captured target before dispatching here.
func (m Model) ExecuteAction(action ActionID) (tea.Model, tea.Cmd) {
	if m.operation != nil || m.modal == ModalProgress {
		return m, nil
	}
	switch action {
	case "verify-build-states":
		if m.workspace != nil {
			return m, m.verifyBuildStatesCmd()
		}
	case "build":
		return m.startBuild(false, packager.BuildOptions{})
	case "build-all":
		return m.startBuild(true, packager.BuildOptions{})
	case "build-options":
		return m, m.beginBuildOptions()
	case "validate":
		return m.startValidate(false)
	case "validate-all":
		return m.startValidate(true)
	case "unpack":
		return m.startUnpack(false, "")
	case "unpack-all":
		return m.startUnpack(true, "")
	case "actions":
		m.beginActions()
	case "create-app":
		return m, m.beginCreate()
	case "workspace-create":
		return m, m.beginWorkspaceCreate()
	case "workspace-add":
		return m, m.beginWorkspacePath("add", "")
	case "workspace-enter", "workspace-r", "workspace-x", "workspace-e":
		return m.executeWorkspaceAction(action)
	case "package-folder":
		if app, ok := m.selectedApplication(); ok && len(app.App.Packages) > 0 && m.opener != nil {
			path, opener := filepath.Dir(app.App.Packages[0]), m.opener
			return m, func() tea.Msg { return folderMsg{err: opener.OpenDirectory(path)} }
		}
	case "folder", "workspace-o":
		return m, m.openFolderCmd()
	case "refresh":
		return m, m.refreshCmd(m.currentRoute().AppID)
	case "diagnostics":
		if m.workspace != nil {
			m.openDiagnostics()
		}
	case "diagnostic-details":
		checks := m.filteredDiagnostics()
		if len(checks) > 0 {
			m.showMessage("Diagnostic details", diagnosticDetail(checks[min(m.diagnosticCursor, len(checks)-1)]))
		}
	case "applications":
		m.pushRoute(Route{Kind: RouteApplications})
	case "workspaces":
		m.pushRoute(Route{Kind: RouteWorkspaces})
		return m, m.registryCmd()
	case "settings":
		m.pushRoute(Route{Kind: RouteSettings})
	case "about":
		m.pushRoute(Route{Kind: RouteAbout})
	case "last-result":
		m.reopenResult()
	case "check-update":
		if !m.checkingUpdate {
			m.checkingUpdate = true
			return m, tea.Batch(m.spinner.Tick, m.manualUpdateCmd())
		}
	case "install-update":
		return m, m.beginUpdate()
	case "setting-edit", "tools-detect", "tool-actions", "tool-download", "tool-clear":
		return m.executeSettingAction(action)
	}
	return m, nil
}
