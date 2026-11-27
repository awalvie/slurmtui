//go:build integration

package app_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/hostlist"
)

func TestRealClusterDrillDownFollowsHostlists(t *testing.T) {
	cluster := connectRealCluster(t)
	running := cluster.runningPlainJobs()
	if len(running) == 0 {
		t.Skip("no running job without array or het fields on the cluster; the test reads running jobs because a submitted job cannot start on a full cluster")
	}
	job := running[0]
	id := fmt.Sprint(job.JobID)
	hosts, err := hostlist.Expand(job.Nodes)
	if err != nil {
		t.Fatalf("expand %q: %s", job.Nodes, err)
	}
	slices.Sort(hosts)

	r := cluster.start()
	r.waitFor("┌ Nodes ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs ")
	r.send("/^" + id + " \r")
	r.waitFor(" · /^" + id + "  ─")
	r.send("o")
	r.waitFor(fmt.Sprintf("┌ Nodes %d · of %s ", len(hosts), id))
	got := strings.Fields(names(r))
	slices.Sort(got)
	if !slices.Equal(got, hosts) {
		t.Fatalf("nodes of job %s = %v, want %v", id, got, hosts)
	}

	r.send("\r")
	r.waitFor("┌ Jobs ")
	r.waitFor(fmt.Sprintf(" · on %s ", hosts[0]))
	r.rowByName(id)
}
