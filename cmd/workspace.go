package cmd

import (
	"charm.land/huh/v2"
	"encoding/json"
	"fmt"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
	"strings"
)

func resolveWorkspace(cmd *cobra.Command) (*workspace.Workspace, error) {
	selector := ""
	for c := cmd; c != nil; c = c.Parent() {
		if flag := c.Flags().Lookup("workspace"); flag != nil && flag.Value.Type() == "string" {
			selector = flag.Value.String()
			break
		}
	}
	return workspace.Resolve(selector, ".")
}
func writeJSONOutput(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
func newWorkspaceCmd() *cobra.Command {
	parent := &cobra.Command{Use: "workspace", Short: "Create, register and switch packaging workspaces"}
	list := &cobra.Command{Use: "list", Short: "List registered workspaces", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		user, err := config.LoadUser()
		if err != nil {
			return err
		}
		views := workspace.Registrations(user)
		if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
			return writeJSONOutput(cmd, views)
		}
		for _, view := range views {
			marker := "-"
			if view.Active {
				marker = "✓"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s · %s · %s\n", marker, view.Name, view.Path, view.Status)
		}
		return nil
	}}
	list.Flags().Bool("json", false, "Write JSON")
	var options workspace.CreateWorkspaceOptions
	create := &cobra.Command{Use: "create <directory>", Short: "Create and activate a workspace", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		options.Root = args[0]
		if strings.TrimSpace(options.Name) == "" {
			return asUsage(fmt.Errorf("--name is required"))
		}
		ws, err := workspace.CreateWorkspace(options)
		if err != nil {
			return err
		}
		newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr()).successDetail(ws.Config.Name, ws.Root)
		return nil
	}}
	create.Flags().StringVar(&options.Name, "name", "", "Workspace name")
	create.Flags().StringVar(&options.ApplicationsDirectory, "applications-directory", "", "Applications directory (uses global default)")
	create.Flags().StringVar(&options.SourceDirectory, "source-directory", "", "Source directory (uses global default)")
	create.Flags().StringVar(&options.OutputDirectory, "output-directory", "", "Output directory (uses global default)")
	add := &cobra.Command{Use: "add <directory>", Short: "Register an existing workspace", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspace.Add(args[0])
		if err == nil {
			newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr()).successDetail("Added workspace", ws.Config.Name)
		}
		return err
	}}
	use := &cobra.Command{Use: "use <name-or-path>", Short: "Activate a registered workspace", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspace.Use(args[0])
		if err == nil {
			newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr()).successDetail("Active workspace", ws.Config.Name)
		}
		return err
	}}
	show := &cobra.Command{Use: "show [name-or-path]", Short: "Inspect a workspace", Args: usageArgs(cobra.MaximumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		var view workspace.RegistrationView
		if len(args) == 0 {
			ws, err := resolveWorkspace(cmd)
			if err != nil {
				return err
			}
			view = workspace.RegistrationView{Name: ws.Config.Name, Status: "ready", Config: &ws.Config}
			view.Path = ws.Root
			view.ID = ws.Config.ID
		} else {
			user, err := config.LoadUser()
			if err != nil {
				return err
			}
			var lookupErr error
			view, lookupErr = workspace.Lookup(user, args[0])
			if lookupErr != nil {
				return lookupErr
			}
		}
		if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
			return writeJSONOutput(cmd, view)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\nPath: %s\nStatus: %s\n", view.Name, view.Path, view.Status)
		if view.Config != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Applications directory: %s\nSource directory: %s\nOutput directory: %s\n", view.Config.ApplicationsDirectory, view.Config.SourceDirectory, view.Config.OutputDirectory)
		}
		if view.Error != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Issue: %s\n", view.Error)
		}
		return nil
	}}
	show.Flags().Bool("json", false, "Write JSON")
	remove := &cobra.Command{Use: "remove <name-or-path>", Short: "Remove registration, preserving the workspace and all files", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			if !interactive(cmd) {
				return fmt.Errorf("use --yes to remove registration; the workspace folder and its files will not be deleted")
			}
			if err := runForm(cmd, huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Remove "+args[0]+" from Inpakker?").Description("The workspace folder and its files will not be deleted.").Value(&yes)))); err != nil {
				return err
			}
		}
		if !yes {
			return nil
		}
		return workspace.Remove(args[0])
	}}
	remove.Flags().Bool("yes", false, "Remove registration without prompting")
	relink := &cobra.Command{Use: "relink <name-or-path> <new-directory>", Short: "Relink a moved workspace with the same UUID", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(cmd *cobra.Command, args []string) error { return workspace.Relink(args[0], args[1]) }}
	parent.AddCommand(list, create, add, use, show, remove, relink)
	return parent
}
func init() {
	rootCmd.PersistentFlags().String("workspace", "", "Workspace name or path (overrides INPAKKER_WORKSPACE)")
	rootCmd.AddCommand(newWorkspaceCmd())
}
