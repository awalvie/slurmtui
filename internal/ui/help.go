package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type keyHelp struct {
	key    string
	action string
	writes bool
}

type helpGroup struct {
	name string
	keys []keyHelp
}

var fixedHelpGroups = []helpGroup{
	{"Dialogs", []keyHelp{
		{key: "tab ←/→", action: "Next field or button"},
		{key: "enter", action: "Push the focused button"},
		{key: "esc", action: "Keep, change nothing"},
	}},
	{"Describe, JSON", []keyHelp{
		{key: "j/k g/G", action: "Scroll"},
		{key: "esc", action: "Back"},
	}},
	{"Commands", []keyHelp{
		{key: ":ov", action: "Overview (:overview)"},
		{key: ":no", action: "Nodes view (:nodes)"},
		{key: ":j", action: "Jobs view (:jobs)"},
		{key: ":q", action: "Quit (:quit, ctrl-c)"},
	}},
}

type legendEntry struct {
	glyph string
	style lipgloss.Style
	word  string
}

type legendGroup struct {
	name    string
	entries []legendEntry
}

func stateEntry(c stateClass, word string) legendEntry {
	return legendEntry{glyph: c.glyph, style: c.style, word: word}
}

func suffixEntry(glyph, word string) legendEntry {
	return legendEntry{glyph: glyph, style: keyStyle, word: word}
}

var legendGroups = []legendGroup{
	{"States", []legendEntry{
		stateEntry(nodeClasses[nodeDown], "down or failed"),
		stateEntry(nodeClasses[nodeDrain], "drain"),
		stateEntry(nodeClasses[nodeMixed], "mixed"),
		stateEntry(nodeClasses[nodeAllocated], "allocated"),
		stateEntry(nodeClasses[nodeIdle], "idle"),
		stateEntry(nodeClasses[nodeOther], "other"),
		stateEntry(jobClasses[jobPending], "pending"),
		stateEntry(jobClasses[jobRunning], "running"),
		stateEntry(jobClasses[jobCompleting], "completing"),
		stateEntry(jobClasses[jobCompleted], "completed"),
	}},
	{"Node suffixes", []legendEntry{
		suffixEntry("*", "not responding"),
		suffixEntry("~", "powered off"),
		suffixEntry("#", "powering up"),
		suffixEntry("!", "power down pending"),
		suffixEntry("%", "powering down"),
		suffixEntry("$", "maintenance"),
		suffixEntry("@", "reboot requested"),
		suffixEntry("^", "reboot issued"),
	}},
}

const (
	helpKeyGap     = 2
	helpGroupInset = dialogBodyInset
	helpRowInset   = dialogBodyInset + dialogBodyInset
	helpRightInset = 2
	helpColumnGap  = 4
	helpMaxColumns = 3
	helpMoreMarker = "↓ more"
	helpLessMarker = "↑ more"
	helpPaneTitle  = "Help"
)

func groupBlocks(readOnly bool) [][]string {
	groups := helpGroupsFromBindings()
	keyWidth := 0
	for _, g := range groups {
		for _, k := range g.keys {
			keyWidth = max(keyWidth, ansi.StringWidth(k.key)+helpKeyGap)
		}
	}
	var blocks [][]string
	for _, g := range groups {
		block := []string{helpGroupInset + keyStyle.Render(g.name)}
		for _, k := range g.keys {
			if k.writes && readOnly {
				continue
			}
			block = append(block, helpRowInset+keyStyle.Render(fitLeft(k.key, keyWidth))+textStyle.Render(k.action))
		}
		blocks = append(blocks, append(block, ""))
	}
	for _, g := range legendGroups {
		block := []string{helpGroupInset + keyStyle.Render(g.name)}
		for _, e := range g.entries {
			block = append(block, helpRowInset+e.style.Render(e.glyph)+fitLeft("", keyWidth-ansi.StringWidth(e.glyph))+textStyle.Render(e.word))
		}
		blocks = append(blocks, append(block, ""))
	}
	return blocks
}

func columnsOf(blocks [][]string, count int) [][]string {
	total := 0
	for _, b := range blocks {
		total += len(b)
	}
	for limit := (total + count - 1) / count; ; limit++ {
		columns := [][]string{nil}
		for _, b := range blocks {
			last := len(columns) - 1
			if len(columns[last]) > 0 && len(columns[last])+len(b) > limit {
				columns = append(columns, nil)
				last++
			}
			columns[last] = append(columns[last], b...)
		}
		if len(columns) <= count {
			return columns
		}
	}
}

func helpLines(readOnly bool, room int) []string {
	blocks := groupBlocks(readOnly)
	columnWidth := 0
	for _, b := range blocks {
		columnWidth = max(columnWidth, widest(b)+helpColumnGap)
	}
	count := max(min((room+helpColumnGap)/columnWidth, helpMaxColumns), 1)
	columns := columnsOf(blocks, count)
	rows := 0
	for _, c := range columns {
		rows = max(rows, len(c))
	}
	var lines []string
	for r := range rows {
		line := ""
		for i, c := range columns {
			cell := ""
			if r < len(c) {
				cell = c[r]
			}
			if i < len(columns)-1 {
				cell = fitLeft(cell, columnWidth)
			}
			line += cell
		}
		lines = append(lines, line)
	}
	for len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (m Model) helpLayout() ([]string, int, int) {
	width, _ := m.size()
	room := width - 2*popupMargin - boxBorders - helpRightInset
	lines := helpLines(m.session.ReadOnly, room)
	rows := max(m.bodyHeight()-2*popupMargin-boxBorders-1, 1)
	return lines, min(rows, len(lines)), min(widest(lines)+helpRightInset+boxBorders, width-2*popupMargin)
}

func (m Model) helpKey(key string) Model {
	lines, rows, _ := m.helpLayout()
	return m.scrollKey(key, len(lines), rows)
}

func (m Model) helpBox() []string {
	lines, rows, width := m.helpLayout()
	top := m.scrollTop(len(lines), rows)
	shown := append([]string{}, lines[top:top+rows]...)
	if top > 0 {
		shown[0] = helpGroupInset + accentStyle.Render(helpLessMarker)
	}
	if top+rows < len(lines) {
		shown[rows-1] = helpGroupInset + accentStyle.Render(helpMoreMarker)
	}
	shown = append(shown, "")
	return box{
		title:   titleStyle.Render(helpPaneTitle),
		width:   width,
		height:  len(shown) + boxBorders,
		focused: true,
		lines:   shown,
	}.render()
}
