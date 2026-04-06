package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/internal/config"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage age encryption recipients",
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured age recipients",
	RunE:  runKeysList,
}

var keysAddCmd = &cobra.Command{
	Use:   "add <name> <age-public-key>",
	Short: "Add an age recipient and re-encrypt secrets",
	Args:  cobra.ExactArgs(2),
	RunE:  runKeysAdd,
}

var keysRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an age recipient and re-encrypt secrets",
	Args:  cobra.ExactArgs(1),
	RunE:  runKeysRemove,
}

func init() {
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysAddCmd)
	keysCmd.AddCommand(keysRemoveCmd)
	rootCmd.AddCommand(keysCmd)
}

func runKeysList(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if len(cfg.SOPS.AgeRecipients) == 0 {
		fmt.Println("No age recipients configured.")
		fmt.Println("\n  Add one with: envctl keys add <name> <age1...>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tKEY")
	for name, key := range cfg.SOPS.AgeRecipients {
		fmt.Fprintf(w, "%s\t%s\n", name, key)
	}
	return w.Flush()
}

func runKeysAdd(_ *cobra.Command, args []string) error {
	name, key := args[0], args[1]

	if !strings.HasPrefix(key, "age1") {
		return fmt.Errorf("invalid age public key: must start with \"age1\"")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if cfg.SOPS.AgeRecipients == nil {
		cfg.SOPS.AgeRecipients = make(map[string]string)
	}

	if existing, ok := cfg.SOPS.AgeRecipients[name]; ok {
		return fmt.Errorf("recipient %q already exists (key: %s)\n\n  To update, remove it first: envctl keys remove %s", name, existing, name)
	}

	cfg.SOPS.AgeRecipients[name] = key

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Added recipient %q\n", name)

	return updateSecretsKeys(cfg)
}

func runKeysRemove(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if _, ok := cfg.SOPS.AgeRecipients[name]; !ok {
		available := make([]string, 0, len(cfg.SOPS.AgeRecipients))
		for n := range cfg.SOPS.AgeRecipients {
			available = append(available, n)
		}
		return fmt.Errorf("recipient %q not found (available: %v)", name, available)
	}

	delete(cfg.SOPS.AgeRecipients, name)

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Removed recipient %q\n", name)

	return updateSecretsKeys(cfg)
}

// updateSecretsKeys runs sops updatekeys on all existing encrypted files.
func updateSecretsKeys(cfg *config.Config) error {
	keys := cfg.AgeRecipientKeys()
	if len(keys) == 0 {
		fmt.Println("No recipients left — skipping sops updatekeys")
		return nil
	}

	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops is not installed or not in PATH\n\n  Fix: brew install sops")
	}

	ageArg := strings.Join(keys, ",")
	updated := 0

	for envName, env := range cfg.Environments {
		if _, err := os.Stat(env.Secrets); os.IsNotExist(err) {
			continue
		}

		sopsCmd := exec.Command(sopsPath, "updatekeys", "--yes", "--age", ageArg, env.Secrets) //nolint:gosec // sops from LookPath, args from config
		sopsCmd.Stderr = os.Stderr
		if err := sopsCmd.Run(); err != nil {
			return fmt.Errorf("sops updatekeys failed for %s (%s): %w", envName, env.Secrets, err)
		}
		updated++
	}

	if updated > 0 {
		fmt.Printf("Updated keys in %d secrets file(s)\n", updated)
	} else {
		fmt.Println("No encrypted secrets files found — keys will apply on next edit")
	}

	return nil
}
