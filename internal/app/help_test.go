package app_test

import (
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

var helpGroups = []string{"Nodes pane", "Jobs pane", "Both panes", "Dialogs", "Describe, JSON", "Commands", "Legend"}

var legendItems = []string{"✕ down or failed", "◆ drain", "◐ mixed", "● allocated", "○ idle", "◌ other",
	"◷ pending", "▶ running", "◼ completing", "✓ completed",
	"* not responding", "~ powered off", "# powering up", "! power down pending",
	"% powering down", "$ maintenance", "@ reboot requested", "^ reboot issued"}

func (r *run) offsetOf(text string) (int, int) {
	r.t.Helper()
	for y, line := range strings.Split(r.screen(), "\n") {
		if at := strings.Index(line, text); at >= 0 {
			return len([]rune(line[:at])), y
		}
	}
	r.t.Fatalf("screen has no %q\n%s", text, r.screen())
	return 0, 0
}

func TestHelpIsACentredPopUp(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1"), idle("n2")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")
	r.moveTo("n2")
	nodesTop := r.mustFrameTop("Nodes")

	r.send("?")

	r.waitFor("┌ Help ")
	if c := r.cellAt(frameLeft, nodesTop); c.Content != "┌" || !isFaintWithoutColour(c.Style) {
		t.Fatalf("Nodes corner under help = %q %+v, want a faint ┌\n%s", c.Content, c.Style, r.screen())
	}
	b := r.dialogBox("Help ")
	r.checkMarginAround(b, nodesTop)
	r.checkFadedUnder(b)
	if left, right := b.x, r.width-b.x-b.width; left-right > 1 || right-left > 1 {
		t.Fatalf("help is at x %d, width %d; want it centred\n%s", b.x, b.width, r.screen())
	}
	if s := r.cellAt(b.x, b.y).Style; !isANSI(s.Fg, ansiBlue) {
		t.Fatalf("help border = %+v, want blue", s)
	}
	rows := r.dialogRows(b)
	if rows[0] == "" || rows[len(rows)-1] != "" {
		t.Fatalf("help rows start with %q and end with %q; want no leading blank row and a blank last row\n%s", rows[0], rows[len(rows)-1], r.screen())
	}
	if x, _ := r.offsetOf("Nodes pane"); x != b.x+3 {
		t.Fatalf("group name at column %d, want %d (2 cells inside the border)\n%s", x, b.x+3, r.screen())
	}
	if x, _ := r.offsetOf("enter    Show"); x != b.x+5 {
		t.Fatalf("key row at column %d, want %d (indented once under the group name)\n%s", x, b.x+5, r.screen())
	}
	gap := -1
	for _, pair := range [][2]string{{"j/k ↑/↓", "Move the cursor"}, {"shift-n", "Sort by name"}, {"tab ←/→", "Next field"}, {":ov", "Overview"}, {"ctrl-d", "Cancel the job"}} {
		kx, ky := r.offsetOf(pair[0])
		ax, ay := r.offsetOf(pair[1])
		if ay != ky || (gap >= 0 && ax-kx != gap) {
			t.Fatalf("action %q starts %d cells after key %q, want %d in every group\n%s", pair[1], ax-kx, pair[0], gap, r.screen())
		}
		gap = ax - kx
	}
	screen := r.screen()
	for _, text := range append(append([]string{}, helpGroups...), legendItems...) {
		if !strings.Contains(screen, text) {
			t.Errorf("help at 160x50 has no %q\n%s", text, screen)
		}
	}
	if strings.Contains(screen, "↓ more") {
		t.Fatalf("help at 160x50 does not fit\n%s", screen)
	}
	if s := r.styleOf("◐ mixed"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("legend is not faint: %+v", s)
	}
	if !strings.HasPrefix(r.bottomLine(), "   j/k scroll · esc back") {
		t.Fatalf("bottom line = %q, want the help hints", r.bottomLine())
	}

	r.send(escKey)

	r.waitGone("┌ Help ")
	r.waitSelected("n2")
}

func TestHelpNeverCoversTheHeader(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	for _, size := range []struct{ width, height int }{{160, 50}, {120, 40}, {80, 24}} {
		r := startSizedOn(t, cluster, size.width, size.height)
		r.waitFor("┌ Nodes 1 ")
		r.send("?")
		r.waitFor("┌ Help ")
		lines := strings.Split(r.screen(), "\n")
		for y := range 5 {
			if strings.ContainsAny(lines[y], "┌┐└┘│─") {
				t.Fatalf("at %dx%d help covers header row %d: %q\n%s", size.width, size.height, y, lines[y], r.screen())
			}
		}
		if !strings.Contains(lines[0], "Context") {
			t.Fatalf("at %dx%d the header is gone\n%s", size.width, size.height, r.screen())
		}
	}
}

func TestLongHelpShowsMoreAndScrolls(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSized(t, clock.NewFake(startTime), 80, 24, []string{"--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")

	r.send("?")

	r.waitFor("┌ Help ")
	b := r.dialogBox("Help ")
	r.checkMarginAround(b, 5)
	rows := r.dialogRows(b)
	if last := strings.TrimSpace(rows[len(rows)-2]); last != "↓ more" || rows[len(rows)-1] != "" {
		t.Fatalf("last help row = %q, want ↓ more\n%s", last, r.screen())
	}
	if s := r.styleOf("↓ more"); !isANSI(s.Fg, ansiCyan) {
		t.Fatalf("↓ more is not cyan: %+v", s)
	}
	if strings.Contains(r.screen(), "^ reboot issued") {
		t.Fatalf("help fits; the test needs a longer help")
	}
	r.send("G")
	r.waitFor("^ reboot issued")
	r.waitGone("↓ more")
	r.send("g")
	r.waitFor("↓ more")
	r.send("jj")
	r.waitGone("Nodes pane")
	r.send(escKey)
	r.waitGone("┌ Help ")
}

func TestKeyThatDoesNothingSaysSo(t *testing.T) {
	_, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("x")

	r.waitFor("x does nothing here · ? lists the keys")
	if !isANSI(r.flashStyle("x does nothing").Fg, ansiYellow) {
		t.Fatalf("flash is not a warning: %q", r.bottomLine())
	}
	r.send("\t" + "U")
	r.waitFor("U works on a node · press tab to focus Nodes")
}

func TestUnknownCommandSaysSo(t *testing.T) {
	_, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send(":xyz\r")

	r.waitFor(":xyz is not a command · ? lists the commands")
	if !isANSI(r.flashStyle(":xyz is not").Fg, ansiYellow) {
		t.Fatalf("flash is not a warning: %q", r.bottomLine())
	}
}
