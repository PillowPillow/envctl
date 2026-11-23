package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"go.yaml.in/yaml/v3"
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
	return &cfg, nil
}

// Dir returns the directory holding the config file.
func (c *Config) Dir() string {
	return filepath.Dir(c.configPath)
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
	for _, env := range c.Environments {
		if filepath.Clean(env.Secrets) != filepath.Clean(file) {
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
		if c.Environments[name].Secrets == "" {
			return fmt.Errorf("environment %q: secrets path is required", name)
		}
	}
	for name := range c.SOPS.AgeRecipients {
		if err := ValidRecipientName(name); err != nil {
			return err
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
			if _, dup := c.SOPS.AgeRecipients[name]; dup {
				return fmt.Errorf("recipient %q is defined both globally and in environment %q", name, envName)
			}
		}
	}
	return nil
}
