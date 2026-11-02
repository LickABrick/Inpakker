package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/process"
	"github.com/LickABrick/inpakker/internal/unpacker"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func (m Model) createCmd() tea.Cmd {
	options, ws := *m.create, m.workspace
	generation, workspaceID := m.generation, ws.Config.ID
	return func() tea.Msg {
		ref, err := ws.Create(options)
		if err != nil {
			return createMsg{err: err, generation: generation, workspaceID: workspaceID}
		}
		app := inspectOperationApplication(ws, ref)
		return createMsg{ref: ref, apps: []ApplicationView{app}, generation: generation, workspaceID: workspaceID}
	}
}

func (m Model) startValidate(all bool) (tea.Model, tea.Cmd) {
	targets, err := m.targetApplications(all)
	if err != nil {
		m.showError("Validation unavailable", err)
		return m, nil
	}
	return m.validateTargets(targets, all)
}

func (m Model) validateTargets(targets []ApplicationView, all bool) (tea.Model, tea.Cmd) {
	op := m.startOperation(operationValidate, validationOperationTitle(all, targets), len(targets))
	op.targets = targets
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		valid := 0
		issues := make([]validationIssue, 0)
		var rows []resultRow
		var apps []ApplicationView
		var operationErr error
		for index, target := range targets {
			if err := op.context.Err(); err != nil {
				operationErr = err
				break
			}
			emitActivity(op, activityEvent{current: index + 1, completed: index, total: len(targets), label: target.App.Label(), phase: "validating"})
			app := ws.InspectContext(op.context, target.App.Ref)
			view := target
			view.App = app
			apps = append(apps, view)
			if app.Status == "valid" {
				valid++
				rows = append(rows, resultRow{Name: app.Label(), Outcome: resultSuccess, Status: "✓ Valid", AppID: app.Ref.Relative})
			} else {
				issues = append(issues, validationIssue{AppID: app.Ref.Relative, Name: app.Label(), Issue: app.Error})
			}
		}
		emitActivity(op, activityEvent{current: len(targets), completed: len(targets), total: len(targets), phase: "validation complete"})
		return validationDoneMsg{id: op.id, valid: valid, rows: rows, issues: issues, apps: apps, all: all, err: operationErr}
	})
}

func (m Model) startBuild(all bool, options packager.BuildOptions) (tea.Model, tea.Cmd) {
	targets, err := m.targetApplications(all)
	if err != nil {
		m.showError("Build unavailable", err)
		return m, nil
	}
	return m.buildTargets(targets, all, options)
}

func (m Model) buildTargets(targets []ApplicationView, all bool, options packager.BuildOptions) (tea.Model, tea.Cmd) {
	if !all && targets[0].App.Status != "valid" {
		m.showMessage("Build unavailable", targets[0].App.Label()+" cannot be built because its configuration is invalid.\n\nX "+targets[0].App.Error)
		return m, nil
	}
	service := packager.Service{Workspace: m.workspace, Runner: m.runner}
	if err := service.Validate(); err != nil {
		if m.workspace != nil && m.runner != nil {
			return m, m.beginToolRecovery("content-prep")
		}
		m.showError("Build unavailable", err)
		return m, nil
	}
	refs := make([]workspace.AppRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, target.App.Ref)
	}
	op := m.startOperation(operationBuild, buildOperationTitle(all, targets), len(targets))
	op.targets = targets
	op.buildOptions = options
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		output := process.NewOutputBuffer(256 << 10)
		options.Diagnostics = output
		options.OnProgress = func(event packager.Event) {
			if event.Phase == "checking" {
				fmt.Fprintln(output, "\n"+event.App)
			}
			emitActivity(op, activityEvent{current: event.Index, completed: event.Index - 1, total: event.Total, label: event.App, phase: event.Phase})
		}
		results, buildErr := service.Build(op.context, refs, options, io.Discard, io.Discard)
		emitActivity(op, activityEvent{current: len(results), completed: len(results), total: len(targets), phase: "build complete"})
		apps := make([]ApplicationView, 0, len(results))
		inspection := service.NewInspectionContext(context.Background())
		for _, result := range results {
			app := ws.InspectContext(context.Background(), result.App.Ref)
			group := filepath.Dir(app.Ref.Relative)
			if group == "." {
				group = "—"
			}
			apps = append(apps, ApplicationView{App: app, Group: group, Build: inspection.InspectBuildStateForApp(context.Background(), app, false)})
		}
		return buildDoneMsg{id: op.id, results: results, apps: apps, all: all, log: strings.TrimSpace(output.String()), err: buildErr}
	})
}

