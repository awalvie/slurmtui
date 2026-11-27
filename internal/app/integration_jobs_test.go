//go:build integration

package app_test

import (
	"fmt"
	"strings"
	"testing"
)

func checkCells(t *testing.T, id string, row tableRow, want map[string]string) {
	t.Helper()
	for col, v := range want {
		if row.cells[col] != v {
			t.Errorf("job %s %s = %q, want %q", id, col, row.cells[col], v)
		}
	}
}

func checkWait(t *testing.T, id string, row tableRow) {
	t.Helper()
	if wait := row.cells["TIME"]; wait == "" || strings.HasPrefix(wait, "-") {
		t.Errorf("job %s TIME = %q, want the wait since submit", id, wait)
	}
}

func TestRealClusterJobsViewShowsHeldJob(t *testing.T) {
	cluster := connectRealCluster(t)
	name := uniqueReason()
	id := fmt.Sprint(cluster.submitUntilCleanup(map[string]any{"name": name, "hold": true}))

	r := cluster.start()
	r.waitFor("┌ Nodes ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor(name)

	row := r.rowByName(id)
	checkCells(t, id, row, map[string]string{
		"NAME": name, "USER": cluster.user, "STATE": "◷ PENDING", "TIME_LEFT": "",
		"NODES": "1", "NODELIST(REASON)": "(JobHeldUser)",
	})
	checkWait(t, id, row)
}

func TestRealClusterJobsViewShowsHeldArrayAsOneRecord(t *testing.T) {
	cluster := connectRealCluster(t)
	name := uniqueReason()
	id := fmt.Sprintf("%d_[1-3]", cluster.submitUntilCleanup(map[string]any{"name": name, "hold": true, "array": "1-3"}))

	r := cluster.start()
	r.waitFor("┌ Nodes ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor(name)

	row := r.rowByName(id)
	checkCells(t, id, row, map[string]string{
		"NAME": name, "STATE": "◷ PENDING", "NODELIST(REASON)": "(JobHeldUser)",
	})
	checkWait(t, id, row)
}

func TestRealClusterJobsViewShowsRunningJobs(t *testing.T) {
	cluster := connectRealCluster(t)
	running := cluster.runningPlainJobs()
	if len(running) == 0 {
		t.Skip("no running job without array or het fields on the cluster; the test reads running jobs because a submitted job cannot start on a full cluster")
	}

	r := cluster.start()
	r.waitFor("┌ Nodes ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs ")

	for _, j := range running {
		id := fmt.Sprint(j.JobID)
		row := r.rowByName(id)
		checkCells(t, id, row, map[string]string{"STATE": "▶ RUNNING", "NODELIST(REASON)": j.Nodes})
		if row.cells["TIME"] == "" {
			t.Errorf("job %s TIME is blank for a running job", id)
		}
	}
}
