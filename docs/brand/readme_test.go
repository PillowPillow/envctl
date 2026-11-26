package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	pictureBlock = regexp.MustCompile(`(?s)<picture>(.*?)</picture>`)
	darkSource   = regexp.MustCompile(`<source media="\(prefers-color-scheme: dark\)" srcset="docs/brand/([a-z0-9-]+)-dark\.svg">`)
	lightImg     = regexp.MustCompile(`<img src="docs/brand/([a-z0-9-]+)-light\.svg"[^>]* alt="[^"]+"`)
	mdLink       = regexp.MustCompile(`\]\(([^)#\s]+)(?:#[^)]*)?\)`)
	htmlRef      = regexp.MustCompile(`(?:src|srcset|href)="([^"#]+)"`)
)

const repoRoot = "../.."

func TestReadmePicturesResolve(t *testing.T) {
	readme := readFile(t, filepath.Join(repoRoot, "README.md"))
	pictures := pictureBlock.FindAllStringSubmatch(readme, -1)
	if len(pictures) == 0 {
		t.Fatal("README has no <picture>")
	}
	for _, p := range pictures {
		dark, light := darkSource.FindStringSubmatch(p[1]), lightImg.FindStringSubmatch(p[1])
		if dark == nil || light == nil {
			t.Errorf("picture lacks a dark <source> or a light <img> with alt: %s", p[1])
			continue
		}
		if dark[1] != light[1] {
			t.Errorf("picture pairs %s-dark with %s-light", dark[1], light[1])
		}
		for _, f := range []string{dark[1] + "-dark.svg", light[1] + "-light.svg"} {
			if _, err := os.Stat(f); err != nil {
				t.Errorf("picture points at missing docs/brand/%s", f)
			}
		}
	}
}

func TestReadmeLinksResolve(t *testing.T) {
	for _, doc := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md"} {
		body := readFile(t, filepath.Join(repoRoot, doc))
		var targets []string
		for _, re := range []*regexp.Regexp{mdLink, htmlRef} {
			for _, m := range re.FindAllStringSubmatch(body, -1) {
				targets = append(targets, m[1])
			}
		}
		for _, target := range targets {
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, err := os.Stat(filepath.Join(repoRoot, target)); err != nil {
				t.Errorf("%s links to missing %s", doc, target)
			}
		}
	}
}
