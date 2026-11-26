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

func TestBackgroundUpdateCheckEnabled_RespectsOptOut(t *testing.T) {
	prevVersion, prevGo := appVersion, installedWithGo
	t.Cleanup(func() { appVersion, installedWithGo = prevVersion, prevGo })
	appVersion, installedWithGo = "2.1.1", false

	cases := []struct {
		value string
		want  bool
	}{
		{"", true},
		{"1", false},
		{"true", false},
		{"0", true},
		{"false", true},
	}
	for _, tc := range cases {
		t.Run("ENVCTL_NO_UPDATE_CHECK="+tc.value, func(t *testing.T) {
			t.Setenv("ENVCTL_NO_UPDATE_CHECK", tc.value)
			assert.Equal(t, tc.want, backgroundUpdateCheckEnabled())
		})
	}
}
