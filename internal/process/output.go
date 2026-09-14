package process

import (
	"strings"
	"sync"
)

// OutputBuffer keeps the tail of process output. Both output streams may write
// concurrently; retaining diagnostics must not depend on terminal verbosity.
type OutputBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func NewOutputBuffer(limit int) *OutputBuffer { return &OutputBuffer{limit: max(1, limit)} }

func (b *OutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if len(b.data)+n > b.limit {
		b.truncated = true
	}
	if n >= b.limit {
		b.data = append(b.data[:0], p[n-b.limit:]...)
	} else {
		if excess := len(b.data) + n - b.limit; excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func (b *OutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := strings.ToValidUTF8(string(b.data), "�")
	if b.truncated {
		return "[Earlier output omitted]\n" + text
	}
	return text
}
