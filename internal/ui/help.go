package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/awalvie/slurmtui/internal/slurm"
	"github.com/charmbracelet/x/ansi"
)

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
		stateEntry(nodeClasses[slurm.NodeDown], "down or failed"),
		stateEntry(nodeClasses[slurm.NodeDrain], "drain"),
		stateEntry(nodeClasses[slurm.NodeMixed], "mixed"),
		stateEntry(nodeClasses[slurm.NodeAllocated], "allocated"),
		stateEntry(nodeClasses[slurm.NodeIdle], "idle"),
		stateEntry(nodeClasses[slurm.NodeOther], "other"),
		stateEntry(jobClasses[slurm.JobPending], "pending"),
		stateEntry(jobClasses[slurm.JobRunning], "running"),
		stateEntry(jobClasses[slurm.JobCompleting], "completing"),
		stateEntry(jobClasses[slurm.JobCompleted], "completed"),
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
	groups := helpGroups()
	keyWidth := 0
	for _, g := range groups {
		for _, b := range g.bindings {
			keyWidth = max(keyWidth, ansi.StringWidth(b.shown)+helpKeyGap)
		}
	}
	var blocks [][]string
	for _, g := range groups {
		block := []string{helpGroupInset + keyStyle.Render(g.name)}
		for _, b := range g.bindings {
			if b.writes && readOnly {
				continue
			}
			block = append(block, helpRowInset+keyStyle.Render(fitLeft(b.shown, keyWidth))+textStyle.Render(b.help))
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
