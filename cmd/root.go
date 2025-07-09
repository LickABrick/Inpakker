/*
Copyright © 2025 LickABrick

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)



var rootCmd = &cobra.Command{
	Use:   "inpakker",
	Short: "📦 Intune Win32 app packager for Windows deployment",
	Long: `Inpakker is a simple CLI tool to help you package Windows applications 
for Microsoft Intune deployment using IntuneWinAppUtil. 

It allows you to organize and define your apps in configuration files, 
and automates the packaging process via clean, structured commands.
`,
}


// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.inpakker.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}


