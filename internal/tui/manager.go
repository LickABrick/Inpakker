package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/toolmanager"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/LickABrick/inpakker/types"
)

type registryMsg struct {
	generation int
	user       types.UserConfig
	views      []workspace.RegistrationView
	err        error
}
type workspaceActionMsg struct {
	ws     *workspace.Workspace
	action string
	err    error
}
type settingsMsg struct {
	ws   *workspace.Workspace
	user *types.UserConfig
	err  error
}
type toolsMsg struct {
	statuses    []toolmanager.Status
	user        *types.UserConfig
	err         error
	operationID int
}
type folderMsg struct{ err error }

type settingRow struct{ label, value, key, scope string }

func (m Model) registryCmd() tea.Cmd {
	generation := m.registryGeneration
	return func() tea.Msg {
		user, err := config.LoadUser()
		if err != nil {
			return registryMsg{generation: generation, err: err}
		}
		return registryMsg{generation: generation, user: *user, views: workspace.Registrations(user)}
	}
}
func (m Model) handleRegistry(msg registryMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.registryGeneration {
		return m, nil
	}
	if msg.err != nil {
		m.showError("Could not load workspaces", msg.err)
		return m, nil
	}
	m.user.Workspaces, m.user.ActiveWorkspaceID = msg.user.Workspaces, msg.user.ActiveWorkspaceID
	m.registrations = msg.views
	m.workspaceCursor = min(m.workspaceCursor, max(0, len(m.filteredWorkspaces())-1))
	return m, nil
}
func (m Model) filteredWorkspaces() []workspace.RegistrationView {
	query := strings.ToLower(strings.TrimSpace(m.workspaceSearch.Value()))
	result := []workspace.RegistrationView{}
	for _, view := range m.registrations {
		if query == "" || strings.Contains(strings.ToLower(view.Name+" "+view.Path), query) {
			result = append(result, view)
		}
	}
	return result
}
func (m Model) highlightedWorkspace() (workspace.RegistrationView, bool) {
	list := m.filteredWorkspaces()
	if m.workspaceCursor < 0 || m.workspaceCursor >= len(list) {
		return workspace.RegistrationView{}, false
	}
	return list[m.workspaceCursor], true
}
func (m Model) updateWorkspaceSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch pressed.Code {
		case tea.KeyEscape, tea.KeyEnter:
			m.workspaceSearching = false
			m.workspaceSearch.Blur()
			return m, nil
		case tea.KeyUp:
			m.workspaceCursor = max(0, m.workspaceCursor-1)
			return m, nil
		case tea.KeyDown:
			m.workspaceCursor = min(max(0, len(m.filteredWorkspaces())-1), m.workspaceCursor+1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.workspaceSearch, cmd = m.workspaceSearch.Update(msg)
	m.workspaceCursor = 0
	return m, cmd
}
func (m Model) updateWorkspaces(pressed tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(pressed, m.keys.Up):
		m.workspaceCursor = max(0, m.workspaceCursor-1)
		return m, nil
	case key.Matches(pressed, m.keys.Down):
		m.workspaceCursor = min(max(0, len(m.filteredWorkspaces())-1), m.workspaceCursor+1)
		return m, nil
	case key.Matches(pressed, m.keys.Search):
		m.workspaceSearching = true
		return m, m.workspaceSearch.Focus()
	case key.Matches(pressed, m.keys.Open):
		if view, ok := m.highlightedWorkspace(); ok {
			if view.Status != "ready" {
				m.showError("Workspace unavailable", fmt.Errorf("%s; use Relink", view.Error))
				return m, nil
			}
			return m, func() tea.Msg {
				ws, err := workspace.Use(view.ID)
				return workspaceActionMsg{ws: ws, action: "use", err: err}
			}
		}
		return m, nil
	case pressed.Text == "n":
		return m, m.beginWorkspaceCreate()
	case pressed.Text == "a":
		return m, m.beginWorkspacePath("add", "")
	case pressed.Text == "r":
		if view, ok := m.highlightedWorkspace(); ok {
			return m, m.beginWorkspacePath("relink", view.ID)
		}
		return m, nil
	case pressed.Text == "x":
		if view, ok := m.highlightedWorkspace(); ok {
			m.workspaceAction, m.workspaceTarget, m.accepted = "remove", view.ID, false
			m.modal, m.modalTitle = ModalWorkspaceForm, "Remove workspace"
			m.form = m.formWithTheme(huh.NewGroup(huh.NewConfirm().Key("accepted").Title("Remove " + view.Name + " from Inpakker?").Description("The workspace folder and its files will not be deleted.")))
			return m, m.form.Init()
		}
		return m, nil
	case pressed.Text == "e":
		if view, ok := m.highlightedWorkspace(); ok && view.Config != nil {
			m.editScope, m.editKey, m.editValue, m.workspaceTarget = "registered", "name", view.Name, view.Path
			return m, m.beginSettingForm()
		}
		return m, nil
	}
	return m.updateGlobalKey(pressed)
}
func (m *Model) beginWorkspaceCreate() tea.Cmd {
	defaults := m.user.WorkspaceDefaults
	m.workspaceDraft = &workspace.CreateWorkspaceOptions{ApplicationsDirectory: defaults.ApplicationsDirectory, SourceDirectory: defaults.SourceDirectory, OutputDirectory: defaults.OutputDirectory}
	m.workspaceAction = "create"
	m.modal, m.modalTitle = ModalWorkspaceForm, "Create workspace"
	m.pathInput = huh.NewInput().Title("Location · F2 browse").Value(&m.workspaceDraft.Root).Validate(huh.ValidateNotEmpty())
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewInput().Title("Name").Value(&m.workspaceDraft.Name).Validate(huh.ValidateNotEmpty()),
		m.pathInput,
		huh.NewInput().Title("Applications directory").Value(&m.workspaceDraft.ApplicationsDirectory).Validate(safeWorkspacePath),
		huh.NewInput().Title("Source directory").Value(&m.workspaceDraft.SourceDirectory).Validate(safeWorkspacePath),
		huh.NewInput().Title("Output directory").Value(&m.workspaceDraft.OutputDirectory).Validate(safeWorkspacePath),
		huh.NewNote().Title("Create workspace").Next(true).NextLabel("Create workspace"),
	))
	return m.form.Init()
}
func (m *Model) beginWorkspacePath(action, target string) tea.Cmd {
	m.workspaceAction, m.workspaceTarget, m.formPath = action, target, ""
	title := "Add existing workspace"
	if action == "relink" {
		title = "Relink workspace"
	}
	m.modal, m.modalTitle = ModalWorkspaceForm, title
	m.pathInput = huh.NewInput().Key("path").Title("Workspace directory · F2 browse").Description("Directory containing inpakker.workspace.json").Validate(huh.ValidateNotEmpty())
	m.form = m.formWithTheme(huh.NewGroup(m.pathInput, huh.NewNote().Title(title).Next(true).NextLabel(title)))
	return m.form.Init()
}
func (m Model) completeManagerForm(formCmd tea.Cmd) (tea.Model, tea.Cmd) {
	switch m.modal {
	case ModalWorkspaceForm:
		action, target, path, accepted := m.workspaceAction, m.workspaceTarget, m.form.GetString("path"), m.form.GetBool("accepted")
		options := workspace.CreateWorkspaceOptions{}
		if m.workspaceDraft != nil {
			options = *m.workspaceDraft
		}
		m.closeModal()
		if action == "remove" && !accepted {
			return m, nil
		}
		m.modal, m.modalTitle, m.modalBody = ModalProgress, "Updating workspaces", "Saving workspace registration…"
		return m, tea.Sequence(formCmd, func() tea.Msg {
			var ws *workspace.Workspace
			var err error
			switch action {
			case "create":
				ws, err = workspace.CreateWorkspace(options)
			case "add":
				ws, err = workspace.Add(path)
				if err == nil {
					ws, err = workspace.Use(ws.Config.ID)
				}
			case "use":
				ws, err = workspace.Use(target)
			case "remove":
				err = workspace.Remove(target)
			case "relink":
				err = workspace.Relink(target, path)
			}
			return workspaceActionMsg{ws: ws, action: action, err: err}
		})
	case ModalSetting:
		scope, setting, value, target := m.editScope, m.editKey, m.form.GetString("value"), m.workspaceTarget
		ws := m.workspace
		m.closeModal()
		m.modal, m.modalTitle, m.modalBody = ModalProgress, "Saving settings", "Saving configuration…"
		return m, tea.Sequence(formCmd, func() tea.Msg {
			var err error
			if scope == "global" {
				err = config.UpdateUser(func(user *types.UserConfig) error { return config.SetUserValue(user, setting, value) })
			} else {
				if scope == "registered" {
					ws, err = workspace.Open(target)
				}
				if err == nil && ws != nil {
					err = ws.SetSetting(setting, value)
					if err == nil {
						ws, err = workspace.Open(ws.Root)
					}
				}
			}
			user, userErr := config.LoadUser()
			if err == nil {
				err = userErr
			}
			return settingsMsg{ws: ws, user: user, err: err}
		})
	case ModalTool:
		id, action, path, accepted := m.toolID, m.toolAction, m.form.GetString("path"), m.form.GetBool("accepted")
		m.closeModal()
		if action == "install" && !accepted {
			return m, nil
		}
		if action == "install" {
			op := m.startOperation(operationUpdate, "Installing external tool", 1)
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				_, err := (toolmanager.Service{}).Install(op.context, id, accepted)
				user, userErr := config.LoadUser()
				if err == nil {
					err = userErr
				}
				return toolsMsg{user: user, err: err, operationID: op.id}
			})
		}
		return m, func() tea.Msg {
			err := toolmanager.Set(id, path)
			user, userErr := config.LoadUser()
			if err == nil {
				err = userErr
			}
			return toolsMsg{user: user, err: err}
		}
	}
	return m, nil
}
func (m Model) handleWorkspaceAction(msg workspaceActionMsg) (tea.Model, tea.Cmd) {
	m.closeModal()
	if msg.err != nil {
		m.showError("Workspace action failed", msg.err)
		return m, nil
	}
	m.registryGeneration++
	if msg.ws != nil {
		m.workspace, m.root = msg.ws, msg.ws.Root
		m.generation++
		m.refreshing = true
		m.groups = nil
		m.diagnostics = nil
		m.apps.search.SetValue("")
		m.apps.searching = false
		m.apps.refresh(nil, "")
		m.routes = []Route{{Kind: RouteApplications}}
		return m, tea.Batch(m.registryCmd(), m.inventoryCmd(""), m.spinner.Tick)
	}
	if msg.action == "remove" && m.workspace != nil && m.workspace.Config.ID == m.workspaceTarget {
		m.workspace = nil
		m.generation++
		m.refreshing = false
		m.apps.refresh(nil, "")
		m.groups = nil
		m.diagnostics = nil
	}
	return m, m.registryCmd()
}
func (m Model) settingsRows() []settingRow {
	d := m.user.WorkspaceDefaults
	rows := []settingRow{
		{"Show tool output", fmt.Sprint(m.user.Preferences.ShowToolOutput), "preferences.showToolOutput", "global"},
		{"Applications directory", d.ApplicationsDirectory, "workspaceDefaults.applicationsDirectory", "global"},
		{"Source directory", d.SourceDirectory, "workspaceDefaults.sourceDirectory", "global"},
		{"Output directory", d.OutputDirectory, "workspaceDefaults.outputDirectory", "global"},
	}
	if m.workspace != nil {
		cfg := m.workspace.Config
		rows = append(rows,
			settingRow{"Name", cfg.Name, "name", "workspace"}, settingRow{"Applications directory", cfg.ApplicationsDirectory, "applicationsDirectory", "workspace"}, settingRow{"Source directory", cfg.SourceDirectory, "sourceDirectory", "workspace"}, settingRow{"Output directory", cfg.OutputDirectory, "outputDirectory", "workspace"})
	}
	rows = append(rows, settingRow{"Content Prep Tool", m.user.Tools.ContentPrepTool.Path, "content-prep", "tool"}, settingRow{"Package decoder · Optional", m.user.Tools.Decoder.Path, "decoder", "tool"})
	return rows
}
func (m Model) updateSettings(pressed tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.settingsRows()
	m.settingCursor = min(m.settingCursor, len(rows)-1)
	switch {
	case key.Matches(pressed, m.keys.Up):
		m.settingCursor = max(0, m.settingCursor-1)
		return m, nil
	case key.Matches(pressed, m.keys.Down):
		m.settingCursor = min(len(rows)-1, m.settingCursor+1)
		return m, nil
	case pressed.Text == "d":
		if !m.detecting {
			m.detecting = true
			return m, tea.Batch(m.spinner.Tick, m.detectToolsCmd())
		}
		return m, nil
	case key.Matches(pressed, m.keys.Open):
		row := rows[m.settingCursor]
		if row.scope == "tool" {
			m.toolID = row.key
			return m, m.beginToolForm("choose")
		}
		m.editScope, m.editKey, m.editValue = row.scope, row.key, row.value
		return m, m.beginSettingForm()
	case pressed.Text == "I":
		if row := rows[m.settingCursor]; row.scope == "tool" {
			m.toolID = row.key
			return m, m.beginToolForm("install")
		}
		return m, nil
	case pressed.Text == "x":
		if row := rows[m.settingCursor]; row.scope == "tool" {
			id := row.key
			return m, func() tea.Msg {
				err := toolmanager.Set(id, "")
				user, _ := config.LoadUser()
				return toolsMsg{user: user, err: err}
			}
		}
		return m, nil
	case pressed.Text == "a":
		row := rows[m.settingCursor]
		for _, status := range m.toolStatuses {
			if status.ID == row.key && status.Candidate != "" {
				id, path := row.key, status.Candidate
				return m, func() tea.Msg {
					err := toolmanager.Set(id, path)
					user, _ := config.LoadUser()
					return toolsMsg{user: user, err: err}
				}
			}
		}
		return m, nil
	}
	return m.updateGlobalKey(pressed)
}
func (m *Model) beginSettingForm() tea.Cmd {
	value := m.editValue
	description := ""
	if m.editScope != "global" && m.editKey != "name" {
		description = "Applications using this workspace default will use the new directory. Existing files will not be moved automatically."
	}
	m.modal, m.modalTitle = ModalSetting, "Edit setting"
	m.form = m.formWithTheme(huh.NewGroup(huh.NewInput().Key("value").Title(m.editKey).Description(description).Value(&value).Validate(huh.ValidateNotEmpty()), huh.NewNote().Title("Save setting").Next(true).NextLabel("Save setting")))
	return m.form.Init()
}
func (m Model) handleSettings(msg settingsMsg) (tea.Model, tea.Cmd) {
	m.closeModal()
	if msg.err != nil {
		m.showError("Could not save settings", msg.err)
		return m, nil
	}
	if msg.user != nil {
		m.user = *msg.user
	}
	m.registryGeneration++
	if msg.ws != nil && m.workspace != nil && msg.ws.Config.ID == m.workspace.Config.ID {
		m.workspace = msg.ws
	}
	if m.workspace != nil {
		copy := *m.workspace
		copy.User = m.user
		m.workspace = &copy
		m.generation++
		m.refreshing = true
		return m, tea.Batch(m.inventoryCmd(""), m.registryCmd(), m.spinner.Tick)
	}
	return m, m.registryCmd()
}
func (m *Model) beginToolForm(action string) tea.Cmd {
	m.toolAction, m.accepted, m.formPath = action, false, ""
	definition, _ := toolmanager.DefinitionFor(m.toolID)
	m.modal, m.modalTitle = ModalTool, definition.Name
	if action == "install" {
		m.form = m.formWithTheme(huh.NewGroup(huh.NewNote().Description("Provided by "+definition.Repository+" under separate upstream terms.\nSource/license: "+definition.LicenseURL), huh.NewConfirm().Key("accepted").Title("Accept and download?").Affirmative("Accept and download").Negative("Configure later")))
	} else {
		m.pathInput = huh.NewInput().Key("path").Title("Executable path · F2 browse").Validate(huh.ValidateNotEmpty())
		m.form = m.formWithTheme(huh.NewGroup(m.pathInput, huh.NewNote().Title("Choose existing executable").Next(true).NextLabel("Save path")))
	}
	return m.form.Init()
}
func (m Model) detectToolsCmd() tea.Cmd {
	root := ""
	if m.workspace != nil {
		root = m.workspace.Root
	}
	return func() tea.Msg {
		user, err := config.LoadUser()
		if err != nil {
			return toolsMsg{err: err}
		}
		statuses, err := (toolmanager.Service{WorkspaceRoot: root}).Detect(m.ctx, user)
		if err == nil {
			err = toolmanager.ConfigureDetected(statuses)
		}
		user, userErr := config.LoadUser()
		if err == nil {
			err = userErr
		}
		return toolsMsg{statuses: statuses, user: user, err: err}
	}
}
func (m Model) handleTools(msg toolsMsg) (tea.Model, tea.Cmd) {
	if msg.operationID != 0 && !m.finishOperation(msg.operationID) {
		return m, nil
	}
	m.detecting = false
	if msg.err != nil {
		m.showError("External tools", msg.err)
		return m, nil
	}
	if msg.user != nil {
		m.user = *msg.user
	}
	if msg.statuses != nil {
		m.toolStatuses = msg.statuses
	}
	if m.workspace != nil {
		copy := *m.workspace
		copy.User = m.user
		m.workspace = &copy
		m.refreshing = false
		return m, m.refreshCmd("")
	}
	return m, nil
}
func (m Model) folderTarget() string {
	switch m.currentRoute().Kind {
	case RouteWorkspaces:
		if view, ok := m.highlightedWorkspace(); ok && view.Status == "ready" {
			return view.Path
		}
	case RouteApplications, RouteApplication:
		if m.workspace != nil {
			if app, ok := m.selectedApplication(); ok {
				return app.App.Ref.Path
			}
		}
	case RouteDiagnostics:
		if m.workspace != nil {
			return m.workspace.Root
		}
	case RouteSettings:
		rows := m.settingsRows()
		if m.workspace != nil && m.settingCursor < len(rows) && rows[m.settingCursor].scope == "workspace" {
			return m.workspace.Root
		}
	}
	return ""
}
func (m Model) openFolderCmd() tea.Cmd {
	path := m.folderTarget()
	if path == "" || m.opener == nil {
		return nil
	}
	opener := m.opener
	return func() tea.Msg { return folderMsg{err: opener.OpenDirectory(path)} }
}
func (m Model) manualUpdateCmd() tea.Cmd {
	service := m.updater
	if service == nil {
		service = updater.New(m.version)
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
		defer cancel()
		result, err := service.Check(ctx, false)
		return updateCheckMsg{result: result, err: err}
	}
}

func inheritedLabel(value string, inherited bool) string {
	if inherited {
		return value + " · inherited"
	}
	return value + " · app override"
}
func shortPath(path string) string {
	if path == "" {
		return "Not configured"
	}
	return filepath.Clean(path)
}
