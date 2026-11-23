package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
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
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub), fileRecipients(t, p.ProdSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))

	want := strings.Replace(string(p.OriginalConfig),
		"    output: .env\n",
		"    output: .env\n    age_recipients:\n      dev: "+devPub+"\n", 1)
	assert.Equal(t, want, string(readFile(t, p.ConfigPath)))

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

	_, _, err = runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.ErrorContains(t, err, `recipient "dev" already exists in scope "local"`)

	_, _, err = runCLI(t, "keys", "add", "dev", devPub)
	require.ErrorContains(t, err, `recipient "dev" already exists in scope "local"`)

	_, _, err = runCLI(t, "keys", "add", "ci", devPub)
	require.ErrorContains(t, err, `recipient "ci" already exists in scope "global"`)

	_, _, err = runCLI(t, "--env", "production", "keys", "add", "ci", devPub)
	require.ErrorContains(t, err, `recipient "ci" already exists in scope "global"`)

	assert.Equal(t, afterFirst, readFile(t, p.ConfigPath))
}

func TestKeysAdd_UndecryptableFileChangesNothing(t *testing.T) {
	p := setupProject(t)
	devPub, devKey := sopstest.NewKey(t)
	evePub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)
	localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)
	t.Setenv("SOPS_AGE_KEY_FILE", devKey)

	_, _, err = runCLI(t, "keys", "add", "eve", evePub)
	require.ErrorContains(t, err, "nothing was changed")

	assert.ErrorContains(t, err, p.ProdSecrets)
	assert.NotContains(t, err.Error(), p.LocalSecrets)
	assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
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

	_, _, err := runCLI(t, "keys", "remove", "nicolas")
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)
	localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)

	_, _, err = runCLI(t, "keys", "remove", "ci")
	require.ErrorContains(t, err, "would have no recipients left")

	assert.Equal(t, configBefore, readFile(t, p.ConfigPath))
	assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
}

func TestKeysRemove_OwnKey(t *testing.T) {
	p := setupProject(t)

	_, _, err := runCLI(t, "keys", "remove", "ci")
	require.NoError(t, err)

	assert.NotContains(t, string(readFile(t, p.ConfigPath)), "ci:")
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.ProdSecrets))
}

func TestKeysRemove_NotFound(t *testing.T) {
	p := setupProject(t)

	_, _, err := runCLI(t, "keys", "remove", "ghost")
	require.ErrorContains(t, err, `recipient "ghost" not found in scope "global"`)

	_, _, err = runCLI(t, "--env", "local", "keys", "remove", "ghost")
	require.ErrorContains(t, err, `recipient "ghost" not found in scope "local"`)

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
}

func TestKeysRemove_GlobalNotFoundHintsEnvironments(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	dev2Pub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	_, _, err = runCLI(t, "--env", "production", "keys", "add", "dev", dev2Pub)
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)

	_, _, err = runCLI(t, "keys", "remove", "dev")
	require.ErrorContains(t, err, `recipient "dev" not found in scope "global" (it exists in: local, production — use --env)`)

	assert.Equal(t, configBefore, readFile(t, p.ConfigPath))
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

func shareLocalSecrets(t *testing.T, p project, env string) {
	t.Helper()
	cfg := strings.Replace(string(readFile(t, p.ConfigPath)), "sops:\n",
		"  "+env+":\n    secrets: .envctl/local.enc.yaml\n    output: .env."+env+"\nsops:\n", 1)
	require.NoError(t, os.WriteFile(p.ConfigPath, []byte(cfg), 0o644)) //nolint:gosec // test file
}

func TestKeysAdd_SharedSecretsFile(t *testing.T) {
	p := setupProject(t)
	shareLocalSecrets(t, p, "staging")
	devPub, _ := sopstest.NewKey(t)
	evePub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	_, _, err = runCLI(t, "keys", "add", "eve", evePub)
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub, evePub), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, evePub), fileRecipients(t, p.ProdSecrets))
}

func TestKeysAdd_EnvScoped_SharedFileKeepsOtherEnvKey(t *testing.T) {
	p := setupProject(t)
	shareLocalSecrets(t, p, "e2e")
	devPub, _ := sopstest.NewKey(t)
	vm2Pub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	_, _, err = runCLI(t, "--env", "e2e", "keys", "add", "vm2", vm2Pub)
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub, vm2Pub), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub), fileRecipients(t, p.ProdSecrets))
}

func TestKeysRemove_SharedFileKeepsOtherEnvRecipients(t *testing.T) {
	p := setupProject(t)
	shareLocalSecrets(t, p, "e2e")
	devPub, _ := sopstest.NewKey(t)
	vm2Pub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)
	_, _, err = runCLI(t, "--env", "e2e", "keys", "add", "vm2", vm2Pub)
	require.NoError(t, err)
	_, _, err = runCLI(t, "--env", "e2e", "keys", "remove", "vm2")
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, devPub), fileRecipients(t, p.LocalSecrets))
}

