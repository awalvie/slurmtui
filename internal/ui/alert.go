package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	actionErrorTitle  = "Error"
	refreshErrorTitle = "Refresh failed"
	okButton          = "OK"
)

type alert struct {
	title   string
	subject string
	message string
}

func (a alert) view(screen int) []string {
	width := dialogWidth(screen)
	room := width - 2 - 2*len(dialogBodyInset)
	lines := []string{"", dialogBodyInset + textStyle.Bold(true).Render(a.subject), ""}
	for _, l := range strings.Split(ansi.Wrap(a.message, room, ""), "\n") {
		lines = append(lines, dialogBodyInset+textStyle.Render(l))
	}
	lines = append(lines, "", centred(buttons([]string{okButton}, 0), width-2), "")
	return box{
		title:  badStyle.Bold(true).Render(a.title),
		width:  width,
		height: len(lines) + boxBorders,
		colour: &badStyle,
		lines:  lines,
	}.render()
}

func (m Model) alertKey(key string) Model {
	if key == "enter" || key == "esc" {
		m.alert = nil
	}
	return m
}

func (m Model) refreshFailed(err error) Model {
	if m.err == nil || m.err.Error() != err.Error() {
		m.alert = &alert{title: refreshErrorTitle, subject: m.session.URL, message: err.Error()}
	}
	m.err = err
	return m
}
