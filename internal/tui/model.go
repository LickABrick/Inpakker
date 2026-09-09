package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/unpacker"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type screen int

const (
	listScreen screen = iota
	detailScreen
	createScreen
	resultScreen
)

type appItem struct{ app workspace.App }

func (i appItem) Title() string { return i.app.Label() }
func (i appItem) Description() string {
	group := filepath.Dir(i.app.Ref.Relative)
	if group == "." {
		group = "ungrouped"
	}
	return fmt.Sprintf("%s  |  %s  |  %d package(s)", group, i.app.Status, len(i.app.Packages))
}
func (i appItem) FilterValue() string {
	return strings.Join([]string{i.app.Label(), i.app.Ref.Relative, i.app.Status}, " ")
}

type operationMsg struct {
	title string
	body  string
	err   error
}

type refreshMsg struct {
	apps []workspace.App
	err  error
}

type activityEvent struct {
	current int
	total   int
	label   string
	phase   string
}

type activityMsg activityEvent
type activityClosedMsg struct{}

type Model struct {
	ctx             context.Context
	workspace       *workspace.Workspace
	runner          process.Runner
	list            list.Model
	apps            []workspace.App
	screen          screen
	busy            bool
	width           int
	height          int
	title           string
	body            string
	err             error
	form            *huh.Form
	create          *workspace.CreateOptions
	spinner         spinner.Model
	progress        progress.Model
	activity        chan activityEvent
	current         activityEvent
	activityContext context.Context
	cancelActivity  context.CancelFunc
}

var (
	headingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func New(ctx context.Context, ws *workspace.Workspace, runner process.Runner) (Model, error) {
	apps, err := ws.List()
	if err != nil {
		return Model{}, err
	}
	items := toItems(apps)
	applicationList := list.New(items, list.NewDefaultDelegate(), 80, 24)
	applicationList.Title = "Inpakker workspace"
	applicationList.SetStatusBarItemName("application", "applications")
	applicationList.SetShowHelp(false)
	activity := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	activity.Style = headingStyle
	bar := progress.New(progress.WithColors(lipgloss.Color("6")), progress.WithWidth(36))
	return Model{ctx: ctx, workspace: ws, runner: runner, list: applicationList, apps: apps, spinner: activity, progress: bar}, nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, max(8, msg.Height-2))
	case refreshMsg:
		m.busy = false
		if msg.err != nil {
			m.title, m.body, m.err, m.screen = "Refresh failed", "", msg.err, resultScreen
			return m, nil
		}
		m.apps = msg.apps
		return m, m.list.SetItems(toItems(msg.apps))
	case operationMsg:
		m.busy = false
		if m.cancelActivity != nil {
			m.cancelActivity()
		}
		m.activity = nil
		m.title, m.body, m.err, m.screen = msg.title, msg.body, msg.err, resultScreen
		return m, nil
	case activityMsg:
		m.current = activityEvent(msg)
		return m, m.waitForActivity()
	case activityClosedMsg:
		return m, nil
	}
	if tick, ok := msg.(spinner.TickMsg); ok && m.busy {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tick)
		return m, cmd
	}

	key, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return m.updateComponent(msg)
	}
	if m.busy {
		if key.String() == "ctrl+c" || key.String() == "q" {
			if m.cancelActivity != nil {
				m.cancelActivity()
			}
			return m, tea.Quit
		}
		return m, nil
	}

	switch m.screen {
	case listScreen:
		if m.list.SettingFilter() {
			return m.updateComponent(msg)
		}
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if app, ok := m.selected(); ok {
				m.title, m.body, m.err, m.screen = app.Label(), detailBody(app), nil, detailScreen
			}
			return m, nil
		case "n":
			return m, m.beginCreate()
		case "r":
			m.busy = true
			return m, m.refreshCmd()
		case "d":
			return m.startDoctor()
		case "v":
			return m.startValidate(false)
		case "ctrl+v":
			return m.startValidate(true)
		case "b":
			return m.startBuild(false)
		case "ctrl+b":
			return m.startBuild(true)
		case "u":
			return m.startUnpack(false)
		case "ctrl+u":
			return m.startUnpack(true)
		}
	case detailScreen, resultScreen:
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "enter":
			m.screen, m.err = listScreen, nil
			return m, m.refreshCmd()
		}
	case createScreen:
		return m.updateComponent(msg)
	}
	return m.updateComponent(msg)
}

