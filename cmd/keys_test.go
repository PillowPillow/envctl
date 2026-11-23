package cmd

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/sopskeys"
	"github.com/PillowPillow/envctl/v2/internal/sopstest"
)

func sortedKeys(keys ...string) []string {
	out := append([]string(nil), keys...)
	sort.Strings(out)
	return out
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test temp path
	require.NoError(t, err)
	return b
}

func fileRecipients(t *testing.T, path string) []string {
	t.Helper()
	got, err := sopskeys.FileRecipients(path)
	require.NoError(t, err)
	return got
}

func TestKeysAdd_EnvScoped(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	prodBefore := readFile(t, p.ProdSecrets)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub), fileRecipients(t, p.ProdSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))

	want := strings.Replace(string(p.OriginalConfig),
		"    output: .env\n",
		"    output: .env\n    age_recipients:\n      dev: "+devPub+"\n", 1)
	assert.Equal(t, want, string(readFile(t, p.ConfigPath)))
}

func TestKeysAdd_Global(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "keys", "add", "dev", devPub)
	require.NoError(t, err)

	want := sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub)
	assert.Equal(t, want, fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, want, fileRecipients(t, p.ProdSecrets))
}

func TestKeysAdd_FromSubdirectory(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	sub := p.Dir + "/a/b"
	require.NoError(t, os.MkdirAll(sub, 0o755)) //nolint:gosec // test dir
	t.Chdir(sub)
	prodBefore := readFile(t, p.ProdSecrets)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
	assert.Contains(t, string(readFile(t, p.ConfigPath)), "dev: "+devPub)

	entries, err := os.ReadDir(sub)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestKeysAdd_InvalidName(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)

	_, _, err := runCLI(t, "keys", "add", "dev: x", devPub)
	require.Error(t, err)

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
	assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
}

func TestKeysAdd_DuplicateAcrossScopes(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	afterFirst := readFile(t, p.ConfigPath)

	_, _, err = runCLI(t, "--env", "production", "keys", "add", "dev", devPub)
	require.ErrorContains(t, err, `recipient "dev" already exists in scope "local"`)

	_, _, err = runCLI(t, "keys", "add", "ci", devPub)
	require.ErrorContains(t, err, `recipient "ci" already exists in scope "global"`)

	assert.Equal(t, afterFirst, readFile(t, p.ConfigPath))
}

func TestKeysAdd_SyncFailureLeavesConfigUntouched(t *testing.T) {
	p := setupProject(t)
	devPub, devKey := sopstest.NewKey(t)
	evePub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)
	t.Setenv("SOPS_AGE_KEY_FILE", devKey)

	_, _, err = runCLI(t, "keys", "add", "eve", evePub)
	require.Error(t, err)

	assert.ErrorContains(t, err, p.LocalSecrets)
	assert.ErrorContains(t, err, "re-run the same command")
	assert.Contains(t, fileRecipients(t, p.LocalSecrets), evePub)
	assert.NotContains(t, fileRecipients(t, p.ProdSecrets), evePub)
	assert.Equal(t, configBefore, readFile(t, p.ConfigPath))
}

func TestKeysRemove_EnvScoped(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	_, stderr, err := runCLI(t, "--env", "local", "keys", "remove", "dev")
	require.NoError(t, err)

	assert.Contains(t, stderr, "git history")
	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub), fileRecipients(t, p.LocalSecrets))
}

func TestKeysRemove_GlobalWithEnv(t *testing.T) {
	p := setupProject(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "remove", "ci")
	require.ErrorContains(t, err, "is global")

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
}

func TestKeysRemove_LastRecipient(t *testing.T) {
	p := setupProject(t)
	localBefore := readFile(t, p.LocalSecrets)
	t.Setenv("SOPS_AGE_KEY_FILE", p.Nicolas.KeyFile)

	_, _, err := runCLI(t, "keys", "remove", "ci")
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)

	_, _, err = runCLI(t, "keys", "remove", "nicolas")
	require.ErrorContains(t, err, "would have no recipients left")

	assert.Equal(t, configBefore, readFile(t, p.ConfigPath))
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.LocalSecrets))
	assert.NotEqual(t, localBefore, readFile(t, p.LocalSecrets))
}

func TestKeysList_Scopes(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	stdout, _, err := runCLI(t, "keys", "list")
	require.NoError(t, err)
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		rows = append(rows, strings.Fields(line))
	}
	require.Len(t, rows, 4)
	assert.Equal(t, []string{"NAME", "KEY", "SCOPE"}, rows[0])
	assert.Equal(t, []string{"ci", p.CI.Pub, "global"}, rows[1])
	assert.Equal(t, []string{"nicolas", p.Nicolas.Pub, "global"}, rows[2])
	assert.Equal(t, []string{"dev", devPub, "local"}, rows[3])

	stdout, _, err = runCLI(t, "--env", "production", "keys", "list")
	require.NoError(t, err)
	assert.NotContains(t, stdout, devPub)
	assert.Contains(t, stdout, p.CI.Pub)
}
