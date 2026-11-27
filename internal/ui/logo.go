package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var canLogoGlyphs = []string{
	" ▄▄█▄▄ ",
	"███████",
	"███████",
	"███████",
	"▀▀▀▀▀▀▀",
}

var canLogoColors = []string{
	" DDDDD ",
	"WSSSSSD",
	"CCCCCCc",
	"WSSSSSD",
	"DDDDDDD",
}

var canLogoPalette = map[rune]ansi.BasicColor{
	'W': logoShineColor,
	'S': logoCanColor,
	'D': logoShadeColor,
	'C': logoBandColor,
	'c': logoBandShade,
}

func canLogo() []string {
	lines := make([]string, len(canLogoGlyphs))
	for r, glyphRow := range canLogoGlyphs {
		keys := []rune(canLogoColors[r])
		var line strings.Builder
		for c, glyph := range []rune(glyphRow) {
			if glyph == ' ' {
				line.WriteRune(' ')
				continue
			}
			line.WriteString(lipgloss.NewStyle().Foreground(canLogoPalette[keys[c]]).Render(string(glyph)))
		}
		lines[r] = line.String()
	}
	return lines
}
