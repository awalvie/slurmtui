package app_test

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func contextOf(cluster *fakeslurm.Server) string {
	return strings.TrimPrefix(cluster.URL, "http://")
}

func (r *run) waitMatch(pattern string) {
	r.t.Helper()
	re := regexp.MustCompile(pattern)
	r.waitUntil("screen to match "+pattern, func() bool { return re.MatchString(r.screen()) })
}

func (r *run) waitGone(text string) {
	r.t.Helper()
	r.waitUntil(text+" to leave the screen", func() bool { return !strings.Contains(r.screen(), text) })
}

func isNodeUpdate(req fakeslurm.Request) bool {
	return req.Method == http.MethodPost
}

func nodeUpdates(cluster *fakeslurm.Server) []fakeslurm.Request {
	return requestsWhere(cluster, isNodeUpdate)
}

func (r *run) waitNodeUpdate(cluster *fakeslurm.Server) fakeslurm.Request {
	r.t.Helper()
	return r.waitOneRequest(cluster, "node update", isNodeUpdate)
}

func drained(name, reason, by string, at time.Time) fakeslurm.Node {
	n := idle(name)
	n.State = []string{"IDLE", "DRAIN"}
	n.Reason, n.ReasonSetByUser, n.ReasonChangedAt = reason, by, fakeslurm.Time(at)
	return n
}

func recordedReply(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatalf("read recorded reply: %s", err)
	}
	return b
}

func startOnSecondNode(t *testing.T, nodes ...fakeslurm.Node) (*fakeslurm.Server, *run) {
	t.Helper()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: nodes})
	r := startProvided(t, cluster)
	r.waitFor(fmt.Sprintf("┌ Nodes %d ", len(nodes)))
	r.moveTo(nodes[1].Name)
	return cluster, r
}

func TestDrainOpensDialogWithReasonFocused(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("D")

	r.waitFor("Drain n02 on " + contextOf(cluster) + " (lamp)")
	r.waitMatch(`│\s+ Drain \s+ Keep \s+│`)
	r.send("bad DIMM")
	r.waitFor("▸ Reason  bad DIMM█")
}

func TestDrainSendsStateAndReasonAndRefreshesAtOnce(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("D")
	r.waitFor("▸ Reason")
	r.send("bad DIMM\r")

	checkBody(t, r.waitNodeUpdate(cluster), "/slurm/v0.0.45/node/n02", `{"state":["DRAIN"],"reason":"bad DIMM"}`)
	r.waitGone("Drain n02 on")
	r.waitFor("drain of n02 accepted")
	r.waitUntil("n02 drained in the table", func() bool { return r.rowByName("n02").cells["REASON"] == "bad DIMM" })
}

func TestDrainRefusesEmptyReason(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("D")
	r.waitFor("▸ Reason")
	r.send("  \r")

	r.waitFor("Reason is required")
	r.send("\t\r")
	r.waitFor("Reason is required")
	if got := nodeUpdates(cluster); len(got) != 0 {
		t.Fatalf("sent %d node updates with an empty reason", len(got))
	}
	r.send(escKey)
	r.waitGone("Drain n02 on")
}

func TestDrainKeepClosesWithoutRequest(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))

	r.send("D")
	r.waitFor("▸ Reason")
	r.send("x\t\t")
	r.waitButton("Keep")
	r.send("\r")

	r.waitGone("Drain n02 on")
	if got := nodeUpdates(cluster); len(got) != 0 {
		t.Fatalf("Keep sent %d node updates", len(got))
	}
}

func TestResumeDialogFocusesKeep(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), drained("n02", "bad DIMM", "bob", startTime.Add(-(2*time.Hour+3*time.Minute))))

	r.send("U")

	r.waitFor("Resume n02 on " + contextOf(cluster) + " (lamp)")
	r.waitFor("Set by bob, 2h3m ago")
	r.waitButton("Keep")
	r.send("\r")
	r.waitGone("Resume n02 on")
	if got := nodeUpdates(cluster); len(got) != 0 {
		t.Fatalf("Keep sent %d node updates", len(got))
	}
}

func TestResumeSendsResume(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), drained("n02", "bad DIMM", "bob", startTime))

	r.send("U")
	r.waitButton("Keep")
	r.send("\x1b[D")
	r.waitButton("Resume")
	r.send("\r")

	checkBody(t, r.waitNodeUpdate(cluster), "/slurm/v0.0.45/node/n02", `{"state":["RESUME"]}`)
	r.waitFor("resume of n02 accepted")
	r.waitUntil("n02 resumed in the table", func() bool { return r.rowByName("n02").cells["STATE"] == "○ idle" })
}

func TestRefreshPausesWhileDialogIsOpen(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))
	r.send("D")
	r.waitFor("Drain n02 on")
	before := nodeRequests(cluster)

	cluster.SetNodes([]fakeslurm.Node{idle("n01"), idle("n02"), idle("n03")})
	r.advance(5 * time.Second)
	r.advance(5 * time.Second)

	if got := nodeRequests(cluster); got != before {
		t.Fatalf("nodes requests = %d while the dialog was open, want %d", got, before)
	}
	r.send(escKey)
	r.waitGone("Drain n02 on")
	r.advance(5 * time.Second)
	r.waitFor("┌ Nodes 3 ")
}

func TestInfoFlashClearsAfterSixSeconds(t *testing.T) {
	_, r := startOnSecondNode(t, idle("n01"), idle("n02"))
	r.send("D")
	r.waitFor("▸ Reason")
	r.send("bad DIMM\r")
	r.waitFor("drain of n02 accepted")

	r.advance(5 * time.Second)
	r.waitFor("● 0s")
	r.waitFor("drain of n02 accepted")
	r.advance(time.Second)

	r.waitGone("drain of n02 accepted")
}
