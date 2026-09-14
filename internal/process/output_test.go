package process

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestOutputBufferRetainsBoundedTail(t *testing.T) {
	b := NewOutputBuffer(8)
	for _, chunk := range []string{"abc", "defg", "hijklmnop"} {
		n, err := b.Write([]byte(chunk))
		if n != len(chunk) || err != nil {
			t.Fatal(n, err)
		}
	}
	if got := b.String(); got != "[Earlier output omitted]\nijklmnop" {
		t.Fatal(got)
	}
	b.Write([]byte("終わり"))
	if !utf8.ValidString(b.String()) {
		t.Fatal("tail starts with broken UTF-8")
	}
}

func TestOutputBufferAcceptsConcurrentStreams(t *testing.T) {
	b := NewOutputBuffer(1024)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Write([]byte(strings.Repeat("x", 40)))
				_ = b.String()
			}
		}()
	}
	wg.Wait()
	if len(b.data) != 1024 {
		t.Fatal(len(b.data))
	}
}
