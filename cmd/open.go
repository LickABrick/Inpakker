package cmd

import (
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/spf13/cobra"
)

func newOpenCmd(opener pathopener.PathOpener) *cobra.Command {
	return &cobra.Command{Use: "open [application]", Short: "Open workspace or application directory in Windows Explorer", Args: usageArgs(cobra.MaximumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
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
		}
		return opener.OpenDirectory(path)
	}}
}
func init() { rootCmd.AddCommand(newOpenCmd(pathopener.Explorer{})) }
