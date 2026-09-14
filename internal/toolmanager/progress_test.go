package toolmanager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/internal/config"
)

func TestInstallReportsStagesAndDownloadBytes(t *testing.T) {
	for _, knownSize := range []bool{false, true} {
		for _, cancelDownload := range []bool{false, true} {
			t.Setenv("INPAKKER_HOME", t.TempDir())
			payload := append(executableFixture(), make([]byte, 128<<10)...)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "commits") {
					fmt.Fprintf(w, `[{"sha":"%s"}]`, strings.Repeat("a", 40))
					return
				}
				if knownSize {
					w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				_, _ = w.Write(payload)
			}))
			ctx, cancel := context.WithCancel(context.Background())
			var events []Event
			service := Service{APIBase: server.URL, RawBase: server.URL, OnProgress: func(e Event) {
				events = append(events, e)
				if cancelDownload && e.Phase == "downloading" && e.Bytes > 0 {
					cancel()
				}
			}}
			_, err := service.Install(ctx, "content-prep", true)
			cancel()
			server.Close()
			if cancelDownload {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				user, err := config.LoadUser()
				if err != nil || user.Tools.ContentPrepTool.Path != "" {
					t.Fatal("cancelled install configured a tool", err)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			last := int64(0)
			phases := map[string]bool{}
			for _, e := range events {
				phases[e.Phase] = true
				if e.Phase == "downloading" {
					if e.Bytes < last {
						t.Fatal("download went backwards", events)
					}
					last = e.Bytes
					if knownSize && e.TotalBytes != int64(len(payload)) || !knownSize && e.TotalBytes != 0 {
						t.Fatal(e)
					}
				}
			}
			if last != int64(len(payload)) {
				t.Fatal("missing final byte count", last)
			}
			for _, phase := range []string{"resolving source", "downloading", "verifying", "installing", "installed"} {
				if !phases[phase] {
					t.Fatal("missing phase", phase, events)
				}
			}
		}
	}
}
