package main

import (
	"fmt"
	"strings"
)

const (
	termFont  = 14.0
	termLineH = 21.0
	termPad   = 16.0
	termLeft  = termPad + 24
	termCharW = termFont * 0.62
	termW     = 600.0
)

func tspan(fill, text string, bold bool) string {
	weight := ""
	if bold {
		weight = ` font-weight="700"`
	}
	return fmt.Sprintf(`<tspan fill="%s"%s>%s</tspan>`, fill, weight, esc(text))
}

// termLine colours one trimmed line of verify output the way a colour terminal shows it.
func termLine(s string) string {
	switch {
	case strings.HasPrefix(s, "$ "):
		return tspan(dark.secondary, "$", false) + esc(s[1:])
	case strings.HasPrefix(s, "FAIL "):
		return tspan(dark.accent, "FAIL", true) + esc(s[4:])
	case strings.HasPrefix(s, "OK "):
		return tspan(dark.success, "OK", true) + esc(s[2:])
	case strings.HasPrefix(s, "✓ "):
		name, src, _ := strings.Cut(s[len("✓ "):], " ")
		out := tspan(dark.success, "✓", false) + " " + esc(name)
		if src != "" {
			out += " " + tspan(dark.secondary, src, false)
		}
		return out
	case strings.HasPrefix(s, "✗ "):
		return tspan(dark.accent, "✗", false) + " " + tspan(dark.text, s[len("✗ "):], true)
	case strings.HasPrefix(s, "Error:"), strings.Contains(s, "missing required secret"):
		return tspan(dark.accent, s, false)
	case strings.HasPrefix(s, "Found "):
		return tspan(dark.secondary, s, false)
	}
	return esc(s)
}

func renderVerify(t theme, transcript string) string {
	rows := strings.Split(strings.TrimRight(transcript, "\n"), "\n")
	winH := 44 + float64(len(rows))*termLineH + 10
	w, h := int(termW+2*termPad), int(winH+2*termPad)

	var b strings.Builder
	fmt.Fprintf(&b, `<rect x="0.5" y="0.5" width="%d" height="%d" rx="18" fill="%s" stroke="%s"/>`, w-1, h-1, t.bg, t.border)
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="12" fill="%s" stroke="%s"/>`,
		num(termPad), num(termPad), num(termW), num(winH), dark.bg, dark.border)
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="5.5" fill="%s"/>`, num(termPad+22+float64(i)*18), num(termPad+20), dark.border)
	}
	fmt.Fprintf(&b, `<g font-family="%s" font-size="%s" fill="%s">`, monoStack, num(termFont), dark.text)
	for i, row := range rows {
		text := strings.TrimLeft(row, " ")
		if text == "" {
			continue
		}
		x := termLeft + float64(len(row)-len(text))*termCharW
		y := termPad + 56 + float64(i)*termLineH
		fmt.Fprintf(&b, `<text x="%s" y="%s">%s</text>`, num(x), num(y), termLine(text))
	}
	b.WriteString(`</g>`)
	return svgDoc(w, h, "envctl verify --env production fails: STRIPE_WEBHOOK_SECRET has no value in production", b.String())
}

func renderVerifyVisual(t theme, in inputs) string {
	return renderVerify(t, in.verifyTranscript)
}
