package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the encrypted secrets file for editing",
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	if envName == "" {
		return fmt.Errorf("--env is required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Check sops is installed
	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops is not installed or not in PATH\n\n  Fix: Install sops:\n    brew install sops")
	}

	// Shell out to sops edit
	sopsCmd := exec.Command(sopsPath, env.Secrets) //nolint:gosec // sops binary from LookPath, arg from validated config
	sopsCmd.Stdin = os.Stdin
	sopsCmd.Stdout = os.Stdout
	sopsCmd.Stderr = os.Stderr

	if err := sopsCmd.Run(); err != nil {
		return fmt.Errorf("sops edit failed: %w", err)
	}

	return nil
}
