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
	r.checkNoBorderNextTo(b)
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

func isBorder(content string) bool {
	return content != "" && strings.ContainsAny(content, "┌┐└┘│─├┤┬┴┼")
}

func (r *run) firstMarkFrom(x, y, dx, dy int) (string, [2]int) {
	for x >= 0 && y >= 0 && x < r.width && y < r.height {
		if c := r.cellAt(x, y).Content; c != " " && c != "" {
			return c, [2]int{x, y}
		}
		x, y = x+dx, y+dy
	}
	return "", [2]int{x, y}
}

func (r *run) checkNoBorderNextTo(b dialogBox) {
	r.t.Helper()
	type ray struct{ x, y, dx, dy int }
	var rays []ray
	for y := b.y; y < b.y+b.height; y++ {
		rays = append(rays, ray{b.x - 1, y, -1, 0}, ray{b.x + b.width, y, 1, 0})
	}
	for x := b.x; x < b.x+b.width; x++ {
		rays = append(rays, ray{x, b.y - 1, 0, -1}, ray{x, b.y + b.height, 0, 1})
	}
	for _, ry := range rays {
		if c, at := r.firstMarkFrom(ry.x, ry.y, ry.dx, ry.dy); isBorder(c) && at[1] < r.height-1 {
			r.t.Fatalf("border piece %q at %v next to the dialog\n%s", c, at, r.screen())
		}
	}
}

func TestDialogsStayInTheBodyWithNoBorderPiecesNextToThem(t *testing.T) {
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
				if b.y < bodyTop || b.y+b.height > r.height-1 {
					t.Fatalf("dialog rows %d to %d leave the body (rows %d to %d)\n%s", b.y, b.y+b.height-1, bodyTop, r.height-2, r.screen())
				}
				r.checkNoBorderNextTo(b)
				r.send(escKey)
				r.waitGone(d.title)
			})
		}
	}
}
