package cmd

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

type console struct {
	out      io.Writer
	err      io.Writer
	heading  lipgloss.Style
	success  lipgloss.Style
	failure  lipgloss.Style
	muted    lipgloss.Style
	errLabel lipgloss.Style
}

func newConsole(out, errOut io.Writer) *console {
	outRenderer := lipgloss.NewRenderer(out)
	errRenderer := lipgloss.NewRenderer(errOut)

	return &console{
		out:      out,
		err:      errOut,
		heading:  outRenderer.NewStyle().Bold(true),
		success:  outRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
		failure:  outRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
		muted:    outRenderer.NewStyle().Foreground(lipgloss.Color("8")),
		errLabel: errRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
	}
}

func (c *console) start(action string, count int) {
	fmt.Fprintf(c.out, "%s %d %s\n", c.heading.Render(action), count, plural(count, "application", "applications"))
}

func (c *console) failureDetail(name string, err error) {
	fmt.Fprintf(c.err, "%s %s: %v\n", c.errLabel.Render("FAILED"), name, err)
}

func (c *console) summary(action string, succeeded, failed, skipped int) {
	parts := []string{c.success.Render(fmt.Sprintf("%d succeeded", succeeded))}
	if failed > 0 {
		parts = append(parts, c.failure.Render(fmt.Sprintf("%d failed", failed)))
	}
	if skipped > 0 {
		parts = append(parts, c.muted.Render(fmt.Sprintf("%d skipped", skipped)))
	}

	fmt.Fprintf(c.out, "%s: %s\n", action, joinSummary(parts))
}

func joinSummary(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + ", " + parts[1]
	default:
		return parts[0] + ", " + parts[1] + ", " + parts[2]
	}
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
