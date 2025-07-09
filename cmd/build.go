package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
)

var allFlag bool

var buildCmd = &cobra.Command{
	Use:   "build [app-name|group-name]",
	Short: "Package one or more apps using IntuneWinAppUtil",
	RunE: func(cmd *cobra.Command, args []string) error {
		globalCfg, err := config.LoadGlobalConfig("inpakker.config.json")
		if err != nil {
			return fmt.Errorf("%w: %s", err, "🔧 could not load global config")
		}

		appsRoot := globalCfg.AppsDir
		if appsRoot == "" {
			appsRoot = "apps"
		}

		defaultOutputDir := globalCfg.DefaultOutputDir
		if defaultOutputDir == "" {
			defaultOutputDir = "output"
		}

		var targets []string

		if allFlag {
			info("Scanning for apps...")

			err := filepath.Walk(appsRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && strings.HasSuffix(path, ".git") {
					return filepath.SkipDir
				}
				if info.Name() == "app.config.json" {
					targets = append(targets, filepath.Dir(path))
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed scanning for apps: %w", err)
			}

			if len(targets) == 0 {
				return errors.New("no app.config.json files found under apps directory")
			}
		} else if len(args) == 0 {
			return errors.New("please provide an app/group name or use --all")
		} else {
			for _, name := range args {
				candidate := filepath.Join(appsRoot, name)
				info, err := os.Stat(candidate)
				if err != nil {
					warn(fmt.Sprintf("Invalid path: %s (%v)", candidate, err))
					continue
				}

				if info.IsDir() {
					cfgFile := filepath.Join(candidate, "app.config.json")
					if _, err := os.Stat(cfgFile); err == nil {
						targets = append(targets, candidate)
					} else {
						entries, _ := os.ReadDir(candidate)
						for _, sub := range entries {
							if sub.IsDir() {
								subCfg := filepath.Join(candidate, sub.Name(), "app.config.json")
								if _, err := os.Stat(subCfg); err == nil {
									targets = append(targets, filepath.Join(candidate, sub.Name()))
								}
							}
						}
					}
				}
			}
		}

		if len(targets) == 0 {
			return errors.New("no valid apps found to build")
		}

		intuneUtilPath := globalCfg.IntuneWinAppUtil
		if intuneUtilPath == "" {
			intuneUtilPath = globalCfg.IntuneWinAppUtilPath
		}
		if intuneUtilPath == "" {
			return errors.New("intunewinapputil path not configured in global config")
		}

		for _, appPath := range targets {
			cfgPath := filepath.Join(appPath, "app.config.json")
			appCfg, err := config.LoadAppConfig(cfgPath)
			if err != nil {
				warn(fmt.Sprintf("Skipping %s: %v", appPath, err))
				continue
			}

			outputDir := appCfg.OutputDir
			if outputDir == "" {
				outputDir = defaultOutputDir
			}
			outputPath := filepath.Join(appPath, outputDir)

			if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
				fail(fmt.Sprintf("Could not create output folder %s: %v", outputPath, err))
				continue
			}

			// Determine group if any by trimming appsRoot and splitting
			relPath, err := filepath.Rel(appsRoot, appPath)
			if err != nil {
				relPath = appPath
			}
			parts := strings.Split(relPath, string(filepath.Separator))
			group := ""
			if len(parts) > 1 {
				group = parts[0]
			}

			if group != "" {
				info(fmt.Sprintf("🛠️  Building app: %s (group: %s%s%s)", appCfg.Name, colorBlue, group, colorReset))
			} else {
				info(fmt.Sprintf("🛠️  Building app: %s", appCfg.Name))
			}

			cmdExec := exec.Command(
				intuneUtilPath,
				"-c", filepath.Join(appPath, appCfg.Source),
				"-s", appCfg.SetupFile,
				"-o", outputPath,
				"-q",
			)
			if !globalCfg.MuteIntuneWinAppUtil {
				cmdExec.Stdout = os.Stdout
				cmdExec.Stderr = os.Stderr
			}
			err = cmdExec.Run()
			if err != nil {
				fail(fmt.Sprintf("Build failed for %s: %v", appCfg.Name, err))
			} else {
				success(fmt.Sprintf("Build complete: %s", appCfg.Name))
			}
		}

		return nil
	},
}

func init() {
	buildCmd.Flags().BoolVar(&allFlag, "all", false, "Build all apps under the apps directory")
	rootCmd.AddCommand(buildCmd)
}
