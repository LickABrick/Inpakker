package cmd

import (
	"charm.land/huh/v2"
	"fmt"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/toolmanager"
	"github.com/spf13/cobra"
)

func newToolsCmd(service toolmanager.Service) *cobra.Command {
	parent := &cobra.Command{Use: "tools", Short: "Manage global external tools"}
	for _, action := range []string{"list", "detect"} {
		action := action
		command := &cobra.Command{Use: action, Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
			user, err := config.LoadUser()
			if err != nil {
				return err
			}
			statuses := toolmanager.List(user)
			if action == "detect" {
				if ws, err := resolveWorkspace(cmd); err == nil {
					service.WorkspaceRoot = ws.Root
				}
				statuses, err = service.Detect(cmd.Context(), user)
				if err != nil {
					return err
				}
				if err := toolmanager.ConfigureDetected(statuses); err != nil {
					return err
				}
			}
			if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
				return writeJSONOutput(cmd, statuses)
			}
			for _, status := range statuses {
				marker, path := "!", status.Config.Path
				if status.Valid {
					marker = "✓"
				}
				if path == "" {
					path = "Not configured"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s · %s\n", marker, status.Name, path)
				if status.Candidate != "" && status.Candidate != status.Config.Path {
					fmt.Fprintf(cmd.OutOrStdout(), "  Detected: %s (%s); use tools set %s to choose it\n", status.Candidate, status.Origin, status.ID)
				}
			}
			return nil
		}}
		command.Flags().Bool("json", false, "Write JSON")
		parent.AddCommand(command)
	}
	install := &cobra.Command{Use: "install <content-prep|decoder>", Short: "Download an external tool from official upstream", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		definition, err := toolmanager.DefinitionFor(args[0])
		if err != nil {
			return err
		}
		accepted, _ := cmd.Flags().GetBool("accept-license")
		fmt.Fprintf(cmd.ErrOrStderr(), "%s is provided by %s under separate upstream terms.\nSource/license: %s\n", definition.Name, definition.Repository, definition.LicenseURL)
		if !accepted && interactive(cmd) {
			if err := runForm(cmd, huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Accept upstream terms and download?").Value(&accepted)))); err != nil {
				return err
			}
		}
		result, err := service.Install(cmd.Context(), args[0], accepted)
		if err != nil {
			return err
		}
		newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr()).successDetail("Installed", result.Path)
		return nil
	}}
	install.Flags().Bool("accept-license", false, "Accept the upstream license terms")
	set := &cobra.Command{Use: "set <content-prep|decoder> <path>", Short: "Choose an existing executable", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(cmd *cobra.Command, args []string) error { return toolmanager.Set(args[0], args[1]) }}
	clear := &cobra.Command{Use: "clear <content-prep|decoder>", Short: "Clear configuration, preserving the executable", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error { return toolmanager.Set(args[0], "") }}
	parent.AddCommand(install, set, clear)
	return parent
}
func init() { rootCmd.AddCommand(newToolsCmd(toolmanager.Service{})) }
