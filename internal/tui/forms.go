package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func (m *Model) formWithTheme(groups ...*huh.Group) *huh.Form {
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit.SetKeys("ctrl+c", "esc")
	keymap.Input.Prev.SetKeys("up", "shift+tab")
	keymap.Input.Next.SetKeys("down", "tab", "enter")
	return huh.NewForm(groups...).
		WithKeyMap(keymap).
		WithTheme(m.theme.HuhTheme()).
		WithShowHelp(false).
		WithWidth(modalInnerWidth(m.width)).
		WithHeight(formContentHeight(m.height))
}

func (m *Model) beginCreate() teaCmd {
	if m.workspace == nil {
		return m.beginWorkspaceCreate()
	}
	m.create = &workspace.CreateOptions{}
	m.directoryEdited = false
	m.newGroup = ""
	options := []huh.Option[string]{huh.NewOption("No group", "")}
	for _, group := range m.groups {
		options = append(options, huh.NewOption(group, group))
	}
	options = append(options, huh.NewOption("+ Create new group…", "__new__"))
	m.directoryInput = huh.NewInput().Key("directory").Title("Directory name").Value(&m.create.DirectoryName).Validate(validApplicationName)
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewInput().Key("name").Title("Name").Value(&m.create.Name).Validate(huh.ValidateNotEmpty()),
		m.directoryInput,
		huh.NewSelect[string]().Title("Group").Options(options...).Value(&m.create.Group),
		huh.NewInput().Key("newGroup").Title("New group (when selected)").Validate(validGroupName),
		huh.NewInput().Title("Setup file").Value(&m.create.SetupFile).Validate(requiredSafePath),
		huh.NewNote().Title("Workspace defaults").Description(fmt.Sprintf("Source directory: %s\nOutput directory: %s", m.workspace.Config.SourceDirectory, m.workspace.Config.OutputDirectory)),
		huh.NewNote().Title("Create application").Next(true).NextLabel("Create application"),
	))
	m.modal, m.modalTitle = ModalNewApplication, "Create application"
	return m.form.Init()
}

func (m *Model) beginBuildOptions() teaCmd {
	if _, ok := m.apps.selected(); !ok {
		m.showMessage("Build unavailable", "No application is selected.")
		return nil
	}
	m.buildMode = "build"
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewNote().Description("Choose how to package the selected application."),
		huh.NewSelect[string]().Key("buildMode").Title("Build mode").Options(
			huh.NewOption("Build", "build"),
			huh.NewOption("Rebuild", "force"),
			huh.NewOption("Build without cache", "no-cache"),
		),
	))
	m.modal, m.modalTitle = ModalBuildOptions, "Build options"
	return m.form.Init()
}

func (m *Model) beginPackageSelect(packages []string) teaCmd {
	m.selectedPackage = packages[0]
	options := make([]huh.Option[string], 0, len(packages))
	for _, item := range packages {
		options = append(options, huh.NewOption(filepath.Base(item), item))
	}
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewNote().Description("This application has multiple packages. Choose one to unpack."),
		huh.NewSelect[string]().Key("package").Title("Package").Options(options...),
	))
	m.modal, m.modalTitle = ModalPackageSelect, "Select package to unpack"
	return m.form.Init()
}

func (m *Model) beginUpdate() teaCmd {
	if m.updater == nil {
		m.showMessage("Update checking disabled", "Automatic update checks are disabled for this session.")
		return nil
	}
	if !m.updateResult.Available {
		m.showMessage("Inpakker is current", "✓ No newer stable release is available.")
		return nil
	}
	m.updateConfirmed = false
	description := fmt.Sprintf("Current       v%s\nAvailable     v%s\n\nThe release will be downloaded and its signature and checksum verified.", m.updateResult.CurrentVersion, m.updateResult.LatestVersion)
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewNote().Description(description),
		huh.NewConfirm().Key("installUpdate").Title("Install update?").Affirmative("Install update").Negative("Cancel"),
	))
	m.modal, m.modalTitle = ModalUpdate, "Update Inpakker"
	return m.form.Init()
}

func validApplicationName(value string) error {
	if !workspace.ValidAppName(value) {
		return errors.New("use a valid Windows directory name")
	}
	return nil
}

func validGroupName(value string) error {
	if !workspace.ValidGroupName(value) {
		return errors.New("use a relative path of valid Windows directory names")
	}
	return nil
}

func tuiSafePath(value string) error {
	if value == "" || !pathutil.IsSafeRelative(value) {
		return errors.New("path must remain within the application directory")
	}
	return nil
}

func requiredSafePath(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("setup file is required")
	}
	return tuiSafePath(value)
}

func safeWorkspacePath(value string) error {
	if strings.TrimSpace(value) == "" || !pathutil.IsSafeRelative(value) {
		return errors.New("use a relative directory within the workspace")
	}
	return nil
}

func optionalAbsolutePath(value string) error {
	if value != "" && !filepath.IsAbs(value) {
		return errors.New("use a full path")
	}
	return nil
}

type teaCmd = tea.Cmd
