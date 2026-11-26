package main

func renderLogo(t theme, _ inputs) string {
	return svgDoc(256, 256, "envctl logo", card(t, 256, 256, 48)+mark(t, 28, 28, 200))
}

func renderMarkOnly(size int) func(theme, inputs) string {
	return func(t theme, _ inputs) string {
		return svgDoc(size, size, "envctl", mark(t, 0, 0, float64(size)))
	}
}
