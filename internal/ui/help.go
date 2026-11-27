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
	helpWidth       = 64
	helpKeyWidth    = 9
	helpGroupInset  = "  "
	helpRowInset    = "    "
	helpMarginRows  = 2
	helpBorderRows  = 2
	helpMoreMarker  = "↓ more"
	legendHeading   = "Legend"
	helpBodyColumns = helpWidth - 2 - len(helpRowInset) - 1
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

func helpLines(readOnly bool) []string {
	lines := []string{""}
	for _, g := range helpGroupsFromBindings() {
		lines = append(lines, helpGroupInset+keyStyle.Render(g.name))
		for _, k := range g.keys {
			if k.writes && readOnly {
				continue
			}
			lines = append(lines, helpRowInset+keyStyle.Render(fitLeft(k.key, helpKeyWidth))+textStyle.Render(k.action))
		}
		lines = append(lines, "")
	}
	lines = append(lines, helpGroupInset+keyStyle.Render(legendHeading))
	for _, l := range packed(legend, helpBodyColumns) {
		lines = append(lines, helpRowInset+dimStyle.Render(l))
	}
	return lines
}

func helpRoom(height int) int {
	return max(height-bottomLineHeight-helpMarginRows-helpBorderRows, 1)
}

func (m Model) helpKey(key string) Model {
	_, height := m.size()
	return m.scrollKey(key, len(helpLines(m.session.ReadOnly)), helpRoom(height))
}

func (m Model) renderHelp(width, height int) []string {
	lines := helpLines(m.session.ReadOnly)
	room := min(helpRoom(height), len(lines))
	top := m.scrollTop(len(lines), room)
	shown := append([]string{}, lines[top:top+room]...)
	if top+room < len(lines) {
		shown[room-1] = helpGroupInset + accentStyle.Render(helpMoreMarker)
	}
	return box{
		title:   titleStyle.Render("Keys"),
		width:   min(helpWidth, width-2),
		height:  room + helpBorderRows,
		focused: true,
		lines:   shown,
	}.render()
}
