package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/PillowPillow/envctl/v2/internal/sopskeys"
	"github.com/PillowPillow/envctl/v2/internal/sopstest"
)

type keyPair struct {
	Pub     string
	KeyFile string
}

type project struct {
	Dir            string
	ConfigPath     string
	LocalSecrets   string
	ProdSecrets    string
	CI             keyPair
	Nicolas        keyPair
	OriginalConfig []byte
}

func resetFlags() {
	cfgFile = ""
	envName = ""
	var walk func(c *cobra.Command)
	reset := func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(reset)
		c.PersistentFlags().VisitAll(reset)
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetFlags()

	outR, outW, perr := os.Pipe()
	require.NoError(t, perr)
	errR, errW, perr := os.Pipe()
	require.NoError(t, perr)

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	outCh, errCh := make(chan string), make(chan string)
	drain := func(r *os.File, ch chan<- string) {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		ch <- buf.String()
	}
	go drain(outR, outCh)
	go drain(errR, errCh)

	rootCmd.SetArgs(args)
	err = rootCmd.Execute()

	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	stdout, stderr = <-outCh, <-errCh
	return stdout, stderr, err
}

func setupProject(t *testing.T) project {
	t.Helper()
	sopstest.RequireTools(t)

	dir := t.TempDir()
	ciPub, ciKey := sopstest.NewKey(t)
	nicPub, nicKey := sopstest.NewKey(t)

	cfg := []byte("version: \"1\"\ntemplate: .env.example\nenvironments:\n" +
		"  local:\n    secrets: .envctl/local.enc.yaml\n    output: .env\n" +
		"  production:\n    secrets: .envctl/production.enc.yaml\n    output: .env.production\n" +
		"sops:\n  age_recipients:\n    ci: " + ciPub + "\n    nicolas: " + nicPub + "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envctl.yaml"), cfg, 0o644))                                                      //nolint:gosec // test file
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte("API_URL=https://example.test\nTOKEN=${TOKEN}\n"), 0o644)) //nolint:gosec // test file
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".envctl"), 0o755))                                                                 //nolint:gosec // test dir

	t.Setenv("SOPS_AGE_KEY_FILE", ciKey)
	sopsPath, err := exec.LookPath("sops")
	require.NoError(t, err)

	p := project{
		Dir:            dir,
		ConfigPath:     filepath.Join(dir, ".envctl.yaml"),
		LocalSecrets:   filepath.Join(dir, ".envctl", "local.enc.yaml"),
		ProdSecrets:    filepath.Join(dir, ".envctl", "production.enc.yaml"),
		CI:             keyPair{ciPub, ciKey},
		Nicolas:        keyPair{nicPub, nicKey},
		OriginalConfig: cfg,
	}
	for _, f := range []string{p.LocalSecrets, p.ProdSecrets} {
		require.NoError(t, os.WriteFile(f, []byte("TOKEN: dummy-value\n"), 0o600))
		require.NoError(t, sopskeys.Encrypt(sopsPath, f, []string{ciPub, nicPub}))
	}
	t.Chdir(dir)
	return p
}
