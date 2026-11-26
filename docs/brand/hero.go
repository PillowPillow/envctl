package main

import (
	"fmt"
	"strings"
)

const (
	monoStack = `ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`
	sansStack = `-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif`
)

type textLine struct {
	text      string
	x, y      float64
	size      float64
	mono      bool
	bold      bool
	secondary bool
}

// textWidth over-estimates rendered width so wider fallback fonts (DejaVu on Linux) still fit.
func textWidth(s string, px float64, mono bool) float64 {
	factor := 0.56
	if mono {
		factor = 0.62
	}
	return float64(len([]rune(s))) * px * factor
}

func (l textLine) svg(t theme) string {
	family, fill, weight := sansStack, t.text, "400"
	if l.mono {
		family = monoStack
	}
	if l.secondary {
		fill = t.secondary
	}
	if l.bold {
		weight = "600"
	}
	return fmt.Sprintf(`<text x="%s" y="%s" font-family="%s" font-size="%s" font-weight="%s" fill="%s">%s</text>`,
		num(l.x), num(l.y), family, num(l.size), weight, fill, esc(l.text))
}

func lines(t theme, ls []textLine) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.svg(t))
	}
	return b.String()
}

const (
	heroW, heroH     = 1040, 240
	socialW, socialH = 1280, 640
)

var heroLines = []textLine{
	{text: "envctl", x: 232, y: 86, size: 46, mono: true, bold: true},
	{text: "Your repo holds every secret it needs. One key opens it.", x: 232, y: 132, size: 24, bold: true},
	{text: "Encrypted in git, no external vault. Each key opens only the environments it should.", x: 232, y: 168, size: 16, secondary: true},
	{text: "Your .env.example is the contract, and CI enforces it.", x: 232, y: 192, size: 16, secondary: true},
}

var socialLines = []textLine{
	{text: "envctl", x: 420, y: 318, size: 104, mono: true, bold: true},
	{text: "the vault that lives in your repo.", x: 424, y: 392, size: 40, secondary: true},
}

func renderHero(t theme, _ inputs) string {
	return svgDoc(heroW, heroH, "envctl — Your repo holds every secret it needs. One key opens it.",
		card(t, heroW, heroH, 18)+mark(t, 44, 45, 150)+lines(t, heroLines))
}

func renderSocial(t theme, _ inputs) string {
	bg := fmt.Sprintf(`<rect width="%d" height="%d" fill="%s"/>`, socialW, socialH, t.bg)
	return svgDoc(socialW, socialH, "envctl — the vault that lives in your repo.",
		bg+mark(t, 150, 200, 240)+lines(t, socialLines))
}
