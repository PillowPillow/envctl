package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/PillowPillow/envctl/v2/internal/agekey"
)

type Config struct {
	Version      string                 `yaml:"version"`
	Template     string                 `yaml:"template"`
	Environments map[string]Environment `yaml:"environments"`
	SOPS         SOPSConfig             `yaml:"sops"`

	configPath string
}

type Environment struct {
	Secrets       string            `yaml:"secrets"`
	Output        string            `yaml:"output"`
	Overrides     map[string]string `yaml:"overrides"`
	Strip         []string          `yaml:"strip"`
	AgeRecipients map[string]string `yaml:"age_recipients,omitempty"`
	Shared        bool              `yaml:"shared,omitempty"`
}

const ScopeGlobal = "global"

type Recipient struct {
	Name  string
	Key   string
	Scope string
}

var recipientNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func ValidRecipientName(name string) error {
	if !recipientNameRe.MatchString(name) {
		return fmt.Errorf("invalid recipient name %q: use letters, digits, '.', '_' or '-'", name)
	}
	return nil
}

type SOPSConfig struct {
	AgeRecipients map[string]string `yaml:"age_recipients"`
}

const DefaultConfigFile = ".envctl.yaml"

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from FindConfigFile or user flag
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	return Parse(data, path)
}

// Parse parses config data read from path; relative secrets paths are resolved against path's directory.
func Parse(data []byte, path string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	cfg.configPath = path
	dir := filepath.Dir(path)
	for name, env := range cfg.Environments {
		if !filepath.IsAbs(env.Secrets) {
			env.Secrets = filepath.Join(dir, env.Secrets)
			cfg.Environments[name] = env
		}
	}
	if err := cfg.validateSharedSecrets(dir); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil
}

// validateSharedSecrets rejects a secrets file shared by environments with different recipients unless each opts in, since every recipient of one can then read the others' secrets.
func (c *Config) validateSharedSecrets(dir string) error {
	groups := map[string][]string{}
	for _, name := range c.EnvNames() {
		file := CanonicalPath(c.Environments[name].Secrets)
		groups[file] = append(groups[file], name)
	}
	files := make([]string, 0, len(groups))
	for file := range groups {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		names := groups[file]
		first := c.Environments[names[0]]
		allShared := true
		for _, name := range names {
			allShared = allShared && c.Environments[name].Shared
		}
		if allShared {
			continue
		}
		for _, name := range names[1:] {
			if sameRecipientKeys(first.AgeRecipients, c.Environments[name].AgeRecipients) {
				continue
			}
			shown := first.Secrets
			if rel, err := filepath.Rel(dir, shown); err == nil {
				shown = rel
			}
			return fmt.Errorf("environments %q and %q share secrets file %s with different age_recipients; give them the same recipients, or set shared: true on each of them to encrypt the file for all of their recipients", names[0], name, shown)
		}
	}
	return nil
}

func sameRecipientKeys(a, b map[string]string) bool {
	set := make(map[string]bool, len(a))
	for _, key := range a {
		set[key] = true
	}
	other := make(map[string]bool, len(b))
	for _, key := range b {
		if !set[key] {
			return false
		}
		other[key] = true
	}
	return len(other) == len(set)
}

// EnvNames returns the environment names in sorted order.
func (c *Config) EnvNames() []string {
	names := make([]string, 0, len(c.Environments))
	for name := range c.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Recipients returns global recipients plus those of envName; an empty envName returns every scope.
func (c *Config) Recipients(envName string) ([]Recipient, error) {
	if envName != "" {
		if _, err := c.GetEnvironment(envName); err != nil {
			return nil, err
		}
	}
	out := scopedRecipients(nil, ScopeGlobal, c.SOPS.AgeRecipients)
	for _, name := range c.EnvNames() {
		if envName == "" || name == envName {
			out = scopedRecipients(out, name, c.Environments[name].AgeRecipients)
		}
	}
	return out, nil
}

// SecretsRecipientKeys returns the sorted, de-duplicated union of the effective keys of every environment whose secrets file is file.
func (c *Config) SecretsRecipientKeys(file string) []string {
	set := map[string]struct{}{}
	shared := false
	target := CanonicalPath(file)
	for _, env := range c.Environments {
		if CanonicalPath(env.Secrets) != target {
			continue
		}
		shared = true
		for _, key := range env.AgeRecipients {
			set[key] = struct{}{}
		}
	}
	if !shared {
		return nil
	}
	for _, key := range c.SOPS.AgeRecipients {
		set[key] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// CanonicalPath resolves symlinks, also when the file does not exist yet, so two spellings of one secrets file compare equal.
func CanonicalPath(path string) string {
	clean := filepath.Clean(path)
	for range maxSymlinkHops {
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			return resolved
		}
		dir := filepath.Dir(clean)
		if realDir, err := filepath.EvalSymlinks(dir); err == nil {
			dir = realDir
		}
		target, err := os.Readlink(clean)
		if err != nil {
			return filepath.Join(dir, filepath.Base(clean))
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		clean = filepath.Clean(target)
	}
	return clean
}

const maxSymlinkHops = 255

func scopedRecipients(out []Recipient, scope string, m map[string]string) []Recipient {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, Recipient{Name: name, Key: m[name], Scope: scope})
	}
	return out
}

// ConfigPath returns the path the config was loaded from.
func (c *Config) ConfigPath() string {
	return c.configPath
}

// FindConfigFile walks up from the current directory looking for .envctl.yaml.
func FindConfigFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, DefaultConfigFile)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found (searched up to filesystem root)", DefaultConfigFile)
		}
		dir = parent
	}
}

// GetEnvironment returns the environment config by name.
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	env, ok := c.Environments[name]
	if !ok {
		available := make([]string, 0, len(c.Environments))
		for k := range c.Environments {
			available = append(available, k)
		}
		return nil, fmt.Errorf("environment %q not found (available: %v)", name, available)
	}
	return &env, nil
}

func (c *Config) validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}
	if c.Template == "" {
		return fmt.Errorf("template is required")
	}
	if len(c.Environments) == 0 {
		return fmt.Errorf("at least one environment is required")
	}
	for _, name := range c.EnvNames() {
		if name == ScopeGlobal {
			return fmt.Errorf("environment name %q is reserved for global recipients", name)
		}
		if c.Environments[name].Secrets == "" {
			return fmt.Errorf("environment %q: secrets path is required", name)
		}
	}
	for name, key := range c.SOPS.AgeRecipients {
		if err := ValidRecipientName(name); err != nil {
			return err
		}
		if err := agekey.ValidateRecipient(key); err != nil {
			return fmt.Errorf("recipient %q: %w", name, err)
		}
	}
	for _, envName := range c.EnvNames() {
		recipients := c.Environments[envName].AgeRecipients
		names := make([]string, 0, len(recipients))
		for name := range recipients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := ValidRecipientName(name); err != nil {
				return err
			}
			if err := agekey.ValidateRecipient(recipients[name]); err != nil {
				return fmt.Errorf("environment %q: recipient %q: %w", envName, name, err)
			}
			if _, dup := c.SOPS.AgeRecipients[name]; dup {
				return fmt.Errorf("recipient %q is defined both globally and in environment %q", name, envName)
			}
		}
	}
	return nil
}
