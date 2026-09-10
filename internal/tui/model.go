package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
)

const (
	minimumWidth  = 60
	minimumHeight = 18
)

type ModalKind int

const (
	ModalNone ModalKind = iota
	ModalMessage
	ModalHelp
	ModalNewApplication
	ModalSetup
	ModalBuildOptions
	ModalPackageSelect
	ModalUpdate
	ModalProgress
)

type operationKind int

const (
	operationBuild operationKind = iota
	operationValidate
	operationUnpack
	operationUpdate
)

type operationState struct {
	id      int
	kind    operationKind
	title   string
	context context.Context
	cancel  context.CancelFunc
	events  chan activityEvent
	current activityEvent
	started time.Time
}

type activityEvent struct {
	current int
	total   int
	label   string
	phase   string
}

type activityMsg struct {
	id int
	activityEvent
}

type activityClosedMsg struct{ id int }

type inventoryMsg struct {
	apps      []ApplicationView
	preferred string
	err       error
}

type setupMsg struct {
	result       workspace.SetupResult
	apps         []ApplicationView
	err          error
	inventoryErr error
}

type createMsg struct {
	ref          workspace.AppRef
	apps         []ApplicationView
	err          error
	inventoryErr error
}

type updateCheckMsg struct{ result updater.Result }

type validationIssue struct {
	AppID string
	Name  string
	Issue string
}

type validationDoneMsg struct {
	id     int
	valid  int
	issues []validationIssue
	apps   []ApplicationView
	all    bool
	err    error
}

type buildDoneMsg struct {
	id      int
	results []packager.Result
	apps    []ApplicationView
	all     bool
	log     string
	err     error
}

type unpackFailure struct {
	Name  string
	Issue string
}

type unpackDoneMsg struct {
	id        int
	succeeded int
	outputs   []string
	failures  []unpackFailure
	apps      []ApplicationView
	all       bool
	err       error
}

type updateDoneMsg struct {
	id      int
	version string
	err     error
}

type resultRow struct {
	Name   string
	Status string
	Detail string
	AppID  string
}

type Model struct {
	ctx       context.Context
	root      string
	workspace *workspace.Workspace
	runner    process.Runner
	updater   *updater.Service
	version   string

	width, height int
	theme         Theme
	keys          KeyMap
	help          help.Model
	apps          applicationsPage
	routes        []Route
	updateResult  updater.Result

	modal           ModalKind
	modalTitle      string
	modalBody       string
	modalErr        error
	form            *huh.Form
	create          *workspace.CreateOptions
	setup           *workspace.SetupOptions
	createConfirmed bool
	setupConfirmed  bool
	updateConfirmed bool
	buildMode       string
	selectedPackage string

	operation       *operationState
	nextOperationID int
	spinner         spinner.Model
	progress        progress.Model

	diagnostics      []workspace.Check
	validationValid  int
	validationIssues []validationIssue
	resultTitle      string
	resultRows       []resultRow
	resultCursor     int
	logTitle         string
	logText          string
	detailViewport   viewport.Model
	helpViewport     viewport.Model
	logViewport      viewport.Model
}

