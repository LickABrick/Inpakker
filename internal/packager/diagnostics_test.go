package packager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type diagnosticRunner struct{ cancel context.CancelFunc }

func (r diagnosticRunner) Run(_ context.Context, _ string, _ []string, out, errOut io.Writer) error {
	if out != nil {
		_, _ = io.WriteString(out, "tool detail\n")
	}
	if errOut != nil {
		_, _ = io.WriteString(errOut, "failure reason\n")
	}
	if r.cancel != nil {
		r.cancel()
	}
	return errors.New("exit status 1")
}

func TestQuietBuildRetainsDiagnosticsWithoutVisibleOutput(t *testing.T) {
	ws, ref, _ := inspectionWorkspace(t)
	ws.User.Preferences.ShowToolOutput = false
	log := process.NewOutputBuffer(1024)
	var visible bytes.Buffer
	results, err := (Service{Workspace: ws, Runner: diagnosticRunner{}}).Build(context.Background(), []workspace.AppRef{ref}, BuildOptions{Diagnostics: log}, &visible, &visible)
	if err != nil || len(results) != 1 || results[0].Err == nil {
		t.Fatal(results, err)
	}
	if log.String() != "tool detail\nfailure reason\n" || visible.Len() != 0 {
		t.Fatal(log.String(), visible.String())
	}
}

func TestCancelledLastBuildReportsCancellationAndKeepsResult(t *testing.T) {
	ws, ref, _ := inspectionWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, err := (Service{Workspace: ws, Runner: diagnosticRunner{cancel: cancel}}).Build(ctx, []workspace.AppRef{ref}, BuildOptions{}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) || len(results) != 1 {
		t.Fatal(results, err)
	}
	if (Service{Workspace: ws}).InspectBuildState(ref).State != BuildStateNotBuilt {
		t.Fatal("cancelled build wrote successful cache state")
	}
}
