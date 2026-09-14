package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func TestNewSetupFromWithoutInteractiveInput(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	t.Setenv("INPAKKER_WORKSPACE", "")
	ws, err := workspace.CreateWorkspace(workspace.CreateWorkspaceOptions{Root: filepath.Join(t.TempDir(), "Workspace"), Name: "Imports"})
	if err != nil {
		t.Fatal(err)
	}
	from := filepath.Join(t.TempDir(), "Setup File.msi")
	if err := os.WriteFile(from, []byte("installer"), 0644); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "inpakker"}
	root.PersistentFlags().String("workspace", ws.Root, "")
	root.AddCommand(newNewCmd())
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"new", "New Application", "--setup-from", from, "--no-input"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	ref, err := ws.Find("New Application")
	if err != nil {
		t.Fatal(err)
	}
	app := ws.Inspect(ref)
	if app.Status != "valid" || app.Config.SetupFile != "Setup File.msi" {
		t.Fatal("CLI did not import setup", app)
	}
}
