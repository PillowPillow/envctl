package main

import (
	"fmt"
	"strings"
)

func label(x, y, size float64, fill, anchor, family, text string, bold bool) string {
	extra := ""
	if anchor != "" {
		extra += fmt.Sprintf(` text-anchor="%s"`, anchor)
	}
	if bold {
		extra += ` font-weight="700"`
	}
	return fmt.Sprintf(`<text x="%s" y="%s" font-family="%s" font-size="%s" fill="%s"%s>%s</text>`,
		num(x), num(y), family, num(size), fill, extra, text)
}

func braces(stroke, width string) string {
	return fmt.Sprintf(`<g fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"><path d="%s"/><path d="%s"/></g>`,
		stroke, width, braceLeft, braceRight)
}

func dots(x, y float64, n int, fill string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="4" fill="%s"/>`, num(x+float64(i)*12), num(y), fill)
	}
	return b.String()
}

const flowW, flowH = 820, 210

func renderFlow(t theme, _ inputs) string {
	var b strings.Builder
	box := func(x float64, fill, stroke string) {
		fmt.Fprintf(&b, `<rect x="%s" y="20.5" width="230" height="150" rx="12" fill="%s" stroke="%s"/>`, num(x), fill, stroke)
	}
	box(0.5, t.bg, t.border)
	b.WriteString(label(16, 46, 13, t.text, "", monoStack, ".env.example", true))
	b.WriteString(label(16, 74, 12, t.secondary, "", monoStack, "APP_ENV=production", false))
	b.WriteString(label(16, 96, 12, t.text, "", monoStack, "DB_URL="+tspan(t.accent, "${DB_URL}", false), false))
	b.WriteString(label(16, 118, 12, t.text, "", monoStack, "PORT="+tspan(t.accent, "${PORT:-8080}", false), false))
	b.WriteString(label(16, 154, 11, t.secondary, "", sansStack, "what exists · committed", false))

	box(295.5, t.bg, t.border)
	b.WriteString(label(311, 46, 13, t.text, "", monoStack, ".envctl/*.enc.yaml", true))
	b.WriteString(label(311, 74, 12, t.text, "", monoStack, "DB_URL:", false))
	b.WriteString(dots(378, 70, 5, t.accent))
	b.WriteString(label(311, 96, 12, t.text, "", monoStack, "PORT:", false))
	b.WriteString(dots(364, 92, 3, t.accent))
	b.WriteString(label(311, 154, 11, t.secondary, "", sansStack, "encrypted values · committed", false))

	// The generated .env is drawn inverted so it reads as output in both themes.
	box(589.5, t.text, t.text)
	b.WriteString(label(606, 46, 13, t.bg, "", monoStack, ".env", true))
	b.WriteString(label(606, 74, 12, t.border, "", monoStack, "APP_ENV=production", false))
	b.WriteString(label(606, 96, 12, t.bg, "", monoStack, "DB_URL=postgres://…", false))
	b.WriteString(label(606, 118, 12, t.bg, "", monoStack, "PORT=9000", false))
	b.WriteString(label(606, 154, 11, t.border, "", sansStack, "generated · gitignored", false))

	fmt.Fprintf(&b, `<g stroke="%s" stroke-width="2" fill="none"><path d="M236 95 H288"/><path d="M281 89 L288 95 L281 101"/><path d="M531 95 H583"/><path d="M576 89 L583 95 L576 101"/></g>`, t.text)
	fmt.Fprintf(&b, `<g stroke="%s" stroke-width="1.5" stroke-dasharray="3 3"><path d="M262 100 V184"/><path d="M557 100 V184"/></g>`, t.accent)
	b.WriteString(label(262, 200, 12, t.accent, "middle", monoStack, "verify", true))
	b.WriteString(label(557, 200, 12, t.accent, "middle", monoStack, "generate", true))
	return svgDoc(flowW, flowH, ".env.example lists the variables, .envctl/*.enc.yaml holds encrypted values, generate writes .env", b.String())
}

const classesW, classesH = 820, 140

func renderClasses(t theme, _ inputs) string {
	var b strings.Builder
	b.WriteString(card(t, classesW, classesH, 12))
	fmt.Fprintf(&b, `<g transform="translate(110 14) scale(.5)"><rect x="22" y="44" width="56" height="12" rx="6" fill="%s"/></g>`, t.secondary)
	fmt.Fprintf(&b, `<g transform="translate(385 14) scale(.5)">%s<circle cx="50" cy="50" r="15" fill="%s"/></g>`, braces(t.text, "7"), t.accent)
	fmt.Fprintf(&b, `<g transform="translate(660 14) scale(.5)">%s<circle cx="50" cy="50" r="13" fill="none" stroke="%s" stroke-width="5"/><path d="M50 37 A13 13 0 0 1 50 63 Z" fill="%s"/></g>`,
		braces(t.text, "7"), t.accent, t.accent)
	for _, c := range []struct {
		x            float64
		syntax, note string
	}{
		{135, "VAR=value", "static · kept as written"},
		{410, "VAR=${VAR}", "required · verify fails without it"},
		{685, "VAR=${VAR:-default}", "optional · falls back to the default"},
	} {
		b.WriteString(label(c.x, 94, 13, t.text, "middle", monoStack, esc(c.syntax), false))
		b.WriteString(label(c.x, 116, 12, t.secondary, "middle", sansStack, esc(c.note), false))
	}
	return svgDoc(classesW, classesH, "Three kinds of variable: static, required and optional", b.String())
}

type grant struct {
	key, note string
	envs      [3]bool
}

var accessMatrix = []grant{
	{"alice", "lead · global key", [3]bool{true, true, true}},
	{"bob", "developer · --env local", [3]bool{true, false, false}},
	{"ci", "CI runner · --env staging, production", [3]bool{false, true, true}},
	{"ops", "ops · --env production", [3]bool{false, false, true}},
}

const accessW, accessH = 820, 290

func renderAccess(t theme, _ inputs) string {
	var b strings.Builder
	b.WriteString(card(t, accessW, accessH, 12))
	cols := [3]float64{450, 570, 690}
	for i, env := range []string{"local", "staging", "production"} {
		b.WriteString(label(cols[i], 42, 13, t.text, "middle", monoStack, env, true))
	}
	for r, g := range accessMatrix {
		y := 86 + float64(r)*46
		if r > 0 {
			fmt.Fprintf(&b, `<path d="M30 %s H790" stroke="%s"/>`, num(y-23), t.border)
		}
		b.WriteString(label(36, y+4, 13, t.text, "", monoStack, esc(g.key), true))
		b.WriteString(label(110, y+4, 12, t.secondary, "", sansStack, esc(g.note), false))
		for i, open := range g.envs {
			cx := cols[i]
			if open {
				fmt.Fprintf(&b, `<g class="open"><circle cx="%s" cy="%s" r="12" fill="%s"/><circle cx="%s" cy="%s" r="3" fill="%s"/><rect x="%s" y="%s" width="3" height="7" rx="1" fill="%s"/></g>`,
					num(cx), num(y), t.accent, num(cx), num(y-3), t.bg, num(cx-1.5), num(y-1), t.bg)
			} else {
				fmt.Fprintf(&b, `<g class="closed"><circle cx="%s" cy="%s" r="11" fill="none" stroke="%s" stroke-width="1.5"/></g>`,
					num(cx), num(y), t.secondary)
			}
		}
	}
	b.WriteString(label(410, 272, 12, t.secondary, "middle", sansStack,
		esc("● can decrypt   ○ cannot — a global key opens every environment, an --env key opens only its own"), false))
	return svgDoc(accessW, accessH, "Who can open what: alice opens every environment, bob only local, ci staging and production, ops only production", b.String())
}
