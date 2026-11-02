package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/internal/packager"
)

func TestResultOutcomeDoesNotDependOnLabel(t *testing.T) {
	rows := []resultRow{{Status: "X not actually failed", Outcome: resultSuccess}, {Status: "✓ misleading label", Outcome: resultFailed}}
	if firstFailedRow(rows) != 1 { t.Fatal("failure selection used display text") }
}

func TestSingleBuildNotificationAndArtifactNames(t *testing.T) {
	for _, status := range []packager.ResultStatus{packager.StatusBuilt, packager.StatusCurrent} {
		m := newTestModel(t)
		app := m.apps.all[0]
		op := m.startOperation(operationBuild, "Building", 1)
		artifact := filepath.Join(t.TempDir(), "example.intunewin")
		updated, _ := m.handleBuildDone(buildDoneMsg{id: op.id, results: []packager.Result{{App: app.App, Status: status, Artifacts: []string{artifact}}}})
		m = updated.(Model)
		want := app.App.Label()+" packaged successfully"
		if status == packager.StatusCurrent { want = app.App.Label()+" is already up to date" }
		if m.notification.Text != want { t.Fatalf("notification = %q, want %q", m.notification.Text, want) }
		if strings.Contains(m.resultRows[0].Detail, filepath.Dir(artifact)) { t.Fatal("artifact detail exposes absolute directory") }
	}
}

func TestCancelledValidationReportsUnprocessedTargets(t *testing.T) {
	m := newTestModel(t)
	op := m.startOperation(operationValidate, "Validating", 3)
	op.targets = []ApplicationView{m.apps.all[0], m.apps.all[0], m.apps.all[0]}
	updated, _ := m.handleValidationDone(validationDoneMsg{id: op.id, valid: 1, rows: []resultRow{{Outcome: resultSuccess}}, err: context.Canceled})
	m = updated.(Model)
	if !strings.Contains(m.resultTitle, "2 not processed") || m.modal != ModalResults { t.Fatalf("partial result = %q modal=%v", m.resultTitle, m.modal) }
}

func TestCreateResultRejectsStaleWorkspace(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(createMsg{workspaceID: "another workspace", generation: m.generation})
	if updated.(Model).lastResult != nil { t.Fatal("stale creation result was applied") }
}
