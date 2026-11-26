package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	appVersion      string
	installedWithGo bool
)

func SetVersion(v string, goInstall bool) {
	appVersion = v
	installedWithGo = goInstall
}

// ResolveVersion prefers the version injected at link time, then the module version recorded by go install.
func ResolveVersion(ldflags string, info *debug.BuildInfo, ok bool) (version string, goInstall bool) {
	if ldflags != "dev" {
		return ldflags, false
	}
	if !ok || info == nil || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return ldflags, false
	}
	// A build from a local checkout also stamps a pseudo-version, but only it records VCS settings.
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return ldflags, false
		}
	}
	return strings.TrimPrefix(info.Main.Version, "v"), true
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of envctl",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("envctl %s\n", appVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
