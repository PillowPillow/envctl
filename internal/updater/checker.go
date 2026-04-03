package updater

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	checkFile      = "envctl-last-update-check"
	throttlePeriod = 24 * time.Hour
	defaultTimeout = 2 * time.Second
)

// Checker verifies in the background if a new version is available.
type Checker struct {
	Client     *GitHubClient
	ConfigDir  string // ~/.envctl/
	CurrentVer Version
	Timeout    time.Duration
}

// NewChecker creates a Checker with default values.
func NewChecker(client *GitHubClient, configDir, currentVersion string) *Checker {
	// Zero-value Version on parse failure means any valid release will be
	// considered newer, which is the safe default for update checking.
	ver, _ := ParseVersion(currentVersion)
	return &Checker{
		Client:     client,
		ConfigDir:  configDir,
		CurrentVer: ver,
		Timeout:    defaultTimeout,
	}
}

// ShouldCheck returns true if enough time has elapsed since the last check.
func (c *Checker) ShouldCheck() bool {
	path := filepath.Join(c.ConfigDir, checkFile)
	data, err := os.ReadFile(path) //nolint:gosec // path is from known config dir
	if err != nil {
		return true
	}

	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) == 0 {
		return true
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil {
		return true
	}

	lastCheck := time.Unix(ts, 0)
	return time.Since(lastCheck) >= throttlePeriod
}

// RunBackground launches the check in a goroutine and returns a channel that
// will receive a notification message if a new version is available.
func (c *Checker) RunBackground() <-chan string {
	ch := make(chan string, 1)

	if !c.ShouldCheck() {
		if msg := c.cachedNotice(); msg != "" {
			ch <- msg
		}
		close(ch)
		return ch
	}

	go func() {
		defer close(ch)

		httpClient := &http.Client{Timeout: c.Timeout}
		client := &GitHubClient{
			BaseURL: c.Client.BaseURL,
			Owner:   c.Client.Owner,
			Repo:    c.Client.Repo,
			Token:   c.Client.Token,
			HTTP:    httpClient,
		}

		latest, err := client.FetchLatestVersion()
		if err != nil {
			// Throttle only — do not cache a bogus version.
			c.writeTimestamp("")
			return
		}

		c.writeTimestamp(latest.String())

		if !latest.NewerThan(c.CurrentVer) {
			return
		}

		ch <- fmt.Sprintf(
			"A new version of envctl is available: %s → %s\nRun 'envctl update' to upgrade.",
			c.CurrentVer.String(), latest.String(),
		)
	}()

	return ch
}

// cachedNotice checks the cache file for a previously discovered newer version.
func (c *Checker) cachedNotice() string {
	path := filepath.Join(c.ConfigDir, checkFile)
	data, err := os.ReadFile(path) //nolint:gosec // path is from known config dir
	if err != nil {
		return ""
	}

	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) < 2 {
		return ""
	}

	cached, err := ParseVersion(strings.TrimSpace(lines[1]))
	if err != nil {
		return ""
	}

	if cached.NewerThan(c.CurrentVer) {
		return fmt.Sprintf(
			"A new version of envctl is available: %s → %s\nRun 'envctl update' to upgrade.",
			c.CurrentVer.String(), cached.String(),
		)
	}
	return ""
}

// writeTimestamp writes the throttle file with the current timestamp and version.
func (c *Checker) writeTimestamp(latestVersion string) {
	path := filepath.Join(c.ConfigDir, checkFile)
	content := fmt.Sprintf("%d\n%s\n", time.Now().Unix(), latestVersion)
	// Best-effort: throttle file is non-critical — failure just means the
	// next check won't be throttled.
	_ = os.MkdirAll(c.ConfigDir, 0o750)
	_ = os.WriteFile(path, []byte(content), 0o644)
}

// ResetCheckTimer resets the update check timer so the next check happens after throttlePeriod.
// Call this after a successful manual update.
func ResetCheckTimer(configDir, currentVersion string) {
	path := filepath.Join(configDir, checkFile)
	content := fmt.Sprintf("%d\n%s\n", time.Now().Unix(), currentVersion)
	_ = os.MkdirAll(configDir, 0o750)
	_ = os.WriteFile(path, []byte(content), 0o644)
}
