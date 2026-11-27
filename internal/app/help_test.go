package app_test

import (
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestHelpIsABlueFrameOverTheFadedScreenWithALegend(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.send("?")

	r.waitFor("┌ Keys ")
	b := r.dialogBox("Keys ")
	if b.width != 64 {
		t.Fatalf("help width = %d, want 64", b.width)
	}
	if s := r.cellAt(b.x, b.y).Style; !isANSI(s.Fg, ansiBlue) {
		t.Fatalf("help border = %+v, want blue", s)
	}
	if c := r.cellAt(gridLeft, 0); !isFaintWithoutColour(c.Style) {
		t.Fatalf("screen under help is not faded: %+v", c.Style)
	}
	rows := strings.Join(r.dialogRows(b), "\n")
	for _, item := range []string{"✕ down or failed", "◆ drain", "◐ mixed", "● allocated", "○ idle", "◌ other",
		"◷ pending", "▶ running", "◼ completing", "✓ completed",
		"* not responding", "~ powered off", "# powering up", "! power down pending",
		"% powering down", "$ maintenance", "@ reboot requested", "^ reboot issued"} {
		if !strings.Contains(rows, item) {
			t.Errorf("legend has no %q\n%s", item, rows)
		}
	}
	if s := r.styleOf("◐ mixed"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("legend is not faint: %+v", s)
	}
	if !strings.HasPrefix(r.bottomLine(), "   j/k scroll · esc close") {
		t.Fatalf("bottom line = %q, want the help hints", r.bottomLine())
	}
}

func TestLongHelpShowsMoreAndScrolls(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSized(t, clock.NewFake(startTime), 80, 24, []string{"--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")

	r.send("?")

	r.waitFor("┌ Keys ")
	b := r.dialogBox("Keys ")
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
	r.waitGone("┌ Keys ")
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
