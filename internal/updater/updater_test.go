package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestArchive creates a tar.gz archive containing a fake "envctl" binary.
func createTestArchive(t *testing.T, dir string) (path string, checksum string) {
	t.Helper()

	archivePath := filepath.Join(dir, "test-archive.tar.gz")
	f, err := os.Create(archivePath) //nolint:gosec // test file with controlled path
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho envctl-test")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "envctl",
		Mode: 0o755,
		Size: int64(len(content)),
	}))
	_, err = tw.Write(content)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	// Compute SHA256.
	data, err := os.ReadFile(archivePath) //nolint:gosec // test file with controlled path
	require.NoError(t, err)
	h := sha256.Sum256(data)
	return archivePath, hex.EncodeToString(h[:])
}

func TestUpdater_Check_UpdateAvailable(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v2.0.0"})
	}))

	u := &Updater{
		Client:     client,
		CurrentVer: Version{Major: 1},
	}

	result, err := u.Check()
	require.NoError(t, err)
	assert.True(t, result.UpdateAvail)
	assert.Equal(t, "v2.0.0", result.Latest.String())
}

func TestUpdater_Check_AlreadyUpToDate(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v1.0.0"})
	}))

	u := &Updater{
		Client:     client,
		CurrentVer: Version{Major: 1},
	}

	result, err := u.Check()
	require.NoError(t, err)
	assert.False(t, result.UpdateAvail)
}

func TestUpdater_Check_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // Close immediately to force connection error.

	client := &GitHubClient{
		BaseURL: srv.URL,
		Owner:   "test",
		Repo:    "test",
		HTTP:    srv.Client(),
	}

	u := &Updater{Client: client, CurrentVer: Version{Major: 1}}
	_, err := u.Check()
	require.Error(t, err)
}

func TestExtractBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := createTestArchive(t, dir)

	destPath := filepath.Join(dir, "extracted-envctl")
	err := extractBinary(archivePath, destPath)
	require.NoError(t, err)

	content, err := os.ReadFile(destPath) //nolint:gosec // test file with controlled path
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho envctl-test", string(content))
}

func TestExtractBinary_NoBinaryInArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "empty.tar.gz")

	f, err := os.Create(archivePath) //nolint:gosec // test file with controlled path
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	content := []byte("not the binary")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "other-file",
		Mode: 0o644,
		Size: int64(len(content)),
	}))
	_, err = tw.Write(content)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	destPath := filepath.Join(dir, "extracted")
	err = extractBinary(archivePath, destPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in archive")
}
