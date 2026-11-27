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

func TestHelpIsAFullViewUnderTheHeader(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1"), idle("n2")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")
	r.moveTo("n2")
	nodesTop := r.mustFrameTop("Nodes")

	r.send("?")

	r.waitFor("┌ Help ")
	r.waitGone("┌ Nodes")
	top, bottom := r.frameRows("Help")
	if top != nodesTop || bottom != r.height-2 {
		t.Fatalf("help frame is rows %d to %d, want %d to %d\n%s", top, bottom, nodesTop, r.height-2, r.screen())
	}
	if s := r.cellAt(frameLeft, top).Style; !isANSI(s.Fg, ansiBlue) {
		t.Fatalf("help border = %+v, want blue", s)
	}
	if s := r.styleOf("Cluster"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("header label = %+v, want it faint as without help", s)
	}
	if s := r.styleOf("lamp"); s.Attrs&uv.AttrFaint != 0 {
		t.Fatalf("header value = %+v, want a header that is not faded", s)
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
	rows := r.dialogRows(b)
	if last := strings.TrimSpace(rows[len(rows)-1]); last != "↓ more" {
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
