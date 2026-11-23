package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	content := `version: "1"
template: .env.example
environments:
  local:
    secrets: .envctl/secrets.local.enc.yaml
    output: .env
    overrides:
      APP_ENV: local
  e2e:
    secrets: .envctl/secrets.local.enc.yaml
    output: .env.e2e
    overrides:
      APP_ENV: e2e
  production:
    secrets: .envctl/secrets.production.enc.yaml
    overrides:
      APP_ENV: production
    strip:
      - DEBUG_VAR
sops:
  age_recipients:
    alice: age10qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqvjmlsk
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".envctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "1", cfg.Version)
	assert.Equal(t, ".env.example", cfg.Template)
	assert.Len(t, cfg.Environments, 3)

	local, err := cfg.GetEnvironment("local")
	require.NoError(t, err)
	assert.Equal(t, "local", local.Overrides["APP_ENV"])
	assert.Equal(t, ".env", local.Output)

	e2e, err := cfg.GetEnvironment("e2e")
	require.NoError(t, err)
	assert.Equal(t, ".env.e2e", e2e.Output)

	prod, err := cfg.GetEnvironment("production")
	require.NoError(t, err)
	assert.Equal(t, []string{"DEBUG_VAR"}, prod.Strip)
	assert.Empty(t, prod.Output)

	_, err = cfg.GetEnvironment("nonexistent")
	assert.Error(t, err)
}

func TestLoad_Validation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing version",
			content: `template: .env.example`,
			wantErr: "version is required",
		},
		{
			name:    "missing template",
			content: `version: "1"`,
			wantErr: "template is required",
		},
		{
			name: "missing secrets path",
			content: `version: "1"
template: .env.example
environments:
  local:
    overrides: {}`,
			wantErr: "secrets path is required",
		},
		{
			name: "reserved environment name",
			content: `version: "1"
template: .env.example
environments:
  global:
    secrets: .envctl/global.enc.yaml`,
			wantErr: `environment name "global" is reserved`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".envctl.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o644))

			_, err := Load(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

const perEnvConfig = `version: "1"
template: .env.example
environments:
  local:
    secrets: .envctl/local.enc.yaml
    age_recipients:
      dev: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh
  production:
    secrets: .envctl/production.enc.yaml
sops:
  age_recipients:
    nicolas: age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q
    ci: age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6
`

func parseConfig(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := Parse([]byte(content), filepath.Join(t.TempDir(), ".envctl.yaml"))
	require.NoError(t, err)
	return cfg
}

func TestLoad_PerEnvRecipients(t *testing.T) {
	cfg := parseConfig(t, perEnvConfig)

	recipients, err := cfg.Recipients("local")
	require.NoError(t, err)
	assert.Equal(t, []Recipient{
		{Name: "ci", Key: "age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", Scope: ScopeGlobal},
		{Name: "nicolas", Key: "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q", Scope: ScopeGlobal},
		{Name: "dev", Key: "age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", Scope: "local"},
	}, recipients)

	prod, err := cfg.GetEnvironment("production")
	require.NoError(t, err)
	assert.Equal(t, []string{"age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q"}, cfg.SecretsRecipientKeys(prod.Secrets))
}

func TestLoad_DuplicateRecipientName(t *testing.T) {
	content := strings.Replace(perEnvConfig, "dev: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", "ci: age15qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqaq0d3m", 1)
	_, err := Parse([]byte(content), ".envctl.yaml")
	require.Error(t, err)
	assert.ErrorContains(t, err, `recipient "ci" is defined both globally and in environment "local"`)
}

func TestLoad_InvalidRecipientKey(t *testing.T) {
	for _, tc := range []struct {
		name, old, new, want string
	}{
		{"global comma-joined", "ci: age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", `ci: "age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6,age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh"`, `recipient "ci": invalid age public key`},
		{"env bad checksum", "dev: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", "dev: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvq", `environment "local": recipient "dev": invalid age public key: bad checksum`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(strings.Replace(perEnvConfig, tc.old, tc.new, 1)), ".envctl.yaml")
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestLoad_InvalidRecipientName(t *testing.T) {
	content := strings.Replace(perEnvConfig, "dev: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", `"a b": age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh`, 1)
	_, err := Parse([]byte(content), ".envctl.yaml")
	require.Error(t, err)
	assert.ErrorContains(t, err, `invalid recipient name "a b"`)
}

func TestLoad_ResolvesSecretsRelativeToConfigDir(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "elsewhere", "abs.enc.yaml")
	content := `version: "1"
