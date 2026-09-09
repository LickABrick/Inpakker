/*
Copyright © 2025 LickABrick
*/
package cmd

import (
	"errors"
	"fmt"
	"io"
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
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		automaticUpdateCheck(cmd)
	},
	PersistentPostRun: func(cmd *cobra.Command, _ []string) {
		printAutomaticUpdateNotice(cmd)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, inputOK := cmd.InOrStdin().(*os.File)
		output, outputOK := cmd.OutOrStdout().(*os.File)
		if inputOK && outputOK && term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd()) {
			return runTUI(cmd, process.ExecRunner{})
		}
		return cmd.Help()
	},
}

var currentVersion = "dev"

// SetVersion configures the version reported by Cobra's --version flag.
func SetVersion(version string) {
	currentVersion = version
	rootCmd.Version = version
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	executed, err := rootCmd.ExecuteC()
	if err != nil {
		reportCommandError(os.Stderr, executed, err)
		os.Exit(1)
	}
}

func reportCommandError(output io.Writer, command *cobra.Command, err error) {
	var reported reportedError
	if errors.As(err, &reported) {
		return
	}
	fmt.Fprintf(output, "Error: %v\n", err)
	var usage usageError
	if errors.As(err, &usage) && command != nil {
		fmt.Fprintln(output)
		fmt.Fprint(output, command.UsageString())
	}
}

func init() {
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err: err}
	})
}