func New(ctx context.Context, root string, ws *workspace.Workspace, runner process.Runner, updateService *updater.Service, version string) (Model, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Model{}, fmt.Errorf("resolve TUI workspace root: %w", err)
	}
	theme := NewTheme(true)
	appPage := newApplicationsPage(theme)
	if ws != nil {
		views, err := inspectApplications(ws)
		if err != nil {
			return Model{}, err
		}
		appPage.refresh(views, "")
	}
	activitySpinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	activitySpinner.Style = theme.StatusWarning
	bar := progress.New(progress.WithColors(theme.BrandColor, theme.AccentColor), progress.WithWidth(36))
	helpModel := help.New()
	helpModel.Styles = theme.HelpStyles()
	logView := viewport.New(viewport.WithWidth(60), viewport.WithHeight(12))
	detailView := viewport.New(viewport.WithWidth(60), viewport.WithHeight(12))
	helpView := viewport.New(viewport.WithWidth(54), viewport.WithHeight(12))
	m := Model{
		ctx: ctx, root: absRoot, workspace: ws, runner: runner, updater: updateService, version: version,
		theme: theme, keys: DefaultKeyMap(), help: helpModel, apps: appPage,
		routes: []Route{{Kind: RouteApplications}}, spinner: activitySpinner, progress: bar,
		detailViewport: detailView, helpViewport: helpView, logViewport: logView,
	}
	if ws == nil {
		m.beginSetup()
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{tea.RequestBackgroundColor}
	if m.form != nil {
		commands = append(commands, m.form.Init())
	}
	if m.updater != nil {
		commands = append(commands, m.checkUpdateCmd())
	}
	return tea.Batch(commands...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		if m.form != nil {
			updated, cmd := m.form.Update(msg)
			m.form = updated.(*huh.Form)
			return m, cmd
		}
		return m, nil
	case tea.BackgroundColorMsg:
		m.applyTheme(NewTheme(msg.IsDark()))
		return m, nil
	case inventoryMsg:
		if msg.err != nil {
			m.showError("Refresh failed", msg.err)
			return m, nil
		}
		m.apps.refresh(msg.apps, msg.preferred)
		return m, nil
	case setupMsg:
		if msg.err != nil {
			m.showError("Setup failed", msg.err)
			return m, nil
		}
		m.workspace, m.root = msg.result.Workspace, msg.result.Workspace.Root
		m.apps.refresh(msg.apps, "")
		m.routes = []Route{{Kind: RouteApplications}}
		if msg.inventoryErr != nil {
			m.showError("Workspace created, but inventory refresh failed", msg.inventoryErr)
			return m, nil
		}
		m.showMessage("Workspace ready", "✓ Inpakker is ready. Your Applications dashboard has been loaded.")
		return m, nil
	case createMsg:
		if msg.err != nil {
			m.showError("Could not create application", msg.err)
			return m, nil
		}
		m.routes = []Route{{Kind: RouteApplications}}
		if msg.inventoryErr != nil {
			m.showError("Application created, but inventory refresh failed", msg.inventoryErr)
			return m, nil
		}
		m.apps.refresh(msg.apps, msg.ref.Relative)
		m.showMessage("Application created", "✓ "+msg.ref.Relative+" was created successfully.")
		return m, nil
	case updateCheckMsg:
		m.updateResult = msg.result
		return m, nil
	case activityMsg:
		if m.operation != nil && msg.id == m.operation.id {
			m.operation.current = msg.activityEvent
			return m, m.waitForActivity(msg.id)
		}
		return m, nil
	case activityClosedMsg:
		return m, nil
	case validationDoneMsg:
		return m.handleValidationDone(msg)
	case buildDoneMsg:
		return m.handleBuildDone(msg)
	case unpackDoneMsg:
		return m.handleUnpackDone(msg)
	case updateDoneMsg:
		return m.handleUpdateDone(msg)
	}

	if tick, ok := msg.(spinner.TickMsg); ok && m.operation != nil {
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(tick)
		return m, command
	}

	keyPress, isKey := msg.(tea.KeyPressMsg)
	if m.operation != nil {
		if isKey && key.Matches(keyPress, m.keys.Cancel) {
			m.cancelOperation()
			m.showMessage("Cancelled", "The operation was cancelled. No other Inpakker actions were closed.")
		}
		return m, nil
	}
	if m.modal != ModalNone {
		return m.updateModal(msg)
	}
	if m.apps.searching && m.currentRoute().Kind == RouteApplications {
		return m.updateSearch(msg)
	}
	if !isKey {
		return m.updatePage(msg)
	}

	if m.currentRoute().Kind == RouteApplications && (key.Matches(keyPress, m.keys.Up) || key.Matches(keyPress, m.keys.Down)) {
		return m.updatePage(msg)
	}
	if m.currentRoute().Kind == RouteApplication && key.Matches(keyPress, m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown) {
		return m.updatePage(msg)
	}
	if (m.currentRoute().Kind == RouteValidationResults || m.currentRoute().Kind == RouteBuildResults || m.currentRoute().Kind == RouteUnpackResults) &&
		(key.Matches(keyPress, m.keys.Up) || key.Matches(keyPress, m.keys.Down)) {
		if key.Matches(keyPress, m.keys.Up) && m.resultCursor > 0 {
			m.resultCursor--
		}
		if key.Matches(keyPress, m.keys.Down) && m.resultCursor < len(m.resultRows)-1 {
			m.resultCursor++
		}
		return m, nil
	}
	if m.currentRoute().Kind == RouteLogs {
		if key.Matches(keyPress, m.keys.Back, m.keys.Escape) {
			m.popRoute()
			return m, nil
		}
		return m.updatePage(msg)
	}
	return m.updateGlobalKey(keyPress)
}

