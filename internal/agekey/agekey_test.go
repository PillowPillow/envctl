package agekey

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encode(hrp string, data []byte) string {
	values := append(expandHRP(hrp), data...)
	mod := polymod(append(values, 0, 0, 0, 0, 0, 0)) ^ 1
	var b strings.Builder
	b.WriteString(hrp + "1")
	for _, v := range data {
		b.WriteByte(charset[v])
	}
	for i := range 6 {
		b.WriteByte(charset[(mod>>(5*(5-i)))&31])
	}
	return b.String()
}

func generateRecipient(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("age-keygen", args...).Output() //nolint:gosec // test helper
	if err != nil {
		t.Skipf("age-keygen %v unavailable: %v", args, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if pub, ok := strings.CutPrefix(line, "# public key: "); ok {
			return pub
		}
	}
	t.Fatal("no public key in age-keygen output")
	return ""
}

func TestValidateRecipient_AcceptsX25519Key(t *testing.T) {
	assert.NoError(t, ValidateRecipient(generateRecipient(t)))
}

func TestValidateRecipient_AcceptsPostQuantumKey(t *testing.T) {
	key := generateRecipient(t, "-pq")
	require.True(t, strings.HasPrefix(key, "age1pq1"))
	assert.NoError(t, ValidateRecipient(key))
}

func TestValidateRecipient_AcceptsPluginRecipients(t *testing.T) {
	data := make([]byte, 53)
	for i := range data {
		data[i] = byte(i % 32)
	}
	for _, hrp := range []string{"age1yubikey", "age1tag", "age1se", "age1fido2-hmac", "age1a.b_c+d"} {
		assert.NoError(t, ValidateRecipient(encode(hrp, data)), hrp)
	}
}

func TestValidateRecipient_RejectsBadChecksum(t *testing.T) {
	valid := encode("age1yubikey", make([]byte, 53))
	for _, key := range []string{
		valid[:len(valid)-1] + "p",
		valid[:len(valid)-1],
		"age1yubikey1" + strings.Repeat("q", 58),
		"age1" + strings.Repeat("q", 58),
	} {
		assert.ErrorContains(t, ValidateRecipient(key), "bad checksum", key)
	}
}

func TestValidateRecipient_RejectsMalformed(t *testing.T) {
	for _, key := range []string{"", "age1abc", "age11qqqqqq", "age1a/b1" + strings.Repeat("q", 58), "AGE1" + strings.Repeat("Q", 58), "age1" + strings.Repeat("q", 57) + ",age1x"} {
		assert.ErrorContains(t, ValidateRecipient(key), "expected an age1... bech32 recipient", key)
	}
}