func (m Model) startUnpack(all bool, selectedPackage string) (tea.Model, tea.Cmd) {
	targets, err := m.targetApplications(all)
	if err != nil {
		m.showError("Unpacking unavailable", err)
		return m, nil
	}
	return m.unpackTargets(targets, all, selectedPackage)
}

func (m Model) unpackTargets(targets []ApplicationView, all bool, selectedPackage string) (tea.Model, tea.Cmd) {
	service := unpacker.Service{DecoderPath: m.workspace.User.Tools.Decoder.Path, Runner: m.runner}
	if err := service.Validate(); err != nil {
		if m.runner != nil {
			return m, m.beginToolRecovery("decoder")
		}
		m.showError("Unpacking unavailable", err)
		return m, nil
	}
	if !all && selectedPackage == "" {
		packages := targets[0].App.Packages
		switch len(packages) {
		case 0:
			m.showMessage("Unpacking unavailable", targets[0].App.Label()+" has no .intunewin package to unpack.")
			return m, nil
		case 1:
			selectedPackage = packages[0]
		default:
			return m, m.beginPackageSelect(packages)
		}
	}
	op := m.startOperation(operationUnpack, unpackOperationTitle(all, targets), len(targets))
	op.targets = targets
	op.packagePath = selectedPackage
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		outputs := make([]string, 0)
		failures := make([]unpackFailure, 0)
		rows := make([]resultRow, 0)
		var operationErr error
		for index, target := range targets {
			if err := op.context.Err(); err != nil {
				operationErr = err
				break
			}
			packagePath := selectedPackage
			if all {
				if len(target.App.Packages) != 1 {
					reason := "no package found"
					if len(target.App.Packages) > 1 {
						reason = "multiple packages found"
					}
					failures = append(failures, unpackFailure{Name: target.App.Label(), Issue: reason, AppID: target.App.Ref.Relative})
					continue
				}
				packagePath = target.App.Packages[0]
			}
			emitActivity(op, activityEvent{current: index + 1, completed: index, total: len(targets), label: target.App.Label(), phase: "decoding"})
			result := service.Unpack(op.context, packagePath, "", false, io.Discard, io.Discard)
			if result.Err != nil {
				failures = append(failures, unpackFailure{Name: target.App.Label(), Issue: result.Err.Error(), AppID: target.App.Ref.Relative})
			} else {
				outputs = append(outputs, result.Destination)
				rows = append(rows, resultRow{Name: target.App.Label(), Outcome: resultSuccess, Status: "✓ Unpacked", Detail: filepath.Base(result.Destination), OutputDir: result.Destination, AppID: target.App.Ref.Relative})
			}
			if op.context.Err() != nil {
				operationErr = op.context.Err()
				break
			}
		}
		emitActivity(op, activityEvent{current: len(targets), completed: len(targets), total: len(targets), phase: "unpack complete"})
		return unpackDoneMsg{id: op.id, rows: rows, succeeded: len(outputs), outputs: outputs, failures: failures, all: all, err: operationErr}
	})
}

func (m Model) startUpdate() (tea.Model, tea.Cmd) {
	if m.updater == nil {
		m.showMessage("Update unavailable", "Update checking is disabled for this session.")
		return m, nil
	}
	op := m.startOperation(operationUpdate, "Updating Inpakker", 4)
	service := m.updater
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		result, err := service.Check(op.context, false)
		if err != nil {
			return updateDoneMsg{id: op.id, err: err}
		}
		if !result.Available {
			return updateDoneMsg{id: op.id}
		}
		err = service.Install(op.context, result, func(event updater.Event) {
			emitActivity(op, activityEvent{current: event.Current, completed: event.Current, total: event.Total, phase: event.Phase})
		})
		return updateDoneMsg{id: op.id, version: result.LatestVersion, err: err}
	})
}

func (m *Model) startOperation(kind operationKind, title string, total int) *operationState {
	m.notification = Notification{}
	m.refreshRequests.stop()
	m.nextOperationID++
	m.generation++
	m.refreshing = false
	ctx, cancel := context.WithCancel(m.ctx)
	op := &operationState{
		id: m.nextOperationID, kind: kind, title: title, context: ctx, cancel: cancel,
		events: make(chan activityEvent), current: activityEvent{total: total, phase: "starting"}, started: timeNow(),
	}
	m.operation, m.modal, m.form = op, ModalProgress, nil
	m.resultRows = nil
	m.resultCursor = 0
	m.resultTitle = ""
	m.modalHasLog = false
	m.displayedResult = nil
	return op
}

func (m *Model) cancelOperation() {
	if m.operation == nil {
		return
	}
	m.operation.cancel()
	m.operation.cancelling = true
}

func emitActivity(operation *operationState, event activityEvent) {
	select {
	case operation.events <- event:
	case <-operation.context.Done():
	}
}

