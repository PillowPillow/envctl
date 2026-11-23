package main

import (
	"os"

	"github.com/PillowPillow/envctl/v2/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	os.Exit(cmd.Run())
}