func (m Model) updatePage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentRoute().Kind {
	case RouteApplications:
		var cmd tea.Cmd
		m.apps.table, cmd = m.apps.table.Update(msg)
		return m, cmd
	case RouteApplication:
		m.detailViewport.SetContent(m.applicationView(max(20, m.width-8)))
		var cmd tea.Cmd
		m.detailViewport, cmd = m.detailViewport.Update(msg)
		return m, cmd
	case RouteLogs:
		var cmd tea.Cmd
		m.logViewport, cmd = m.logViewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyPress, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyPress, m.keys.Escape):
			m.apps.searching = false
			m.apps.search.Blur()
			m.apps.search.SetValue("")
			m.apps.applyFilter("")
			return m, nil
		case key.Matches(keyPress, m.keys.Open):
			if app, ok := m.apps.selected(); ok {
				m.apps.searching = false
				m.apps.search.Blur()
				m.pushRoute(Route{Kind: RouteApplication, AppID: app.App.Ref.Relative})
			}
			return m, nil
		case key.Matches(keyPress, m.keys.Up, m.keys.Down):
			var cmd tea.Cmd
			m.apps.table, cmd = m.apps.table.Update(msg)
			return m, cmd
		}
	}
	before := m.apps.search.Value()
	updated, cmd := m.apps.search.Update(msg)
	m.apps.search = updated
	if before != m.apps.search.Value() {
		m.apps.applyFilter("")
	}
	return m, cmd
}

