package app_test

import (
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func names(r *run) string {
	var got []string
	for _, row := range r.table() {
		got = append(got, row.key())
	}
	return strings.Join(got, " ")
}

func waitNames(r *run, want string) {
	r.t.Helper()
	r.waitUntil("rows "+want, func() bool { return names(r) == want })
}

func startOnNodes(t *testing.T, nodes ...fakeslurm.Node) *run {
	t.Helper()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: nodes})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes ")
	return r
}

func TestFilterKeepsRowsThatMatchTheRegex(t *testing.T) {
	r := startOnNodes(t, idle("n1"), idle("n2"), idle("gpu1"))
	waitNames(r, "gpu1 n1 n2")

	r.send("/^n\r")

	r.waitFor("┌ Nodes 2 · /^n ─")
	waitNames(r, "n1 n2")
}

func TestFilterWithBangInvertsTheMatch(t *testing.T) {
	r := startOnNodes(t, idle("n1"), idle("n2"), idle("gpu1"))

	r.send("/! gpu\r")

	r.waitFor("┌ Nodes 2 · /! gpu ─")
	waitNames(r, "n1 n2")
}

func TestFilterMatchesAnyCell(t *testing.T) {
	drained := idle("n2")
	drained.State = []string{"IDLE", "DRAIN"}
	drained.Reason = "bad DIMM"
	r := startOnNodes(t, idle("n1"), drained)

	r.send("/DIMM\r")

	waitNames(r, "n2")
}

func TestFilterWithNoMatchShowsZeroRows(t *testing.T) {
	r := startOnNodes(t, idle("n1"))

	r.send("/x\r")

	r.waitFor("┌ Nodes 0 · /x ─")
}

func TestEscWhileTypingPutsBackTheFilterFromBefore(t *testing.T) {
	r := startOnNodes(t, idle("n1"), idle("n2"), idle("gpu1"))
	r.send("/^n\r")
	r.waitFor("┌ Nodes 2 · /^n ─")

	r.send("/gpu")
	r.waitFor("┌ Nodes 1 · /gpu ─")
	r.send(escKey)

	r.waitFor("┌ Nodes 2 · /^n ─")
	waitNames(r, "n1 n2")
}

func TestBadFilterRegexIsAnError(t *testing.T) {
	r := startOnNodes(t, idle("n1"))

	r.send("/n[\r")

	r.waitFor("┌ Nodes 0 · /n[ ─")
	r.waitFor("filter \"n[\": error parsing regexp")
}

func TestEscClearsFilterThenGoesBackOneCrumb(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1"), idle("n2")},
		Jobs:  []fakeslurm.Job{runningJob(1), runningJob(2)},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")
	r.fullScreen("nodes", "Nodes")

	r.send("\r")
	r.waitFor("┌ Jobs 2 · on n1 ")
	r.send("/^1\r")
	r.waitFor("┌ Jobs 1 · on n1 · 1 running · /^1 ─")

	r.send(escKey)
	r.waitFor("┌ Jobs 2 · on n1 · 2 running ─")
	r.send(escKey)
	r.waitUntil("Nodes at full screen", func() bool { return r.frameTop("Nodes") >= 0 && r.frameTop("Jobs") < 0 })
	r.waitSelected("n1")
}

func TestShiftNSortsByNameWithNewestIDFirstAndAgainReverses(t *testing.T) {
	named := func(id int32, name string) fakeslurm.Job {
		j := runningJob(id)
		j.Name = name
		return j
	}
	r, _ := startOnJobs(t, []fakeslurm.Job{named(1, "beta"), named(2, "alpha"), named(3, "gamma"), named(4, "alpha")})
	waitNames(r, "4 3 2 1")

	r.send("N")
	waitNames(r, "4 2 1 3")
	r.waitFor("NAME ↑")
	r.send("N")
	waitNames(r, "3 1 2 4")
	r.waitFor("NAME ↓")
}

func TestShiftSReversesTheDefaultOrder(t *testing.T) {
	drained := nodeIn("n1", "IDLE", "DRAIN")
	down := nodeIn("n2", "DOWN")
	r := startOnNodes(t, drained, down, idle("n3"))
	waitNames(r, "n2 n1 n3")
	r.waitFor("STATE ↑")

	r.send("S")
	waitNames(r, "n3 n1 n2")
	r.waitFor("STATE ↓")
	r.send("S")
	waitNames(r, "n2 n1 n3")
	r.waitFor("STATE ↑")
}

func TestShiftSAfterShiftNGoesBackToTheDefaultOrder(t *testing.T) {
	r := startOnNodes(t, nodeIn("a1", "IDLE"), nodeIn("b1", "DOWN"))
	waitNames(r, "b1 a1")

	r.send("N")
	waitNames(r, "a1 b1")
	r.send("S")
	waitNames(r, "b1 a1")
	r.waitFor("STATE ↑")
}

