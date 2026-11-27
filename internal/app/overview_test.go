package app_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func manyNodes(n int) []fakeslurm.Node {
	out := make([]fakeslurm.Node, n)
	for i := range out {
		out[i] = idle(fmt.Sprintf("n%02d", i+1))
	}
	return out
}

func startOverviewSized(t *testing.T, width, height int, cluster fakeslurm.Cluster) *run {
	t.Helper()
	r := startSizedOn(t, fakeslurm.Start(t, cluster), width, height)
	r.waitFor("┌ Jobs ")
	return r
}

func TestOverviewShowsNodesAboveJobsWithFocusOnNodes(t *testing.T) {
	t.Parallel()
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(3), Jobs: []fakeslurm.Job{runningJob(1)}})
	r.waitFor("Nodes 3 ")
	r.waitFor("Jobs 1 ")
	r.waitSelected("n01")

	_, nodesBottom := r.frameRows("Nodes")
	jobsTop, _ := r.frameRows("Jobs")
	if jobsTop <= nodesBottom {
		t.Fatalf("Jobs frame starts on row %d, want it under the Nodes frame (ends on row %d)", jobsTop, nodesBottom)
	}
	if !isANSI(r.cellAt(gridLeft, r.frameTop("Nodes")).Style.Fg, ansiBlue) {
		t.Fatalf("Nodes pane has no focus")
	}
}

func TestPaneHeightsFollowTheNodeCountAndTheBody(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		width, height, nodes                 int
		nodesTop, nodesEnd, jobsTop, jobsEnd int
	}{
		{160, 50, 3, 6, 11, 13, 48},
		{160, 50, 40, 6, 22, 24, 48},
		{80, 24, 40, 5, 11, 12, 22},
	} {
		t.Run(fmt.Sprintf("%dx%d with %d nodes", c.width, c.height, c.nodes), func(t *testing.T) {
			r := startOverviewSized(t, c.width, c.height, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(c.nodes), Jobs: []fakeslurm.Job{runningJob(1)}})
			check := func(when string) {
				t.Helper()
				nt, nb := r.frameRows("Nodes")
				jt, jb := r.frameRows("Jobs")
				if nt != c.nodesTop || nb != c.nodesEnd || jt != c.jobsTop || jb != c.jobsEnd {
					t.Fatalf("%s: Nodes rows %d-%d, Jobs rows %d-%d; want %d-%d and %d-%d\nscreen:\n%s", when, nt, nb, jt, jb, c.nodesTop, c.nodesEnd, c.jobsTop, c.jobsEnd, r.screen())
				}
			}
			r.waitFor(fmt.Sprintf("Nodes %d ", c.nodes))
			check("at start")
			r.send("\t")
			r.waitUntil("focus on Jobs", func() bool { return isANSI(r.cellAt(gridLeft, r.frameTop("Jobs")).Style.Fg, ansiBlue) })
			check("after tab")
			r.send("\tGk")
			r.waitUntil("cursor move", func() bool { return r.selected() != "n01" })
			check("after cursor moves")
			r.send("\r")
			r.waitFor("· on n")
			check("after enter on a node")
		})
	}
}

func TestTabMovesTheFocusAndDimsTheOtherPane(t *testing.T) {
	t.Parallel()
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(2), Jobs: []fakeslurm.Job{runningJob(1), runningJob(2)}})

	r.send("\t")
	r.waitSelected("2")

	jobsTop, nodesTop := r.frameTop("Jobs"), r.frameTop("Nodes")
	if c := r.cellAt(frameLeft, jobsTop); !isANSI(c.Style.Fg, ansiBlue) {
		t.Fatalf("Jobs border = %v, want blue", c.Style)
	}
	if c := r.cellAt(gridLeft, jobsTop); !isANSI(c.Style.Fg, ansiBlue) || c.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("Jobs name = %v, want blue bold", c.Style)
	}
	if c := r.cellAt(frameLeft, nodesTop); c.Style.Fg != nil || c.Style.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("Nodes border = %v, want faint", c.Style)
	}
	row := r.pane("Nodes")[0]
	lead := r.cellAt(leadX, row.y)
	if lead.Content != "▌" || lead.Style.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("Nodes cursor lead = %q %v, want a faint ▌", lead.Content, lead.Style)
	}
	if r.cellAt(gridLeft, row.y).Style.Attrs&uv.AttrReverse != 0 {
		t.Fatalf("Nodes cursor row is reversed without focus")
	}

	r.send("\x1b[Z")
	r.waitSelected("n01")
}

func TestEachPaneKeepsItsOwnFilter(t *testing.T) {
	t.Parallel()
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(3), Jobs: []fakeslurm.Job{runningJob(1), runningJob(2)}})

	r.send("/n0[12]\r")
	r.waitFor("┌ Nodes 2 · /n0[12] ")
	r.send("\t/^2\r")
	r.waitFor("┌ Jobs 1 · 1 running · /^2 ")
	r.waitFor("┌ Nodes 2 · /n0[12] ")
}

func TestEnterOnNodeScopesTheJobsPane(t *testing.T) {
	t.Parallel()
	onN02 := runningJob(1)
	onN02.Nodes = "n[01-02]"
	failed := endedJob(2, "FAILED")
	failed.Nodes = "n02"
	elsewhere := runningJob(3)
	elsewhere.Nodes = "n03"
	second := runningJob(4)
	second.Nodes = "n02"
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(3), Jobs: []fakeslurm.Job{onN02, failed, elsewhere, second}})
	r.waitSelected("n01")

	r.send("j\r")

	r.waitFor("┌ Jobs 3 · on n02 ")
	r.waitFor("Jobs shows n02 only · esc shows all jobs")
	waitNames(r, "4 1 2")
	r.waitSelected("4")
	r.send("\tk\t")
	r.waitSelected("4")
	r.waitUntil("Nodes cursor on n01", func() bool { return r.cellAt(leadX, r.rowByNameIn("Nodes", "n01").y).Content == "▌" })
	mark := r.cellAt(leadX, r.rowByNameIn("Nodes", "n02").y)
	if mark.Content != "▸" || mark.Style.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("scoped node lead = %q %v, want a faint ▸", mark.Content, mark.Style)
	}
}

func TestEscInJobsClearsFilterThenScopeThenWarns(t *testing.T) {
	t.Parallel()
	jobs := []fakeslurm.Job{runningJob(1), runningJob(2)}
	for i := range jobs {
		jobs[i].Nodes = "n01"
	}
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(2), Jobs: jobs})
	r.send("\r")
	r.waitFor("┌ Jobs 2 · on n01 ")
	r.send("/^1\r")
	r.waitFor("┌ Jobs 1 · on n01 · 1 running · /^1 ")
	r.waitHint("clear filter")

	r.send(escKey)
	r.waitFor("┌ Jobs 2 · on n01 · 2 running ─")
	r.waitHint("all jobs")
	r.send(escKey)
	r.waitFor("┌ Jobs 2 · 2 running ─")
	r.send(escKey)
	r.waitFor("nothing to clear")
	if !isANSI(r.flashStyle("nothing to clear").Fg, ansiYellow) {
		t.Fatalf("esc warning is not yellow: %q", r.bottomLine())
	}
}

func TestRightTextGoesWhenItDoesNotFit(t *testing.T) {
	t.Parallel()
	r := startOverviewSized(t, 80, 24, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(3)})
	r.waitFor("cpu 0% · mem 0%")
	r.send("/" + strings.Repeat("n", 50) + "\r")
	r.waitFor("/" + strings.Repeat("n", 50) + " ─")
	if strings.Contains(r.screen(), "cpu 0%") {
		t.Fatalf("right text shows with a long filter:\n%s", r.screen())
	}
}

func (r *run) hintArea() string {
	return strings.Join(r.lines()[:5], "\n")
}

func (r *run) waitHint(text string) {
	r.t.Helper()
	r.waitUntil("hint "+text, func() bool { return strings.Contains(r.hintArea(), text) })
}

func (r *run) waitNoHint(text string) {
	r.t.Helper()
	r.waitUntil("no hint "+text, func() bool { return !strings.Contains(r.hintArea(), text) })
}

func TestHintsFollowTheFocusedPaneAndRow(t *testing.T) {
	t.Parallel()
	drained := nodeIn("n1", "IDLE", "DRAIN")
	drained.Reason = "x"
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{drained, nodeIn("n2", "DOWN"), idle("n3")}, Jobs: []fakeslurm.Job{runningJob(1)}})

	r.waitSelected("n2")
	r.waitHint("D     drain")
	r.waitHint("U     resume")
	r.waitHint("tab   jobs pane")
	r.moveTo("n1")
	r.waitHint("D     edit reason")
	r.waitHint("U     resume")
	r.send("j")
	r.waitSelected("n3")
	r.waitNoHint("resume")
	r.waitHint("D     drain")
	r.send("/n\r")
	r.waitHint("clear filter")

	r.send("\t")
	r.waitHint("ctrl-d cancel")
	r.waitHint("tab    nodes pane")
	r.send("\t" + escKey + "k\r")
	r.waitHint("esc    all jobs")
}

func TestActionKeysInTheWrongPaneNameThePaneToFocus(t *testing.T) {
	t.Parallel()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(1), Jobs: []fakeslurm.Job{runningJob(1)}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 1 ")

	r.send("\x04")
	r.waitFor("ctrl-d cancels a job · press tab to focus Jobs")
	if !isANSI(r.flashStyle("ctrl-d cancels").Fg, ansiYellow) {
		t.Fatalf("warning is not yellow: %q", r.bottomLine())
	}
	r.send("\tD")
	r.waitFor("D works on a node · press tab to focus Nodes")
	r.send("U")
	r.waitFor("U works on a node · press tab to focus Nodes")
	if strings.Contains(r.screen(), "Reason") || strings.Contains(r.screen(), "Keep") {
		t.Fatalf("a dialog opened:\n%s", r.screen())
	}
}

func TestFullScreenViewsAndOverviewCommand(t *testing.T) {
	t.Parallel()
	job := runningJob(1234)
	job.Nodes = "n[02-03]"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(3), Jobs: []fakeslurm.Job{job}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 1 ")

	r.send(":nodes\r")
	r.waitUntil("one pane", func() bool { return r.frameTop("Jobs") < 0 })
	r.waitNoHint("tab")
	if top, bottom := r.frameRows("Nodes"); top != 6 || bottom != 48 {
		t.Fatalf("full-screen Nodes rows %d-%d, want 6-48", top, bottom)
	}
	r.send("j\r")
	r.waitFor("┌ Jobs 1 · on n02 ")
	r.waitUntil("one pane", func() bool { return r.frameTop("Nodes") < 0 })
	r.send("o")
	r.waitFor("┌ Nodes 2 · of 1234 ")
	r.send(escKey)
	r.waitFor("┌ Jobs 1 · on n02 ")
	r.send(escKey)
	r.waitUntil("Nodes alone", func() bool { return r.frameTop("Jobs") < 0 && r.frameTop("Nodes") >= 0 })
	r.send(escKey)
	r.waitUntil("Overview", func() bool { return r.frameTop("Jobs") >= 0 && r.frameTop("Nodes") >= 0 })

	r.send(":jobs\r")
	r.waitUntil("Jobs alone", func() bool { return r.frameTop("Nodes") < 0 })
	r.send(":ov\r")
	r.waitUntil("Overview", func() bool { return r.frameTop("Jobs") >= 0 && r.frameTop("Nodes") >= 0 })
	r.send(":jobs\r:overview\r")
	r.waitUntil("Overview", func() bool { return r.frameTop("Jobs") >= 0 && r.frameTop("Nodes") >= 0 })
	r.fullScreen("no", "Nodes")
	r.fullScreen("j", "Jobs")
}

func TestDescribeFromAPaneGoesBackWithTheSameFocusAndRow(t *testing.T) {
	t.Parallel()
	r := startOverviewSized(t, screenWidth, screenHeight, fakeslurm.Cluster{Name: "lamp", Nodes: manyNodes(2), Jobs: []fakeslurm.Job{runningJob(1), runningJob(2), runningJob(3)}})
	r.send("\tj")
	r.waitSelected("2")

	for _, key := range []string{"d", "y", "\r"} {
		r.send(key)
		r.waitUntil("detail view", func() bool { return r.frameTop("Jobs") < 0 })
		r.send(escKey)
		r.waitSelected("2")
		if !isANSI(r.cellAt(gridLeft, r.frameTop("Jobs")).Style.Fg, ansiBlue) {
			t.Fatalf("after %s and esc the Jobs pane has no focus", key)
		}
	}
}
