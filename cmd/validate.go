package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all app configs in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "apps"
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appPath := filepath.Join(root, entry.Name(), "app.config.json5")
			_, err := config.LoadAppConfig(appPath)
			if err != nil {
				fmt.Printf("❌ %s: %v\n", entry.Name(), err)
			} else {
				fmt.Printf("✅ %s: valid\n", entry.Name())
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
