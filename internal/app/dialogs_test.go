package app_test

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

type dialogBox struct {
	x, y, width, height int
}

func (r *run) dialogBox(title string) dialogBox {
	r.t.Helper()
	lines := strings.Split(r.screen(), "\n")
	for y, line := range lines {
		at := strings.Index(line, "┌ "+title)
		if at < 0 {
			continue
		}
		row := []rune(line)
		x := len([]rune(line[:at]))
		right := x + 1
		for right < len(row) && row[right] != '┐' {
			right++
		}
		bottom := y + 1
		for bottom < len(lines) && (x >= len([]rune(lines[bottom])) || []rune(lines[bottom])[x] != '└') {
			bottom++
		}
		return dialogBox{x: x, y: y, width: right - x + 1, height: bottom - y + 1}
	}
	r.t.Fatalf("no dialog titled %q\nscreen:\n%s", title, r.screen())
	return dialogBox{}
}

func (r *run) dialogRows(b dialogBox) []string {
	lines := strings.Split(r.screen(), "\n")
	out := make([]string, 0, b.height-2)
	for y := b.y + 1; y < b.y+b.height-1; y++ {
		row := []rune(lines[y])
		end := min(b.x+b.width-1, len(row))
		out = append(out, strings.TrimRight(string(row[b.x+1:end]), " "))
	}
	return out
}

func isFaintWithoutColour(s uv.Style) bool {
	return s.Fg == nil && s.Attrs&uv.AttrFaint != 0
}

func TestDialogIsCentredOverAFadedScreen(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("D")

	title := "Drain n02 on " + contextOf(cluster) + " (lamp)"
	r.waitFor(title)
	b := r.dialogBox(title)
	if b.width != 78 || b.x != 41 {
		t.Fatalf("dialog at x %d, width %d; want x 41, width 78\n%s", b.x, b.width, r.screen())
	}
	corner := r.cellAt(b.x, b.y)
	if !isANSI(corner.Style.Fg, ansiYellow) {
		t.Fatalf("Drain border is not yellow: %+v", corner.Style)
	}
	if c := r.cellAt(b.x+2, b.y); !isANSI(c.Style.Fg, ansiYellow) || c.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("Drain title is not yellow bold: %+v", c.Style)
	}
	for _, at := range [][2]int{{b.x - 1, b.y}, {b.x + 5, b.y - 1}, {b.x + b.width, b.y + 2}, {b.x + 5, b.y + b.height}} {
		if c := r.cellAt(at[0], at[1]); c.Content != " " && c.Content != "" {
			t.Fatalf("cell %v next to the dialog = %q, want blank\n%s", at, c.Content, r.screen())
		}
	}
	if c := r.cellAt(gridLeft, 0); c.Content != "C" || !isFaintWithoutColour(c.Style) {
		t.Fatalf("header cell = %q %+v, want a faint C without colour", c.Content, c.Style)
	}
	logoLine := strings.Split(r.screen(), "\n")[1]
	logo := len([]rune(logoLine[:strings.Index(logoLine, "███")]))
	if c := r.cellAt(logo, 1); !isFaintWithoutColour(c.Style) {
		t.Fatalf("logo cell %+v is not faded", c.Style)
	}
	bottom := r.bottomLine()
	if !strings.HasPrefix(bottom, "   type reason · tab next button · enter confirm · esc keep") {
		t.Fatalf("bottom line = %q, want the dialog hints at column 3", bottom)
	}
	if c := r.cellAt(gridLeft, r.height-1); c.Content != "t" || c.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("hint key = %q %+v, want bold", c.Content, c.Style)
	}
	if c := r.cellAt(r.width-6, r.height-1); !isANSI(c.Style.Fg, ansiGreen) {
		t.Fatalf("freshness lost its colour under the dialog: %+v", c.Style)
	}
}
