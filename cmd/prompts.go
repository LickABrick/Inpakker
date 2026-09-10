package cmd

import (
	"errors"
	"fmt"
	"os"

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

func promptNew(cmd *cobra.Command, ws *workspace.Workspace, options *workspace.CreateOptions) error {
	groups, err := ws.Groups()
	if err != nil {
		return err
	}
	choices := []huh.Option[string]{huh.NewOption("No group", "")}
	for _, group := range groups {
		choices = append(choices, huh.NewOption(group, group))
	}
	choices = append(choices, huh.NewOption("+ Create new group…", "__new__"))
	newGroup := ""
	if options.DirectoryName == "" {
		options.DirectoryName = workspace.Slug(options.Name)
	}
	err = runForm(cmd, huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Name").Value(&options.Name).Validate(huh.ValidateNotEmpty()),
		huh.NewInput().Title("Directory name").Description("Leave empty to generate from Name").Value(&options.DirectoryName),
		huh.NewSelect[string]().Title("Group").Options(choices...).Value(&options.Group),
		huh.NewInput().Title("New group (when selected)").Value(&newGroup),
		huh.NewInput().Title("Setup file").Value(&options.SetupFile).Validate(huh.ValidateNotEmpty()),
		huh.NewNote().Title("Workspace defaults").Description(fmt.Sprintf("Source directory: %s\nOutput directory: %s", ws.Config.SourceDirectory, ws.Config.OutputDirectory)),
		huh.NewNote().Title("Create application").Next(true).NextLabel("Create application"),
	)))
	if options.Group == "__new__" {
		options.Group = newGroup
	}
	return err
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
