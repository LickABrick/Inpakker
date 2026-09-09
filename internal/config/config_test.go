package config

import (
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/types"
)

func TestValidateApp(t *testing.T) {
	valid := &types.AppConfig{
		Name:        "example",
		DisplayName: "Example",
		Source:      "source",
		SetupFile:   "setup.exe",
		OutputDir:   "output",
	}
	if err := ValidateApp(valid); err != nil {
		t.Fatalf("ValidateApp(valid) returned %v", err)
	}

	invalid := &types.AppConfig{Source: "../outside", SetupFile: "/setup.exe"}
	err := ValidateApp(invalid)
	if err == nil {
		t.Fatal("ValidateApp(invalid) returned nil")
	}
	for _, message := range []string{
		"name is required",
		"displayName is required",
		"source must be a relative path",
		"setupFile must be a relative path",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestValidateGlobal(t *testing.T) {
	if err := ValidateGlobal(&types.GlobalConfig{AppsDir: "packages", DefaultOutputDir: "output"}); err != nil {
		t.Fatalf("ValidateGlobal(valid) returned %v", err)
	}

	err := ValidateGlobal(&types.GlobalConfig{AppsDir: "../apps", DefaultOutputDir: "/output"})
	if err == nil {
		t.Fatal("ValidateGlobal(invalid) returned nil")
	}
	if !strings.Contains(err.Error(), "appsDir") || !strings.Contains(err.Error(), "defaultOutputDir") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
