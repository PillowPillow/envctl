package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/sopstest"
)

func TestEdit_BootstrapUsesEffectiveRecipients(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	require.NoError(t, os.Remove(p.LocalSecrets))

	editor := filepath.Join(t.TempDir(), "editor.sh")
	script := "#!/bin/sh\necho 'NEW_VAR: dummy' > \"$1\"\n"
	require.NoError(t, os.WriteFile(editor, []byte(script), 0o700)) //nolint:gosec // test script
	t.Setenv("EDITOR", editor)

	_, _, err = runCLI(t, "--env", "local", "edit")
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub), fileRecipients(t, p.LocalSecrets))
}

func TestEdit_BootstrapUsesRecipientsOfEveryEnvSharingTheFile(t *testing.T) {
	p := setupProject(t)
	shareLocalSecrets(t, p, "staging")
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	require.NoError(t, os.Remove(p.LocalSecrets))

	editor := filepath.Join(t.TempDir(), "editor.sh")
	script := "#!/bin/sh\necho 'NEW_VAR: dummy' > \"$1\"\n"
	require.NoError(t, os.WriteFile(editor, []byte(script), 0o700)) //nolint:gosec // test script
	t.Setenv("EDITOR", editor)

	_, _, err = runCLI(t, "--env", "staging", "edit")
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub), fileRecipients(t, p.LocalSecrets))
}
