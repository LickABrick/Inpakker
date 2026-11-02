package packager

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type successfulCancelRunner struct{ cancel context.CancelFunc }

func (r successfulCancelRunner) Run(context.Context, string, []string, io.Writer, io.Writer) error {
	r.cancel()
	return nil
}

func TestSuccessfulRunnerAfterCancellationDoesNotCache(t *testing.T) {
	for _, options := range []BuildOptions{{}, {Force: true}, {NoCache: true}} {
		t.Run(stringOption(options), func(t *testing.T) {
			ws, ref, _ := inspectionWorkspace(t)
			// Even a pre-existing artifact must not make the cancelled run successful.
			output := filepath.Join(ref.Path, "output")
			if err := os.MkdirAll(output, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(output, "old.intunewin"), []byte("package"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			results, err := (Service{Workspace: ws, Runner: successfulCancelRunner{cancel}}).Build(ctx, []workspace.AppRef{ref}, options, io.Discard, io.Discard)
			if !errors.Is(err, context.Canceled) || len(results) != 1 || results[0].Status != StatusFailed {
				t.Fatalf("results = %#v, error = %v", results, err)
			}
			for _, name := range []string{buildcache.FileName, buildcache.EvidenceFileName} {
				if _, err := os.Stat(filepath.Join(cacheDirectory(t, ws), name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("cancelled run wrote %s: %v", name, err)
				}
			}
		})
	}
}

func stringOption(options BuildOptions) string {
	if options.NoCache {
		return "no-cache"
	}
	if options.Force {
		return "force"
	}
	return "normal"
}

func TestCancelledLastAppCheckReturnsCancellation(t *testing.T) {
	ws, ref, _ := inspectionWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, err := (Service{Workspace: ws, Runner: diagnosticRunner{}}).Build(ctx, []workspace.AppRef{ref}, BuildOptions{OnProgress: func(event Event) {
		if event.Phase == "checking" {
			cancel()
		}
	}}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) || len(results) != 0 {
		t.Fatalf("results = %#v, error = %v", results, err)
	}
}
