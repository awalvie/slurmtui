package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) unknownKey(key string) Model {
	return m.say(flashWarning, keyName(key)+" does nothing here · ? lists the keys")
}

func keyName(key string) string {
	return strings.ReplaceAll(key, "+", "-")
}

func (m Model) filterKey(msg tea.KeyPressMsg) Model {
	f := m.current()
	switch msg.String() {
	case "enter":
		m.filtering = false
		return m
	case "esc":
		m.filtering = false
		f.filter = m.filterBefore
	case "backspace":
		f.filter = dropLastRune(f.filter)
	default:
		f.filter += msg.Text
	}
	return m.withCurrent(f)
}

func (m Model) body(width, height int) ([]string, error) {
	f := m.top()
	if isDetail(f.kind) {
		return m.detailPane(f, width, height), nil
	}
	if f.kind == overviewView {
		return m.overview(width, height)
	}
	t := m.table(f)
	return m.paneOf(f, t, true).render(width, height), t.err
}
