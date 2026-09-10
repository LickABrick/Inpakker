package config

import (
	"github.com/LickABrick/inpakker/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserCreationRoundTripAndHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("INPAKKER_HOME", home)
	cfg, err := EnsureUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceDefaults.SourceDirectory != "source" {
		t.Fatal(cfg)
	}
	path, _ := UserPath()
	if path != filepath.Join(home, "config.json") {
		t.Fatal(path)
	}
	cfg.Preferences.ShowToolOutput = true
	if err := SaveUser(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUser()
	if err != nil || !loaded.Preferences.ShowToolOutput {
		t.Fatal(loaded, err)
	}
	cfg.SchemaVersion = 2
	if err := SaveUser(cfg); err == nil {
		t.Fatal("accepted schema")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"schemaVersion": 2`) {
		t.Fatal("invalid save replaced config")
	}
	for _, data := range []string{`{broken`, `{"schemaVersion":2}`, `{"schemaVersion":0}`, `{"schemaVersion":1,"unknown":true}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadUser(); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
func TestUUIDAndRegistrationValidation(t *testing.T) {
	a, b := NewUUID(), NewUUID()
	if !ValidUUID(a) || a == b {
		t.Fatal(a, b)
	}
	for _, bad := range []string{"../escape", "", strings.ReplaceAll(a, "-", ""), strings.ToUpper(a)} {
		if ValidUUID(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	user := DefaultUser()
	user.Workspaces = []types.WorkspaceRegistration{{ID: a, Path: t.TempDir()}}
	user.ActiveWorkspaceID = a
	if err := ValidateUser(&user); err != nil {
		t.Fatal(err)
	}
	user.Workspaces = append(user.Workspaces, user.Workspaces[0])
	if err := ValidateUser(&user); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestValidateAppAndEffectiveInheritance(t *testing.T) {
	app := types.AppConfig{SchemaVersion: 1, ID: NewUUID(), Name: "Mozilla Firefox", SetupFile: "Firefox Setup.exe"}
	if err := ValidateApp(&app); err != nil {
		t.Fatal(err)
	}
	effective := EffectiveApp(types.WorkspaceConfig{SourceDirectory: "source", OutputDirectory: "output"}, app)
	if !effective.SourceInherited || !effective.OutputInherited || effective.SourceDirectory != "source" {
		t.Fatal(effective)
	}
	app.SourceDirectory = "installer"
	app.OutputDirectory = "packages"
	effective = EffectiveApp(types.WorkspaceConfig{SourceDirectory: "source", OutputDirectory: "output"}, app)
	if effective.SourceInherited || effective.OutputInherited || effective.OutputDirectory != "packages" {
		t.Fatal(effective)
	}
	for _, value := range []string{"../escape", `C:\outside`, `..\escape`, "CON", "bad:stream"} {
		app.SourceDirectory = value
		if err := ValidateApp(&app); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
