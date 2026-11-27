package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func nodeRequests(cluster *fakeslurm.Server) int {
	return requestsTo(cluster, "/slurm/v0.0.45/nodes")
}

func TestRefreshShowsNewDataEachInterval(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
	r.advance(5 * time.Second)

	r.waitFor("┌ Nodes 2 ")
	r.waitFor("● ")
}

func TestRefreshFlagChangesInterval(t *testing.T) {
	for _, flag := range []string{"--refresh", "-r"} {
		t.Run(flag, func(t *testing.T) {
			cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
			r := start(t, []string{"--url", cluster.URL, "--user", "alice", flag, "2"}, map[string]string{"SLURM_JWT": aliceToken})
			r.waitFor("┌ Nodes 1 ")

			cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
			r.advance(2 * time.Second)

			r.waitFor("┌ Nodes 2 ")
		})
	}
}

func TestRefreshBelowTwoSecondsIsAStartupError(t *testing.T) {
	r := start(t, []string{"--url", "http://127.0.0.1:1", "--user", "alice", "--refresh", "1"}, map[string]string{"SLURM_JWT": aliceToken})

	if code := r.waitExit(); code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if got := r.stderr.String(); !strings.Contains(got, "--refresh must be at least 2 seconds, got 1") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestFooterShowsDataAge(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("● 0s")

	r.advance(time.Second)
	r.waitFor("● 1s")
	r.advance(time.Second)
	r.waitFor("● 2s")
}

func TestCtrlRRefreshesNow(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
	r.send("\x12")

	r.waitFor("┌ Nodes 2 ")
}

func TestFailedRefreshKeepsRowsAndShowsError(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetStatus(http.StatusInternalServerError)
	r.advance(5 * time.Second)

	r.waitFor("/slurm/v0.0.45/nodes: HTTP 500")
	r.waitFor("○ stale 5s")
	r.rowByName("n1")
	for _, secret := range []string{aliceToken, "X-SLURM-USER-TOKEN", "X-SLURM-USER-NAME"} {
		if strings.Contains(r.screen(), secret) {
			t.Fatalf("screen shows %q:\n%s", secret, r.screen())
		}
	}

	r.advance(5 * time.Second)
	r.waitFor("○ stale 10s")
	r.waitFor("HTTP 500")

	cluster.SetStatus(http.StatusOK)
	r.advance(5 * time.Second)
	r.waitFor("● 0s")
	if strings.Contains(r.screen(), "HTTP 500") {
		t.Fatalf("error still shown after a good refresh:\n%s", r.screen())
	}
}

func TestNonOKReplyShowsSlurmErrorDescription(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetStatus(http.StatusUnauthorized)
	cluster.SetErrors("Authentication failure")
	r.advance(5 * time.Second)

	r.waitFor("/slurm/v0.0.45/nodes: HTTP 401: slurm: Authentication failure")
}

func TestErrorsArrayIsAFailure(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetErrors("Unable to query nodes")
	r.advance(5 * time.Second)

	r.waitFor("Unable to query nodes")
	r.waitFor("○ stale")
	r.rowByName("n1")
}

func TestFirstLoadFailureRetriesEachTick(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	cluster.SetStatus(http.StatusBadGateway)
	r := startProvided(t, cluster)

	r.waitFor("HTTP 502")
	r.waitFor("┌ Nodes 0 ")

	cluster.SetStatus(http.StatusOK)
	r.advance(5 * time.Second)
	r.waitFor("┌ Nodes 1 ")
}

func TestOnlyOneRefreshAtATime(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	release := cluster.Hold()
	r.advance(5 * time.Second)
	r.waitUntil("second nodes request", func() bool { return nodeRequests(cluster) == 2 })
	r.advance(5 * time.Second)
	r.advance(5 * time.Second)
	cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
	release()

	r.waitFor("┌ Nodes 2 ")
	if got := nodeRequests(cluster); got != 2 {
		t.Fatalf("nodes requests = %d, want 2", got)
	}
}

func TestCursorFollowsNodeAcrossRefresh(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1"), idle("n2"), idle("n3")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 3 ")
	r.waitSelected("n1")

	r.send("j")
	r.waitSelected("n2")

	cluster.SetNodes([]fakeslurm.Node{idle("n0"), idle("n1"), idle("n2"), idle("n3")})
	r.advance(5 * time.Second)
	r.waitFor("┌ Nodes 4 ")
	r.waitSelected("n2")

	cluster.SetNodes([]fakeslurm.Node{idle("n0"), idle("n1"), idle("n3")})
	r.advance(5 * time.Second)
	r.waitFor("┌ Nodes 3 ")
	r.waitSelected("n3")

	cluster.SetNodes([]fakeslurm.Node{idle("n0")})
	r.advance(5 * time.Second)
	r.waitFor("┌ Nodes 1 ")
	r.waitSelected("n0")
}
