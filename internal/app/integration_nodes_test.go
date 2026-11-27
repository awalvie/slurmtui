//go:build integration

package app_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestRealClusterNodesViewListsEveryNode(t *testing.T) {
	cluster := connectRealCluster(t)
	nodes := cluster.nodes()

	r := cluster.start()

	r.waitFor(fmt.Sprintf("┌ Nodes %d ", len(nodes)))
	r.waitFor("Cluster  slurmtui")
	r.waitFor("Slurm    26.05.")
	for _, n := range nodes {
		if gres := r.rowByName(n.Name).cells["GRES"]; gres != "" {
			t.Errorf("%s GRES = %q, want blank on a cluster with no GPU", n.Name, gres)
		}
	}
}

func TestRealClusterNodesViewShowsDrainReasonAndUser(t *testing.T) {
	cluster := connectRealCluster(t)
	node := cluster.nodeNotDrained()
	reason := uniqueReason()
	cluster.drainUntilCleanup(node.Name, reason)

	r := cluster.start()

	r.waitFor(reason)
	row := r.rowByName(node.Name)
	if got := row.cells["REASON"]; got != reason {
		t.Errorf("%s REASON = %q, want %q", node.Name, got, reason)
	}
	if state := row.cells["STATE"]; !strings.HasPrefix(stateLabel(state), "drain") {
		t.Errorf("%s STATE = %q, want drained or draining", node.Name, state)
	}
	if by := row.cells["BY"]; by != cluster.user {
		t.Errorf("%s BY = %q, want %q", node.Name, by, cluster.user)
	}
}
