package app_test

import (
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

var helpGroups = []string{"Nodes pane", "Jobs pane", "Both panes", "Dialogs", "Describe, JSON", "Commands", "States", "Node suffixes"}

var legendGroups = []struct {
	name    string
	entries [][2]string
}{
	{"States", [][2]string{{"✕", "down or failed"}, {"◆", "drain"}, {"◐", "mixed"}, {"●", "allocated"}, {"○", "idle"}, {"◌", "other"},
		{"◷", "pending"}, {"▶", "running"}, {"◼", "completing"}, {"✓", "completed"}}},
	{"Node suffixes", [][2]string{{"*", "not responding"}, {"~", "powered off"}, {"#", "powering up"}, {"!", "power down pending"},
		{"%", "powering down"}, {"$", "maintenance"}, {"@", "reboot requested"}, {"^", "reboot issued"}}},
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
	for _, text := range helpGroups {
		if !strings.Contains(screen, text) {
			t.Errorf("help at 160x50 has no %q\n%s", text, screen)
		}
	}
	if strings.Contains(screen, "↓ more") {
		t.Fatalf("help at 160x50 does not fit\n%s", screen)
	}
	for _, g := range legendGroups {
		gx, gy := r.offsetOf(g.name)
		for i, e := range g.entries {
			x, y := r.offsetOf(e[0] + strings.Repeat(" ", gap-1) + e[1])
			if x != gx+2 || y != gy+1+i {
				t.Fatalf("%s %s at %d,%d, want %d,%d: one entry per row in the key column\n%s", e[0], e[1], x, y, gx+2, gy+1+i, r.screen())
			}
		}
	}
	for _, c := range []struct {
		glyph  string
		colour int
	}{{"✕", ansiRed}, {"◆", ansiYellow}, {"▶", ansiGreen}, {"◼", ansiCyan}} {
		if s := r.styleOf(c.glyph + " "); !isANSI(s.Fg, c.colour) {
			t.Fatalf("legend %s = %+v, want ANSI %d", c.glyph, s, c.colour)
		}
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
	r := startSizedOn(t, cluster, 80, 24)
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
	if strings.Contains(r.screen(), "reboot issued") {
		t.Fatalf("help fits; the test needs a longer help")
	}
	if strings.Contains(r.screen(), "↑ more") {
		t.Fatalf("help at the top shows ↑ more\n%s", r.screen())
	}
	r.send("G")
	r.waitFor("reboot issued")
	r.waitGone("↓ more")
	r.waitFor("↑ more")
	if first := strings.TrimSpace(r.dialogRows(r.dialogBox("Help "))[0]); first != "↑ more" {
		t.Fatalf("first help row = %q, want ↑ more\n%s", first, r.screen())
	}
	r.send("g")
	r.waitFor("↓ more")
	r.waitGone("↑ more")
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
