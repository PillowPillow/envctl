package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_FailureDoesNotPrintUsage(t *testing.T) {
	stdout, stderr, err := runCLI(t, "verify")
	require.EqualError(t, err, "--env is required")
	assert.NotContains(t, stdout+stderr, "Usage:")
	assert.Equal(t, 1, strings.Count(stderr, "Error: --env is required"))
}

func TestUnknownFlag_StillReportsError(t *testing.T) {
	_, stderr, err := runCLI(t, "verify", "--no-such-flag")
	require.Error(t, err)
	assert.Contains(t, stderr, "Error: unknown flag: --no-such-flag")
}