func TestKeysAdd_InvalidKey(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	evePub, _ := sopstest.NewKey(t)
	localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)

	for _, key := range []string{
		devPub + "," + evePub,
		devPub + "\n    eve: " + evePub,
		"age1abc",
		strings.ToUpper(devPub),
	} {
		_, _, err := runCLI(t, "keys", "add", "dev", key)
		require.ErrorContains(t, err, "invalid age public key: expected an age1... bech32 recipient", key)
	}

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
	assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
}

func TestKeysAdd_BadChecksum(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	last := devPub[len(devPub)-1]
	typo := "q"
	if last == 'q' {
		typo = "p"
	}

	_, _, err := runCLI(t, "keys", "add", "dev", devPub[:len(devPub)-1]+typo)
	require.ErrorContains(t, err, "bad checksum")

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
}

func TestKeysAdd_ReservedName(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)

	for _, name := range []string{"null", "NULL", "Null"} {
		_, _, err := runCLI(t, "keys", "add", name, devPub)
		require.ErrorContains(t, err, "refusing to write .envctl.yaml: the edit did not produce the expected recipients", name)
		_, _, err = runCLI(t, "--env", "local", "keys", "add", name, devPub)
		require.ErrorContains(t, err, "refusing to write .envctl.yaml: the edit did not produce the expected recipients", name)
	}

	assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
	assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
	assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
}

func TestKeysAdd_SameNameInTwoEnvs(t *testing.T) {
	p := setupProject(t)
	localDev, _ := sopstest.NewKey(t)
	prodDev, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", localDev)
	require.NoError(t, err)
	_, _, err = runCLI(t, "--env", "production", "keys", "add", "dev", prodDev)
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, localDev), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, prodDev), fileRecipients(t, p.ProdSecrets))

	stdout, _, err := runCLI(t, "keys", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "dev      "+localDev+"  local")
	assert.Contains(t, stdout, "dev      "+prodDev+"  production")

	_, _, err = runCLI(t, "--env", "local", "keys", "remove", "dev")
	require.NoError(t, err)

	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub), fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, sortedKeys(p.CI.Pub, p.Nicolas.Pub, prodDev), fileRecipients(t, p.ProdSecrets))
	stdout, _, err = runCLI(t, "keys", "list")
	require.NoError(t, err)
	assert.NotContains(t, stdout, localDev)
	assert.Contains(t, stdout, "dev      "+prodDev+"  production")
}

func TestKeysRemove_RerunSkipsFileOperatorCanNoLongerDecrypt(t *testing.T) {
	p := setupProject(t)
	sopsPath, err := exec.LookPath("sops")
	require.NoError(t, err)
	require.NoError(t, sopskeys.Sync(sopsPath, p.LocalSecrets, []string{p.Nicolas.Pub}))

	stdout, stderr, err := runCLI(t, "keys", "remove", "ci")
	require.NoError(t, err)

	assert.Contains(t, stdout, p.LocalSecrets+" already up to date")
	assert.Contains(t, stderr, p.LocalSecrets+" was not rotated")
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.LocalSecrets))
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.ProdSecrets))
	assert.NotContains(t, string(readFile(t, p.ConfigPath)), "ci:")
}

func TestKeysRemove_RotatesWhenRecipientLabelsAlreadyMatch(t *testing.T) {
	p := setupProject(t)
	sopsCfg := filepath.Join(t.TempDir(), "sops.yaml")
	require.NoError(t, os.WriteFile(sopsCfg, []byte("creation_rules:\n  - age: "+p.Nicolas.Pub+"\n"), 0o600))
	out, err := exec.Command("sops", "--config", sopsCfg, "updatekeys", "--yes", p.LocalSecrets).CombinedOutput() //nolint:gosec // test command
	require.NoError(t, err, string(out))
	require.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.LocalSecrets))
	localBefore := readFile(t, p.LocalSecrets)
	t.Setenv("SOPS_AGE_KEY_FILE", p.Nicolas.KeyFile)

	stdout, _, err := runCLI(t, "keys", "remove", "ci")
	require.NoError(t, err)

	assert.NotContains(t, stdout, "already up to date")
	assert.NotEqual(t, tokenLine(t, localBefore), tokenLine(t, readFile(t, p.LocalSecrets)))
	assert.Equal(t, []string{p.Nicolas.Pub}, fileRecipients(t, p.LocalSecrets))
}

