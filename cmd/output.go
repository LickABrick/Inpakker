package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

type console struct {
	out      io.Writer
	err      io.Writer
	heading  lipgloss.Style
	success  lipgloss.Style
	failure  lipgloss.Style
	warning  lipgloss.Style
	muted    lipgloss.Style
	errLabel lipgloss.Style
}

func newConsole(out, errOut io.Writer) *console {
	return &console{
		out:      colorprofile.NewWriter(out, os.Environ()),
		err:      colorprofile.NewWriter(errOut, os.Environ()),
		heading:  lipgloss.NewStyle().Bold(true),
		success:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
		failure:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
		warning:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		errLabel: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
	}
}

func (c *console) start(action string, count int) {
	c.startCount(action, count, "application", "applications")
}

func (c *console) startCount(action string, count int, singular, multiple string) {
	fmt.Fprintf(c.out, "%s %d %s\n", c.heading.Render(action), count, plural(count, singular, multiple))
}

func (c *console) failureDetail(name string, err error) {
	fmt.Fprintf(c.err, "%s %s: %v\n", c.errLabel.Render("X"), name, err)
}

func (c *console) successDetail(name, message string) {
	fmt.Fprintf(c.out, "%s %s: %s\n", c.success.Render("✓"), name, message)
}

func (c *console) warningDetail(name, message string) {
	fmt.Fprintf(c.out, "%s %s: %s\n", c.warning.Render("!"), name, message)
}

func (c *console) summary(action string, succeeded, failed, skipped int) {
	parts := []string{c.success.Render(fmt.Sprintf("%d succeeded", succeeded))}
	if failed > 0 {
		parts = append(parts, c.failure.Render(fmt.Sprintf("%d failed", failed)))
	}
	if skipped > 0 {
		parts = append(parts, c.muted.Render(fmt.Sprintf("%d skipped", skipped)))
	}

	fmt.Fprintf(c.out, "%s: %s\n", action, strings.Join(parts, ", "))
}

func (c *console) buildSummary(built, current, failed, skipped int) {
	parts := []string{c.success.Render(fmt.Sprintf("%d built", built))}
	if current > 0 {
		parts = append(parts, c.muted.Render(fmt.Sprintf("%d up to date", current)))
	}
	if failed > 0 {
		parts = append(parts, c.failure.Render(fmt.Sprintf("%d failed", failed)))
	}
	if skipped > 0 {
		parts = append(parts, c.muted.Render(fmt.Sprintf("%d not found", skipped)))
	}
	fmt.Fprintf(c.out, "%s: %s\n", c.heading.Render("Build finished"), strings.Join(parts, ", "))
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// reportedError signals that a command already printed actionable details and
// only needs a non-zero exit code from Execute.
type reportedError struct {
	err error
}

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }
