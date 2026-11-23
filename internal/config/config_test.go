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
    alice: age1abc123
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
      dev: age1dev
  production:
    secrets: .envctl/production.enc.yaml
sops:
  age_recipients:
    nicolas: age1nicolas
    ci: age1ci
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
		{Name: "ci", Key: "age1ci", Scope: ScopeGlobal},
		{Name: "nicolas", Key: "age1nicolas", Scope: ScopeGlobal},
		{Name: "dev", Key: "age1dev", Scope: "local"},
	}, recipients)

	keys, err := cfg.EffectiveRecipientKeys("production")
	require.NoError(t, err)
	assert.Equal(t, []string{"age1ci", "age1nicolas"}, keys)
}

func TestLoad_DuplicateRecipientName(t *testing.T) {
	content := strings.Replace(perEnvConfig, "dev: age1dev", "ci: age1other", 1)
	_, err := Parse([]byte(content), ".envctl.yaml")
	require.Error(t, err)
	assert.ErrorContains(t, err, `recipient "ci" is defined both globally and in environment "local"`)
}

func TestLoad_InvalidRecipientName(t *testing.T) {
	content := strings.Replace(perEnvConfig, "dev: age1dev", `"a b": age1dev`, 1)
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
	assert.Equal(t, dir, cfg.Dir())
}

func TestRecipients_AllScopes(t *testing.T) {
	cfg := parseConfig(t, perEnvConfig)

	recipients, err := cfg.Recipients("")
	require.NoError(t, err)
	assert.Equal(t, []Recipient{
		{Name: "ci", Key: "age1ci", Scope: ScopeGlobal},
		{Name: "nicolas", Key: "age1nicolas", Scope: ScopeGlobal},
		{Name: "dev", Key: "age1dev", Scope: "local"},
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

func TestSave_KeepsRelativeSecretsPaths(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "elsewhere", "abs.enc.yaml")
	content := `version: "1"
template: .env.example
environments:
  local:
    secrets: .envctl/s.enc.yaml
  other:
    secrets: ` + abs + `
`
	path := filepath.Join(dir, ".envctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.NoError(t, cfg.Save())

	raw, err := os.ReadFile(path) //nolint:gosec // test temp path
	require.NoError(t, err)
	assert.Contains(t, string(raw), "secrets: .envctl/s.enc.yaml")
	assert.Contains(t, string(raw), "secrets: "+abs)
}
