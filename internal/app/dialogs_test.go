package app_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

type dialogBox struct {
	x, y, width, height int
}

func (r *run) dialogBox(title string) dialogBox {
	r.t.Helper()
	lines := r.lines()
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
	lines := r.lines()
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
	r.checkMarginAround(b, 6)
	r.checkFadedUnder(b)
	if c := r.cellAt(frameLeft, 6); c.Content != "┌" {
		t.Fatalf("pane corner under the dialog = %q, want a faint ┌\n%s", c.Content, r.screen())
	}
	if c := r.cellAt(gridLeft, 0); c.Content != "C" || !isFaintWithoutColour(c.Style) {
		t.Fatalf("header cell = %q %+v, want a faint C without colour", c.Content, c.Style)
	}
	logoLine := r.lines()[1]
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

func isBlank(c uv.Cell) bool {
	return c.Content == "" || c.Content == " "
}

func (r *run) checkMarginAround(b dialogBox, bodyTop int) {
	r.t.Helper()
	left, right, top, bottom := b.x-1, b.x+b.width, b.y-1, b.y+b.height
	if left < 0 || right >= r.width || top < bodyTop || bottom > r.height-2 {
		r.t.Fatalf("box %+v and its margin leave the body (rows %d to %d, columns 0 to %d)\n%s", b, bodyTop, r.height-2, r.width-1, r.screen())
	}
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			ring := y == top || y == bottom || x == left || x == right
			if c := r.cellAt(x, y); ring && !isBlank(c) {
				r.t.Fatalf("margin cell (%d,%d) = %q, want blank\n%s", x, y, c.Content, r.screen())
			}
		}
	}
}

func (r *run) checkFadedUnder(b dialogBox) {
	r.t.Helper()
	for y := range r.height - 1 {
		for x := range r.width {
			inside := x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
			if c := r.cellAt(x, y); !inside && !isBlank(c) && !isFaintWithoutColour(c.Style) {
				r.t.Fatalf("cell (%d,%d) %q = %+v under a pop-up, want faint without colour\n%s", x, y, c.Content, c.Style, r.screen())
			}
		}
	}
}

func TestDialogsHaveABlankMarginInsideTheBody(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1"), nodeIn("n2", "IDLE", "DRAIN")},
		Jobs:  []fakeslurm.Job{runningJob(1234)},
	})
	for _, size := range []struct{ width, height int }{{160, 50}, {120, 40}, {80, 24}} {
		for _, d := range []struct{ node, keys, title string }{{"n1", "D", "Drain n1"}, {"n2", "U", "Resume n2"}, {"n1", "\t\x04", "Cancel job 1234"}} {
			t.Run(fmt.Sprintf("%dx%d %s", size.width, size.height, d.title), func(t *testing.T) {
				r := startSizedOn(t, cluster, size.width, size.height)
				r.waitFor("┌ Nodes 2 ")
				bodyTop := r.mustFrameTop("Nodes")
				r.moveTo(d.node)
				r.send(d.keys)

				r.waitFor(d.title)
				b := r.dialogBox(d.title)
				r.checkMarginAround(b, bodyTop)
				r.checkFadedUnder(b)
				r.send(escKey)
				r.waitGone(d.title)
			})
		}
	}
}
