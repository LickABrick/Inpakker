package cmd

import (
	"fmt"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/types"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	parent := &cobra.Command{Use: "config", Short: "Inspect and edit typed global or workspace settings"}
	show := &cobra.Command{Use: "show", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		local, _ := cmd.Flags().GetBool("workspace-settings")
		if local {
			ws, err := resolveWorkspace(cmd)
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, ws.Config)
		}
		user, err := config.LoadUser()
		if err != nil {
			return err
		}
		return writeJSONOutput(cmd, user)
	}}
	set := &cobra.Command{Use: "set <known-key> <value>", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(cmd *cobra.Command, args []string) error {
		local, _ := cmd.Flags().GetBool("workspace-settings")
		if local {
			ws, err := resolveWorkspace(cmd)
			if err != nil {
				return err
			}
			if err := ws.SetSetting(args[0], args[1]); err != nil {
				return err
			}
			if args[0] != "name" {
				fmt.Fprintln(cmd.ErrOrStderr(), "! Existing files are not moved; inherited applications now use the new workspace setting.")
			}
			return nil
		}
		return config.UpdateUser(func(user *types.UserConfig) error { return config.SetUserValue(user, args[0], args[1]) })
	}}
	for _, c := range []*cobra.Command{show, set} {
		c.Flags().Bool("workspace-settings", false, "Use settings of the resolved workspace")
	}
	parent.AddCommand(show, set)
	return parent
}
func init() { rootCmd.AddCommand(newConfigCmd()) }
