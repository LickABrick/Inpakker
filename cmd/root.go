/*
Copyright © 2025 LickABrick
*/
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/LickABrick/inpakker/internal/process"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "inpakker",
	Short:         "Package Win32 applications for Microsoft Intune",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, inputOK := cmd.InOrStdin().(*os.File)
		output, outputOK := cmd.OutOrStdout().(*os.File)
		if inputOK && outputOK && term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd()) {
			return runTUI(cmd, process.ExecRunner{})
		}
		return cmd.Help()
	},
}

// SetVersion configures the version reported by Cobra's --version flag.
func SetVersion(version string) {
	rootCmd.Version = version
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		var reported reportedError
		if !errors.As(err, &reported) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}
