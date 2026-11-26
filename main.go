package main

import (
	"os"
	"runtime/debug"

	"github.com/PillowPillow/envctl/v2/cmd"
)

var version = "dev"

func main() {
	info, ok := debug.ReadBuildInfo()
	cmd.SetVersion(cmd.ResolveVersion(version, info, ok))
	os.Exit(cmd.Run())
}
