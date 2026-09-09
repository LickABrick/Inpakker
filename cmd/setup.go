package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	options := workspace.SetupOptions{Root: ".", AppsDir: "apps", OutputDir: "output", CreateExample: true}
	var noInput, noExample bool
	command := &cobra.Command{
		Use:   "setup [workspace-directory]",
		Short: "Initialize an Inpakker workspace",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				options.Root = args[0]
			}
			options.CreateExample = !noExample
			if interactive(cmd) && !noInput {
				if _, err := os.Stat(filepath.Join(options.Root, workspace.ConfigFile)); err == nil {
					return fmt.Errorf("workspace is already initialized; run 'inpakker doctor' or pass another directory")
				}
				if err := promptSetup(cmd, &options); err != nil {
					return err
				}
			}
			return runSetup(cmd, options)
		},
	}
	command.Flags().StringVar(&options.AppsDir, "apps-dir", "apps", "Applications directory within the workspace")
	command.Flags().StringVar(&options.OutputDir, "output-dir", "output", "Default output directory within each application")
	command.Flags().StringVar(&options.IntuneWinAppUtil, "intune-util", "", "Full path to IntuneWinAppUtil.exe")
	command.Flags().StringVar(&options.DecoderPath, "decoder", "", "Full path to IntuneWinAppUtilDecoder.exe")
	command.Flags().BoolVar(&options.MuteUtility, "mute-utility", false, "Suppress output from IntuneWinAppUtil.exe")
	command.Flags().BoolVar(&noExample, "no-example", false, "Do not create the example PowerShell application")
	command.Flags().BoolVar(&noInput, "no-input", false, "Disable interactive prompts")
	return command
}

func promptSetup(cmd *cobra.Command, options *workspace.SetupOptions) error {
	return runForm(cmd, huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Welcome to Inpakker").Description("Set up a workspace for packaging Win32 applications for Microsoft Intune."),
			huh.NewInput().Title("Workspace directory").Value(&options.Root).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Applications directory").Value(&options.AppsDir).Validate(safeWorkspacePath),
			huh.NewInput().Title("Default output directory").Value(&options.OutputDir).Validate(safeWorkspacePath),
		),
		huh.NewGroup(
			huh.NewInput().Title("IntuneWinAppUtil.exe").Description("Full path; may be left empty and configured later").Value(&options.IntuneWinAppUtil).Validate(optionalAbsolutePath),
			huh.NewInput().Title("IntuneWinAppUtilDecoder.exe").Description("Optional full path; enables unpacking").Value(&options.DecoderPath).Validate(optionalAbsolutePath),
			huh.NewConfirm().Title("Mute output from the packaging utility?").Value(&options.MuteUtility),
			huh.NewConfirm().Title("Create a valid PowerShell example application?").Affirmative("Yes").Negative("No").Value(&options.CreateExample),
		),
	))
}

func optionalAbsolutePath(value string) error {
	if value != "" && !filepath.IsAbs(value) {
		return errors.New("use a full path")
	}
	return nil
}

func safeWorkspacePath(value string) error {
	if strings.TrimSpace(value) == "" || !pathutil.IsSafeRelative(value) {
		return errors.New("use a relative directory within the workspace")
	}
	return nil
}

func runSetup(cmd *cobra.Command, options workspace.SetupOptions) error {
	result, err := workspace.Initialize(options)
	if err != nil {
		return err
	}
	console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
	console.successDetail("Workspace", result.Workspace.Root)
	console.successDetail("Configuration", workspace.ConfigFile)
	if result.Example != nil {
		console.successDetail("Example", result.Example.Relative)
	}
	if result.Workspace.Config.IntuneWinAppUtil == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNext: add the full IntuneWinAppUtil.exe path to inpakker.config.json, then run inpakker doctor")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNext: run inpakker doctor, then inpakker build --all")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newSetupCmd())
}
