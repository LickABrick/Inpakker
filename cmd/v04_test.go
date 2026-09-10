package cmd

import (
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
	}{{nil, ws.Root}, {[]string{"Mozilla Firefox"}, app.Path}, {[]string{app.Relative}, app.Path}} {
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
