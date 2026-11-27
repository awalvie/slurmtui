//go:build integration

package app_test

import (
	"fmt"
	"strings"
	"testing"
)

func (r *run) selectRow(name string) {
	r.t.Helper()
	for range r.table() {
		at := r.selected()
		if at == name {
			return
		}
		r.send("j")
		r.waitUntil("the cursor to leave "+at, func() bool { return r.selected() != at })
	}
	r.waitSelected(name)
}

func TestRealClusterDrainAndResume(t *testing.T) {
	cluster := connectRealCluster(t)
	node := cluster.nodeNotDrained()
	cluster.resumeAtCleanupIfDrained(node.Name)
	reason := uniqueReason()

	r := cluster.start()
	r.waitFor("Cluster  slurmtui")
	r.rowByName(node.Name)
	r.selectRow(node.Name)

	r.send("D")
	r.waitFor("Drain " + node.Name + " on ")
	r.send(reason + "\r")

	r.waitFor("drain of " + node.Name + " accepted")
	r.waitUntil(node.Name+" drained", func() bool {
		row := r.rowByName(node.Name)
		return row.cells["REASON"] == reason && strings.HasPrefix(stateLabel(row.cells["STATE"]), "drain") && row.cells["BY"] == cluster.user
	})

	r.send("U")
	r.waitFor("Resume " + node.Name + " on ")
	r.waitFor("Reason: " + reason)
	r.send("\x1b[D")
	r.waitButton("Resume")
	r.send("\r")

	r.waitFor("resume of " + node.Name + " accepted")
	r.waitUntil(node.Name+" resumed", func() bool {
		row := r.rowByName(node.Name)
		return row.cells["REASON"] == "" && !strings.HasPrefix(stateLabel(row.cells["STATE"]), "drain")
	})
}

func TestRealClusterCancelHeldJob(t *testing.T) {
	cluster := connectRealCluster(t)
	id := fmt.Sprint(cluster.submitUntilCleanup(map[string]any{"name": "it-cancel", "hold": true}))

	r := cluster.start()
	r.waitFor("┌ Nodes ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs ")
	r.rowByName(id)
	r.selectRow(id)

	r.send(ctrlD)
	r.waitFor("Cancel job " + id + " on ")
	r.confirmCancel()

	r.waitFor("cancel of job " + id + " accepted")
	r.waitUntil(id+" cancelled", func() bool { return r.rowByName(id).cells["STATE"] == "✕ CANCELLED" })
}
