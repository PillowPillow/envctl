package secret

import (
	"errors"
	"fmt"
	"os/exec"

	"gopkg.in/yaml.v3"
)

// SOPSProvider decrypts SOPS-encrypted YAML files by shelling out to the sops CLI.
type SOPSProvider struct{}

func (p *SOPSProvider) Load(path string) (map[string]string, error) {
	if err := checkSOPSInstalled(); err != nil {
		return nil, err
	}

	cmd := exec.Command("sops", "--decrypt", "--", path) //nolint:gosec // args from validated config, not user input
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("sops decrypt failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("sops decrypt failed: %w", err)
	}

	var secrets map[string]string
	if err := yaml.Unmarshal(output, &secrets); err != nil {
		return nil, fmt.Errorf("cannot parse decrypted secrets from %s: %w", path, err)
	}

	return secrets, nil
}

func checkSOPSInstalled() error {
	_, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops is not installed or not in PATH\n\n  Fix: Install sops:\n    brew install sops\n    # or: go install github.com/getsops/sops/v3/cmd/sops@latest")
	}
	return nil
}
