package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [app-folder]",
	Short: "Package one or more apps using IntuneWinAppUtil",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		globalCfg, err := config.LoadGlobalConfig("inpakker.config.json5")
		if err != nil {
			return fmt.Errorf("could not load global config: %w", err)
		}

		for _, appPath := range args {
			cfgPath := filepath.Join(appPath, "app.config.json5")
			appCfg, err := config.LoadAppConfig(cfgPath)
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", appPath, err)
				continue
			}

			outputDir := filepath.Join(appPath, appCfg.OutputDir)
			err = os.MkdirAll(outputDir, os.ModePerm)
			if err != nil {
				fmt.Printf("Could not create output folder: %v\n", err)
				continue
			}

			fmt.Printf("Building app: %s\n", appCfg.Name)

			cmd := exec.Command(
				globalCfg.IntuneWinAppUtil,
				"-c", filepath.Join(appPath, appCfg.Source),
				"-s", appCfg.SetupFile,
				"-o", outputDir,
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				fmt.Printf("Build failed for %s: %v\n", appCfg.Name, err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
