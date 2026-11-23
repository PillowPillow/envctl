package sopskeys

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/sopstest"
)

func sopsBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("sops")
	require.NoError(t, err)
	return p
}

func encryptedFile(t *testing.T, recipients ...string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "secrets.enc.yaml")
	require.NoError(t, os.WriteFile(file, []byte("FOO: bar\n"), 0o600))
	require.NoError(t, Encrypt(sopsBin(t), file, recipients))
	return file
}

func sorted(s ...string) []string {
	sort.Strings(s)
	return s
}

func decrypt(t *testing.T, file, keyFile string) (string, error) {
	t.Helper()
	t.Setenv("SOPS_AGE_KEY_FILE", keyFile)
	out, err := exec.Command(sopsBin(t), "-d", file).Output() //nolint:gosec // test helper
	return string(out), err
}

func fooLine(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file) //nolint:gosec // test file
	require.NoError(t, err)
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "FOO:") {
			return l
		}
	}
	t.Fatal("FOO line not found")
	return ""
}

func TestSync_AddsRecipientAndRotates(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	b, _ := sopstest.NewKey(t)
	c, cKey := sopstest.NewKey(t)
	file := encryptedFile(t, a, b)
	t.Setenv("SOPS_AGE_KEY_FILE", aKey)
	before := fooLine(t, file)

	require.NoError(t, Sync(sopsBin(t), file, []string{a, b, c}))

	got, err := FileRecipients(file)
	require.NoError(t, err)
	assert.Equal(t, sorted(a, b, c), got)
	assert.NotEqual(t, before, fooLine(t, file))
	out, err := decrypt(t, file, cKey)
	require.NoError(t, err)
	assert.Contains(t, out, "bar")
}

func TestSync_RemoveRevokesFutureAccess(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	b, bKey := sopstest.NewKey(t)
	file := encryptedFile(t, a, b)
	t.Setenv("SOPS_AGE_KEY_FILE", aKey)

	require.NoError(t, Sync(sopsBin(t), file, []string{a}))

	got, err := FileRecipients(file)
	require.NoError(t, err)
	assert.Equal(t, []string{a}, got)
	_, err = decrypt(t, file, bKey)
	assert.Error(t, err)
	_, err = decrypt(t, file, aKey)
	assert.NoError(t, err)
}

func TestSync_IgnoresRepoSopsYaml(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	b, _ := sopstest.NewKey(t)
	c, _ := sopstest.NewKey(t)
	file := encryptedFile(t, a, b)
	t.Setenv("SOPS_AGE_KEY_FILE", aKey)
	repoCfg := filepath.Join(filepath.Dir(file), ".sops.yaml")
	content := []byte("creation_rules:\n  - age: " + b + "\n")
	require.NoError(t, os.WriteFile(repoCfg, content, 0o600))

	require.NoError(t, Sync(sopsBin(t), file, []string{a, c}))

	got, err := FileRecipients(file)
	require.NoError(t, err)
	assert.Equal(t, sorted(a, c), got)
	after, err := os.ReadFile(repoCfg) //nolint:gosec // test file
	require.NoError(t, err)
	assert.Equal(t, content, after)
}

func TestSync_EmptyRecipients(t *testing.T) {
	sopstest.RequireTools(t)
	a, _ := sopstest.NewKey(t)
	file := encryptedFile(t, a)
	before, err := os.ReadFile(file) //nolint:gosec // test file
	require.NoError(t, err)

	require.Error(t, Sync(sopsBin(t), file, nil))

	after, err := os.ReadFile(file) //nolint:gosec // test file
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestEncrypt_UsesGivenRecipients(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	b, _ := sopstest.NewKey(t)
	file := encryptedFile(t, a, b)

	got, err := FileRecipients(file)
	require.NoError(t, err)
	assert.Equal(t, sorted(a, b), got)
	out, err := decrypt(t, file, aKey)
	require.NoError(t, err)
	assert.Contains(t, out, "bar")
}
