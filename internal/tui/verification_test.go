package tui

import (
	"errors"
	"testing"
)

func TestVerificationFailureDoesNotClaimSuccess(t *testing.T) {
	m := newTestModel(t)
	op := m.startOperation(operationVerify, "Verifying", 1)
	updated, _ := m.handleVerificationDone(verificationDoneMsg{id: op.id, workspaceID: m.workspace.Config.ID, err: errors.New("tool unavailable")})
	m = updated.(Model)
	if m.modal != ModalMessage || m.notification.Status != resultFailed {
		t.Fatal("verification failure was hidden")
	}
}