func tokenLine(t *testing.T, data []byte) string {
	t.Helper()
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "TOKEN:") {
			return l
		}
	}
	t.Fatal("TOKEN line not found")
	return ""
}

func TestKeysRemove_LastRecipientWithoutSecretsFile(t *testing.T) {
	p := setupProject(t)
	require.NoError(t, os.Remove(p.LocalSecrets))
	require.NoError(t, os.Remove(p.ProdSecrets))

	_, _, err := runCLI(t, "keys", "remove", "nicolas")
	require.NoError(t, err)
	_, _, err = runCLI(t, "keys", "remove", "ci")
	require.NoError(t, err)

	stdout, _, err := runCLI(t, "keys", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No age recipients configured.")
}

func TestKeysRemove_LastRecipientRefusedOnlyWhereSecretsFileExists(t *testing.T) {
	p := setupProject(t)
	require.NoError(t, os.Remove(p.ProdSecrets))
	_, _, err := runCLI(t, "keys", "remove", "nicolas")
	require.NoError(t, err)
	configBefore := readFile(t, p.ConfigPath)

	_, _, err = runCLI(t, "keys", "remove", "ci")
	require.ErrorContains(t, err, `environment "local" would have no recipients left`)

	assert.Equal(t, configBefore, readFile(t, p.ConfigPath))
}

func TestKeysAdd_KeepsConfigModeAndLeavesNoTempFile(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	require.NoError(t, os.Chmod(p.ConfigPath, 0o640))

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	info, err := os.Stat(p.ConfigPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	assert.Contains(t, string(readFile(t, p.ConfigPath)), "dev: "+devPub)
	entries, err := os.ReadDir(p.Dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{".envctl", ".envctl.yaml", ".env.example"}, names)
}

func TestKeysAdd_ConfigSymlinkStaysALink(t *testing.T) {
	p := setupProject(t)
	devPub, _ := sopstest.NewKey(t)
	real := p.ConfigPath + ".real"
	require.NoError(t, os.Rename(p.ConfigPath, real))
	require.NoError(t, os.Symlink(real, p.ConfigPath))

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	info, err := os.Lstat(p.ConfigPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
	assert.Contains(t, string(readFile(t, real)), "dev: "+devPub)
}

func TestKeysRemove_WarnsWhenSameKeyKeepsAccess(t *testing.T) {
	p := setupProject(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "ci-alias", p.CI.Pub)
	require.NoError(t, err)

	_, stderr, err := runCLI(t, "--env", "local", "keys", "remove", "ci-alias")
	require.NoError(t, err)

	assert.Contains(t, stderr, "another recipient has the same key")
	assert.Contains(t, fileRecipients(t, p.LocalSecrets), p.CI.Pub)
}

func TestKeysRemove_NoSameKeyWarningWhenRevoked(t *testing.T) {
	setupProject(t)
	devPub, _ := sopstest.NewKey(t)

	_, _, err := runCLI(t, "--env", "local", "keys", "add", "dev", devPub)
	require.NoError(t, err)

	_, stderr, err := runCLI(t, "--env", "local", "keys", "remove", "dev")
	require.NoError(t, err)

	assert.NotContains(t, stderr, "another recipient has the same key")
}

func injectVaultKeySource(t *testing.T, file string) {
	t.Helper()
	tampered := strings.Replace(string(readFile(t, file)), "\nsops:\n",
		"\nsops:\n    hc_vault:\n        vault_address: http://127.0.0.1:1\n        engine_path: transit\n        key_name: k\n        created_at: \"2026-11-23T00:00:00Z\"\n        enc: vault:v1:ZmFrZQ==\n", 1)
	require.NoError(t, os.WriteFile(file, []byte(tampered), 0o600))
}

func TestKeys_RefusesFileWithForeignKeySource(t *testing.T) {
	for _, args := range [][]string{
		{"keys", "add", "eve", ""},
		{"keys", "remove", "nicolas"},
	} {
		t.Run(args[1], func(t *testing.T) {
			p := setupProject(t)
			injectVaultKeySource(t, p.ProdSecrets)
			localBefore, prodBefore := readFile(t, p.LocalSecrets), readFile(t, p.ProdSecrets)
			if args[1] == "add" {
				args[3], _ = sopstest.NewKey(t)
			}

			_, _, err := runCLI(t, args...)
			require.ErrorContains(t, err, "key sources other than age")

			assert.ErrorContains(t, err, p.ProdSecrets+": hc_vault")
			assert.Equal(t, localBefore, readFile(t, p.LocalSecrets))
			assert.Equal(t, prodBefore, readFile(t, p.ProdSecrets))
			assert.Equal(t, p.OriginalConfig, readFile(t, p.ConfigPath))
		})
	}
}