func (m Model) updateGlobalKey(pressed tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(pressed, m.keys.Help) {
		m.modal, m.modalTitle = ModalHelp, "Keyboard shortcuts"
		m.helpViewport.GotoTop()
		return m, nil
	}
	if key.Matches(pressed, m.keys.Back, m.keys.Escape) && m.popRoute() {
		return m, nil
	}
	if key.Matches(pressed, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(pressed, m.keys.Update) {
		return m, m.beginUpdate()
	}

	route := m.currentRoute()
	switch route.Kind {
	case RouteApplications:
		switch {
		case key.Matches(pressed, m.keys.Open):
			if app, ok := m.apps.selected(); ok {
				m.pushRoute(Route{Kind: RouteApplication, AppID: app.App.Ref.Relative})
			}
		case key.Matches(pressed, m.keys.Search):
			m.apps.searching = true
			return m, m.apps.search.Focus()
		case key.Matches(pressed, m.keys.NewApp):
			return m, m.beginCreate()
		case key.Matches(pressed, m.keys.Refresh):
			return m, m.refreshCmd("")
		case key.Matches(pressed, m.keys.Diagnostics):
			m.openDiagnostics()
		case key.Matches(pressed, m.keys.Build):
			return m.startBuild(false, packager.BuildOptions{})
		case key.Matches(pressed, m.keys.BuildAll):
			return m.startBuild(true, packager.BuildOptions{})
		case key.Matches(pressed, m.keys.BuildOptions):
			return m, m.beginBuildOptions()
		case key.Matches(pressed, m.keys.Validate):
			return m.startValidate(false)
		case key.Matches(pressed, m.keys.ValidateAll):
			return m.startValidate(true)
		case key.Matches(pressed, m.keys.Unpack):
			return m.startUnpack(false, "")
		case key.Matches(pressed, m.keys.UnpackAll):
			return m.startUnpack(true, "")
		}
	case RouteApplication:
		switch {
		case key.Matches(pressed, m.keys.Build):
			return m.startBuild(false, packager.BuildOptions{})
		case key.Matches(pressed, m.keys.BuildOptions):
			return m, m.beginBuildOptions()
		case key.Matches(pressed, m.keys.Validate):
			return m.startValidate(false)
		case key.Matches(pressed, m.keys.Unpack):
			return m.startUnpack(false, "")
		case key.Matches(pressed, m.keys.Diagnostics):
			m.openDiagnostics()
		case key.Matches(pressed, m.keys.Refresh):
			return m, m.refreshCmd(route.AppID)
		}
	case RouteDiagnostics:
		if key.Matches(pressed, m.keys.Refresh) {
			m.openDiagnostics()
		}
	case RouteValidationResults, RouteBuildResults, RouteUnpackResults:
		if key.Matches(pressed, m.keys.Open) && m.resultCursor >= 0 && m.resultCursor < len(m.resultRows) && m.resultRows[m.resultCursor].AppID != "" {
			m.pushRoute(Route{Kind: RouteApplication, AppID: m.resultRows[m.resultCursor].AppID})
		}
		if key.Matches(pressed, m.keys.Logs) && strings.TrimSpace(m.logText) != "" {
			m.pushRoute(Route{Kind: RouteLogs})
		}
	}
	return m, nil
}

func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form != nil {
		updated, cmd := m.form.Update(msg)
		m.form = updated.(*huh.Form)
		if m.form.State == huh.StateAborted {
			wasSetup := m.modal == ModalSetup
			m.closeModal()
			if wasSetup && m.workspace == nil {
				return m, tea.Quit
			}
			return m, cmd
		}
		if m.form.State == huh.StateCompleted {
			return m.completeForm(cmd)
		}
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if m.modal == ModalHelp && key.Matches(pressed, m.keys.Up, m.keys.Down, m.keys.PageUp, m.keys.PageDown) {
			m.helpViewport.SetContent(m.helpContent())
			var cmd tea.Cmd
			m.helpViewport, cmd = m.helpViewport.Update(msg)
			return m, cmd
		}
		if m.modal == ModalMessage && key.Matches(pressed, m.keys.Diagnostics) {
			m.closeModal()
			m.openDiagnostics()
			return m, nil
		}
		if m.modal == ModalMessage && key.Matches(pressed, m.keys.Logs) && strings.TrimSpace(m.logText) != "" {
			m.closeModal()
			m.pushRoute(Route{Kind: RouteLogs})
			return m, nil
		}
		if key.Matches(pressed, m.keys.Escape, m.keys.Back, m.keys.Open, m.keys.Help) {
			m.closeModal()
		}
	}
	return m, nil
}

