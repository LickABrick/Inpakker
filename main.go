/*
Copyright © 2025 LickABrick
*/
package main

import "github.com/LickABrick/inpakker/cmd"

// version is replaced by GoReleaser at build time.
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
