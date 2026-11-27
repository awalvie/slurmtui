package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	headerRows      = 5
	infoLabelWidth  = 9
	gridAfterInfo   = 6
	gridColumnGap   = 4
	logoAfterGrid   = 4
	logoRightMargin = 2
	expiresLayout   = "15:04"
)

func (m Model) infoRows() []string {
	row := func(label, value string) string {
		return gridEdge + dimStyle.Render(fitLeft(label, infoLabelWidth)) + value
	}
	context := row("Context", keyStyle.Render(m.session.Context))
	if m.session.ReadOnly {
		context += "  " + warnStyle.Bold(true).Render("[read-only]")
	}
	user := textStyle.Render(m.session.User)
	if m.session.User == "root" {
		user = badStyle.Bold(true).Render(m.session.User)
	}
	expires := ""
	if !m.session.Expires.IsZero() {
		expires = row("Expires", textStyle.Render(m.session.Expires.Local().Format(expiresLayout)))
	}
	return []string{
		context,
		row("Cluster", textStyle.Render(m.meta.Cluster)),
		row("User", user),
		row("Slurm", textStyle.Render(m.meta.Version)),
		expires,
	}
}

func hintGrid(hints []hint) []string {
	grid := make([]string, headerRows)
	for c := 0; c*headerRows < len(hints); c++ {
		chunk := hints[c*headerRows : min((c+1)*headerRows, len(hints))]
		keyWidth, labelWidth := 0, 0
		for _, h := range chunk {
			keyWidth = max(keyWidth, ansi.StringWidth(h.key))
			labelWidth = max(labelWidth, ansi.StringWidth(h.label))
		}
		for r := range grid {
			if c > 0 {
				grid[r] += strings.Repeat(" ", gridColumnGap)
			}
			if r < len(chunk) {
				grid[r] += keyStyle.Render(fitLeft(chunk[r].key, keyWidth)) + " " + dimStyle.Render(fitLeft(chunk[r].label, labelWidth))
				continue
			}
			grid[r] += strings.Repeat(" ", keyWidth+1+labelWidth)
		}
	}
	return grid
}

func widest(lines []string) int {
	out := 0
	for _, l := range lines {
		out = max(out, ansi.StringWidth(l))
	}
	return out
}

func (m Model) header(width int) []string {
	logo := canLogo()
	info := m.infoRows()
	infoWidth := widest(info)
	grid := hintGrid(m.hints())
	gridWidth := widest(grid)
	logoWidth := widest(logo) + logoRightMargin
	showGrid := infoWidth+gridAfterInfo+gridWidth <= width
	showLogo := showGrid && infoWidth+gridAfterInfo+gridWidth+logoAfterGrid+logoWidth <= width
	out := make([]string, headerRows)
	for r := range out {
		line := info[r]
		if showGrid {
			line = fitLeft(line, infoWidth) + strings.Repeat(" ", gridAfterInfo) + grid[r]
		}
		if showLogo && r < len(logo) {
			line = fitLeft(line, width-logoWidth) + logo[r]
		}
		out[r] = line
	}
	return out
}
