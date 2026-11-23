package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/config"
	"github.com/PillowPillow/envctl/v2/internal/resolver"
	"github.com/PillowPillow/envctl/v2/internal/secret"
	"github.com/PillowPillow/envctl/v2/internal/template"
)

func TestParseTemplate_Dotenv(t *testing.T) {
	content := "DB_HOST=localhost\nDB_PASSWORD=${DB_PASSWORD}\nDB_NAME=${DB_NAME:-myapp}\n"
	path := writeTempFile(t, ".env.example", content)

	parser, parsed, err := ParseTemplate(path)
	require.NoError(t, err)
	require.NotNil(t, parser)
	require.NotNil(t, parsed)

	assert.Len(t, parsed.Variables, 3)
	assert.Equal(t, template.Static, parsed.Variables[0].Classification)
	assert.Equal(t, template.RequiredSecret, parsed.Variables[1].Classification)
	assert.Equal(t, template.OptionalSecret, parsed.Variables[2].Classification)
}

func TestParseTemplate_FileNotFound(t *testing.T) {
	_, _, err := ParseTemplate("/nonexistent/.env.example")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot open template")
}

func TestResolveEnv(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "SECRET_VAR", Classification: template.RequiredSecret},
			{Name: "OPTIONAL_VAR", Classification: template.OptionalSecret, Default: "fallback"},
			{Name: "STATIC_VAR", Classification: template.Static, Default: "hardcoded"},
		},
	}

	env := &config.Environment{
		Secrets:   "unused-path",
		Overrides: map[string]string{"STATIC_VAR": "overridden"},
		Strip:     []string{},
	}

	provider := &secret.MemoryProvider{
		Secrets: map[string]string{"SECRET_VAR": "secret-value"},
	}

	rs, err := ResolveEnv(provider, env, parsed, false)
	require.NoError(t, err)

	values := rs.Values()
	assert.Equal(t, "secret-value", values["SECRET_VAR"])
	assert.Equal(t, "fallback", values["OPTIONAL_VAR"])
	assert.Equal(t, "overridden", values["STATIC_VAR"])
	assert.False(t, rs.HasErrors())
}

func TestResolveEnv_Strict(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "USED", Classification: template.RequiredSecret},
		},
	}

	env := &config.Environment{Secrets: "unused-path"}
	provider := &secret.MemoryProvider{
		Secrets: map[string]string{"USED": "val", "UNUSED_KEY": "val"},
	}

	rs, err := ResolveEnv(provider, env, parsed, true)
	require.NoError(t, err)

	// In strict mode, unused secret warning is promoted to error
	assert.Equal(t, 1, rs.ErrorCount())
}

func TestResolveEnv_MissingRequired(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "MISSING", Classification: template.RequiredSecret},
		},
	}

	env := &config.Environment{Secrets: "unused-path"}
	provider := &secret.MemoryProvider{Secrets: map[string]string{}}

	rs, err := ResolveEnv(provider, env, parsed, false)
	require.NoError(t, err)

	assert.True(t, rs.HasErrors())
	assert.Equal(t, resolver.StatusMissing, rs.Vars[0].Status)
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
