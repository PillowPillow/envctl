package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/config"
	"github.com/PillowPillow/envctl/v2/internal/sopskeys"
)

// The second form covers recipients with a longer prefix, such as age1pq1 (post-quantum) or age1yubikey1 (plugins).
var agePublicKeyRe = regexp.MustCompile(`^age1(?:[02-9ac-hj-np-z]{58}|[a-z0-9]*1[02-9ac-hj-np-z]{6,})$`)

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
	if !agePublicKeyRe.MatchString(key) {
		return fmt.Errorf("invalid age public key: expected an age1... bech32 recipient")
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
	scope := scopeOf(envName)
	for _, r := range all {
		if r.Name == name && (scope == config.ScopeGlobal || r.Scope == config.ScopeGlobal || r.Scope == scope) {
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
	added := config.Recipient{Name: name, Key: key, Scope: scope}
	if err := checkRecipients(newCfg, append(all, added)); err != nil {
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

	if envName != "" {
		if _, err := cfg.GetEnvironment(envName); err != nil {
			return err
		}
	}
	scope := scopeOf(envName)

	all, err := cfg.Recipients("")
	if err != nil {
		return err
	}
	var inScope, otherEnvs []string
	var kept []config.Recipient
	var removedKey string
	found := false
	for _, r := range all {
		switch {
		case r.Scope == scope:
			inScope = append(inScope, r.Name)
		case r.Name == name && r.Scope == config.ScopeGlobal:
			return fmt.Errorf("recipient %q is global; remove it without --env (it cannot be removed from a single environment)", name)
		case r.Name == name:
			otherEnvs = append(otherEnvs, r.Scope)
		}
		if r.Scope == scope && r.Name == name {
			found = true
			removedKey = r.Key
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		if scope == config.ScopeGlobal && len(otherEnvs) > 0 {
			return fmt.Errorf("recipient %q not found in scope %q (it exists in: %s — use --env)", name, scope, strings.Join(otherEnvs, ", "))
		}
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
	if err := checkRecipients(newCfg, kept); err != nil {
		return err
	}
	for _, target := range keysTargets(newCfg) {
		secrets := newCfg.Environments[target].Secrets
		if _, err := os.Stat(secrets); os.IsNotExist(err) {
			continue
		}
		if len(newCfg.SecretsRecipientKeys(secrets)) == 0 {
			return fmt.Errorf("cannot remove %q: environment %q would have no recipients left", name, target)
		}
	}

	if err := syncAndSave(cfg, newCfg, newBytes); err != nil {
		return err
	}
	fmt.Printf("Removed recipient %q\n", name)
	if files := filesStillOpenTo(newCfg, removedKey); len(files) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: another recipient has the same key, so it can still decrypt: %s\n", strings.Join(files, ", "))
	}
	fmt.Fprintln(os.Stderr, "Warning: the removed key can still decrypt older versions of these files from git history. Rotate the secret values themselves to fully revoke access.")
	return nil
}

func filesStillOpenTo(cfg *config.Config, key string) []string {
	var out []string
	for _, file := range keysTargetFiles(cfg) {
		if slices.Contains(cfg.SecretsRecipientKeys(file), key) {
			out = append(out, file)
		}
	}
	return out
}

func scopeOf(env string) string {
	if env == "" {
		return config.ScopeGlobal
	}
	return env
}

// checkRecipients guards against line edits that YAML reads back differently, such as a name parsed as null.
func checkRecipients(newCfg *config.Config, want []config.Recipient) error {
	got, err := newCfg.Recipients("")
	if err != nil {
		return err
	}
	count := map[config.Recipient]int{}
	for _, r := range want {
		count[r]++
	}
	for _, r := range got {
		count[r]--
	}
	for _, n := range count {
		if n != 0 {
			return fmt.Errorf("refusing to write %s: the edit did not produce the expected recipients", config.DefaultConfigFile)
		}
	}
	return nil
}

func keysTargets(cfg *config.Config) []string {
	if envName != "" {
		return []string{envName}
	}
	return cfg.EnvNames()
}

func keysTargetFiles(cfg *config.Config) []string {
	seen := map[string]bool{}
	var files []string
	for _, target := range keysTargets(cfg) {
		file := filepath.Clean(cfg.Environments[target].Secrets)
		if canonical := config.CanonicalPath(file); !seen[canonical] {
			seen[canonical] = true
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files
}

// syncAndSave re-keys every targeted secrets file for all environments sharing it, then writes .envctl.yaml last so a failure leaves it untouched.
func syncAndSave(cfg, newCfg *config.Config, newBytes []byte) error {
	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops is not installed or not in PATH\n\n  Fix: brew install sops")
	}

	var pending, undecryptable, foreign []string
	upToDate := 0
	for _, file := range keysTargetFiles(newCfg) {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}
		sources, err := sopskeys.ForeignKeySources(file)
		if err != nil {
			return fmt.Errorf("%w; nothing was changed", err)
		}
		if len(sources) > 0 {
			foreign = append(foreign, fmt.Sprintf("  %s: %s", file, strings.Join(sources, ", ")))
			continue
		}
		decryptErr := sopskeys.CanDecrypt(sopsPath, file)
		if decryptErr == nil {
			pending = append(pending, file)
			continue
		}
		// Recipient labels are not authenticated, so they only justify a skip when the operator can no longer decrypt (re-run after removing their own key).
		if current, err := sopskeys.FileRecipients(file); err == nil && slices.Equal(current, newCfg.SecretsRecipientKeys(file)) {
			fmt.Printf("%s already up to date\n", file)
			upToDate++
			continue
		}
		undecryptable = append(undecryptable, fmt.Sprintf("  %s: %v", file, decryptErr))
	}
	if len(foreign) > 0 {
		return fmt.Errorf("refusing to re-key: these secrets files list key sources other than age, which would receive the new data key; nothing was changed — remove them from the file's sops metadata and re-run:\n%s", strings.Join(foreign, "\n"))
	}
	if len(undecryptable) > 0 {
		return fmt.Errorf("cannot decrypt these secrets files with your age key; nothing was changed — run this command as a recipient of every targeted file:\n%s", strings.Join(undecryptable, "\n"))
	}

	var done []string
	for _, file := range pending {
		if err := sopskeys.Sync(sopsPath, file, newCfg.SecretsRecipientKeys(file)); err != nil {
			return fmt.Errorf("re-keyed %s (and possibly %s) before failing; .envctl.yaml was not modified — fix the error and re-run the same command: %w", doneList(done), file, err)
		}
		done = append(done, file)
	}

	if err := writeFileAtomic(cfg.ConfigPath(), newBytes); err != nil {
		return fmt.Errorf("re-keyed %s but cannot write config %s — fix the error and re-run the same command: %w", doneList(done), cfg.ConfigPath(), err)
	}

	switch {
	case len(done) > 0:
		fmt.Printf("Updated keys in %d secrets file(s)\n", len(done))
	case upToDate == 0:
		fmt.Println("No encrypted secrets files found — keys will apply on next edit")
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	// Resolve symlinks so the rename replaces the target file, not the link.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	renamed := false
	defer func() {
		if !renamed {
			os.Remove(tmp.Name()) //nolint:errcheck,gosec // best-effort cleanup
		}
	}()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close() //nolint:errcheck,gosec // already failing
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck,gosec // already failing
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck,gosec // already failing
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return err
	}
	renamed = true
	return nil
}

func doneList(done []string) string {
	if len(done) == 0 {
		return "no files"
	}
	return strings.Join(done, ", ")
}
