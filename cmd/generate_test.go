package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/sopstest"
)

func TestGenerate_DevKeyOnly(t *testing.T) {
	setupProject(t)
	devPub, devKey := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	t.Setenv("SOPS_AGE_KEY_FILE", devKey)
	out := t.TempDir()
	localOut := filepath.Join(out, ".env")
	prodOut := filepath.Join(out, ".env.prod")

	_, _, err = runCLI(t, "--env", "local", "generate", "--output", localOut)
	require.NoError(t, err)
	content := readFile(t, localOut)
	assert.NotRegexp(t, regexp.MustCompile(`(?m)^[A-Z_]+=$`), string(content))

	_, _, err = runCLI(t, "--env", "production", "generate", "--output", prodOut)
	require.Error(t, err)
	_, statErr := os.Stat(prodOut)
	assert.True(t, os.IsNotExist(statErr))
}
