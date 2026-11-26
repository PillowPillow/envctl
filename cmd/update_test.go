package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/updater"
)

type failingHTTPClient struct{ t *testing.T }

func (f failingHTTPClient) Do(*http.Request) (*http.Response, error) {
	f.t.Fatal("update must not reach the network after go install")
	return nil, nil
}

func TestRunUpdate_GoInstallPrintsHintWithoutNetwork(t *testing.T) {
	prevVersion, prevGo, prevNoColor, prevOut := appVersion, installedWithGo, color.NoColor, color.Output
	t.Cleanup(func() {
		appVersion, installedWithGo, color.NoColor, color.Output = prevVersion, prevGo, prevNoColor, prevOut
	})
	appVersion, installedWithGo, color.NoColor = "2.1.0", true, true

	r, w, err := os.Pipe()
	require.NoError(t, err)
	prevStdout := os.Stdout
	os.Stdout, color.Output = w, w
	t.Cleanup(func() { os.Stdout = prevStdout })

	err = runUpdateWith(&updater.GitHubClient{BaseURL: "http://127.0.0.1:0", Owner: "o", Repo: "r", HTTP: failingHTTPClient{t}})
	require.NoError(t, w.Close())
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "go install github.com/PillowPillow/envctl/v2@latest")
}