template: .env.example
environments:
  local:
    secrets: .envctl/s.enc.yaml
    output: .env
  other:
    secrets: ` + abs + `
`
	path := filepath.Join(dir, ".envctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)

	local, err := cfg.GetEnvironment("local")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".envctl/s.enc.yaml"), local.Secrets)
	assert.Equal(t, ".env", local.Output)
	assert.Equal(t, ".env.example", cfg.Template)

	other, err := cfg.GetEnvironment("other")
	require.NoError(t, err)
	assert.Equal(t, abs, other.Secrets)
}

func TestRecipients_AllScopes(t *testing.T) {
	cfg := parseConfig(t, perEnvConfig)

	recipients, err := cfg.Recipients("")
	require.NoError(t, err)
	assert.Equal(t, []Recipient{
		{Name: "ci", Key: "age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", Scope: ScopeGlobal},
		{Name: "nicolas", Key: "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q", Scope: ScopeGlobal},
		{Name: "dev", Key: "age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", Scope: "local"},
	}, recipients)
}

func TestRecipients_UnknownEnv(t *testing.T) {
	cfg := parseConfig(t, perEnvConfig)

	_, err := cfg.Recipients("staging")
	assert.ErrorContains(t, err, `environment "staging" not found`)
}

func TestValidRecipientName(t *testing.T) {
	for _, name := range []string{"dev", "ci-1", "a.b_c"} {
		assert.NoError(t, ValidRecipientName(name), name)
	}
	for _, name := range []string{"", "a b", "dev: x", "a#b", "-x"} {
		assert.Error(t, ValidRecipientName(name), name)
	}
}

func TestSecretsRecipientKeys_UnionOfEnvironmentsSharingAFile(t *testing.T) {
	content := strings.Replace(perEnvConfig, "  production:\n",
		"  e2e:\n    secrets: ./.envctl/../.envctl/local.enc.yaml\n    age_recipients:\n      vm2: age17qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqdvjpzr\n      dup: age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh\n  production:\n", 1)
	cfg := parseConfig(t, content)

	local, err := cfg.GetEnvironment("local")
	require.NoError(t, err)
	assert.Equal(t, []string{"age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", "age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q", "age17qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqdvjpzr"}, cfg.SecretsRecipientKeys(local.Secrets))

	prod, err := cfg.GetEnvironment("production")
	require.NoError(t, err)
	assert.Equal(t, []string{"age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q"}, cfg.SecretsRecipientKeys(prod.Secrets))
}

func TestSecretsRecipientKeys_SymlinkedSharedFile(t *testing.T) {
	content := strings.Replace(perEnvConfig, "  production:\n",
		"  e2e:\n    secrets: .envctl/e2e.enc.yaml\n    age_recipients:\n      vm2: age17qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqdvjpzr\n  production:\n", 1)
	path := filepath.Join(t.TempDir(), ".envctl.yaml")
	cfg, err := Parse([]byte(content), path)
	require.NoError(t, err)
	envctlDir := filepath.Join(filepath.Dir(path), ".envctl")
	require.NoError(t, os.MkdirAll(envctlDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envctlDir, "local.enc.yaml"), []byte("{}\n"), 0o600))
	require.NoError(t, os.Symlink("local.enc.yaml", filepath.Join(envctlDir, "e2e.enc.yaml")))

	want := []string{"age12qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqy5ped6", "age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", "age14qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0eq50q", "age17qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqdvjpzr"}
	assert.Equal(t, want, cfg.SecretsRecipientKeys(cfg.Environments["local"].Secrets))
	assert.Equal(t, want, cfg.SecretsRecipientKeys(cfg.Environments["e2e"].Secrets))
}

func TestLoad_SameRecipientNameInTwoEnvs(t *testing.T) {
	content := strings.Replace(perEnvConfig, "    secrets: .envctl/production.enc.yaml\n",
		"    secrets: .envctl/production.enc.yaml\n    age_recipients:\n      dev: age16qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqhn87p5\n", 1)
	cfg := parseConfig(t, content)

	recipients, err := cfg.Recipients("")
	require.NoError(t, err)
	assert.Contains(t, recipients, Recipient{Name: "dev", Key: "age13qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq4x4tvh", Scope: "local"})
	assert.Contains(t, recipients, Recipient{Name: "dev", Key: "age16qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqhn87p5", Scope: "production"})
}
