package cmd

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [app-name]",
	Short: "Create a new app folder with a default config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		appDir := filepath.Join("apps", filepath.Clean(appName))

		if err := os.MkdirAll(filepath.Join(appDir, "source"), os.ModePerm); err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(appDir, "app.config.json"))
		if err != nil {
			return err
		}
		defer f.Close()

		const tmpl = `{
  "name": "{{.Name}}",
  "displayName": "{{.DisplayName}}",
  "source": "./source",
  "setupFile": "",
  "installCommand": "",
  "uninstallCommand": "",
  "outputDir": "./output"
}`

		t := template.Must(template.New("app").Parse(tmpl))
		return t.Execute(f, map[string]string{
			"Name":        appName,
			"DisplayName": appName,
		})
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
