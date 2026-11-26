// Command brand regenerates the envctl visuals: go run ./docs/brand <repo-root>.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type inputs struct{}

type visual struct {
	stem   string
	render func(t theme, in inputs) string
}

var visuals = []visual{
	{"logo", renderLogo},
	{"logo-32", renderMarkOnly(32)},
	{"logo-16", renderMarkOnly(16)},
	{"hero", renderHero},
	{"social", renderSocial},
}

func generate(srcDir, outDir string) error {
	var in inputs
	if err := os.MkdirAll(outDir, 0o755); err != nil { //nolint:gosec // output dir comes from the maintainer's command line
		return err
	}
	for _, v := range visuals {
		for _, t := range themes {
			path := filepath.Join(outDir, v.stem+"-"+t.name+".svg")
			if err := os.WriteFile(path, []byte(v.render(t, in)), 0o644); err != nil { //nolint:gosec // output dir comes from the maintainer's command line
				return err
			}
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./docs/brand <repo-root>")
		os.Exit(2)
	}
	dir := filepath.Join(os.Args[1], "docs", "brand")
	if err := generate(dir, dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
