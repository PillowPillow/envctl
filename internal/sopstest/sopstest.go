package sopstest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func RequireTools(t testing.TB) {
	t.Helper()
	for _, tool := range []string{"sops", "age-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("CI") == "true" {
				t.Fatalf("%s is required in CI", tool)
			}
			t.Skipf("%s not installed", tool)
		}
	}
}

func NewKey(t testing.TB) (pub, keyFile string) {
	t.Helper()
	keyFile = filepath.Join(t.TempDir(), "key.txt")
	if out, err := exec.Command("age-keygen", "-o", keyFile).CombinedOutput(); err != nil { //nolint:gosec // fixed binary, temp path
		t.Fatalf("age-keygen failed: %v: %s", err, redact(out))
	}
	out, err := exec.Command("age-keygen", "-y", keyFile).Output() //nolint:gosec // fixed binary, temp path
	if err != nil {
		t.Fatalf("age-keygen -y failed: %v", err)
	}
	return strings.TrimSpace(string(out)), keyFile
}

func redact(b []byte) string {
	if len(b) > 0 {
		return "(output suppressed)"
	}
	return ""
}
