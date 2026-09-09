package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/LickABrick/inpakker/internal/updater"
)

func TestUpdateJSONReportsAvailableReleaseWithoutPrompting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":   "v0.3.0",
			"html_url":   "https://example.test/releases/v0.3.0",
			"draft":      false,
			"prerelease": false,
			"assets": []map[string]string{
				{"name": "inpakker_v0.3.0_windows_amd64.zip", "browser_download_url": base + "/archive"},
				{"name": "inpakker_v0.3.0_checksums.txt", "browser_download_url": base + "/checksums"},
				{"name": "inpakker_v0.3.0_checksums.txt.sig", "browser_download_url": base + "/signature"},
			},
		})
	}))
	defer server.Close()

	previousFactory, previousVersion := newUpdateService, currentVersion
	t.Cleanup(func() {
		newUpdateService, currentVersion = previousFactory, previousVersion
	})
	currentVersion = "0.2.0"
	newUpdateService = func(version string) *updater.Service {
		service := updater.New(version)
		service.APIURL = server.URL
		service.GOOS, service.GOARCH = "windows", "amd64"
		service.StatePath = filepath.Join(t.TempDir(), "update-state.json")
		service.Now = func() time.Time { return time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC) }
		return service
	}

	command := newUpdateCmd()
	var output bytes.Buffer
	command.SetIn(bytes.NewReader(nil))
	command.SetOut(&output)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("update --json returned %v", err)
	}
	var result updater.Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output %q: %v", output.String(), err)
	}
	if !result.Available || result.CurrentVersion != "0.2.0" || result.LatestVersion != "0.3.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestUpdateRejectsConflictingNonInteractiveFlags(t *testing.T) {
	command := newUpdateCmd()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--json", "--yes"})
	if err := command.Execute(); err == nil {
		t.Fatal("update accepted --json with --yes")
	}
}
