package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func fade(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = dimStyle.Render(ansi.Strip(l))
	}
	return out
}

func withMargin(box []string) []string {
	blank := strings.Repeat(" ", widest(box)+2)
	out := []string{blank}
	for _, l := range box {
		out = append(out, " "+l+" ")
	}
	return append(out, blank)
}

func (m Model) overlayBox(width, height int) []string {
	switch {
	case m.dialog != nil:
		return m.dialog.view(width)
	case m.top().kind == helpView:
		return m.renderHelp(width, height)
	}
	return nil
}

func (m Model) overlayHints() []hint {
	switch {
	case m.dialog != nil:
		return m.dialog.hints()
	case m.top().kind == helpView:
		return []hint{{"j/k", "scroll"}, {"esc", "close"}}
	}
	return nil
}

func (m Model) beneathOverlay() Model {
	if m.top().kind == helpView {
		return m.pop()
	}
	return m
}

func overlaid(lines []string, box []string, width, height int) string {
	screen := append(fade(lines[:height-bottomLineHeight]), lines[height-bottomLineHeight:]...)
	boxed := withMargin(box)
	x := max((width-widest(boxed))/2, 0)
	y := max((height-bottomLineHeight-len(boxed))/2, 0)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(screen, "\n")),
		lipgloss.NewLayer(strings.Join(boxed, "\n")).X(x).Y(y).Z(1),
	).Render()
}

func hintLine(hints []hint, width int) string {
	keep := func(h hint) bool { return h.key == "enter" || h.key == "esc" }
	for {
		parts := make([]string, len(hints))
		for i, h := range hints {
			parts[i] = keyStyle.Render(h.key) + " " + dimStyle.Render(h.label)
		}
		line := gridEdge + strings.Join(parts, dimStyle.Render(factSeparator))
		drop := -1
		for i, h := range hints {
			if !keep(h) {
				drop = i
			}
		}
		if ansi.StringWidth(line) <= width || drop < 0 {
			return line
		}
		hints = slices.Delete(slices.Clone(hints), drop, drop+1)
	}
}
