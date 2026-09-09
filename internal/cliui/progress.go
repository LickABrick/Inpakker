package cliui

import (
	"context"
	"fmt"
	"io"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Event struct {
	Current int
	Total   int
	Label   string
	Phase   string
}

type Work func(context.Context, func(Event)) (any, error)

type eventMsg Event

type doneMsg struct {
	value any
	err   error
}

type progressModel struct {
	ctx      context.Context
	cancel   context.CancelFunc
	title    string
	events   chan Event
	work     Work
	spinner  spinner.Model
	progress progress.Model
	event    Event
	done     bool
	value    any
	err      error
}

func Run(ctx context.Context, input io.Reader, output io.Writer, title string, total int, work Work) (any, error) {
	operationContext, cancel := context.WithCancel(ctx)
	model := progressModel{
		ctx:      operationContext,
		cancel:   cancel,
		title:    title,
		events:   make(chan Event),
		work:     work,
		spinner:  spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		progress: progress.New(progress.WithColors(lipgloss.Color("6")), progress.WithWidth(36)),
		event:    Event{Total: total, Phase: "starting"},
	}
	program := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output))
	final, err := program.Run()
	cancel()
	if err != nil {
		return nil, err
	}
	completed := final.(progressModel)
	return completed.value, completed.err
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.waitForEvent(), m.runWork())
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			m.done, m.err = true, context.Canceled
			return m, tea.Quit
		}
	case eventMsg:
		m.event = Event(msg)
		return m, m.waitForEvent()
	case doneMsg:
		m.done, m.value, m.err = true, msg.value, msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		updated, cmd := m.progress.Update(msg)
		m.progress = updated
		return m, cmd
	}
	return m, nil
}

func (m progressModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	total := m.event.Total
	if total < 1 {
		total = 1
	}
	percent := float64(m.event.Current) / float64(total)
	line := fmt.Sprintf("%s %s", m.spinner.View(), m.event.Phase)
	if m.event.Label != "" {
		line += " " + m.event.Label
	}
	content := lipgloss.NewStyle().Bold(true).Render(m.title) + "\n" +
		m.progress.ViewAs(percent) + fmt.Sprintf("  %d/%d\n", m.event.Current, m.event.Total) + line
	return tea.NewView(content)
}

func (m progressModel) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.events:
			return eventMsg(event)
		case <-m.ctx.Done():
			return doneMsg{err: m.ctx.Err()}
		}
	}
}

func (m progressModel) runWork() tea.Cmd {
	return func() tea.Msg {
		value, err := m.work(m.ctx, func(event Event) {
			select {
			case m.events <- event:
			case <-m.ctx.Done():
			}
		})
		return doneMsg{value: value, err: err}
	}
}
