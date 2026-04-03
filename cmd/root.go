package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/internal/config"
	"github.com/PillowPillow/envctl/internal/updater"
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

// Run executes the root command and prints any update notification.
// Returns the exit code.
func Run() int {
	updateCh := startBackgroundUpdateCheck()

	err := rootCmd.Execute()

	printUpdateNotice(updateCh)

	if err != nil {
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: .envctl.yaml)")
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "target environment")
}

// skipUpdateCheckCommands lists commands that should not trigger background update checks.
var skipUpdateCheckCommands = map[string]bool{
	"version":    true,
	"update":     true,
	"help":       true,
	"completion": true,
}

func startBackgroundUpdateCheck() <-chan string {
	if appVersion == "" || appVersion == "dev" || isHomebrewInstall() {
		return nil
	}

	// Determine which command is being run.
	cmd, _, _ := rootCmd.Find(os.Args[1:])
	if cmd != nil && skipUpdateCheckCommands[cmd.Name()] {
		return nil
	}

	client := updater.NewGitHubClient(&http.Client{})
	checker := updater.NewChecker(client, configDir(), appVersion)
	return checker.RunBackground()
}

func printUpdateNotice(ch <-chan string) {
	if ch == nil {
		return
	}
	select {
	case msg, ok := <-ch:
		if ok && msg != "" {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, msg)
		}
	default:
	}
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
