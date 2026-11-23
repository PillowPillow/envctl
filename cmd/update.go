package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update envctl to the latest version",
	Long:  "Downloads and installs the latest version of envctl from GitHub.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runUpdate()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate() error {
	return runUpdateWith(updater.NewGitHubClient(&http.Client{}))
}

func runUpdateWith(client *updater.GitHubClient) error {
	if appVersion == "dev" {
		color.Yellow("Development mode, update not available.")
		return nil
	}

	if isHomebrewInstall() {
		color.Yellow("envctl is managed by Homebrew. Run 'brew upgrade envctl' instead.")
		return nil
	}

	current, err := updater.ParseVersion(appVersion)
	if err != nil {
		return err
	}

	u := &updater.Updater{
		Client:     client,
		CurrentVer: current,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	}

	fmt.Fprintln(os.Stderr, "Checking for updates...")

	result, err := u.Check()
	if err != nil {
		return err
	}

	if !result.UpdateAvail {
		color.Green("envctl is already up to date (%s).", result.Current.String())
		return nil
	}

	fmt.Fprintf(os.Stderr, "Downloading %s...\n", result.Latest.String())

	result, err = u.Update()
	if err != nil {
		return err
	}

	color.Green("envctl updated: %s → %s", current.String(), result.Latest.String())

	// Reset the check timer so background check doesn't trigger right after a manual update.
	updater.ResetCheckTimer(configDir(), result.Latest.String())

	return nil
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".envctl")
}

// isHomebrewInstall returns true if the current binary is managed by Homebrew.
func isHomebrewInstall() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return false
	}
	// Homebrew installs live under /opt/homebrew/Cellar or /usr/local/Cellar.
	return strings.Contains(resolved, "/Cellar/")
}
