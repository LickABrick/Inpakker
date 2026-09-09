package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "list [app-name|group-name...]",
		Short: "List applications and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Open(".")
			if err != nil {
				return err
			}
			selection, err := ws.Discover(args, len(args) == 0)
			if err != nil {
				return err
			}
			apps := make([]workspace.App, 0, len(selection.Apps))
			for _, ref := range selection.Apps {
				apps = append(apps, ws.Inspect(ref))
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(apps)
			}
			if len(apps) == 0 {
				return noApplicationsError(selection.Skipped)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "APP\tGROUP\tSTATUS\tPACKAGES")
			for _, app := range apps {
				group := filepath.Dir(app.Ref.Relative)
				if group == "." {
					group = "-"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%d\n", app.Label(), group, app.Status, len(app.Packages))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable JSON")
	return command
}

func init() {
	rootCmd.AddCommand(newListCmd())
}
