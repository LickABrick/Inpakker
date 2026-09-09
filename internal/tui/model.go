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
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/LickABrick/inpakker/internal/packager"
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

type Model struct {
	ctx       context.Context
	workspace *workspace.Workspace
	runner    process.Runner
	list      list.Model
	apps      []workspace.App
	screen    screen
	busy      bool
	width     int
	height    int
	title     string
	body      string
	err       error
	inputs    []textinput.Model
	focus     int
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
	return Model{ctx: ctx, workspace: ws, runner: runner, list: applicationList, apps: apps}, nil
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
		m.title, m.body, m.err, m.screen = msg.title, msg.body, msg.err, resultScreen
		return m, nil
	}

	key, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return m.updateComponent(msg)
	}
	if m.busy {
		if key.String() == "ctrl+c" {
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
			m.beginCreate()
			return m, nil
		case "r":
			m.busy = true
			return m, m.refreshCmd()
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
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.screen = listScreen
			return m, nil
		case "tab", "down":
			m.moveFocus(1)
			return m, nil
		case "shift+tab", "up":
			m.moveFocus(-1)
			return m, nil
		case "ctrl+s":
			m.busy = true
			return m, m.createCmd()
		}
	}
	return m.updateComponent(msg)
}

func (m Model) updateComponent(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.screen == createScreen {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
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
		labels := []string{"Name", "Group", "Display name", "Source directory", "Setup file", "Output directory"}
		var builder strings.Builder
		builder.WriteString(headingStyle.Render("Create application") + "\n\n")
		for index := range m.inputs {
			fmt.Fprintf(&builder, "%s\n%s\n\n", labels[index], m.inputs[index].View())
		}
		builder.WriteString(helpStyle.Render("tab/shift+tab fields  •  ctrl+s create  •  esc cancel"))
		content = builder.String()
	default:
		content = m.list.View() + "\n" + helpStyle.Render("enter details  •  / search  •  n create  •  v/b/u selected  •  ctrl+v/b/u all  •  r refresh  •  q quit")
	}
	if m.busy {
		content += "\n\n" + headingStyle.Render("Working…")
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
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
	return m, func() tea.Msg {
		valid := 0
		var issues []string
		for _, target := range targets {
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
	return m, func() tea.Msg {
		var output bytes.Buffer
		results := service.Build(m.ctx, refs, &output, &output)
		var failures []string
		for _, result := range results {
			if result.Err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", result.App.Label(), result.Err))
			}
		}
		body := fmt.Sprintf("%d succeeded, %d failed", len(results)-len(failures), len(failures))
		if len(failures) > 0 {
			body += "\n\n" + strings.Join(failures, "\n")
		}
		if strings.TrimSpace(output.String()) != "" {
			body += "\n\nTool output:\n" + strings.TrimSpace(output.String())
		}
		return operationMsg{title: "Build finished", body: body}
	}
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
	return m, func() tea.Msg {
		var failures, outputs []string
		for _, app := range targets {
			if len(app.Packages) != 1 {
				problem := "no .intunewin package found"
				if len(app.Packages) > 1 {
					problem = "multiple packages found; use the CLI with an explicit package path"
				}
				failures = append(failures, app.Label()+": "+problem)
				continue
			}
			result := service.Unpack(m.ctx, app.Packages[0], "", false, io.Discard, io.Discard)
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
}

func (m *Model) beginCreate() {
	placeholders := []string{"example", "optional/team", "Example application", "source", "setup.exe", m.workspace.DefaultOutputDir()}
	m.inputs = make([]textinput.Model, len(placeholders))
	for index, placeholder := range placeholders {
		input := textinput.New()
		input.Prompt = "> "
		input.Placeholder = placeholder
		input.SetWidth(max(20, m.width-6))
		m.inputs[index] = input
	}
	m.inputs[3].SetValue("source")
	m.inputs[5].SetValue(m.workspace.DefaultOutputDir())
	m.focus = 0
	m.inputs[0].Focus()
	m.screen = createScreen
}

func (m *Model) moveFocus(delta int) {
	m.inputs[m.focus].Blur()
	m.focus = (m.focus + delta + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focus].Focus()
}

func (m Model) createCmd() tea.Cmd {
	options := workspace.CreateOptions{
		Name:        strings.TrimSpace(m.inputs[0].Value()),
		Group:       strings.TrimSpace(m.inputs[1].Value()),
		DisplayName: strings.TrimSpace(m.inputs[2].Value()),
		Source:      strings.TrimSpace(m.inputs[3].Value()),
		SetupFile:   strings.TrimSpace(m.inputs[4].Value()),
		OutputDir:   strings.TrimSpace(m.inputs[5].Value()),
	}
	return func() tea.Msg {
		ref, err := m.workspace.Create(options)
		if err != nil {
			return operationMsg{title: "Create failed", err: err}
		}
		return operationMsg{title: "Application created", body: ref.Relative}
	}
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
