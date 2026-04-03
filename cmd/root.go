package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/internal/config"
)

var (
	cfgFile string
	envName string
)

var rootCmd = &cobra.Command{
	Use:   "envctl",
	Short: "Environment configuration manager with SOPS encryption",
	Long:  `envctl manages environment variables across projects, with secret encryption via SOPS/age.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: .envctl.yaml)")
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "target environment")
}

func loadConfig() (*config.Config, error) {
	path := cfgFile
	if path == "" {
		var err error
		path, err = config.FindConfigFile()
		if err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

func isCI() bool {
	return strings.EqualFold(os.Getenv("CI"), "true")
}
