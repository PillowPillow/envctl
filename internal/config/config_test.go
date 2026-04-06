package config

import (
	"os"
	"path/filepath"
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
    - age1abc123
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
