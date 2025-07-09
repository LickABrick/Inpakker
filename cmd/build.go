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
	Use:   "build [app-name]",
	Short: "Package one or more apps using IntuneWinAppUtil",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		globalCfg, err := config.LoadGlobalConfig("inpakker.config.json")
		if err != nil {
			return fmt.Errorf("could not load global config: %w", err)
		}

		// Use AppsDir from global config, fallback to "apps"
		appsRoot := globalCfg.AppsDir
		if appsRoot == "" {
			appsRoot = "apps"
		}

		// Use DefaultOutputDir from global config, fallback to "output"
		defaultOutputDir := globalCfg.DefaultOutputDir
		if defaultOutputDir == "" {
			defaultOutputDir = "output"
		}

		for _, appName := range args {
			appPath := filepath.Join(appsRoot, appName)
			cfgPath := filepath.Join(appPath, "app.config.json")

			appCfg, err := config.LoadAppConfig(cfgPath)
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", appName, err)
				continue
			}

			// If appCfg.OutputDir empty, use global defaultOutputDir
			outputDir := appCfg.OutputDir
			if outputDir == "" {
				outputDir = defaultOutputDir
			}

			outputPath := filepath.Join(appPath, outputDir)
			err = os.MkdirAll(outputPath, os.ModePerm)
			if err != nil {
				fmt.Printf("Could not create output folder %s: %v\n", outputPath, err)
				continue
			}

			fmt.Printf("Building app: %s\n", appCfg.Name)

			intuneUtilPath := globalCfg.IntuneWinAppUtil
			if intuneUtilPath == "" {
				intuneUtilPath = globalCfg.IntuneWinAppUtilPath
			}
			if intuneUtilPath == "" {
				fmt.Println("IntuneWinAppUtil path not configured")
				continue
			}

			cmdExec := exec.Command(
				intuneUtilPath,
				"-c", filepath.Join(appPath, appCfg.Source),
				"-s", appCfg.SetupFile,
				"-o", outputPath,
				"-q",
			)
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			err = cmdExec.Run()
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
