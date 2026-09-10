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
	return huh.NewForm(groups...).
		WithKeyMap(keymap).
		WithTheme(m.theme.HuhTheme()).
		WithShowHelp(false).
		WithWidth(modalInnerWidth(m.width)).
		WithHeight(formContentHeight(m.height))
}

func (m *Model) beginCreate() teaCmd {
	if m.workspace == nil {
		m.showMessage("Workspace required", "Set up a workspace before creating an application.")
		return nil
	}
	m.createConfirmed = false
	m.create = &workspace.CreateOptions{Source: "source", OutputDir: m.workspace.DefaultOutputDir()}
	m.form = m.formWithTheme(
		huh.NewGroup(
			huh.NewNote().Title("1 of 4 · Identity").Description("Choose the directory identifier and user-facing name."),
			huh.NewInput().Title("Application name").Description("Used as the workspace directory and application identifier.").Value(&m.create.Name).Validate(validApplicationName),
			huh.NewInput().Title("Display name").Value(&m.create.DisplayName),
		),
		huh.NewGroup(
			huh.NewNote().Title("2 of 4 · Organization").Description("Groups are optional paths below the applications directory."),
			huh.NewInput().Title("Group").Placeholder("Browsers").Value(&m.create.Group).Validate(validGroupName),
		),
		huh.NewGroup(
			huh.NewNote().Title("3 of 4 · Package source"),
			huh.NewInput().Title("Source directory").Value(&m.create.Source).Validate(tuiSafePath),
			huh.NewInput().Title("Setup file").Value(&m.create.SetupFile).Validate(requiredSafePath),
			huh.NewInput().Title("Output directory").Value(&m.create.OutputDir).Validate(tuiSafePath),
		),
		huh.NewGroup(
			huh.NewNote().Title("4 of 4 · Review").DescriptionFunc(func() string { return createReview(*m.create) }, m.create),
			huh.NewConfirm().Title("Create application?").Affirmative("Create application").Negative("Cancel").Value(&m.createConfirmed),
		),
	)
	m.modal, m.modalTitle = ModalNewApplication, "New application"
	return m.form.Init()
}

func (m *Model) beginSetup() teaCmd {
	m.setupConfirmed = false
	m.setup = &workspace.SetupOptions{Root: m.root, AppsDir: "apps", OutputDir: "output", CreateExample: true}
	m.form = m.formWithTheme(
		huh.NewGroup(
			huh.NewNote().Title("Welcome to Inpakker").Description("Package Win32 applications for Microsoft Intune from an organized local workspace."),
			huh.NewNote().Title("1 of 4 · Workspace"),
			huh.NewInput().Title("Workspace directory").Value(&m.setup.Root).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Applications directory").Value(&m.setup.AppsDir).Validate(safeWorkspacePath),
			huh.NewInput().Title("Default output directory").Value(&m.setup.OutputDir).Validate(safeWorkspacePath),
		),
		huh.NewGroup(
			huh.NewNote().Title("2 of 4 · Packaging utility"),
			huh.NewInput().Title("IntuneWinAppUtil.exe").Description("Full path; may be left empty and configured later.").Value(&m.setup.IntuneWinAppUtil).Validate(optionalAbsolutePath),
			huh.NewConfirm().Title("Mute packaging utility output?").Value(&m.setup.MuteUtility),
		),
		huh.NewGroup(
			huh.NewNote().Title("3 of 4 · Optional decoder").Description("The decoder is only required for unpacking .intunewin files."),
			huh.NewInput().Title("IntuneWinAppUtilDecoder.exe").Description("Optional full path.").Value(&m.setup.DecoderPath).Validate(optionalAbsolutePath),
		),
		huh.NewGroup(
			huh.NewNote().Title("4 of 4 · Review").DescriptionFunc(func() string { return setupReview(*m.setup) }, m.setup),
			huh.NewConfirm().Title("Create example application?").Affirmative("Yes").Negative("No").Value(&m.setup.CreateExample),
			huh.NewConfirm().Title("Initialize this workspace?").Affirmative("Initialize").Negative("Cancel").Value(&m.setupConfirmed),
		),
	)
	m.modal, m.modalTitle = ModalSetup, "Set up Inpakker"
	return m.form.Init()
}

func (m *Model) beginBuildOptions() teaCmd {
	if _, ok := m.apps.selected(); !ok {
		m.showMessage("Build unavailable", "No application is selected.")
		return nil
	}
	m.buildMode = "smart"
	m.form = m.formWithTheme(huh.NewGroup(
		huh.NewNote().Description("Choose how to package the selected application."),
		huh.NewSelect[string]().Title("Build mode").Options(
			huh.NewOption("Smart build · Recommended", "smart"),
			huh.NewOption("Force rebuild", "force"),
			huh.NewOption("Build without cache", "no-cache"),
		).Value(&m.buildMode),
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
		huh.NewSelect[string]().Title("Package").Options(options...).Value(&m.selectedPackage),
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
		huh.NewConfirm().Title("Install update?").Affirmative("Install update").Negative("Cancel").Value(&m.updateConfirmed),
	))
	m.modal, m.modalTitle = ModalUpdate, "Update Inpakker"
	return m.form.Init()
}

func createReview(options workspace.CreateOptions) string {
	display := options.DisplayName
	if strings.TrimSpace(display) == "" {
		display = options.Name
	}
	group := options.Group
	if group == "" {
		group = "—"
	}
	return strings.Join([]string{
		keyValue("Name", options.Name, 14), keyValue("Display", display, 14),
		keyValue("Group", group, 14), keyValue("Source", options.Source, 14),
		keyValue("Setup", options.SetupFile, 14), keyValue("Output", options.OutputDir, 14),
	}, "\n")
}

func setupReview(options workspace.SetupOptions) string {
	utility := options.IntuneWinAppUtil
	if utility == "" {
		utility = "Configure later"
	}
	decoder := options.DecoderPath
	if decoder == "" {
		decoder = "Not configured"
	}
	return strings.Join([]string{
		keyValue("Workspace", options.Root, 18), keyValue("Applications", options.AppsDir, 18),
		keyValue("Output", options.OutputDir, 18), keyValue("Packaging utility", utility, 18),
		keyValue("Decoder", decoder, 18),
	}, "\n")
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
