package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecker_ShouldCheck_NoFile(t *testing.T) {
	c := &Checker{ConfigDir: t.TempDir()}
	assert.True(t, c.ShouldCheck())
}

func TestChecker_ShouldCheck_RecentCheck(t *testing.T) {
	dir := t.TempDir()
	content := fmt.Sprintf("%d\nv1.0.0\n", time.Now().Unix())
	require.NoError(t, os.WriteFile(filepath.Join(dir, checkFile), []byte(content), 0o644))

	c := &Checker{ConfigDir: dir}
	assert.False(t, c.ShouldCheck())
}

func TestChecker_ShouldCheck_OldCheck(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-25 * time.Hour).Unix()
	content := fmt.Sprintf("%d\nv1.0.0\n", oldTime)
	require.NoError(t, os.WriteFile(filepath.Join(dir, checkFile), []byte(content), 0o644))

	c := &Checker{ConfigDir: dir}
	assert.True(t, c.ShouldCheck())
}

func TestChecker_ShouldCheck_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, checkFile), []byte("not-a-number"), 0o644))

	c := &Checker{ConfigDir: dir}
	assert.True(t, c.ShouldCheck())
}

func TestChecker_RunBackground_UpdateAvailable(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v2.0.0"})
	}))

	c := &Checker{
		Client:     client,
		ConfigDir:  t.TempDir(),
		CurrentVer: Version{Major: 1},
		Timeout:    2 * time.Second,
	}

	ch := c.RunBackground()
	msg := <-ch
	assert.Contains(t, msg, "v1.0.0")
	assert.Contains(t, msg, "v2.0.0")
	assert.Contains(t, msg, "envctl update")
}

func TestChecker_RunBackground_AlreadyUpToDate(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v1.0.0"})
	}))

	c := &Checker{
		Client:     client,
		ConfigDir:  t.TempDir(),
		CurrentVer: Version{Major: 1},
		Timeout:    2 * time.Second,
	}

	ch := c.RunBackground()
	msg, ok := <-ch
	assert.False(t, ok || msg != "")
}

func TestChecker_CachedNotice(t *testing.T) {
	dir := t.TempDir()
	content := fmt.Sprintf("%d\nv3.0.0\n", time.Now().Unix())
	require.NoError(t, os.WriteFile(filepath.Join(dir, checkFile), []byte(content), 0o644))

	c := &Checker{
		ConfigDir:  dir,
		CurrentVer: Version{Major: 1},
	}

	msg := c.cachedNotice()
	assert.Contains(t, msg, "v3.0.0")
	assert.Contains(t, msg, "envctl update")
}

func TestChecker_CachedNotice_NoNewerVersion(t *testing.T) {
	dir := t.TempDir()
	content := fmt.Sprintf("%d\nv1.0.0\n", time.Now().Unix())
	require.NoError(t, os.WriteFile(filepath.Join(dir, checkFile), []byte(content), 0o644))

	c := &Checker{
		ConfigDir:  dir,
		CurrentVer: Version{Major: 1},
	}

	assert.Empty(t, c.cachedNotice())
}

func TestResetCheckTimer(t *testing.T) {
	dir := t.TempDir()
	ResetCheckTimer(dir, "v2.0.0")

	data, err := os.ReadFile(filepath.Join(dir, checkFile)) //nolint:gosec // test file with controlled path
	require.NoError(t, err)
	assert.Contains(t, string(data), "v2.0.0")
}