func (m Model) waitForActivity(id int) tea.Cmd {
	operation := m.operation
	return func() tea.Msg {
		if operation == nil || operation.id != id {
			return activityClosedMsg{id: id}
		}
		select {
		case event, ok := <-operation.events:
			if !ok {
				return activityClosedMsg{id: id}
			}
			return activityMsg{id: id, activityEvent: event}
		case <-operation.context.Done():
			return activityClosedMsg{id: id}
		}
	}
}

func (m Model) handleValidationDone(msg validationDoneMsg) (tea.Model, tea.Cmd) {
	targets := m.operationTargets(msg.id)
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	m.apps.merge(msg.apps, "")
	m.resultRows = append([]resultRow(nil), msg.rows...)
	for _, issue := range msg.issues {
		m.resultRows = append(m.resultRows, resultRow{Name: issue.Name, Status: "X Invalid", Detail: issue.Issue, AppID: issue.AppID, Outcome: resultFailed})
	}
	m.resultTitle = fmt.Sprintf("%d valid · %d invalid", msg.valid, len(msg.issues))
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.resultTitle += fmt.Sprintf(" · ! Cancelled · %d not processed", max(0, len(targets)-len(m.resultRows)))
		} else {
			m.resultRows = append(m.resultRows, resultRow{Name: "Validation", Outcome: resultFailed, Status: "X Failed", Detail: msg.err.Error()})
		}
		m.showResults("Validation results")
	}
	status := resultSuccess
	if errors.Is(msg.err, context.Canceled) {
		status = resultCancelled
	} else if msg.err != nil || len(msg.issues) > 0 {
		status = resultFailed
	}
	m.presentResult("Validation results", status)
	if status == resultSuccess && !msg.all && len(targets) == 1 {
		m.notification.Text = targets[0].App.Label() + " validation passed"
	}
	m.notification.Summary = []resultCount{{Status: resultSuccess, Count: msg.valid, Label: "valid"}, {Status: resultFailed, Count: len(msg.issues), Label: "invalid"}}
	return m, nil
}

func (m Model) handleBuildDone(msg buildDoneMsg) (tea.Model, tea.Cmd) {
	if m.operation == nil || m.operation.id != msg.id {
		return m, nil
	}
	updated, cmd := m.buildResult(msg)
	m = updated.(Model)
	m.modalHasLog = strings.TrimSpace(m.logText) != ""
	return m, cmd
}

func (m Model) buildResult(msg buildDoneMsg) (tea.Model, tea.Cmd) {
	targets := m.operationTargets(msg.id)
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	m.setLog("Packaging tool output", msg.log)
	if len(msg.apps) > 0 {
		m.apps.merge(msg.apps, "")
	}
	built, current, failed := 0, 0, 0
	m.resultRows = make([]resultRow, 0, len(msg.results))
	for _, result := range msg.results {
		row := resultRow{Name: result.App.Label(), AppID: result.App.Ref.Relative, Outcome: resultSuccess}
		switch {
		case result.Err != nil || result.Status == packager.StatusFailed:
			failed++
			row.Status, row.Detail, row.Outcome = "X Failed", result.Reason, resultFailed
			if result.Err != nil {
				row.Detail = result.Err.Error()
			}
		case result.Status == packager.StatusCurrent:
			current++
			row.Status, row.Detail = "✓ Up to date", result.Reason
		default:
			built++
			names := make([]string, 0, len(result.Artifacts))
			for _, artifact := range result.Artifacts {
				names = append(names, filepath.Base(artifact))
			}
			row.Status, row.Detail = "✓ Built", strings.Join(names, ", ")
		}
		if len(result.Artifacts) > 0 {
			row.OutputDir = filepath.Dir(result.Artifacts[0])
		}
		m.resultRows = append(m.resultRows, row)
	}
	if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
		failed++
		m.resultRows = append(m.resultRows, resultRow{Name: "Build operation", Outcome: resultFailed, Status: "X Failed", Detail: msg.err.Error()})
	}
	m.resultTitle = fmt.Sprintf("%d built · %d up to date · %d failed", built, current, failed)
	m.resultCursor = 0
	status := resultSuccess
	if errors.Is(msg.err, context.Canceled) {
		status = resultCancelled
		m.resultTitle += fmt.Sprintf(" · ! Cancelled · %d not processed", max(0, len(targets)-len(m.resultRows)))
	} else if failed > 0 {
		status = resultFailed
	}
	m.notification = Notification{Status: status, Summary: []resultCount{{Status: resultSuccess, Count: built, Label: "built"}, {Status: resultSuccess, Count: current, Label: "up to date"}, {Status: resultFailed, Count: failed, Label: "failed"}}}
	if status == resultSuccess {
		m.presentResult("Build results", status)
		if !msg.all && len(msg.results) == 1 {
			m.notification.Text = msg.results[0].App.Label() + " packaged successfully"
			if msg.results[0].Status == packager.StatusCurrent {
				m.notification.Text = msg.results[0].App.Label() + " is already up to date"
			}
		}
		return m, nil
	}
	if status != resultCancelled {
		if !msg.all && msg.err != nil {
			m.showError("Build failed", msg.err)
			return m, nil
		}
		if !msg.all && len(msg.results) == 1 && msg.results[0].Err != nil {
			result := msg.results[0]
			if result.Err != nil {
				m.showError("Could not build "+result.App.Label(), result.Err)
			}
			return m, nil
		}

	}
	m.showResults("Build results")
	m.resultCursor = firstFailedRow(m.resultRows)
	return m, nil
}

