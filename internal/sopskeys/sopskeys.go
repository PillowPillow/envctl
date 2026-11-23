package sopskeys

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Rotate runs before updatekeys so an operator can remove their own key: updatekeys first would leave rotate unable to decrypt.
func Sync(sopsPath, file string, recipients []string) error {
	cfg, cleanup, err := isolatedConfig(recipients)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runSops(sopsPath, "--config", cfg, "rotate", "-i", file); err != nil {
		return fmt.Errorf("sops rotate failed for %s: %w", file, err)
	}
	if err := runSops(sopsPath, "--config", cfg, "updatekeys", "--yes", file); err != nil {
		return fmt.Errorf("sops updatekeys failed for %s: %w", file, err)
	}
	return nil
}

func Encrypt(sopsPath, file string, recipients []string) error {
	cfg, cleanup, err := isolatedConfig(recipients)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runSops(sopsPath, "--config", cfg, "--encrypt", "--in-place", "--age", strings.Join(recipients, ","), file); err != nil {
		return fmt.Errorf("sops encrypt failed for %s: %w", file, err)
	}
	return nil
}

func FileRecipients(file string) ([]string, error) {
	data, err := os.ReadFile(file) //nolint:gosec // path from caller
	if err != nil {
		return nil, err
	}
	var doc struct {
		Sops struct {
			Age []struct {
				Recipient string `yaml:"recipient"`
			} `yaml:"age"`
		} `yaml:"sops"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", file, err)
	}
	out := make([]string, 0, len(doc.Sops.Age))
	for _, a := range doc.Sops.Age {
		out = append(out, a.Recipient)
	}
	sort.Strings(out)
	return out, nil
}

// The config lives in a private temp dir so sops never searches for or touches a repo .sops.yaml.
func isolatedConfig(recipients []string) (path string, cleanup func(), err error) {
	if len(recipients) == 0 {
		return "", nil, errors.New("no recipients given")
	}
	dir, err := os.MkdirTemp("", "envctl-sops-")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(dir) } //nolint:errcheck,gosec // best-effort cleanup
	path = filepath.Join(dir, "sops.yaml")
	content, err := yaml.Marshal(map[string][]map[string]string{
		"creation_rules": {{"age": strings.Join(recipients, ",")}},
	})
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("encoding sops config: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("writing sops config: %w", err)
	}
	return path, cleanup, nil
}

func runSops(sopsPath string, args ...string) error {
	cmd := exec.Command(sopsPath, args...) //nolint:gosec // sops from LookPath, args built internally
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
