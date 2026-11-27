package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	outlineEdge     = "  "
	frameMargin     = " "
	factSeparator   = " · "
	titleRightSpace = 3
)

func cut(s string, width int) string {
	if ansi.StringWidth(s) <= width {
		return s
	}
	head := ansi.Truncate(s, width-1, "")
	if ansi.Strip(head) == head {
		head = strings.TrimRight(head, " ")
	}
	return head + "…"
}

func fitLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = cut(s, width)
	return s + strings.Repeat(" ", width-ansi.StringWidth(s))
}

func fitRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = cut(s, width)
	return strings.Repeat(" ", width-ansi.StringWidth(s)) + s
}

type box struct {
	title   string
	right   string
	width   int
	height  int
	focused bool
	colour  *lipgloss.Style
	lines   []string
}

func (b box) border() lipgloss.Style {
	switch {
	case b.colour != nil:
		return *b.colour
	case b.focused:
		return focusStyle
	}
	return dimStyle
}

func (b box) render() []string {
	border := b.border()
	inner := b.width - 2
	title := ansi.Truncate(b.title, max(b.width-4, 0), "…")
	titleWidth, rightWidth := ansi.StringWidth(title), ansi.StringWidth(b.right)
	right := b.right
	if right != "" && 2+titleWidth+1+titleRightSpace+1+rightWidth+2 > b.width {
		right, rightWidth = "", 0
	}
	top := border.Render("┌ ") + title + border.Render(" "+strings.Repeat("─", max(inner-2-titleWidth, 0))+"┐")
	if right != "" {
		top = border.Render("┌ ") + title + border.Render(" "+strings.Repeat("─", max(inner-4-titleWidth-rightWidth, 0))+" ") + right + border.Render(" ┐")
	}
	out := make([]string, 0, b.height)
	out = append(out, top)
	for i := range max(b.height-2, 0) {
		line := ""
		if i < len(b.lines) {
			line = b.lines[i]
		}
		out = append(out, border.Render("│")+fitLeft(line, inner)+border.Render("│"))
	}
	return append(out, border.Render("└"+strings.Repeat("─", max(inner, 0))+"┘"))
}

type paneTitle struct {
	name    string
	count   int
	focused bool
	scope   string
	facts   []string
	filter  string
}

func (t paneTitle) render() string {
	name := dimStyle.Bold(true)
	if t.focused {
		name = titleStyle
	}
	out := name.Render(t.name) + " " + dimStyle.Render(strconv.Itoa(t.count))
	if t.scope != "" {
		out += dimStyle.Render(factSeparator) + textStyle.Render(t.scope)
	}
	for _, fact := range t.facts {
		out += dimStyle.Render(factSeparator) + fact
	}
	if t.filter != "" {
		out += dimStyle.Render(factSeparator) + accentStyle.Render("/"+t.filter)
	}
	return out
}