func (m Model) completeForm(formCmd tea.Cmd) (tea.Model, tea.Cmd) {
	switch m.modal {
	case ModalNewApplication:
		if !m.createConfirmed {
			m.closeModal()
			return m, formCmd
		}
		m.modal, m.form = ModalProgress, nil
		m.modalTitle, m.modalBody = "Creating application", "Preparing application workspace…"
		return m, tea.Sequence(formCmd, m.createCmd())
	case ModalSetup:
		if !m.setupConfirmed {
			m.closeModal()
			if m.workspace == nil {
				return m, tea.Quit
			}
			return m, formCmd
		}
		m.modal, m.form = ModalProgress, nil
		m.modalTitle, m.modalBody = "Setting up Inpakker", "Creating workspace and example application…"
		return m, tea.Sequence(formCmd, m.setupCmd())
	case ModalBuildOptions:
		mode := m.buildMode
		m.closeModal()
		options := packager.BuildOptions{Force: mode == "force", NoCache: mode == "no-cache"}
		model, cmd := m.startBuild(false, options)
		return model, tea.Sequence(formCmd, cmd)
	case ModalPackageSelect:
		selected := m.selectedPackage
		m.closeModal()
		model, cmd := m.startUnpack(false, selected)
		return model, tea.Sequence(formCmd, cmd)
	case ModalUpdate:
		if !m.updateConfirmed {
			m.closeModal()
			return m, formCmd
		}
		m.closeModal()
		model, cmd := m.startUpdate()
		return model, tea.Sequence(formCmd, cmd)
	}
	return m, formCmd
}

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height
	contentWidth := max(20, width-8)
	contentHeight := max(5, height-6)
	m.apps.resize(contentWidth, contentHeight)
	m.help.SetWidth(contentWidth)
	m.detailViewport.SetWidth(contentWidth)
	m.detailViewport.SetHeight(max(3, height-7))
	m.helpViewport.SetWidth(modalInnerWidth(width))
	m.helpViewport.SetHeight(max(4, min(14, height-10)))
	m.logViewport.SetWidth(contentWidth)
	m.logViewport.SetHeight(contentHeight - 3)
	m.progress.SetWidth(min(48, max(10, contentWidth-10)))
	if m.form != nil {
		m.form.WithWidth(modalInnerWidth(width)).WithHeight(formContentHeight(height))
	}
}

func (m *Model) applyTheme(theme Theme) {
	m.theme = theme
	m.help.Styles = theme.HelpStyles()
	m.apps.setTheme(theme)
	m.spinner.Style = theme.StatusWarning
	m.progress = progress.New(progress.WithColors(theme.BrandColor, theme.AccentColor), progress.WithWidth(min(48, max(10, m.width-14))))
	if m.form != nil {
		m.form.WithTheme(theme.HuhTheme())
	}
}

func (m *Model) closeModal() {
	m.modal, m.modalTitle, m.modalBody, m.modalErr, m.form = ModalNone, "", "", nil, nil
}

func (m *Model) showMessage(title, body string) {
	m.modal, m.modalTitle, m.modalBody, m.modalErr, m.form = ModalMessage, title, body, nil, nil
}

func (m *Model) showError(title string, err error) {
	m.modal, m.modalTitle, m.modalBody, m.modalErr, m.form = ModalMessage, title, "", err, nil
}

func (m Model) selectedApplication() (ApplicationView, bool) {
	if route := m.currentRoute(); route.Kind == RouteApplication {
		for _, app := range m.apps.all {
			if app.App.Ref.Relative == route.AppID {
				return app, true
			}
		}
	}
	return m.apps.selected()
}

func (m Model) targetApplications(all bool) ([]ApplicationView, error) {
	if all {
		if len(m.apps.all) == 0 {
			return nil, errors.New("no applications found")
		}
		return append([]ApplicationView(nil), m.apps.all...), nil
	}
	app, ok := m.selectedApplication()
	if !ok {
		return nil, errors.New("no application selected")
	}
	return []ApplicationView{app}, nil
}

func (m *Model) openDiagnostics() {
	m.diagnostics = workspace.Diagnose(m.root)
	m.pushRoute(Route{Kind: RouteDiagnostics})
}

func (m Model) refreshCmd(preferred string) tea.Cmd {
	ws := m.workspace
	return func() tea.Msg {
		if ws == nil {
			return inventoryMsg{err: errors.New("workspace is not initialized")}
		}
		apps, err := inspectApplications(ws)
		return inventoryMsg{apps: apps, preferred: preferred, err: err}
	}
}

func (m Model) checkUpdateCmd() tea.Cmd {
	service := m.updater
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 2*time.Second)
		defer cancel()
		result, err := service.Check(ctx, true)
		if err != nil {
			return updateCheckMsg{}
		}
		return updateCheckMsg{result: result}
	}
}
