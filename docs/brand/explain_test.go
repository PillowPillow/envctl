package main

import (
	"fmt"
	"html"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	textElem = regexp.MustCompile(`<text[^>]*>(.*?)</text>`)
	anyTag   = regexp.MustCompile(`<[^>]+>`)
)

func textContents(svg string) []string {
	var out []string
	for _, m := range textElem.FindAllStringSubmatch(svg, -1) {
		out = append(out, html.UnescapeString(anyTag.ReplaceAllString(m[1], "")))
	}
	return out
}

func TestVerifyCardKeepsIndentation(t *testing.T) {
	svg := renderVerify(light, "a\n  b\n    c\n")
	for _, tc := range []struct {
		text   string
		indent int
	}{{"b", 2}, {"c", 4}} {
		want := fmt.Sprintf(`x="%s"`, num(termLeft+float64(tc.indent)*termCharW))
		found := false
		for _, m := range textElem.FindAllStringSubmatch(svg, -1) {
			if anyTag.ReplaceAllString(m[1], "") == tc.text && strings.Contains(m[0], want) {
				found = true
			}
		}
		if !found {
			t.Errorf("line %q is not placed at %s", tc.text, want)
		}
	}
	for _, c := range textContents(svg) {
		if strings.HasPrefix(c, " ") {
			t.Errorf("text %q starts with a space the browser would collapse", c)
		}
	}
}

func TestVerifyCardRendersEveryLine(t *testing.T) {
	transcript := readFile(t, "verify.txt")
	got := strings.Join(textContents(readFile(t, filepath.Join(generated(t), "verify-light.svg"))), "\n")
	for _, line := range strings.Split(transcript, "\n") {
		if line = strings.TrimLeft(line, " "); line != "" && !strings.Contains(got, line) {
			t.Errorf("terminal card lacks %q", line)
		}
	}
}

func TestVerifyCardHasNoUsageBlock(t *testing.T) {
	if strings.Contains(readFile(t, "verify.txt"), "Usage:") {
		t.Error("verify.txt still carries the cobra usage block")
	}
}

func TestAccessMatrixMatchesSpec(t *testing.T) {
	want := []grant{
		{"alice", "lead · global key", [3]bool{true, true, true}},
		{"bob", "developer · --env local", [3]bool{true, false, false}},
		{"ci", "CI runner · --env staging, production", [3]bool{false, true, true}},
		{"ops", "ops · --env production", [3]bool{false, false, true}},
	}
	if !reflect.DeepEqual(accessMatrix, want) {
		t.Errorf("accessMatrix = %v", accessMatrix)
	}
	svg := readFile(t, filepath.Join(generated(t), "access-light.svg"))
	if n := strings.Count(svg, `class="open"`); n != 7 {
		t.Errorf("%d open seals, want 7", n)
	}
	if n := strings.Count(svg, `class="closed"`); n != 5 {
		t.Errorf("%d closed rings, want 5", n)
	}
}

func TestClassesShowTheThreeSyntaxes(t *testing.T) {
	got := textContents(readFile(t, filepath.Join(generated(t), "classes-light.svg")))
	for _, want := range []string{"VAR=value", "VAR=${VAR}", "VAR=${VAR:-default}"} {
		if !slices.Contains(got, want) {
			t.Errorf("classes visual lacks %q", want)
		}
	}
}
