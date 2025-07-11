package cmd

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "embed"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/spf13/cobra"
	"github.com/charmbracelet/log"
)

var unpackCmd = &cobra.Command{
	Use:   "unpack [app-name|group-name|path-to-intunewin]",
	Short: "Unpack the .intunewin file for the specified app(s) using embedded decoder",
	RunE: func(cmd *cobra.Command, args []string) error {
		globalCfg, err := config.LoadGlobalConfig("inpakker.config.json")
		if err != nil {
			return fmt.Errorf("%w: 🔧 could not load global config", err)
		}

		appsRoot := globalCfg.AppsDir
		if appsRoot == "" {
			appsRoot = "apps"
		}

		defaultOutputDir := globalCfg.DefaultOutputDir
		if defaultOutputDir == "" {
			defaultOutputDir = "output"
		}

		if len(args) == 0 {
			return errors.New("please provide an app/group name or path to a .intunewin file")
		}

		// Create temp dir for decoder exe
		tmpDir, err := os.MkdirTemp("", "inpakker-decoder-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		decoderPath, err := writeDecoderExe(tmpDir)
		if err != nil {
			return fmt.Errorf("failed to write decoder exe: %w", err)
		}

		for _, arg := range args {
			if strings.HasSuffix(strings.ToLower(arg), ".intunewin") {
				// Treat argument as path to .intunewin file
				intunewinFile := arg
				if !filepath.IsAbs(intunewinFile) {
					intunewinFile = filepath.Clean(intunewinFile)
				}

				log.Info("Decoding .intunewin file.")

				decodedDir := filepath.Join(filepath.Dir(intunewinFile), "decoded")
				if err := os.MkdirAll(decodedDir, os.ModePerm); err != nil {
					log.Error(fmt.Sprintf("Failed to create decoded output folder: %v", err))
					continue
				}

				cmdExec := exec.Command(decoderPath, intunewinFile, "/s")
				cmdExec.Stdout = os.Stdout
				cmdExec.Stderr = os.Stderr
				err := cmdExec.Run()
				if err != nil {
					log.Error(fmt.Sprintf("Unpacking failed for %s: %v", filepath.Base(intunewinFile), err))
					continue
				}

				decodedZipPath := intunewinFile + ".decoded"
				if _, err := os.Stat(decodedZipPath); err != nil {
					log.Error(fmt.Sprintf("Decoded zip not found for %s", filepath.Base(intunewinFile)))
					continue
				}

				err = unzip(decodedZipPath, decodedDir)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to unzip decoded content for %s: %v", filepath.Base(intunewinFile), err))
					continue
				}

				os.Remove(decodedZipPath)
				log.Info(fmt.Sprintf("✅ Unpack complete for %s. Output at %s", filepath.Base(intunewinFile), decodedDir))

			} else {
				// Treat argument as app/group name and find app(s)
				targets := []string{}
				candidate := filepath.Join(appsRoot, arg)
				log.Info(fmt.Sprintf("Scanning for app or group: %s", candidate))

				infoMsg := fmt.Sprintf("Looking for apps under %s...", candidate)
				log.Info(infoMsg)

				fileInfo, err := os.Stat(candidate)
				if err != nil {
					log.Warn(fmt.Sprintf("Invalid path: %s (%v)", candidate, err))
					continue
				}

				if fileInfo.IsDir() {
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

				if len(targets) == 0 {
					log.Warn(fmt.Sprintf("No valid apps found to unpack for %s", arg))
					continue
				}

				for _, appPath := range targets {
					cfgPath := filepath.Join(appPath, "app.config.json")
					appCfg, err := config.LoadAppConfig(cfgPath)
					if err != nil {
						log.Warn(fmt.Sprintf("Skipping %s: failed to load app config: %v", appPath, err))
						continue
					}

					outputDir := appCfg.OutputDir
					if outputDir == "" {
						outputDir = defaultOutputDir
					}
					outputPath := filepath.Join(appPath, outputDir)

					intunewinFile, err := findIntuneWinFile(outputPath)
					if err != nil {
						log.Error(fmt.Sprintf("Unpack failed for %s: %v", appCfg.Name, err))
						continue
					}

					decodedDir := filepath.Join(outputPath, "decoded")
					if err := os.MkdirAll(decodedDir, os.ModePerm); err != nil {
						log.Error(fmt.Sprintf("Failed to create decoded output folder: %v", err))
						continue
					}

					log.Info(fmt.Sprintf("Unpacking %s for app %s...", filepath.Base(intunewinFile), appCfg.Name))

					cmdExec := exec.Command(decoderPath, intunewinFile, "/s")
					cmdExec.Stdout = os.Stdout
					cmdExec.Stderr = os.Stderr
					err = cmdExec.Run()
					if err != nil {
						log.Error(fmt.Sprintf("Decode failed for %s: %v", appCfg.Name, err))
						continue
					}

					decodedZipPath := intunewinFile + ".decoded"
					if _, err := os.Stat(decodedZipPath); err != nil {
						log.Error(fmt.Sprintf("Decoded zip not found for %s", appCfg.Name))
						continue
					}

					err = unzip(decodedZipPath, decodedDir)
					if err != nil {
						log.Error(fmt.Sprintf("Failed to unzip decoded content for %s: %v", appCfg.Name, err))
						continue
					}

					os.Remove(decodedZipPath)
					log.Info(fmt.Sprintf("✅ Unpack complete for %s. Output at %s", appCfg.Name, decodedDir))
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(unpackCmd)
}

//go:embed assets/sandbox/IntuneWinAppUtilDecoder.exe
var decoderExeBytes []byte

func writeDecoderExe(tmpDir string) (string, error) {
	decoderPath := filepath.Join(tmpDir, "IntuneWinAppUtilDecoder.exe")
	err := ioutil.WriteFile(decoderPath, decoderExeBytes, 0755)
	if err != nil {
		return "", err
	}
	return decoderPath, nil
}

func findIntuneWinFile(folder string) (string, error) {
	files, err := os.ReadDir(folder)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".intunewin") {
			return filepath.Join(folder, f.Name()), nil
		}
	}
	return "", errors.New("no .intunewin file found in output folder")
}

func unzip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(fpath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
