package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// Keep validation separate from Huh's blur/next-field validation so every
// input remains freely navigable, including after an unsuccessful submission.
type checkedInput struct {
	*huh.Input
	label    string
	validate func(string) error
}

func checkInput(input *huh.Input, label string, validate func(string) error) *checkedInput {
	return &checkedInput{Input: input, label: label, validate: validate}
}

func (i *checkedInput) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	_, cmd := i.Input.Update(msg)
	return i, cmd
}

func requiredText(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("a value is required")
	}
	return nil
}

func (m *Model) validateForm() (tea.Cmd, bool) {
	for _, field := range m.formFields {
		input, ok := field.(*checkedInput)
		if !ok {
			continue
		}
		if err := input.validate(input.GetValue().(string)); err != nil {
			m.formError = fmt.Errorf("%s: %w", input.label, err)
			m.form.WithHeight(m.formHeight())
			var cmds []tea.Cmd
			for steps := 0; steps < len(m.formFields) && m.form.GetFocusedField() != input; steps++ {
				_, cmd := m.form.Update(huh.PrevField())
				cmds = append(cmds, cmd)
			}
			return tea.Batch(cmds...), true
		}
	}
	return nil, false
}
