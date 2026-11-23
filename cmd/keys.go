package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/config"
	"github.com/PillowPillow/envctl/v2/internal/sopskeys"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage age encryption recipients",
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured age recipients (all scopes, or one environment with --env)",
	RunE:  runKeysList,
}

var keysAddCmd = &cobra.Command{
	Use:   "add <name> <age-public-key>",
	Short: "Add an age recipient (global, or to one environment with --env) and re-encrypt secrets",
	Args:  cobra.ExactArgs(2),
	RunE:  runKeysAdd,
}

var keysRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an age recipient (global, or from one environment with --env) and re-encrypt secrets",
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

	recipients, err := cfg.Recipients(envName)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		fmt.Println("No age recipients configured.")
		fmt.Println("\n  Add one with: envctl keys add <name> <age1...>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tKEY\tSCOPE")
	for _, r := range recipients {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Key, r.Scope)
	}
	return w.Flush()
}

func runKeysAdd(_ *cobra.Command, args []string) error {
	name, key := args[0], args[1]

	if err := config.ValidRecipientName(name); err != nil {
		return err
	}
	if !strings.HasPrefix(key, "age1") {
		return fmt.Errorf("invalid age public key: must start with \"age1\"")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if envName != "" {
		if _, err := cfg.GetEnvironment(envName); err != nil {
			return err
		}
	}

	all, err := cfg.Recipients("")
	if err != nil {
		return err
	}
	for _, r := range all {
		if r.Name == name {
			return fmt.Errorf("recipient %q already exists in scope %q", name, r.Scope)
		}
	}

	src, err := os.ReadFile(cfg.ConfigPath())
	if err != nil {
		return fmt.Errorf("cannot read config file %s: %w", cfg.ConfigPath(), err)
	}
	newBytes, err := config.AddRecipient(src, envName, name, key)
	if err != nil {
		return err
	}
	newCfg, err := config.Parse(newBytes, cfg.ConfigPath())
	if err != nil {
		return err
	}

	if err := syncAndSave(cfg, newCfg, newBytes); err != nil {
		return err
	}
	fmt.Printf("Added recipient %q\n", name)
	return nil
}

func runKeysRemove(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	scope := config.ScopeGlobal
	if envName != "" {
		if _, err := cfg.GetEnvironment(envName); err != nil {
			return err
		}
		scope = envName
	}

	all, err := cfg.Recipients("")
	if err != nil {
		return err
	}
	var inScope []string
	found := false
	for _, r := range all {
		if r.Scope != scope {
			if r.Name == name && r.Scope == config.ScopeGlobal {
				return fmt.Errorf("recipient %q is global; remove it without --env (it cannot be removed from a single environment)", name)
			}
			continue
		}
		inScope = append(inScope, r.Name)
		if r.Name == name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("recipient %q not found in scope %q (available: %v)", name, scope, inScope)
	}

	src, err := os.ReadFile(cfg.ConfigPath())
	if err != nil {
		return fmt.Errorf("cannot read config file %s: %w", cfg.ConfigPath(), err)
	}
	newBytes, err := config.RemoveRecipient(src, envName, name)
	if err != nil {
		return err
	}
	newCfg, err := config.Parse(newBytes, cfg.ConfigPath())
	if err != nil {
		return err
	}
	for _, target := range keysTargets(newCfg) {
		keys, err := newCfg.EffectiveRecipientKeys(target)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return fmt.Errorf("cannot remove %q: environment %q would have no recipients left", name, target)
		}
	}

	if err := syncAndSave(cfg, newCfg, newBytes); err != nil {
		return err
	}
	fmt.Printf("Removed recipient %q\n", name)
	fmt.Fprintln(os.Stderr, "Warning: the removed key can still decrypt older versions of these files from git history. Rotate the secret values themselves to fully revoke access.")
	return nil
}

func keysTargets(cfg *config.Config) []string {
	if envName != "" {
		return []string{envName}
	}
	return cfg.EnvNames()
}

// syncAndSave re-keys every targeted secrets file, then writes .envctl.yaml last so a failure leaves it untouched.
func syncAndSave(cfg, newCfg *config.Config, newBytes []byte) error {
	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops is not installed or not in PATH\n\n  Fix: brew install sops")
	}

	var done []string
	for _, target := range keysTargets(newCfg) {
		file := newCfg.Environments[target].Secrets
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}
		keys, err := newCfg.EffectiveRecipientKeys(target)
		if err != nil {
			return err
		}
		if err := sopskeys.Sync(sopsPath, file, keys); err != nil {
			return fmt.Errorf("re-keyed %s before failing; .envctl.yaml was not modified — fix the error and re-run the same command: %w", doneList(done), err)
		}
		done = append(done, file)
	}

	info, err := os.Stat(cfg.ConfigPath())
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.ConfigPath(), newBytes, info.Mode().Perm()); err != nil { //nolint:gosec // path from FindConfigFile or --config
		return fmt.Errorf("cannot write config %s: %w", cfg.ConfigPath(), err)
	}

	if len(done) > 0 {
		fmt.Printf("Updated keys in %d secrets file(s)\n", len(done))
	} else {
		fmt.Println("No encrypted secrets files found — keys will apply on next edit")
	}
	return nil
}

func doneList(done []string) string {
	if len(done) == 0 {
		return "no files"
	}
	return strings.Join(done, ", ")
}
