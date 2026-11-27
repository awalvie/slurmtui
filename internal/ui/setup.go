package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/charmbracelet/x/ansi"
)

type TokenKind int

const (
	KeyFileToken TokenKind = iota
	EnvToken
	PastedToken
)

var tokenKindLabels = []string{"Key file", "Env var", "Paste"}

var tokenKinds = TokenKind(len(tokenKindLabels))

const pastedTokenNote = "A pasted token is used for this session only."

type Draft struct {
	Name    string
	URL     string
	CAFile  string
	User    string
	Kind    TokenKind
	KeyFile string
	Env     string
	Pasted  auth.Token
}

type Connected struct {
	Source  Source
	Session Session
}

type Connect func(Draft) (Connected, error)

type setupField int

const (
	nameField setupField = iota
	urlField
	caFileField
	userField
	kindField
	valueField
	testButton
	quitButton
	setupFieldCount
)

type Setup struct {
	draft    Draft
	focus    setupField
	connect  Connect
	clock    clock.Clock
	interval time.Duration
	testing  bool
	err      error
	width    int
	height   int
}

func NewSetup(draft Draft, connect Connect, clk clock.Clock, interval time.Duration) Setup {
	return Setup{draft: draft, connect: connect, clock: clk, interval: interval}
}

type connectedMsg struct {
	connected Connected
	err       error
}

func (m Setup) Init() tea.Cmd {
	return nil
}

func (m Setup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.PasteMsg:
		if !m.testing {
			m.editText(func(s string) string { return s + msg.Content })
		}
	case connectedMsg:
		m.testing = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		var nodes tea.Model = New(msg.connected.Source, msg.connected.Session, m.clock, m.interval)
		nodes, _ = nodes.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		return nodes, nodes.Init()
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Setup) editText(change func(string) string) {
	switch m.focus {
	case nameField:
		m.draft.Name = change(m.draft.Name)
	case urlField:
		m.draft.URL = change(m.draft.URL)
	case caFileField:
		m.draft.CAFile = change(m.draft.CAFile)
	case userField:
		m.draft.User = change(m.draft.User)
	case valueField:
		switch m.draft.Kind {
		case KeyFileToken:
			m.draft.KeyFile = change(m.draft.KeyFile)
		case EnvToken:
			m.draft.Env = change(m.draft.Env)
		case PastedToken:
			m.draft.Pasted = auth.Token(change(string(m.draft.Pasted)))
		}
	}
}

func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

func (m Setup) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.testing {
		return m, nil
	}
	switch key {
	case "tab", "down":
		m.focus = (m.focus + 1) % setupFieldCount
		return m, nil
	case "shift+tab", "up":
		m.focus = (m.focus + setupFieldCount - 1) % setupFieldCount
		return m, nil
	case "enter":
		switch m.focus {
		case testButton:
			m.testing = true
			m.err = nil
			draft, connect := m.draft, m.connect
			return m, func() tea.Msg {
				c, err := connect(draft)
				return connectedMsg{connected: c, err: err}
			}
		case quitButton:
			return m, tea.Quit
		}
		m.focus++
		return m, nil
	}
	if m.focus == kindField {
		switch key {
		case "left", "h":
			m.draft.Kind = (m.draft.Kind + tokenKinds - 1) % tokenKinds
		case "right", "l", "space":
			m.draft.Kind = (m.draft.Kind + 1) % tokenKinds
		}
		return m, nil
	}
	switch key {
	case "backspace":
		m.editText(dropLastRune)
	case "ctrl+u":
		m.editText(func(string) string { return "" })
	default:
		m.editText(func(s string) string { return s + msg.Text })
	}
	return m, nil
}

const (
	setupFormWidth = 64
	setupLabelGap  = "  "
	setupFocusMark = "▸ "
	setupPlainMark = "  "
	setupInset     = " "
)

var setupHints = []hint{{"tab", "next"}, {"shift-tab", "previous"}, {"←/→", "change"}, {"enter", "select"}}

func (m Setup) tokenValue() (string, string) {
	switch m.draft.Kind {
	case EnvToken:
		return "Env var", m.draft.Env
	case PastedToken:
		return "Token", fmt.Sprintf("[hidden, %d characters]", utf8.RuneCountInString(string(m.draft.Pasted)))
	}
	return "Key file", m.draft.KeyFile
}

func (m Setup) formLines(width int) []string {
	valueLabel, value := m.tokenValue()
	fields := []struct {
		field setupField
		label string
		value string
	}{
		{nameField, "Context name", m.draft.Name},
		{urlField, "URL", m.draft.URL},
		{caFileField, "CA file", m.draft.CAFile},
		{userField, "User", m.draft.User},
		{kindField, "Token source", ""},
		{valueField, valueLabel, value},
	}
	labels := make([]string, len(fields))
	for i, f := range fields {
		labels[i] = f.label
	}
	labelWidth := widest(labels)
	lines := []string{""}
	for _, f := range fields {
		focused := f.field == m.focus
		mark, label, shown := setupPlainMark, dimStyle.Render(fitLeft(f.label, labelWidth)), textStyle.Render(f.value)
		if focused {
			mark, label = accentStyle.Render(setupFocusMark), textStyle.Render(fitLeft(f.label, labelWidth))
		}
		switch {
		case f.field == kindField:
			shown = dimStyle.Render("‹ ") + textStyle.Render(tokenKindLabels[m.draft.Kind]) + dimStyle.Render(" ›")
		case focused:
			shown = accentStyle.Render(f.value + "█")
		}
		lines = append(lines, setupInset+mark+label+setupLabelGap+shown)
	}
	lines = append(lines, "", centred(buttons([]string{"Test and save", "Quit"}, buttonIndex(m.focus, testButton, quitButton)), width-2))
	var notes []string
	if m.draft.Kind == PastedToken {
		notes = append(notes, dimStyle.Render(pastedTokenNote))
	}
	switch {
	case m.testing:
		notes = append(notes, dimStyle.Render("Testing the connection…"))
	case m.err != nil:
		wrapped := ansi.Wrap(m.err.Error(), width-2-2*len(setupInset), " ")
		for _, l := range strings.Split(wrapped, "\n") {
			notes = append(notes, badStyle.Render(l))
		}
	}
	if len(notes) > 0 {
		lines = append(lines, "")
		for _, n := range notes {
			lines = append(lines, setupInset+n)
		}
	}
	return append(lines, "")
}

func (m Setup) View() tea.View {
	width, height := screenSize(m.width, m.height)
	formWidth := min(setupFormWidth, width-2)
	lines := m.formLines(formWidth)
	form := box{
		title:   titleStyle.Render("Connect to Slurm"),
		width:   formWidth,
		height:  len(lines) + 2,
		focused: true,
		lines:   lines,
	}.render()
	screen := make([]string, height)
	screen[height-bottomLineHeight] = hintLine(setupHints, width)
	return altView(overlaid(screen, form, width, height))
}