func (m Model) updateComponent(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.screen == createScreen {
		updated, cmd := m.form.Update(msg)
		m.form = updated.(*huh.Form)
		if m.form.State == huh.StateCompleted {
			m.busy = true
			return m, tea.Sequence(cmd, m.createCmd())
		}
		if m.form.State == huh.StateAborted {
			m.screen = listScreen
		}
		return m, cmd
	}
	if m.screen == listScreen {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case detailScreen, resultScreen:
		content = headingStyle.Render(m.title) + "\n\n"
		if m.err != nil {
			content += errorStyle.Render("Error: "+m.err.Error()) + "\n"
		}
		if m.body != "" {
			content += m.body + "\n"
		}
		content += "\n" + helpStyle.Render("enter/esc back  •  q quit")
	case createScreen:
		content = m.form.View()
	default:
		content = m.list.View() + "\n" + helpStyle.Render("enter details  •  / search  •  n create  •  d doctor  •  v/b/u selected  •  ctrl+v/b/u all  •  r refresh  •  q quit")
	}
	if m.busy {
		total := max(1, m.current.total)
		content += "\n\n" + m.progress.ViewAs(float64(m.current.current)/float64(total)) +
			fmt.Sprintf("  %d/%d\n", m.current.current, m.current.total) +
			m.spinner.View() + " " + m.current.phase + " " + m.current.label +
			"\n" + helpStyle.Render("ctrl+c cancel")
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m Model) startDoctor() (tea.Model, tea.Cmd) {
	checks := workspace.Diagnose(m.workspace.Root)
	lines := make([]string, 0, len(checks))
	for _, check := range checks {
		marker := "✓"
		if check.Status == workspace.CheckWarning {
			marker = "!"
		} else if check.Status == workspace.CheckFailure {
			marker = "X"
		}
		lines = append(lines, fmt.Sprintf("%s %s: %s", marker, check.Name, check.Detail))
	}
	m.title, m.body, m.err, m.screen = "Workspace doctor", strings.Join(lines, "\n"), nil, resultScreen
	return m, nil
}

func (m Model) selected() (workspace.App, bool) {
	item, ok := m.list.SelectedItem().(appItem)
	return item.app, ok
}

func (m Model) targets(all bool) ([]workspace.App, error) {
	if all {
		if len(m.apps) == 0 {
			return nil, errors.New("no applications found")
		}
		return append([]workspace.App(nil), m.apps...), nil
	}
	app, ok := m.selected()
	if !ok {
		return nil, errors.New("no application selected")
	}
	return []workspace.App{app}, nil
}

func (m Model) startValidate(all bool) (tea.Model, tea.Cmd) {
	targets, err := m.targets(all)
	if err != nil {
		m.title, m.err, m.screen = "Validation failed", err, resultScreen
		return m, nil
	}
	m.busy = true
	operationContext := m.startActivity(len(targets))
	work := func() tea.Msg {
		defer close(m.activity)
		valid := 0
		var issues []string
		for index, target := range targets {
			m.emitActivity(activityEvent{current: index, total: len(targets), label: target.Label(), phase: "validating"})
			select {
			case <-operationContext.Done():
				return operationMsg{title: "Validation cancelled", err: operationContext.Err()}
			default:
			}
			app := m.workspace.Inspect(target.Ref)
			if app.Status == "valid" {
				valid++
			} else {
				issues = append(issues, fmt.Sprintf("%s: %s", app.Label(), app.Error))
			}
		}
		body := fmt.Sprintf("%d valid, %d invalid", valid, len(issues))
		if len(issues) > 0 {
			body += "\n\n" + strings.Join(issues, "\n")
		}
		return operationMsg{title: "Validation finished", body: body}
	}
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(), work)
}

func (m Model) startBuild(all bool) (tea.Model, tea.Cmd) {
	targets, err := m.targets(all)
	if err != nil {
		m.title, m.err, m.screen = "Build failed", err, resultScreen
		return m, nil
	}
	refs := make([]workspace.AppRef, 0, len(targets))
	for _, app := range targets {
		refs = append(refs, app.Ref)
	}
	service := packager.Service{Workspace: m.workspace, Runner: m.runner}
	if err := service.Validate(); err != nil {
		m.title, m.err, m.screen = "Build unavailable", err, resultScreen
		return m, nil
	}
	m.busy = true
	operationContext := m.startActivity(len(targets))
	work := func() tea.Msg {
		defer close(m.activity)
		var output bytes.Buffer
		results, buildErr := service.Build(operationContext, refs, packager.BuildOptions{OnProgress: func(event packager.Event) {
			m.emitActivity(activityEvent{current: event.Index - 1, total: event.Total, label: event.App, phase: event.Phase})
		}}, &output, &output)
		built, current := 0, 0
		var failures []string
		for _, result := range results {
			if result.Err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", result.App.Label(), result.Err))
			} else if result.Status == packager.StatusCurrent {
				current++
			} else {
				built++
			}
		}
		body := fmt.Sprintf("%d built, %d up to date, %d failed", built, current, len(failures))
		if len(failures) > 0 {
			body += "\n\n" + strings.Join(failures, "\n")
		}
		if strings.TrimSpace(output.String()) != "" {
			body += "\n\nTool output:\n" + strings.TrimSpace(output.String())
		}
		return operationMsg{title: "Build finished", body: body, err: buildErr}
	}
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(), work)
}

