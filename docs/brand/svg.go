package main

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type theme struct {
	name, bg, border, text, secondary, accent, success string
}

var (
	light = theme{
		name: "light", bg: "#F6F3EA", border: "#E5DED3", text: "#1F1B18",
		secondary: "#5A514A", accent: "#B4472E", success: "#3F7A3A",
	}
	dark = theme{
		name: "dark", bg: "#1F1B19", border: "#3A332E", text: "#F4F0EA",
		secondary: "#BDB3A9", accent: "#E58A6B", success: "#8FBF7A",
	}
	themes = []theme{light, dark}
)

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func num(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

const (
	braceLeft  = "M30 15 C20 15 20 20 20 30 L20 42 C20 48 16 50 12 50 C16 50 20 52 20 58 L20 70 C20 80 20 85 30 85"
	braceRight = "M70 15 C80 15 80 20 80 30 L80 42 C80 48 84 50 88 50 C84 50 80 52 80 58 L80 70 C80 80 80 85 70 85"
)

// mark draws {●} from its 100-unit grid; below 32px the keyhole would blur, so it is dropped.
func mark(t theme, x, y, size float64) string {
	stroke, keyhole := "6", true
	if size < 32 {
		stroke, keyhole = "9", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<g transform="translate(%s %s) scale(%s)">`, num(x), num(y), num(size/100))
	fmt.Fprintf(&b, `<g fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"><path d="%s"/><path d="%s"/></g>`,
		t.text, stroke, braceLeft, braceRight)
	fmt.Fprintf(&b, `<circle cx="50" cy="50" r="15" fill="%s"/>`, t.accent)
	if keyhole {
		fmt.Fprintf(&b, `<circle cx="50" cy="46" r="4" fill="%s"/><rect x="48" y="48" width="4" height="9" rx="1" fill="%s"/>`, t.bg, t.bg)
	}
	b.WriteString(`</g>`)
	return b.String()
}

func svgDoc(w, h int, title, body string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img"><title>%s</title>%s</svg>`+"\n",
		w, h, w, h, esc(title), body)
}

func card(t theme, w, h int, rx float64) string {
	return fmt.Sprintf(`<rect x="0.5" y="0.5" width="%d" height="%d" rx="%s" fill="%s" stroke="%s"/>`,
		w-1, h-1, num(rx), t.bg, t.border)
}
