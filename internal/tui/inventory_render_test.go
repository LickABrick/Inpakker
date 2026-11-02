package tui

import (
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/charmbracelet/x/ansi"
)

func TestInventoryHasOnePrimaryStatus(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	for _, width := range []int{40, 70, 90, 120} {
		columns := applicationColumns(width)
		row := applicationRow(app, width)
		if len(columns) != len(row) || columns[len(columns)-1].Title != "STATUS" {
			t.Fatalf("width %d: columns %#v, row %#v", width, columns, row)
		}
		for _, column := range columns {
			if column.Title == "VALIDATION" || column.Title == "BUILD" || column.Title == "PACKAGE" {
				t.Fatalf("width %d has secondary status column %q", width, column.Title)
			}
		}
		app.App.Status = "invalid"
		app.Build.State = packager.BuildStateCurrent
		row = applicationRow(app, width)
		if row[len(row)-1] != "X Invalid" {
			t.Fatalf("invalid application must take precedence: %#v", row)
		}
		app.App.Status = "valid"
	}
}

func TestInventoryPreviewAndDetailUsePackageFilename(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	app.App.Packages = []string{`C:\private\output\example.intunewin`}
	app.App.Config.InstallCommand = "long-install-command-not-for-preview"
	m.apps.refresh([]ApplicationView{app}, app.App.Ref.Relative)
	preview := ansi.Strip(m.inventoryPreview(app, 80))
	if !strings.Contains(preview, "example.intunewin") || strings.Contains(preview, "private") || strings.Contains(preview, "long-install-command") || strings.Contains(preview, "Validation") {
		t.Fatalf("preview is not concise: %s", preview)
	}
	detail := ansi.Strip(m.applicationView(80))
	if !strings.Contains(detail, "example.intunewin") || strings.Contains(detail, "private") || strings.Contains(detail, "decoder is not configured") {
		t.Fatalf("unexpected package detail: %s", detail)
	}
	if !strings.Contains(detail, "long-install-command-not-for-preview") {
		t.Fatal("full metadata must remain available in detail")
	}
}

func TestPackageFilenameSeparators(t *testing.T) {
	for _, path := range []string{"/private/output/example.intunewin", `C:\private\output\example.intunewin`, "example.intunewin"} {
		if got := packageFilename(path); got != "example.intunewin" {
			t.Errorf("packageFilename(%q) = %q", path, got)
		}
	}
}

func TestPadBetweenBoundsWideExternalMetadata(t *testing.T) {
	for _, width := range []int{20, 40, 70, 120} {
		got := padBetween(strings.Repeat("界", 100), strings.Repeat("界", 100), width)
		if ansi.StringWidth(got) > width {
			t.Errorf("width %d: rendered %d columns", width, ansi.StringWidth(got))
		}
	}
}
