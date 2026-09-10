package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/unpacker"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func (m Model) createCmd() tea.Cmd {
	options, ws := *m.create, m.workspace
	return func() tea.Msg {
		ref, err := ws.Create(options)
		if err != nil {
			return createMsg{err: err}
		}
		apps, inspectErr := inspectApplications(ws)
		return createMsg{ref: ref, apps: apps, inventoryErr: inspectErr}
	}
}

func (m Model) startValidate(all bool) (tea.Model, tea.Cmd) {
	targets, err := m.targetApplications(all)
	if err != nil {
		m.showError("Validation unavailable", err)
		return m, nil
	}
	op := m.startOperation(operationValidate, validationOperationTitle(all, targets), len(targets))
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		valid := 0
		issues := make([]validationIssue, 0)
		for index, target := range targets {
			if err := op.context.Err(); err != nil {
				return validationDoneMsg{id: op.id, all: all, err: err}
			}
			emitActivity(op, activityEvent{current: index + 1, completed: index, total: len(targets), label: target.App.Label(), phase: "validating"})
			app := ws.Inspect(target.App.Ref)
			if app.Status == "valid" {
				valid++
			} else {
				issues = append(issues, validationIssue{AppID: app.Ref.Relative, Name: app.Label(), Issue: app.Error})
			}
		}
		emitActivity(op, activityEvent{current: len(targets), completed: len(targets), total: len(targets), phase: "validation complete"})
		apps, inspectErr := inspectApplications(ws)
		return validationDoneMsg{id: op.id, valid: valid, issues: issues, apps: apps, all: all, err: inspectErr}
	})
}

