package tui

import "github.com/charmbracelet/x/ansi"

// resultStatus describes an outcome independently of its user-facing wording.
type resultStatus uint8

// NotificationSuccess is the quiet, non-modal completion status.
const NotificationSuccess = resultSuccess

func (m *Model) notify(status resultStatus, text string) {
	m.notification = Notification{Status: status, Text: text}
	m.modal = ModalNone
	r := &operationResult{status: status, modal: ModalMessage, title: "Application created", body: text}
	if m.workspace != nil {
		r.workspaceID, r.root = m.workspace.Config.ID, m.workspace.Root
	}
	m.lastResult = r
	m.displayedResult = nil
}

const (
	resultUnknown resultStatus = iota
	resultSuccess
	resultFailed
	resultCancelled
)

// Notification is session-local feedback. It stays visible until another
// operation starts; no timer or background work is required to render it.
type Notification struct {
	Status resultStatus
	Text   string
	Summary []resultCount
}

// resultCount carries summary styling without parsing translated display text.
type resultCount struct {
	Status resultStatus
	Count int
	Label string
}

func (m *Model) presentResult(title string, status resultStatus) {
	m.notification.Status = status
	m.notification.Text = ""
	if status == resultSuccess {
		m.notification.Text = title + ": " + m.resultTitle + " · Last Result for details"
		m.modal = ModalNone
		// Preserve the inspectable result metadata even though it is not opened.
		m.modalTitle, m.modalBody, m.modalErr = title, "", nil
		return
	}
	m.showResults(title)
}

// notificationView is render-only. The page renderer can place this immediately
// above its footer; suppress it while a modal or foreground operation is open.
func (m Model) notificationView() string {
	if m.modal != ModalNone || m.operation != nil || m.notification.Text == "" {
		return ""
	}
	text := ansi.Truncate("✓ "+m.notification.Text, max(1, m.width-4), "…")
	return m.theme.StatusSuccess.Render(text)
}