func (m Model) handleUnpackDone(msg unpackDoneMsg) (tea.Model, tea.Cmd) {
	targets := m.operationTargets(msg.id)
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	m.resultRows = append([]resultRow(nil), msg.rows...)
	if len(msg.rows) == 0 {
		for _, output := range msg.outputs {
			m.resultRows = append(m.resultRows, resultRow{Name: filepath.Base(output), Outcome: resultSuccess, Status: "✓ Unpacked", Detail: filepath.Base(output), OutputDir: output})
		}
	}
	for _, failure := range msg.failures {
		m.resultRows = append(m.resultRows, resultRow{Name: failure.Name, Outcome: resultFailed, Status: "X Failed", Detail: failure.Issue, AppID: failure.AppID})
	}
	m.resultTitle = fmt.Sprintf("%d succeeded · %d failed", msg.succeeded, len(msg.failures))
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.resultTitle += fmt.Sprintf(" · ! Cancelled · %d not processed", max(0, len(targets)-len(m.resultRows)))
		} else {
			m.resultRows = append(m.resultRows, resultRow{Name: "Unpacking", Outcome: resultFailed, Status: "X Failed", Detail: msg.err.Error()})
		}
		m.showResults("Unpack results")
	}
	status := resultSuccess
	if errors.Is(msg.err, context.Canceled) {
		status = resultCancelled
	} else if msg.err != nil || len(msg.failures) > 0 {
		status = resultFailed
	}
	m.presentResult("Unpack results", status)
	if status == resultSuccess && !msg.all && len(targets) == 1 {
		m.notification.Text = targets[0].App.Label() + " unpacked successfully"
	}
	m.notification.Summary = []resultCount{{Status: resultSuccess, Count: msg.succeeded, Label: "succeeded"}, {Status: resultFailed, Count: len(msg.failures), Label: "failed"}}
	return m, nil
}

func (m Model) handleUpdateDone(msg updateDoneMsg) (tea.Model, tea.Cmd) {
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	if errors.Is(msg.err, context.Canceled) {
		m.showMessage("Cancelled", "! The update was cancelled.")
	} else if msg.err != nil {
		m.showError("Update failed", msg.err)
	} else if msg.version == "" {
		m.updateResult.Available = false
		m.showMessage("Inpakker is current", "✓ No newer stable release is available.")
	} else {
		m.updateResult.Available = false
		m.showMessage("Update installed", "✓ Inpakker v"+msg.version+" was installed.\n\nRestart Inpakker to use the new version.")
	}
	return m, nil
}

func (m *Model) finishOperation(id int) bool {
	if m.operation == nil || m.operation.id != id {
		return false
	}
	m.operation.cancel()
	m.operation = nil
	m.modal = ModalNone
	return true
}

func (m *Model) setLog(title, contents string) {
	m.logTitle, m.logText = title, strings.TrimSpace(contents)
	m.logViewport.SetContent(m.logText)
	m.logViewport.GotoTop()
}

func validationOperationTitle(all bool, targets []ApplicationView) string {
	if all {
		return "Validating applications"
	}
	return "Validating " + targets[0].App.Label()
}

func buildOperationTitle(all bool, targets []ApplicationView) string {
	if all {
		return "Building applications"
	}
	return "Building " + targets[0].App.Label()
}

func unpackOperationTitle(all bool, targets []ApplicationView) string {
	if all {
		return "Unpacking applications"
	}
	return "Unpacking " + targets[0].App.Label()
}

// inspectOperationApplication inspects only a processed application's state.
func inspectOperationApplication(ws *workspace.Workspace, ref workspace.AppRef) ApplicationView {
	group := filepath.Dir(ref.Relative)
	if group == "." {
		group = "—"
	}
	service := packager.Service{Workspace: ws}
	app := ws.InspectContext(context.Background(), ref)
	return ApplicationView{App: app, Group: group, Build: service.InspectBuildStateForApp(context.Background(), app, false)}
}

var timeNow = time.Now