func (m Model) startBuild(all bool, options packager.BuildOptions) (tea.Model, tea.Cmd) {
	targets, err := m.targetApplications(all)
	if err != nil {
		m.showError("Build unavailable", err)
		return m, nil
	}
	if !all && targets[0].App.Status != "valid" {
		m.showMessage("Build unavailable", targets[0].App.Label()+" cannot be built because its configuration is invalid.\n\nX "+targets[0].App.Error)
		return m, nil
	}
	service := packager.Service{Workspace: m.workspace, Runner: m.runner}
	if err := service.Validate(); err != nil {
		m.showError("Build unavailable", err)
		return m, nil
	}
	refs := make([]workspace.AppRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, target.App.Ref)
	}
	op := m.startOperation(operationBuild, buildOperationTitle(all, targets), len(targets))
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		var output bytes.Buffer
		options.OnProgress = func(event packager.Event) {
			emitActivity(op, activityEvent{current: event.Index, completed: event.Index - 1, total: event.Total, label: event.App, phase: event.Phase})
		}
		results, buildErr := service.Build(op.context, refs, options, &output, &output)
		emitActivity(op, activityEvent{current: len(results), completed: len(results), total: len(targets), phase: "build complete"})
		apps, inspectErr := inspectApplications(ws)
		if buildErr == nil {
			buildErr = inspectErr
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
	service := unpacker.Service{DecoderPath: m.workspace.User.Tools.Decoder.Path, Runner: m.runner}
	if err := service.Validate(); err != nil {
		m.showMessage("Unpacking unavailable", "The IntuneWinAppUtilDecoder is not configured or cannot be used for this workspace.\n\n"+err.Error()+"\n\nPress d from the application page to open Diagnostics.")
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
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		outputs := make([]string, 0)
		failures := make([]unpackFailure, 0)
		for index, target := range targets {
			if err := op.context.Err(); err != nil {
				return unpackDoneMsg{id: op.id, all: all, err: err}
			}
			packagePath := selectedPackage
			if all {
				if len(target.App.Packages) != 1 {
					reason := "no package found"
					if len(target.App.Packages) > 1 {
						reason = "multiple packages found"
					}
					failures = append(failures, unpackFailure{Name: target.App.Label(), Issue: reason})
					continue
				}
				packagePath = target.App.Packages[0]
			}
			emitActivity(op, activityEvent{current: index + 1, completed: index, total: len(targets), label: target.App.Label(), phase: "decoding"})
			result := service.Unpack(op.context, packagePath, "", false, io.Discard, io.Discard)
			if result.Err != nil {
				failures = append(failures, unpackFailure{Name: target.App.Label(), Issue: result.Err.Error()})
			} else {
				outputs = append(outputs, result.Destination)
			}
		}
		emitActivity(op, activityEvent{current: len(targets), completed: len(targets), total: len(targets), phase: "unpack complete"})
		apps, inspectErr := inspectApplications(ws)
		return unpackDoneMsg{id: op.id, succeeded: len(outputs), outputs: outputs, failures: failures, apps: apps, all: all, err: inspectErr}
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
	m.nextOperationID++
	m.generation++
	m.refreshing = false
	ctx, cancel := context.WithCancel(m.ctx)
	op := &operationState{
		id: m.nextOperationID, kind: kind, title: title, context: ctx, cancel: cancel,
		events: make(chan activityEvent), current: activityEvent{total: total, phase: "starting"}, started: timeNow(),
	}
	m.operation, m.modal, m.form = op, ModalProgress, nil
	return op
}

func (m *Model) cancelOperation() {
	if m.operation == nil {
		return
	}
	m.operation.cancel()
	m.nextOperationID++
	m.operation = nil
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
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	if errors.Is(msg.err, context.Canceled) {
		m.showMessage("Cancelled", "Validation was cancelled.")
		return m, nil
	}
	if msg.err != nil {
		m.showError("Validation failed", msg.err)
		return m, nil
	}
	m.apps.refresh(msg.apps, "")
	if !msg.all {
		if len(msg.issues) == 0 {
			m.showMessage("Validation complete", "✓ Application configuration and source files are valid.")
		} else {
			m.showMessage("Validation failed", "X "+msg.issues[0].Name+" is invalid.\n\n"+msg.issues[0].Issue)
		}
		return m, nil
	}
	m.validationValid, m.validationIssues = msg.valid, msg.issues
	m.resultTitle = fmt.Sprintf("%d valid · %d invalid", msg.valid, len(msg.issues))
	m.resultRows = make([]resultRow, 0, len(msg.issues))
	for _, issue := range msg.issues {
		m.resultRows = append(m.resultRows, resultRow{Name: issue.Name, Status: "X Invalid", Detail: issue.Issue, AppID: issue.AppID})
	}
	m.resultCursor = 0
	m.pushRoute(Route{Kind: RouteValidationResults})
	return m, nil
}

func (m Model) handleBuildDone(msg buildDoneMsg) (tea.Model, tea.Cmd) {
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	m.setLog("Packaging tool output", msg.log)
	if errors.Is(msg.err, context.Canceled) {
		m.showMessage("Cancelled", "Build was cancelled.")
		return m, nil
	}
	if len(msg.apps) > 0 {
		m.apps.refresh(msg.apps, "")
	}
	if !msg.all && msg.err != nil {
		m.showError("Build failed", msg.err)
		return m, nil
	}
	if !msg.all && len(msg.results) == 1 {
		result := msg.results[0]
		if result.Err != nil {
			m.showError("Could not build "+result.App.Label(), result.Err)
		} else if result.Status == packager.StatusCurrent {
			m.showMessage("Build complete", "✓ "+result.App.Label()+" is already up to date.\n\nNo package needed to be rebuilt.")
		} else {
			body := "✓ " + result.App.Label() + " was packaged successfully."
			if len(result.Artifacts) > 0 {
				body += "\n\nOutput\n" + strings.Join(result.Artifacts, "\n")
			}
			m.showMessage("Build complete", body)
		}
		return m, nil
	}
	built, current, failed := 0, 0, 0
	m.resultRows = make([]resultRow, 0, len(msg.results))
	for _, result := range msg.results {
		row := resultRow{Name: result.App.Label(), AppID: result.App.Ref.Relative}
		switch {
		case result.Err != nil:
			failed++
			row.Status, row.Detail = "X Failed", result.Err.Error()
		case result.Status == packager.StatusCurrent:
			current++
			row.Status, row.Detail = "✓ Up to date", result.Reason
		default:
			built++
			row.Status, row.Detail = "✓ Built", strings.Join(result.Artifacts, ", ")
		}
		m.resultRows = append(m.resultRows, row)
	}
	if msg.err != nil {
		failed++
		m.resultRows = append(m.resultRows, resultRow{Name: "Build operation", Status: "X Failed", Detail: msg.err.Error()})
	}
	m.resultTitle = fmt.Sprintf("%d built · %d up to date · %d failed", built, current, failed)
	m.resultCursor = 0
	m.pushRoute(Route{Kind: RouteBuildResults})
	return m, nil
}

func (m Model) handleUnpackDone(msg unpackDoneMsg) (tea.Model, tea.Cmd) {
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	if errors.Is(msg.err, context.Canceled) {
		m.showMessage("Cancelled", "Unpacking was cancelled.")
		return m, nil
	}
	if msg.err != nil {
		m.showError("Unpack failed", msg.err)
		return m, nil
	}
	m.apps.refresh(msg.apps, "")
	if !msg.all {
		if len(msg.failures) > 0 {
			m.showMessage("Unpack failed", "X "+msg.failures[0].Name+" could not be unpacked.\n\n"+msg.failures[0].Issue)
		} else {
			body := "✓ Package unpacked successfully."
			if len(msg.outputs) > 0 {
				body += "\n\nDestination\n" + msg.outputs[0]
			}
			m.showMessage("Unpack complete", body)
		}
		return m, nil
	}
	m.resultRows = make([]resultRow, 0, msg.succeeded+len(msg.failures))
	for _, output := range msg.outputs {
		m.resultRows = append(m.resultRows, resultRow{Name: filepath.Base(output), Status: "✓ Unpacked", Detail: output})
	}
	for _, failure := range msg.failures {
		m.resultRows = append(m.resultRows, resultRow{Name: failure.Name, Status: "X Failed", Detail: failure.Issue})
	}
	m.resultTitle = fmt.Sprintf("%d succeeded · %d failed", msg.succeeded, len(msg.failures))
	m.resultCursor = 0
	m.pushRoute(Route{Kind: RouteUnpackResults})
	return m, nil
}

func (m Model) handleUpdateDone(msg updateDoneMsg) (tea.Model, tea.Cmd) {
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	if errors.Is(msg.err, context.Canceled) {
		m.showMessage("Cancelled", "The update was cancelled.")
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

var timeNow = time.Now
