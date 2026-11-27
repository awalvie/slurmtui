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
	helpKeyWidth    = 9
	helpGroupInset  = "  "
	helpRowInset    = "    "
	helpColumnGap   = 4
	helpMaxColumns  = 3
	helpMoreMarker  = "↓ more"
	legendHeading   = "Legend"
	helpPaneTitle   = "Help"
	helpFrameInside = 2
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
	var blocks [][]string
	for _, g := range helpGroupsFromBindings() {
		block := []string{helpGroupInset + keyStyle.Render(g.name)}
		for _, k := range g.keys {
			if k.writes && readOnly {
				continue
			}
			block = append(block, helpRowInset+keyStyle.Render(fitLeft(k.key, helpKeyWidth))+textStyle.Render(k.action))
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

func helpLines(readOnly bool, inner int) []string {
	blocks := groupBlocks(readOnly)
	columnWidth := 0
	for _, b := range blocks {
		columnWidth = max(columnWidth, widest(b)+helpColumnGap)
	}
	count := max(min((inner+helpColumnGap)/columnWidth, helpMaxColumns), 1)
	columns := columnsOf(blocks, count)
	rows := 0
	for _, c := range columns {
		rows = max(rows, len(c))
	}
	lines := []string{""}
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
	lines = append(lines, helpGroupInset+keyStyle.Render(legendHeading))
	for _, l := range packed(legend, max(inner-len(helpRowInset)-1, 1)) {
		lines = append(lines, helpRowInset+dimStyle.Render(l))
	}
	return lines
}

func (m Model) helpRoom() int {
	return max(m.bodyHeight()-boxBorders, 1)
}

func (m Model) helpKey(key string) Model {
	width, _ := m.size()
	return m.scrollKey(key, len(helpLines(m.session.ReadOnly, width-2-helpFrameInside)), m.helpRoom())
}

func (m Model) helpPane(width, height int) []string {
	lines := helpLines(m.session.ReadOnly, width-helpFrameInside)
	room := min(m.helpRoom(), len(lines))
	top := m.scrollTop(len(lines), room)
	shown := append([]string{}, lines[top:top+room]...)
	if top+room < len(lines) {
		shown[room-1] = helpGroupInset + accentStyle.Render(helpMoreMarker)
	}
	return box{
		title:   titleStyle.Render(helpPaneTitle),
		width:   width,
		height:  height,
		focused: true,
		lines:   shown,
	}.render()
}
