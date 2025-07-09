package logger

import "fmt"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
)

func Info(msg string) {
	fmt.Printf("%sℹ️ %s%s\n", colorCyan, msg, colorReset)
}

func Warn(msg string) {
	fmt.Printf("%s⚠️ %s%s\n", colorYellow, msg, colorReset)
}

func Error(msg string) {
	fmt.Printf("%s❌ %s%s\n", colorRed, msg, colorReset)
}

func Success(msg string) {
	fmt.Printf("%s✅ %s%s\n", colorGreen, msg, colorReset)
}

func Debug(msg string) {
	fmt.Printf("%s🐛 %s%s\n", colorBlue, msg, colorReset)
}