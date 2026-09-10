package toolmanager

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/LickABrick/inpakker/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectionPrecedenceAndNoRecursiveScan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("INPAKKER_HOME", home)
	dirs := []string{t.TempDir(), t.TempDir(), filepath.Join(home, "tools", "content-prep"), t.TempDir(), t.TempDir(), t.TempDir()}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "IntuneWinAppUtil.exe"), []byte("tool"), 0755)
	}
	user := config.DefaultUser()
	user.Tools.ContentPrepTool.Path = filepath.Join(dirs[0], "IntuneWinAppUtil.exe")
	service := Service{WorkingDirectory: dirs[3], ExecutableDirectory: dirs[4], WorkspaceRoot: dirs[5], LookPath: func(string) (string, error) {
		p := filepath.Join(dirs[1], "IntuneWinAppUtil.exe")
		if _, err := os.Stat(p); err != nil {
			return "", err
		}
		return p, nil
	}}
	for i := range dirs {
		statuses, err := service.Detect(context.Background(), &user)
		if err != nil {
			t.Fatal(err)
		}
		if statuses[0].Candidate != filepath.Join(dirs[i], "IntuneWinAppUtil.exe") {
			t.Fatalf("step %d: %#v", i, statuses)
		}
		os.Remove(filepath.Join(dirs[i], "IntuneWinAppUtil.exe"))
	}
	nested := filepath.Join(dirs[5], "nested")
	os.MkdirAll(nested, 0755)
	os.WriteFile(filepath.Join(nested, "IntuneWinAppUtil.exe"), []byte("tool"), 0755)
	statuses, err := service.Detect(context.Background(), &user)
	if err != nil || statuses[0].Candidate != "" {
		t.Fatal(statuses, err)
	}
}

// A minimal structurally valid PE32 image, with no native process invocation.
func executableFixture() []byte {
	data := make([]byte, 512)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[60:], 128)
	copy(data[128:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[132:], 0x14c)
	binary.LittleEndian.PutUint16(data[148:], 224)
	binary.LittleEndian.PutUint16(data[150:], 2)
	binary.LittleEndian.PutUint16(data[152:], 0x10b)
	binary.LittleEndian.PutUint32(data[244:], 16)
	return data
}
func TestOfficialInstallLicenseAtomicityAndFailures(t *testing.T) {
	for _, mode := range []string{"success", "http", "truncated", "empty", "invalid", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("INPAKKER_HOME", t.TempDir())
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if strings.Contains(r.URL.Path, "commits") {
					fmt.Fprintf(w, `[{"sha":"%s"}]`, strings.Repeat("a", 40))
					return
				}
				switch mode {
				case "http":
					w.WriteHeader(500)
				case "truncated":
					w.Header().Set("Content-Length", "1000")
					w.Write([]byte("MZ"))
				case "empty":
				case "invalid":
					w.Write([]byte("not an executable"))
				default:
					w.Write(executableFixture())
				}
			}))
			defer server.Close()
			service := Service{APIBase: server.URL, RawBase: server.URL}
			if _, err := service.Install(context.Background(), "content-prep", false); err == nil || requests != 0 {
				t.Fatal("license not enforced")
			}
			for _, id := range []string{"content-prep", "decoder"} {
				ctx, cancel := context.WithCancel(context.Background())
				if mode == "cancel" {
					cancel()
				}
				defer cancel()
				result, err := service.Install(ctx, id, true)
				if mode == "success" {
					if err != nil {
						t.Fatal(err)
					}
					if result.Version != strings.Repeat("a", 40) || result.SHA256 == "" {
						t.Fatal(result)
					}
					if _, err := os.Stat(result.Path); err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("bad download installed")
				}
			}
		})
	}
}
func TestDetectionPreservesConfiguredInvalidPath(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	user := config.DefaultUser()
	user.Tools.ContentPrepTool.Path = filepath.Join(t.TempDir(), "missing.exe")
	config.SaveUser(&user)
	if err := ConfigureDetected([]Status{{ID: "content-prep", Candidate: "replacement"}}); err != nil {
		t.Fatal(err)
	}
	after, _ := config.LoadUser()
	if after.Tools.ContentPrepTool.Path != user.Tools.ContentPrepTool.Path {
		t.Fatal("overwrote explicit path")
	}
}
