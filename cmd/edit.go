package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/sopskeys"
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

	if _, err := os.Stat(env.Secrets); os.IsNotExist(err) {
		recipients, err := cfg.EffectiveRecipientKeys(envName)
		if err != nil {
			return err
		}
		if len(recipients) == 0 {
			return fmt.Errorf("no age recipients for environment %q — add one with: envctl keys add <name> <age1...>", envName)
		}
		if err := os.MkdirAll(filepath.Dir(env.Secrets), 0o755); err != nil {
			return fmt.Errorf("cannot create directory for secrets file: %w", err)
		}
		if err := os.WriteFile(env.Secrets, []byte("{}\n"), 0o600); err != nil {
			return fmt.Errorf("cannot create secrets file: %w", err)
		}
		if err := sopskeys.Encrypt(sopsPath, env.Secrets, recipients); err != nil {
			os.Remove(env.Secrets) //nolint:errcheck,gosec // best-effort cleanup
			return fmt.Errorf("failed to encrypt new secrets file: %w", err)
		}
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
