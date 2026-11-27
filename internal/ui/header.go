package ui

import "github.com/charmbracelet/x/ansi"

const (
	headerRows      = 5
	infoLabelWidth  = 9
	gridColumn      = 40
	gridAfterInfo   = 6
	gridColumnGap   = 4
	logoAfterGrid   = 4
	logoRightMargin = 1
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

func (m Model) infoRowCount() int {
	if m.session.Expires.IsZero() {
		return headerRows - 1
	}
	return headerRows
}

func hintGrid(hints []hint, rows int) []string {
	grid := make([]string, headerRows)
	if len(hints) == 0 {
		return grid
	}
	columns := (len(hints) + rows - 1) / rows
	rows = (len(hints) + columns - 1) / columns
	keyWidth, labelWidth := 0, 0
	for _, h := range hints {
		keyWidth = max(keyWidth, ansi.StringWidth(h.key))
		labelWidth = max(labelWidth, ansi.StringWidth(h.label))
	}
	for i, h := range hints {
		r := i % rows
		if i >= rows {
			grid[r] = fitLeft(grid[r], (i/rows)*(keyWidth+1+labelWidth+gridColumnGap))
		}
		grid[r] += keyStyle.Render(fitLeft(h.key, keyWidth)) + " " + dimStyle.Render(h.label)
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
	grid := hintGrid(m.hints(), m.infoRowCount())
	gridWidth := widest(grid)
	gridAt := max(gridColumn, infoWidth+gridAfterInfo)
	logoWidth := widest(logo) + logoRightMargin
	showGrid := gridAt+gridWidth <= width
	showLogo := showGrid && gridAt+gridWidth+logoAfterGrid+logoWidth <= width
	out := make([]string, headerRows)
	for r := range out {
		line := info[r]
		if showGrid {
			line = fitLeft(line, gridAt) + grid[r]
		}
		if showLogo && r < len(logo) {
			line = fitLeft(line, width-logoWidth) + logo[r]
		}
		out[r] = line
	}
	return out
}
