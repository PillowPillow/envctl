package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.Handler) (*GitHubClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &GitHubClient{
		BaseURL: srv.URL,
		Owner:   "testowner",
		Repo:    "testrepo",
		HTTP:    srv.Client(),
	}, srv
}

func TestGitHubClient_FetchLatestVersion(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/testowner/testrepo/releases/latest", r.URL.Path)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v1.2.3"})
	}))

	ver, err := client.FetchLatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", ver.String())
}

func TestGitHubClient_FetchLatestVersion_HTTPError(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	_, err := client.FetchLatestVersion()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestGitHubClient_FetchLatestVersion_WithToken(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v2.0.0"})
	}))
	client.Token = "my-token"

	ver, err := client.FetchLatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", ver.String())
}

func TestGitHubClient_DownloadArchive(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/repos/testowner/testrepo/releases/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{
			TagName: "v1.0.0",
			Assets: []asset{
				{
					Name:               "envctl_darwin_arm64.tar.gz",
					BrowserDownloadURL: srvURL + "/download/envctl_darwin_arm64.tar.gz",
				},
			},
		})
	})

	mux.HandleFunc("/download/envctl_darwin_arm64.tar.gz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fake-archive-content"))
	})

	client, srv := newTestClient(t, mux)
	srvURL = srv.URL

	body, err := client.DownloadArchive(Version{Major: 1}, "darwin", "arm64")
	require.NoError(t, err)
	defer func() { _ = body.Close() }()

	data := make([]byte, 100)
	n, _ := body.Read(data)
	assert.Equal(t, "fake-archive-content", string(data[:n]))
}

func TestGitHubClient_DownloadArchive_AssetNotFound(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v1.0.0", Assets: []asset{}})
	}))

	_, err := client.DownloadArchive(Version{Major: 1}, "darwin", "arm64")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGitHubClient_DownloadArchive_PrivateRepo(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/repos/testowner/testrepo/releases/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{
			TagName: "v1.0.0",
			Assets: []asset{
				{
					Name:               "envctl_linux_amd64.tar.gz",
					BrowserDownloadURL: srvURL + "/download/public",
					URL:                srvURL + "/api/assets/123",
				},
			},
		})
	})

	mux.HandleFunc("/api/assets/123", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/octet-stream", r.Header.Get("Accept"))
		assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("private-content"))
	})

	client, srv := newTestClient(t, mux)
	srvURL = srv.URL
	client.Token = "secret-token"

	body, err := client.DownloadArchive(Version{Major: 1}, "linux", "amd64")
	require.NoError(t, err)
	defer func() { _ = body.Close() }()

	data := make([]byte, 100)
	n, _ := body.Read(data)
	assert.Equal(t, "private-content", string(data[:n]))
}

func TestGitHubClient_FetchChecksums(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/repos/testowner/testrepo/releases/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{
			TagName: "v1.0.0",
			Assets: []asset{
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: srvURL + "/download/checksums.txt",
				},
			},
		})
	})

	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("abc123  envctl_darwin_arm64.tar.gz\ndef456  envctl_linux_amd64.tar.gz\n"))
	})

	client, srv := newTestClient(t, mux)
	srvURL = srv.URL

	checksums, err := client.FetchChecksums(Version{Major: 1})
	require.NoError(t, err)
	assert.Equal(t, "abc123", checksums["envctl_darwin_arm64.tar.gz"])
	assert.Equal(t, "def456", checksums["envctl_linux_amd64.tar.gz"])
}

func TestParseChecksums(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			"normal",
			"abc123  file1.tar.gz\ndef456  file2.tar.gz\n",
			map[string]string{"file1.tar.gz": "abc123", "file2.tar.gz": "def456"},
		},
		{
			"empty",
			"",
			map[string]string{},
		},
		{
			"malformed line",
			"abc123  file1.tar.gz\nbadline\ndef456  file2.tar.gz\n",
			map[string]string{"file1.tar.gz": "abc123", "file2.tar.gz": "def456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseChecksums(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArchiveFilename(t *testing.T) {
	assert.Equal(t, "envctl_darwin_arm64.tar.gz", ArchiveFilename("darwin", "arm64"))
	assert.Equal(t, "envctl_linux_amd64.tar.gz", ArchiveFilename("linux", "amd64"))
}
