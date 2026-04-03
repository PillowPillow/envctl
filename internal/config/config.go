package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version      string                 `yaml:"version"`
	Template     string                 `yaml:"template"`
	Environments map[string]Environment `yaml:"environments"`
	SOPS         SOPSConfig             `yaml:"sops"`
}

type Environment struct {
	Secrets   string            `yaml:"secrets"`
	Overrides map[string]string `yaml:"overrides"`
	Strip     []string          `yaml:"strip"`
}

type SOPSConfig struct {
	AgeRecipients []string `yaml:"age_recipients"`
}

const DefaultConfigFile = ".envctl.yaml"

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from FindConfigFile or user flag
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	return &cfg, nil
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
	for name, env := range c.Environments {
		if env.Secrets == "" {
			return fmt.Errorf("environment %q: secrets path is required", name)
		}
	}
	return nil
}
