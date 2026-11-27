package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const popupMargin = 1

func fade(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = dimStyle.Render(ansi.Strip(l))
	}
	return out
}

func clamped(box []string, height int) []string {
	if len(box) <= height {
		return box
	}
	return append(slices.Clone(box[:height-1]), box[len(box)-1])
}

func margined(box []string) []string {
	blank := strings.Repeat(" ", widest(box)+2*popupMargin)
	side := strings.Repeat(" ", popupMargin)
	out := []string{blank}
	for _, l := range box {
		out = append(out, side+l+side)
	}
	return append(out, blank)
}

func (m Model) overlayHints() []hint {
	switch {
	case m.dialog != nil:
		return m.dialog.hints()
	case m.top().kind == helpView:
		return m.hints()
	}
	return nil
}

func overlaid(lines []string, box []string, width, bodyTop, bodyHeight int) string {
	box = clamped(box, bodyHeight-2*popupMargin)
	return overlaidAt(lines, box, width, bodyTop, bodyHeight, max((bodyHeight-len(box))/2, popupMargin))
}

func overlaidAt(lines []string, box []string, width, bodyTop, bodyHeight, row int) string {
	height := len(lines)
	screen := append(fade(lines[:height-bottomLineHeight]), lines[height-bottomLineHeight:]...)
	box = margined(box)
	x := max((width-widest(box))/2, 0)
	y := bodyTop + max(min(row-popupMargin, bodyHeight-len(box)), 0)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(screen, "\n")),
		lipgloss.NewLayer(strings.Join(box, "\n")).X(x).Y(y).Z(1),
	).Render()
}

func hintLine(hints []hint, width int) string {
	keep := func(h hint) bool { return h.key == "enter" || h.key == "esc" }
	for {
		parts := make([]string, len(hints))
		for i, h := range hints {
			parts[i] = keyStyle.Render(h.key) + " " + dimStyle.Render(h.label)
		}
		line := outlineEdge + strings.Join(parts, dimStyle.Render(factSeparator))
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
