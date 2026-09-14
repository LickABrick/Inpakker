package cmd

import (
	"errors"
	"fmt"
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/spf13/cobra"
)

func newOpenCmd(opener pathopener.PathOpener) *cobra.Command {
	var output bool
	command := &cobra.Command{Use: "open [application]", Short: "Open workspace, application or output directory in Windows Explorer", Args: usageArgs(func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if output && len(args) == 0 {
			return errors.New("--output requires an application")
		}
		return nil
	}), RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := resolveWorkspace(cmd)
		if err != nil {
			return err
		}
		path := ws.Root
		if len(args) > 0 {
			ref, err := ws.Find(args[0])
			if err != nil {
				return err
			}
			path = ref.Path
			if output {
				app := ws.Inspect(ref)
				if app.Config == nil {
					return fmt.Errorf("read application settings: %s", app.Error)
				}
				path, err = ws.OutputDir(ref, app.Config)
				if err != nil {
					return err
				}
			}
		}
		return opener.OpenDirectory(path)
	}}
	command.Flags().BoolVar(&output, "output", false, "Open the application's effective package output directory")
	return command
}
func init() { rootCmd.AddCommand(newOpenCmd(pathopener.Explorer{})) }