func (m Model) startUnpack(all bool) (tea.Model, tea.Cmd) {
	targets, err := m.targets(all)
	if err != nil {
		m.title, m.err, m.screen = "Unpack failed", err, resultScreen
		return m, nil
	}
	service := unpacker.Service{DecoderPath: m.workspace.Config.DecoderPath, Runner: m.runner}
	if err := service.Validate(); err != nil {
		m.title, m.err, m.screen = "Unpack unavailable", err, resultScreen
		return m, nil
	}
	m.busy = true
	operationContext := m.startActivity(len(targets))
	work := func() tea.Msg {
		defer close(m.activity)
		var failures, outputs []string
		for index, app := range targets {
			m.emitActivity(activityEvent{current: index, total: len(targets), label: app.Label(), phase: "decoding"})
			if len(app.Packages) != 1 {
				problem := "no .intunewin package found"
				if len(app.Packages) > 1 {
					problem = "multiple packages found; use the CLI with an explicit package path"
				}
				failures = append(failures, app.Label()+": "+problem)
				continue
			}
			result := service.Unpack(operationContext, app.Packages[0], "", false, io.Discard, io.Discard)
			if result.Err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", app.Label(), result.Err))
			} else {
				outputs = append(outputs, result.Destination)
			}
		}
		body := fmt.Sprintf("%d succeeded, %d failed", len(outputs), len(failures))
		if len(outputs) > 0 {
			body += "\n\nOutputs:\n" + strings.Join(outputs, "\n")
		}
		if len(failures) > 0 {
			body += "\n\n" + strings.Join(failures, "\n")
		}
		return operationMsg{title: "Unpack finished", body: body}
	}
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(), work)
}

func (m *Model) startActivity(total int) context.Context {
	m.activityContext, m.cancelActivity = context.WithCancel(m.ctx)
	m.activity = make(chan activityEvent)
	m.current = activityEvent{total: total, phase: "starting"}
	return m.activityContext
}

func (m Model) emitActivity(event activityEvent) {
	select {
	case m.activity <- event:
	case <-m.activityContext.Done():
	}
}

func (m Model) waitForActivity() tea.Cmd {
	activity := m.activity
	return func() tea.Msg {
		if activity == nil {
			return activityClosedMsg{}
		}
		event, ok := <-activity
		if !ok {
			return activityClosedMsg{}
		}
		return activityMsg(event)
	}
}

func (m *Model) beginCreate() tea.Cmd {
	m.create = &workspace.CreateOptions{Source: "source", OutputDir: m.workspace.DefaultOutputDir()}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Application name").Value(&m.create.Name).Validate(func(value string) error {
			if !workspace.ValidAppName(value) {
				return errors.New("use a valid Windows directory name")
			}
			return nil
		}),
		huh.NewInput().Title("Group").Description("Optional path below the applications directory").Value(&m.create.Group).Validate(func(value string) error {
			if !workspace.ValidGroupName(value) {
				return errors.New("use a relative path of valid Windows directory names")
			}
			return nil
		}),
		huh.NewInput().Title("Display name").Value(&m.create.DisplayName),
		huh.NewInput().Title("Source directory").Value(&m.create.Source).Validate(tuiSafePath),
		huh.NewInput().Title("Setup file").Value(&m.create.SetupFile).Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New("setup file is required")
			}
			return tuiSafePath(value)
		}),
		huh.NewInput().Title("Output directory").Value(&m.create.OutputDir).Validate(tuiSafePath),
		huh.NewConfirm().Title("Create this application?").Affirmative("Create").Negative("Cancel").Validate(func(value bool) error {
			if !value {
				return errors.New("confirm creation or press esc to cancel")
			}
			return nil
		}),
	)).WithWidth(max(40, m.width-4)).WithHeight(max(12, m.height-4))
	m.screen = createScreen
	return m.form.Init()
}

func (m Model) createCmd() tea.Cmd {
	options := *m.create
	return func() tea.Msg {
		ref, err := m.workspace.Create(options)
		if err != nil {
			return operationMsg{title: "Create failed", err: err}
		}
		return operationMsg{title: "Application created", body: ref.Relative}
	}
}

func tuiSafePath(value string) error {
	if value == "" || !pathutil.IsSafeRelative(value) {
		return errors.New("path must remain within the application directory")
	}
	return nil
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		apps, err := m.workspace.List()
		return refreshMsg{apps: apps, err: err}
	}
}

func detailBody(app workspace.App) string {
	lines := []string{
		"Path: " + app.Ref.Relative,
		"Status: " + app.Status,
		fmt.Sprintf("Packages: %d", len(app.Packages)),
	}
	if app.Config != nil {
		lines = append(lines,
			"Display name: "+app.Config.DisplayName,
			"Source: "+app.Config.Source,
			"Setup file: "+app.Config.SetupFile,
			"Output directory: "+app.Config.OutputDir,
		)
	}
	if app.Error != "" {
		lines = append(lines, "Issue: "+app.Error)
	}
	if len(app.Packages) > 0 {
		lines = append(lines, "", "Package files:", strings.Join(app.Packages, "\n"))
	}
	return strings.Join(lines, "\n")
}

func toItems(apps []workspace.App) []list.Item {
	items := make([]list.Item, 0, len(apps))
	for _, app := range apps {
		items = append(items, appItem{app: app})
	}
	return items
}
