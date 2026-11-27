package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type confirmField struct {
	label      string
	value      string
	required   bool
	mustChange bool
}

type confirmSpec struct {
	title  string
	colour lipgloss.Style
	body   []string
	jobs   jobList
	field  *confirmField
	action string
	kept   string
	submit func(input string) tea.Cmd
}

type confirmFocus int

const (
	focusField confirmFocus = iota
	focusAction
	focusKeep
)

type confirmOutcome int

const (
	stillOpen confirmOutcome = iota
	keptAsItWas
	keptUnchanged
	submitted
)

type confirm struct {
	spec    confirmSpec
	input   string
	focus   confirmFocus
	problem string
}

const (
	keepButton      = "Keep"
	minDialogWidth  = 64
	dialogBodyInset = "  "
	buttonGap       = "    "
)

func newConfirm(spec confirmSpec) *confirm {
	c := &confirm{spec: spec, focus: focusKeep}
	if spec.field != nil {
		c.focus = focusField
		c.input = spec.field.value
	}
	return c
}

func (c confirm) focusOrder() []confirmFocus {
	if c.spec.field != nil {
		return []confirmFocus{focusField, focusAction, focusKeep}
	}
	return []confirmFocus{focusAction, focusKeep}
}

func (c confirm) moveFocus(delta int) confirm {
	order := c.focusOrder()
	at := 0
	for i, f := range order {
		if f == c.focus {
			at = i
		}
	}
	c.focus = order[(at+delta+len(order))%len(order)]
	return c
}

func (c confirm) key(msg tea.KeyPressMsg) (confirm, confirmOutcome, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return c, keptAsItWas, nil
	case "tab":
		return c.moveFocus(1), stillOpen, nil
	case "shift+tab":
		return c.moveFocus(-1), stillOpen, nil
	case "enter":
		if c.focus == focusKeep {
			return c, keptAsItWas, nil
		}
		if f := c.spec.field; f != nil && f.required && strings.TrimSpace(c.input) == "" {
			c.problem = f.label + " is required"
			return c, stillOpen, nil
		}
		if f := c.spec.field; f != nil && f.mustChange && c.input == f.value {
			return c, keptUnchanged, nil
		}
		return c, submitted, c.spec.submit(c.input)
	}
	if c.focus != focusField {
		switch msg.String() {
		case "left", "h":
			c.focus = focusAction
		case "right", "l":
			c.focus = focusKeep
		}
		return c, stillOpen, nil
	}
	if msg.String() == "backspace" {
		c.input = dropLastRune(c.input)
		return c, stillOpen, nil
	}
	return c.typed(msg.Text), stillOpen, nil
}

func (c confirm) typed(text string) confirm {
	if c.focus == focusField {
		c.input += text
	}
	return c
}

func buttonIndex[F comparable](focus F, order ...F) int {
	return slices.Index(order, focus)
}

func buttons(labels []string, focused int) string {
	parts := make([]string, len(labels))
	for i, l := range labels {
		style := dimStyle
		if i == focused {
			style = cursorStyle
		}
		parts[i] = style.Render(" " + l + " ")
	}
	return strings.Join(parts, buttonGap)
}

func centred(s string, width int) string {
	return fitLeft(strings.Repeat(" ", max((width-ansi.StringWidth(s))/2, 0))+s, width)
}

func dialogWidth(screen int) int {
	return min(max(screen/2-2, minDialogWidth), screen-2)
}

func (c confirm) field() string {
	f := c.spec.field
	if c.focus != focusField {
		return dialogBodyInset + "  " + f.label + "  " + textStyle.Render(c.input)
	}
	return dialogBodyInset + warnStyle.Render("▸ ") + f.label + "  " + textStyle.Render(c.input) + warnStyle.Render("█")
}

func (c confirm) view(screen, height int) []string {
	width := dialogWidth(screen)
	jobs := c.spec.jobs
	without := c.lines(width, jobs.head, jobs.tail)
	errorRows := 0
	if c.spec.field != nil && c.problem == "" {
		errorRows = 2
	}
	room := height - boxBorders - len(without) - errorRows
	body := append(append(slices.Clone(jobs.head), jobs.shown(room)...), jobs.tail...)
	lines := c.lines(width, body)
	return box{
		title:  c.spec.colour.Bold(true).Render(c.spec.title),
		width:  width,
		height: len(lines) + boxBorders,
		colour: &c.spec.colour,
		lines:  lines,
	}.render()
}

func (c confirm) lines(width int, jobs ...[]string) []string {
	lines := []string{""}
	for _, l := range append(slices.Clone(c.spec.body), slices.Concat(jobs...)...) {
		lines = append(lines, dialogBodyInset+l)
	}
	if c.spec.field != nil {
		lines = append(lines, "", c.field())
	}
	lines = append(lines, "", centred(buttons([]string{c.spec.action, keepButton}, buttonIndex(c.focus, focusAction, focusKeep)), width-2))
	if c.problem != "" {
		lines = append(lines, "", dialogBodyInset+badStyle.Render(c.problem))
	}
	return append(lines, "")
}

func (c confirm) hints() []hint {
	out := []hint{{"tab", "next button"}, {"enter", "confirm"}, {"esc", "keep"}}
	if c.spec.field != nil {
		out = append([]hint{{"type", "reason"}}, out...)
	}
	return out
}
