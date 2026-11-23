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
	"go.yaml.in/yaml/v3"

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

func TestSync_RemovesOperatorsOwnKey(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	b, bKey := sopstest.NewKey(t)
	file := encryptedFile(t, a, b)
	t.Setenv("SOPS_AGE_KEY_FILE", aKey)

	require.NoError(t, Sync(sopsBin(t), file, []string{b}))

	got, err := FileRecipients(file)
	require.NoError(t, err)
	assert.Equal(t, []string{b}, got)
	_, err = decrypt(t, file, aKey)
	assert.Error(t, err)
	out, err := decrypt(t, file, bKey)
	require.NoError(t, err)
	assert.Contains(t, out, "bar")
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

func TestCanDecrypt(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	_, bKey := sopstest.NewKey(t)
	file := encryptedFile(t, a)

	t.Setenv("SOPS_AGE_KEY_FILE", aKey)
	assert.NoError(t, CanDecrypt(sopsBin(t), file))

	t.Setenv("SOPS_AGE_KEY_FILE", bKey)
	assert.Error(t, CanDecrypt(sopsBin(t), file))
}

func TestCanDecrypt_IgnoresMalformedRepoSopsYaml(t *testing.T) {
	sopstest.RequireTools(t)
	a, aKey := sopstest.NewKey(t)
	file := encryptedFile(t, a)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(file), ".sops.yaml"), []byte("creation_rules: [broken\n"), 0o600))
	t.Chdir(filepath.Dir(file))
	t.Setenv("SOPS_AGE_KEY_FILE", aKey)

	assert.NoError(t, CanDecrypt(sopsBin(t), file))
}

func TestForeignKeySources_AgeOnly(t *testing.T) {
	sopstest.RequireTools(t)
	a, _ := sopstest.NewKey(t)

	got, err := ForeignKeySources(encryptedFile(t, a))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestForeignKeySources_FlagsEveryNonAgeSourceShape(t *testing.T) {
	sopstest.RequireTools(t)
	a, _ := sopstest.NewKey(t)
	file := encryptedFile(t, a)
	data, err := os.ReadFile(file) //nolint:gosec // test file
	require.NoError(t, err)

	for _, tc := range []struct {
		name, metadata, want string
	}{
		{"list", "    hc_vault:\n        - vault_address: http://127.0.0.1:1\n", "hc_vault"},
		{"block map", "    hc_vault:\n        vault_address: http://127.0.0.1:1\n        key_name: k\n", "hc_vault"},
		{"flow map", "    hc_vault: {vault_address: http://127.0.0.1:1, key_name: k}\n", "hc_vault"},
		{"upper case", "    HC_VAULT:\n        - vault_address: http://127.0.0.1:1\n", "HC_VAULT"},
		{"age in another case", "    AGE:\n        - recipient: " + a + "\n", "AGE"},
		{"key_groups map", "    key_groups:\n        hc_vault:\n            vault_address: http://127.0.0.1:1\n", "key_groups"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tampered := strings.Replace(string(data), "\nsops:\n", "\nsops:\n"+tc.metadata, 1)
			require.NoError(t, os.WriteFile(file, []byte(tampered), 0o600)) //nolint:gosec // test file

			got, err := ForeignKeySources(file)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, got)
		})
	}
}

func TestIsolatedConfig_KeepsRecipientsVerbatim(t *testing.T) {
	recipients := []string{"age1a #not-a-comment", "age1b: x"}
	path, cleanup, err := isolatedConfig(recipients)
	require.NoError(t, err)
	defer cleanup()
	data, err := os.ReadFile(path) //nolint:gosec // test temp path
	require.NoError(t, err)

	var cfg struct {
		CreationRules []struct {
			Age string `yaml:"age"`
		} `yaml:"creation_rules"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	require.Len(t, cfg.CreationRules, 1)
	assert.Equal(t, "age1a #not-a-comment,age1b: x", cfg.CreationRules[0].Age)
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
