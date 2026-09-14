package cmd

import (
	"errors"
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
	"io"
	"path/filepath"
	"testing"
)

func TestOpenWorkspaceAndHumanApplicationName(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	t.Setenv("INPAKKER_WORKSPACE", "")
	ws, err := workspace.CreateWorkspace(workspace.CreateWorkspaceOptions{Root: filepath.Join(t.TempDir(), "Customer Apps"), Name: "ADS Groep"})
	if err != nil {
		t.Fatal(err)
	}
	app, err := ws.Create(workspace.CreateOptions{Name: "Mozilla Firefox", Group: "Customer Apps/Legacy", SetupFile: "Firefox Setup.exe"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{{nil, ws.Root}, {[]string{"Mozilla Firefox"}, app.Path}, {[]string{app.Relative}, app.Path}, {[]string{"Mozilla Firefox", "--output"}, filepath.Join(app.Path, "output")}} {
		opened := ""
		root := &cobra.Command{Use: "inpakker"}
		root.PersistentFlags().String("workspace", "ADS Groep", "")
		root.AddCommand(newOpenCmd(pathopener.Func(func(path string) error { opened = path; return nil })))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(append([]string{"open"}, test.args...))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if opened != test.want {
			t.Fatal(opened, test.want)
		}
	}
}

func TestOpenOutputRequiresApplication(t *testing.T) {
	called := false
	command := newOpenCmd(pathopener.Func(func(string) error { called = true; return nil }))
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--output"})
	err := command.Execute()
	var usage usageError
	if !errors.As(err, &usage) || called {
		t.Fatal("missing application was not rejected as usage error", err)
	}
}
