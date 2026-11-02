package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/packager"
)

type verificationDoneMsg struct {
	id          int
	workspaceID string
	apps        []ApplicationView
	err         error
}

// verifyBuildStatesCmd explicitly opts into authoritative content reads. The
// initial inventory and ordinary r refresh never need to wait for these reads.
func (m *Model) verifyBuildStatesCmd() tea.Cmd {
	if m.workspace == nil || m.operation != nil {
		return nil
	}
	targets := append([]ApplicationView(nil), m.apps.all...)
	ws := m.workspace
	op := m.startOperation(operationVerify, "Verifying build states", len(targets))
	op.targets = targets
	return tea.Batch(m.spinner.Tick, m.waitForActivity(op.id), func() tea.Msg {
		defer close(op.events)
		inspection := (packager.Service{Workspace: ws}).NewInspectionContext(op.context)
		updated := make([]ApplicationView, 0, len(targets))
		var err error
		for index, target := range targets {
			if err = op.context.Err(); err != nil {
				break
			}
			emitActivity(op, activityEvent{current: index + 1, completed: index, total: len(targets), label: target.App.Label(), phase: "verifying"})
			target.App = ws.InspectContext(op.context, target.App.Ref)
			target.Build = inspection.InspectBuildStateForApp(op.context, target.App, true)
			if target.Build.State == packager.BuildStateUnknown || target.Build.State == packager.BuildStateUnavailable {
				err = fmt.Errorf("verify %s: %s", target.App.Label(), target.Build.Reason)
				break
			}
			if err = op.context.Err(); err != nil {
				break
			}
			updated = append(updated, target)
		}
		// Persist completed evidence even on cancellation; incomplete apps are never
		// added to the evidence by the inspection service.
		saveErr := inspection.SaveEvidence(context.WithoutCancel(op.context))
		if err == nil {
			err = saveErr
		}
		return verificationDoneMsg{id: op.id, workspaceID: ws.Config.ID, apps: updated, err: err}
	})
}

func (m Model) handleVerificationDone(msg verificationDoneMsg) (tea.Model, tea.Cmd) {
	if !m.finishOperation(msg.id) {
		return m, nil
	}
	if m.workspace == nil || m.workspace.Config.ID != msg.workspaceID {
		return m, nil
	}
	m.apps.merge(msg.apps, "")
	m.resultTitle = fmt.Sprintf("%d applications verified", len(msg.apps))
	m.resultRows = make([]resultRow, 0, len(msg.apps))
	for _, app := range msg.apps {
		m.resultRows = append(m.resultRows, resultRow{Name: app.App.Label(), AppID: app.App.Ref.Relative, Status: buildLabel(app.Build.State), Detail: app.Build.Reason})
	}
	m.modalTitle = "Build-state verification"
	m.modal = ModalNone
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.notify(resultCancelled, "Build-state verification cancelled")
		} else {
			m.notification = Notification{Status: resultFailed}
			m.showError("Verification failed", msg.err)
		}
	} else {
		m.notify(NotificationSuccess, fmt.Sprintf("Verified %d build states", len(msg.apps)))
	}
	return m, nil
}
