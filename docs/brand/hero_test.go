package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	heroTitle     = "Your repo holds every secret it needs. One key opens it."
	heroSubtitle1 = "Encrypted in git, no external vault. Each key opens only the environments it should."
	heroSubtitle2 = "Your .env.example is the contract, and CI enforces it."
)

func TestHeroCarriesTheApprovedCopy(t *testing.T) {
	body := readFile(t, filepath.Join(generated(t), "hero-light.svg"))
	for _, want := range []string{"envctl", heroTitle, heroSubtitle1, heroSubtitle2} {
		if !strings.Contains(body, ">"+esc(want)+"<") {
			t.Errorf("hero lacks %q", want)
		}
	}
}

func TestSocialIs1280x640WithDescriptor(t *testing.T) {
	body := readFile(t, filepath.Join(generated(t), "social-light.svg"))
	if !strings.Contains(body, `viewBox="0 0 1280 640"`) {
		t.Error("social preview is not 1280x640")
	}
	if !strings.Contains(body, "the vault that lives in your repo") {
		t.Error("social preview lacks the descriptor")
	}
}

func TestTextFitsWithMargin(t *testing.T) {
	layouts := []struct {
		name  string
		width float64
		lines []textLine
	}{
		{"hero", heroW, heroLines},
		{"social", socialW, socialLines},
	}
	for _, l := range layouts {
		for _, line := range l.lines {
			if end := line.x + textWidth(line.text, line.size, line.mono); end > l.width-24 {
				t.Errorf("%s: %q ends at %.0f, past %.0f", l.name, line.text, end, l.width-24)
			}
		}
	}
}
