package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/workspace"
)

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, string, []string, io.Writer, io.Writer) error { return nil }

func TestNewSupportsAnEmptyWorkspaceAndGuidedCreate(t *testing.T) {
	root := t.TempDir()
	ws := &workspace.Workspace{Root: root}
	model, err := New(context.Background(), ws, noOpRunner{})
	if err != nil {
		t.Fatalf("New returned %v", err)
	}
	if len(model.apps) != 0 {
		t.Fatalf("New loaded %d apps, want 0", len(model.apps))
	}

	model.beginCreate()
	model.create.Name = "example"
	model.create.Group = "team"
	model.create.DisplayName = "Example"
	model.create.SetupFile = "setup.exe"
	message := model.createCmd()()
	result, ok := message.(operationMsg)
	if !ok || result.err != nil {
		t.Fatalf("create result = %#v", message)
	}
	configPath := filepath.Join(root, "apps", "team", "example", "app.config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat created config: %v", err)
	}
}
