package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	// DefaultBaseURL is the GitHub API base URL.
	DefaultBaseURL = "https://api.github.com"
	// DefaultOwner is the GitHub repository owner.
	DefaultOwner = "PillowPillow"
	// DefaultRepo is the GitHub repository name.
	DefaultRepo = "envctl"
	// binaryName is the GoReleaser project name used in archive filenames.
	binaryName = "envctl"
)

// HTTPClient is the interface for HTTP calls (compatible with *http.Client).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// GitHubClient interacts with the GitHub API for releases.
type GitHubClient struct {
	BaseURL string
	Owner   string
	Repo    string
	Token   string // optional, from GITHUB_TOKEN env var
	HTTP    HTTPClient
}

// release represents the GitHub release API response.
type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	URL                string `json:"url"` // API URL for private repo downloads
}

// NewGitHubClient creates a GitHub client with default values.
func NewGitHubClient(httpClient HTTPClient) *GitHubClient {
	return &GitHubClient{
		BaseURL: DefaultBaseURL,
		Owner:   DefaultOwner,
		Repo:    DefaultRepo,
		Token:   os.Getenv("GITHUB_TOKEN"),
		HTTP:    httpClient,
	}
}

// FetchLatestVersion fetches the latest version from GitHub releases.
func (c *GitHubClient) FetchLatestVersion() (Version, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.BaseURL, c.Owner, c.Repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Version{}, fmt.Errorf("creating releases request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	c.setAuth(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Version{}, fmt.Errorf("releases request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Version{}, fmt.Errorf("releases API: status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Version{}, fmt.Errorf("decoding release: %w", err)
	}

	return ParseVersion(rel.TagName)
}

// fetchRelease fetches a specific release by tag from the GitHub API.
func (c *GitHubClient) fetchRelease(ver Version) (release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.BaseURL, c.Owner, c.Repo, ver.String())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return release{}, fmt.Errorf("creating release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	c.setAuth(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("release request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return release{}, fmt.Errorf("release API: status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("decoding release: %w", err)
	}

	return rel, nil
}

// findAssetURL finds the download URL for a given filename in release assets.
// For private repos with a token, it returns the API URL for authenticated download.
func (r *release) findAssetURL(filename, token string) (string, bool, error) {
	for _, a := range r.Assets {
		if a.Name == filename {
			if token != "" && a.URL != "" {
				return a.URL, true, nil
			}
			return a.BrowserDownloadURL, false, nil
		}
	}
	return "", false, fmt.Errorf("asset %q not found in release", filename)
}

// ArchiveFilename returns the expected archive filename for a given os/arch.
// Note: envctl archives do not include the version in the filename.
func ArchiveFilename(goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, goos, goarch)
}

// DownloadArchive downloads the binary archive for a given OS/arch from release assets.
func (c *GitHubClient) DownloadArchive(ver Version, goos, goarch string) (io.ReadCloser, error) {
	rel, err := c.fetchRelease(ver)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}

	filename := ArchiveFilename(goos, goarch)
	assetURL, isAPIURL, err := rel.findAssetURL(filename, c.Token)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}
	c.setAuth(req)
	if isAPIURL {
		req.Header.Set("Accept", "application/octet-stream")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading archive: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("downloading archive: status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// FetchChecksums fetches checksums.txt from release assets and returns a map[filename]sha256.
func (c *GitHubClient) FetchChecksums(ver Version) (map[string]string, error) {
	rel, err := c.fetchRelease(ver)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}

	checksumURL, isAPIURL, err := rel.findAssetURL("checksums.txt", c.Token)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, checksumURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating checksums request: %w", err)
	}
	c.setAuth(req)
	if isAPIURL {
		req.Header.Set("Accept", "application/octet-stream")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checksums request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading checksums: %w", err)
	}

	return parseChecksums(string(body)), nil
}

// parseChecksums parses the "sha256  filename\n" format.
func parseChecksums(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			result[fields[1]] = fields[0]
		}
	}
	return result
}

func (c *GitHubClient) setAuth(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}
