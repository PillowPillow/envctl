package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name      string
		ldflags   string
		info      *debug.BuildInfo
		ok        bool
		want      string
		goInstall bool
	}{
		{"goreleaser build", "2.1.0", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "2.1.0", false},
		{"go install tag", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v2.1.0"}}, true, "2.1.0", true},
		{"go install pseudo-version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v2.0.1-0.20261126120000-abcdef123456"}}, true, "2.0.1-0.20261126120000-abcdef123456", true},
		{"local go build stamped from vcs", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v2.0.1-0.20261126203432-5b6357b331ba+dirty"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "5b6357b331ba"}}}, true, "dev", false},
		{"local go build", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "dev", false},
		{"no build info", "dev", nil, false, "dev", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, goInstall := ResolveVersion(tc.ldflags, tc.info, tc.ok)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.goInstall, goInstall)
		})
	}
}
