package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
)

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, string, []string, io.Writer, io.Writer) error { return nil }

func TestNewSupportsAnEmptyWorkspaceAndGuidedCreate(t *testing.T) {
	root := t.TempDir()
	ws := &workspace.Workspace{Root: root}
	model, err := New(context.Background(), ws, noOpRunner{}, nil)
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

func TestUpdateAvailabilityAppearsInTUI(t *testing.T) {
	model := Model{}
	updated, _ := model.Update(updateCheckMsg{result: updater.Result{
		CurrentVersion: "0.2.0",
		LatestVersion:  "0.3.0",
		Available:      true,
		ReleaseURL:     "https://example.test/releases/v0.3.0",
		CheckedAt:      time.Now(),
	}})
	result := updated.(Model)
	if !result.updateResult.Available || result.list.Title != "Inpakker workspace  •  Update v0.3.0 available" {
		t.Fatalf("update was not shown: %#v", result.updateResult)
	}
}
