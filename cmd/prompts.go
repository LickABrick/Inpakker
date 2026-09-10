package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func interactive(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.ErrOrStderr().(*os.File)
	return inputOK && outputOK && term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd())
}

func runForm(cmd *cobra.Command, form *huh.Form) error {
	accessible := os.Getenv("INPAKKER_ACCESSIBLE") != "" || os.Getenv("ACCESSIBLE") != ""
	return form.WithInput(cmd.InOrStdin()).WithOutput(cmd.ErrOrStderr()).WithAccessible(accessible).Run()
}

func promptNew(cmd *cobra.Command, options *workspace.CreateOptions) error {
	return runForm(cmd, huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("1 of 4 · Identity").Description("Choose the directory identifier and user-facing name."),
			huh.NewInput().Title("Application name").Value(&options.Name).Validate(func(value string) error {
				if !workspace.ValidAppName(value) {
					return errors.New("use a valid Windows directory name")
				}
				return nil
			}),
			huh.NewInput().Title("Display name").Value(&options.DisplayName),
		),
		huh.NewGroup(
			huh.NewNote().Title("2 of 4 · Organization"),
			huh.NewInput().Title("Group").Description("Optional path below the applications directory").Value(&options.Group).Validate(func(value string) error {
				if !workspace.ValidGroupName(value) {
					return errors.New("use a relative path of valid Windows directory names")
				}
				return nil
			}),
		),
		huh.NewGroup(
			huh.NewNote().Title("3 of 4 · Package source"),
			huh.NewInput().Title("Source directory").Value(&options.Source).Validate(safeOptionalPath),
			huh.NewInput().Title("Setup file").Value(&options.SetupFile).Validate(func(value string) error {
				if strings.TrimSpace(value) == "" {
					return errors.New("setup file is required")
				}
				return safeOptionalPath(value)
			}),
			huh.NewInput().Title("Output directory").Value(&options.OutputDir).Validate(safeOptionalPath),
		),
		huh.NewGroup(
			huh.NewNote().Title("4 of 4 · Review").DescriptionFunc(func() string {
				display := options.DisplayName
				if display == "" {
					display = options.Name
				}
				group := options.Group
				if group == "" {
					group = "—"
				}
				return fmt.Sprintf("Name          %s\nDisplay       %s\nGroup         %s\nSource        %s\nSetup         %s\nOutput        %s", options.Name, display, group, options.Source, options.SetupFile, options.OutputDir)
			}, options),
		),
	))
}

func promptApplications(cmd *cobra.Command, ws *workspace.Workspace, title string, multiple bool, requirePackage bool) ([]string, error) {
	apps, err := ws.List()
	if err != nil {
		return nil, err
	}
	options := make([]huh.Option[string], 0, len(apps))
	for _, app := range apps {
		if requirePackage && len(app.Packages) == 0 {
			continue
		}
		label := app.Label()
		if app.Ref.Relative != label {
			label += "  " + app.Ref.Relative
		}
		options = append(options, huh.NewOption(label, app.Ref.Relative))
	}
	if len(options) == 0 {
		return nil, errors.New("no matching applications found")
	}
	if multiple {
		var selected []string
		err = runForm(cmd, huh.NewForm(huh.NewGroup(
			huh.NewMultiSelect[string]().Title(title).Options(options...).Value(&selected).Validate(func(values []string) error {
				if len(values) == 0 {
					return errors.New("select at least one application")
				}
				return nil
			}),
		)))
		return selected, err
	}
	var selected string
	err = runForm(cmd, huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Options(options...).Value(&selected),
	)))
	if err != nil {
		return nil, err
	}
	return []string{selected}, nil
}

func safeOptionalPath(value string) error {
	if value != "" && !pathutil.IsSafeRelative(value) {
		return errors.New("path must remain within the application directory")
	}
	return nil
}
