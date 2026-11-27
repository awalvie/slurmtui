package ui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCanLogoColourKeysMatchItsGlyphs(t *testing.T) {
	if len(canLogoGlyphs) != len(canLogoColors) {
		t.Fatalf("can logo: %d glyph rows but %d colour rows", len(canLogoGlyphs), len(canLogoColors))
	}
	for r, glyphRow := range canLogoGlyphs {
		glyphs, keys := []rune(glyphRow), []rune(canLogoColors[r])
		if len(glyphs) != len(keys) {
			t.Fatalf("can logo row %d: %d glyphs but %d colour keys", r, len(glyphs), len(keys))
		}
		for c, glyph := range glyphs {
			_, known := canLogoPalette[keys[c]]
			switch {
			case glyph == ' ' && keys[c] != ' ':
				t.Fatalf("can logo row %d column %d: colour key %q on a blank cell", r, c, keys[c])
			case glyph != ' ' && !known:
				t.Fatalf("can logo row %d column %d: unknown colour key %q", r, c, keys[c])
			}
		}
	}
}

func TestCanLogoFitsTheHeader(t *testing.T) {
	lines := canLogo()
	if len(lines) > headerRows {
		t.Fatalf("can logo has %d rows, header has %d", len(lines), headerRows)
	}
	for r, line := range lines {
		if width := ansi.StringWidth(line); width != ansi.StringWidth(canLogoGlyphs[0]) || width > 12 {
			t.Fatalf("can logo row %d is %d cells wide", r, width)
		}
	}
}