func TestEnterOnNodeShowsJobsWhoseHostlistHasTheNode(t *testing.T) {
	onN2 := runningJob(1)
	onN2.Nodes = "n[1-3]"
	elsewhere := runningJob(2)
	elsewhere.Nodes = "n[3-4]"
	alone := runningJob(3)
	alone.Nodes = "n2"
	pending := pendingJob(4, "Priority")
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1"), idle("n2")},
		Jobs:  []fakeslurm.Job{onN2, elsewhere, alone, pending},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")

	r.send("j\r")

	r.waitFor("┌ Jobs 2 · on n2 ")
	waitNames(r, "3 1")
}

func TestOOnJobShowsNodesOfItsHostlist(t *testing.T) {
	job := runningJob(7)
	job.Nodes = "n[2-3]"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1"), idle("n2"), idle("n3")},
		Jobs:  []fakeslurm.Job{job},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 3 ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 1 ")

	r.send("o")

	r.waitFor("┌ Nodes 2 · of 7 ")
	waitNames(r, "n2 n3")
}

func TestOInTheOverviewJobsPaneShowsNodesOfTheJob(t *testing.T) {
	job := runningJob(7)
	job.Nodes = "n[2-3]"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1"), idle("n2"), idle("n3")},
		Jobs:  []fakeslurm.Job{job},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 1 ")

	r.send("\to")

	r.waitFor("┌ Nodes 2 · of 7 ")
	waitNames(r, "n2 n3")
	r.send(escKey)
	r.waitFor("┌ Jobs 1 ")
	r.waitFor("┌ Nodes 3 ")
}

func TestBadHostlistIsAnErrorThatNamesJobAndValue(t *testing.T) {
	bad := runningJob(9)
	bad.Nodes = "n[1-"
	r, _ := startOnJobs(t, []fakeslurm.Job{bad})

	r.send("o")

	r.waitFor("┌ Nodes 0 · of 9 ")
	r.waitFor(`job 9: nodes "n[1-": hostlist "n[1-": missing ']'`)
}

func TestCursorKeysMove(t *testing.T) {
	r := startOnNodes(t, idle("n1"), idle("n2"), idle("n3"))
	r.waitSelected("n1")

	r.send("G")
	r.waitSelected("n3")
	r.send("k")
	r.waitSelected("n2")
	r.send("\x1b[A")
	r.waitSelected("n1")
	r.send("\x1b[B")
	r.waitSelected("n2")
	r.send("g")
	r.waitSelected("n1")
}

func TestHelpListsTheKeysInGroups(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{runningJob(1)})

	r.send("?")

	r.waitFor("┌ Help ")
	for _, group := range []string{"Nodes pane", "Jobs pane", "Both panes", "Dialogs", "Describe, JSON", "Commands"} {
		r.waitFor(group)
	}
	r.waitMatch(`│    ctrl-d   Cancel the job`)
	r.waitMatch(`│    shift-n  Sort by name`)

	r.send(escKey)
	r.waitGone("┌ Help ")
}

func TestQQuitsFromTableViews(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}, Jobs: []fakeslurm.Job{runningJob(1)}})
	for _, command := range []string{"", "nodes", "jobs"} {
		t.Run("view "+command, func(t *testing.T) {
			r := startProvided(t, cluster)
			r.waitFor("┌ Nodes 1 ")
			if command != "" {
				r.fullScreen(command, map[string]string{"nodes": "Nodes", "jobs": "Jobs"}[command])
			}

			r.send("q")

			if code := r.waitExit(); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
		})
	}
}

func TestQGoesBackFromDetailAndHelpAndTypesInInputs(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")
	if !strings.Contains(r.screen(), "q     quit") {
		t.Fatalf("hint grid has no q quit\n%s", r.screen())
	}

	r.send("d")
	r.waitFor("┌ Describe")
	r.send("q")
	r.waitGone("┌ Describe")
	r.send("y")
	r.waitFor("┌ JSON")
	r.send("q")
	r.waitGone("┌ JSON")

	r.send("?")
	r.waitFor("┌ Help")
	if !strings.Contains(r.screen(), "q        Quit") {
		t.Fatalf("help has no q row\n%s", r.screen())
	}
	r.send("q")
	r.waitGone("┌ Help")

	r.send("/q")
	r.waitFor("/q█")
	r.send(escKey)
	r.send(":q")
	r.waitFor(":q█")
	r.send(escKey)
	r.waitGone(":q█")

	r.send("D")
	r.waitFor("▸ Reason")
	r.send("q")
	r.waitFor("▸ Reason  q█")
	r.send(escKey)
	r.waitGone("▸ Reason")
	r.waitFor("┌ Nodes 1 ")
}
