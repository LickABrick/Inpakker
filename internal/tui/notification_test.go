package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/LickABrick/inpakker/internal/packager"
)

func TestQuietSuccessRetainsInspectableLastResult(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[all], func(t *testing.T) {
			m := newTestModel(t)
			app := m.apps.all[0]
			op := m.startOperation(operationBuild, "Building", 1)
			updated, _ := m.rememberResult(m.handleBuildDone(buildDoneMsg{
				id: op.id, all: all, log: "tool output",
				results: []packager.Result{{App: app.App, Status: packager.StatusCurrent}},
			}))
			m = updated.(Model)
			if m.modal != ModalNone || m.displayedResult != nil || m.lastResult == nil || m.lastResult.status != resultSuccess {
				t.Fatalf("quiet result: modal=%v result=%#v", m.modal, m.lastResult)
			}
			m.beginActions()
			if m.actionAppID != app.App.Ref.Relative {
				t.Fatal("quiet result replaced the page's application actions")
			}
			m.closeModal()
			m.reopenResult()
			if m.modal != ModalResults || len(m.resultRows) != 1 || m.logText != "tool output" {
				t.Fatalf("reopened result: modal=%v rows=%d log=%q", m.modal, len(m.resultRows), m.logText)
			}
		})
	}
}

func TestValidationNotificationsAndPartialCancellation(t *testing.T) {
	for _, err := range []error{nil, context.Canceled, errors.New("inspection failed")} {
		m := newTestModel(t)
		op := m.startOperation(operationValidate, "Validating", 2)
		updated, _ := m.rememberResult(m.handleValidationDone(validationDoneMsg{
			id: op.id, all: true, valid: 1, rows: []resultRow{{Name: "completed", Status: "arbitrary display wording"}}, err: err,
		}))
		m = updated.(Model)
		if err == nil {
			if m.modal != ModalNone || m.notificationView() == "" {
				t.Fatal("successful validation should show a quiet notification")
			}
		} else if m.modal != ModalResults || m.notificationView() != "" {
			t.Fatal("errors and cancellation must remain prominent")
		}
		if m.lastResult == nil || len(m.lastResult.rows) == 0 {
			t.Fatal("completed rows were not retained")
		}
	}
}

func TestUnpackSuccessDoesNotReplaceInventory(t *testing.T) {
	m := newTestModel(t)
	count := len(m.apps.all)
	op := m.startOperation(operationUnpack, "Unpacking", 1)
	updated, _ := m.handleUnpackDone(unpackDoneMsg{id: op.id, all: true, succeeded: 1, outputs: []string{"destination"}})
	m = updated.(Model)
	if m.modal != ModalNone || len(m.apps.all) != count || m.notification.Status != resultSuccess {
		t.Fatalf("unpack result: modal=%v inventory=%d notification=%#v", m.modal, len(m.apps.all), m.notification)
	}
}

func TestCreateNotificationRetainsLastResult(t *testing.T) {
	m := newTestModel(t)
	m.notify(NotificationSuccess, "example created")
	if m.modal != ModalNone || m.lastResult == nil {
		t.Fatal("create should be quiet and retain Last Result")
	}
	m.reopenResult()
	if m.modal != ModalMessage || m.modalBody != "example created" {
		t.Fatal("create result could not be reopened")
	}
}
