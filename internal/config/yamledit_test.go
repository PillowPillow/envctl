package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func readGolden(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "envctl.golden.yaml"))
	require.NoError(t, err)
	return src
}

func TestAddRemoveRecipient_GoldenRoundTrip(t *testing.T) {
	src := readGolden(t)
	for _, envName := range []string{"", "local"} {
		t.Run("env="+envName, func(t *testing.T) {
			added, err := AddRecipient(src, envName, "dev", "age1dev")
			require.NoError(t, err)
			assert.NotEqual(t, string(src), string(added))

			removed, err := RemoveRecipient(added, envName, "dev")
			require.NoError(t, err)
			assert.Equal(t, string(src), string(removed))
		})
	}
}

func TestAddRecipient_Env_CreatesBlock(t *testing.T) {
	src := readGolden(t)

	out, err := AddRecipient(src, "local", "dev", "age1dev")
	require.NoError(t, err)

	block := "\n    age_recipients:\n      dev: age1dev\n"
	assert.Contains(t, string(out), "      APP_ENV: local"+block+"\n  production:\n")
	assert.Equal(t, 1, strings.Count(string(out), block))

	cfg, err := Parse(out, ".envctl.yaml")
	require.NoError(t, err)
	assert.Equal(t, "age1dev", cfg.Environments["local"].AgeRecipients["dev"])
	assert.Empty(t, cfg.Environments["production"].AgeRecipients)
}

func TestAddRecipient_Global_AppendsToBlock(t *testing.T) {
	src := readGolden(t)

	out, err := AddRecipient(src, "", "dev", "age1dev")
	require.NoError(t, err)

	want := strings.Replace(string(src), "    nicolas: age1nicolas\n", "    nicolas: age1nicolas\n    dev: age1dev\n", 1)
	assert.Equal(t, want, string(out))
}

func TestAddRecipient_GlobalMissingSops(t *testing.T) {
	src := "version: \"1\"\ntemplate: .env.example\nenvironments:\n  local:\n    secrets: .envctl/secrets.local.enc.yaml\n"

	out, err := AddRecipient([]byte(src), "", "dev", "age1dev")
	require.NoError(t, err)

	assert.Equal(t, src+"sops:\n  age_recipients:\n    dev: age1dev\n", string(out))
}

func TestAddRecipient_EmptyFlowMap(t *testing.T) {
	src, err := yaml.Marshal(Config{
		Version:  "1",
		Template: ".env.example",
		Environments: map[string]Environment{
			"local":      {Secrets: ".envctl/secrets.local.enc.yaml"},
			"production": {Secrets: ".envctl/secrets.production.enc.yaml"},
		},
		SOPS: SOPSConfig{AgeRecipients: map[string]string{}},
	})
	require.NoError(t, err)
	require.Contains(t, string(src), "age_recipients: {}")

	out, err := AddRecipient(src, "", "dev", "age1dev")
	require.NoError(t, err)
	assert.Contains(t, string(out), "\n    age_recipients:\n        dev: age1dev\n")

	cfg, err := Parse(out, ".envctl.yaml")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"dev": "age1dev"}, cfg.SOPS.AgeRecipients)

	removed, err := RemoveRecipient(out, "", "dev")
	require.NoError(t, err)
	assert.Equal(t, string(src), string(removed))
}

func TestAddRecipient_NonEmptyFlowMap(t *testing.T) {
	src := "version: \"1\"\ntemplate: .env.example\nenvironments:\n  local:\n    secrets: s.enc.yaml\nsops:\n  age_recipients: {ci: age1ci}\n"

	_, err := AddRecipient([]byte(src), "", "dev", "age1dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, "flow style is not supported")
}

func TestAddRecipient_PreservesCRLF(t *testing.T) {
	src := strings.ReplaceAll(string(readGolden(t)), "\n", "\r\n")

	for _, envName := range []string{"", "local"} {
		out, err := AddRecipient([]byte(src), envName, "dev", "age1dev")
		require.NoError(t, err)

		lines := strings.SplitAfter(string(out), "\n")
		assert.Empty(t, lines[len(lines)-1])
		for _, line := range lines[:len(lines)-1] {
			assert.True(t, strings.HasSuffix(line, "\r\n"), "line %q", line)
		}
	}
}

func TestAddRecipient_NoTrailingNewline(t *testing.T) {
	src := strings.TrimSuffix(string(readGolden(t)), "\n")

	out, err := AddRecipient([]byte(src), "", "dev", "age1dev")
	require.NoError(t, err)

	cfg, err := Parse(out, ".envctl.yaml")
	require.NoError(t, err)
	assert.Equal(t, "age1nicolas", cfg.SOPS.AgeRecipients["nicolas"])
	assert.Equal(t, "age1dev", cfg.SOPS.AgeRecipients["dev"])
	assert.True(t, strings.HasSuffix(string(out), "    nicolas: age1nicolas\n    dev: age1dev\n"))
}

func TestAddRecipient_UnknownEnv(t *testing.T) {
	_, err := AddRecipient(readGolden(t), "staging", "dev", "age1dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, `environment "staging" not found`)
}

func TestRemoveRecipient_NotFound(t *testing.T) {
	_, err := RemoveRecipient(readGolden(t), "local", "bob")
	require.Error(t, err)
	assert.ErrorContains(t, err, `recipient "bob" not found in scope "local"`)
}
