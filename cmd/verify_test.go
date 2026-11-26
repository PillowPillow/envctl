package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_StrictUnusedSecretIsNotReportedAsMissing(t *testing.T) {
	p := setupProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.Dir, ".env.example"), []byte("API_URL=https://example.test\n"), 0o644)) //nolint:gosec // test file

	stdout, _, err := runCLI(t, "--env", "local", "verify", "--strict")
	require.Error(t, err)
	assert.Equal(t, "1 unused secret(s) in strict mode", err.Error())
	assert.Contains(t, stdout, "TOKEN (unused)")
	assert.NotContains(t, stdout, "missing required")
}
