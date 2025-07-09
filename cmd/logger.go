package cmd

import "fmt"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
)

func info(msg string) {
	fmt.Printf("%sℹ️ %s%s\n", colorCyan, msg, colorReset)
}

func warn(msg string) {
	fmt.Printf("%s⚠️  %s%s\n", colorYellow, msg, colorReset)
}

func fail(msg string) {
	fmt.Printf("%s❌ %s%s\n", colorRed, msg, colorReset)
}

func success(msg string) {
	fmt.Printf("%s✅ %s%s\n", colorGreen, msg, colorReset)
}
