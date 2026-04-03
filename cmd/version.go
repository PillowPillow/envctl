package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var appVersion string

func SetVersion(v string) {
	appVersion = v
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
