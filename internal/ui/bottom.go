package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	bottomGap  = "  "
	promptHelp = "   enter keep · esc undo"
)

type flashLevel int

const (
	flashInfo flashLevel = iota
	flashWarning
	flashError
)

func (l flashLevel) style() lipgloss.Style {
	switch l {
	case flashWarning:
		return warnStyle
	case flashError:
		return badStyle
	}
	return okStyle
}

func (m Model) freshness() string {
	if m.lastOK.IsZero() {
		return dimStyle.Render("○ no data")
	}
	since := age(m.clock.Now().Sub(m.lastOK))
	if m.err != nil {
		return warnStyle.Render("○") + dimStyle.Render(" stale "+since)
	}
	return okStyle.Render("●") + dimStyle.Render(" "+since)
}

func (m Model) flashLine(problem error) (string, flashLevel) {
	switch {
	case m.flash != nil:
		return m.flash.text, m.flash.level
	case m.err != nil:
		return m.err.Error(), flashError
	case problem != nil:
		return problem.Error(), flashError
	}
	return "", flashInfo
}

func (m Model) prompt() (string, bool) {
	switch {
	case m.command != nil:
		return m.promptLine(":", *m.command, ""), true
	case m.filtering:
		return m.promptLine("/", m.current().filter, " · filtering "+specOf(m.current().kind).name), true
	}
	return "", false
}

func (m Model) promptLine(mark, text, what string) string {
	return gridEdge + accentStyle.Bold(true).Render(mark) + textStyle.Render(text) + accentStyle.Render("█") + dimStyle.Render(promptHelp+what)
}

func (m Model) bottomLine(width int, problem error) string {
	right := m.freshness() + bottomGap
	left, typing := m.prompt()
	if hints := m.overlayHints(); hints != nil {
		left, typing = hintLine(hints, max(width-ansi.StringWidth(right), 0)), true
	}
	if typing {
		return fitLeft(left, max(width-ansi.StringWidth(right), 0)) + right
	}
	text, level := m.flashLine(problem)
	text = singleLine(text)
	if text != "" {
		room := max(width-ansi.StringWidth(right)-len(bottomGap)-len(gridEdge), 0)
		right = level.style().Render(ansi.Truncate(text, room, "…")) + bottomGap + right
	}
	return fitRight(right, width)
}

func singleLine(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}
