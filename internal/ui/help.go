package ui

import "github.com/charmbracelet/x/ansi"

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

var legend = []string{
	"✕ down or failed", "◆ drain", "◐ mixed", "● allocated", "○ idle", "◌ other",
	"◷ pending", "▶ running", "◼ completing", "✓ completed",
	"* not responding", "~ powered off", "# powering up", "! power down pending",
	"% powering down", "$ maintenance", "@ reboot requested", "^ reboot issued",
}

const (
	helpKeyGap     = 2
	helpGroupInset = dialogBodyInset
	helpRowInset   = dialogBodyInset + dialogBodyInset
	helpRightInset = 2
	helpColumnGap  = 4
	helpMaxColumns = 3
	helpMoreMarker = "↓ more"
	legendHeading  = "Legend"
	helpPaneTitle  = "Help"
)

func packed(items []string, width int) []string {
	var out []string
	line := ""
	for _, item := range items {
		next := item
		if line != "" {
			next = line + factSeparator + item
		}
		if line != "" && ansi.StringWidth(next) > width {
			out = append(out, line)
			next = item
		}
		line = next
	}
	return append(out, line)
}

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
	width := min(widest(lines), room)
	lines = append(lines, helpGroupInset+keyStyle.Render(legendHeading))
	for _, l := range packed(legend, max(width-len(helpRowInset), 1)) {
		lines = append(lines, helpRowInset+dimStyle.Render(l))
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
