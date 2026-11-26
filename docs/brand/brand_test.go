package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generated(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := generate(".", dir); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return dir
}

func svgFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no SVG generated")
	}
	return files
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test reads files it generated
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGenerateWritesLightAndDarkForEveryVisual(t *testing.T) {
	dir := generated(t)
	for _, v := range visuals {
		for _, th := range themes {
			path := filepath.Join(dir, v.stem+"-"+th.name+".svg")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("missing %s", filepath.Base(path))
			}
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := generated(t), generated(t)
	for _, f := range svgFiles(t, a) {
		other := filepath.Join(b, filepath.Base(f))
		if !bytes.Equal([]byte(readFile(t, f)), []byte(readFile(t, other))) {
			t.Errorf("%s differs between runs", filepath.Base(f))
		}
	}
}

func TestEverySVGIsWellFormed(t *testing.T) {
	for _, f := range svgFiles(t, generated(t)) {
		dec := xml.NewDecoder(strings.NewReader(readFile(t, f)))
		root := ""
		for {
			tok, err := dec.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Errorf("%s: %v", filepath.Base(f), err)
				break
			}
			if se, ok := tok.(xml.StartElement); ok && root == "" {
				root = se.Name.Local
			}
		}
		if root != "svg" {
			t.Errorf("%s: root element %q, want svg", filepath.Base(f), root)
		}
	}
}

func TestThemesDoNotLeak(t *testing.T) {
	colors := func(th theme) []string {
		return []string{th.bg, th.border, th.text, th.secondary, th.accent, th.success}
	}
	dir := generated(t)
	for _, f := range svgFiles(t, dir) {
		body := strings.ToUpper(readFile(t, f))
		other := light
		if strings.HasSuffix(f, "-light.svg") {
			other = dark
		}
		for _, c := range colors(other) {
			if strings.Contains(body, strings.ToUpper(c)) && !sharedTerminalColor(f, c) {
				t.Errorf("%s contains %s from the %s theme", filepath.Base(f), c, other.name)
			}
		}
	}
}

func TestEscEscapesMarkup(t *testing.T) {
	got := esc(`a & <b> "c" 'd'`)
	want := "a &amp; &lt;b&gt; &#34;c&#34; &#39;d&#39;"
	if got != want {
		t.Errorf("esc = %q, want %q", got, want)
	}
}

func TestSmallMarkDropsKeyhole(t *testing.T) {
	small := mark(light, 0, 0, 16)
	if n := strings.Count(small, "<circle"); n != 1 {
		t.Errorf("16px mark has %d circles, want 1", n)
	}
	if strings.Contains(small, "<rect") {
		t.Error("16px mark keeps the keyhole stem")
	}
	big := mark(light, 0, 0, 64)
	if strings.Count(big, "<circle") != 2 || strings.Count(big, "<rect") != 1 {
		t.Errorf("64px mark lacks the keyhole: %s", big)
	}
}

func sharedTerminalColor(file, _ string) bool {
	return strings.HasPrefix(filepath.Base(file), "verify-")
}
