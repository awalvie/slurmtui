package app_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func idle(name string) fakeslurm.Node {
	return fakeslurm.Node{Name: name, State: []string{"IDLE"}, CPUs: 64, RealMemory: 262144}
}

func TestNodesViewListsNodeNames(t *testing.T) {
	t.Parallel()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:    "lamp",
		Version: fakeslurm.Version{Major: "26", Minor: "05", Micro: "5"},
		Nodes:   []fakeslurm.Node{idle("n01"), idle("n02"), idle("n03")},
	})

	r := startProvided(t, cluster)

	r.waitFor("┌ Nodes 3 ")
	r.rowByName("n01")
	r.rowByName("n02")
	r.rowByName("n03")
	r.waitFor("Cluster  lamp")
	r.waitFor("User     alice")
	r.waitFor("Slurm    26.05.5")

	req := cluster.Requests()[0]
	if req.Method != "GET" || req.Path != "/slurm/v0.0.45/nodes" {
		t.Fatalf("first request = %s %s, want GET /slurm/v0.0.45/nodes", req.Method, req.Path)
	}
	if got := req.Header.Get("X-SLURM-USER-NAME"); got != "alice" {
		t.Fatalf("X-SLURM-USER-NAME = %q, want alice", got)
	}
	if got := req.Header.Get("X-SLURM-USER-TOKEN"); got != aliceToken {
		t.Fatalf("X-SLURM-USER-TOKEN = %q, want the Provided token", got)
	}
}

func TestNodeStateUsesSinfoLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		state []string
		want  string
	}{
		{"n-idle", []string{"IDLE"}, "idle"},
		{"n-mixed", []string{"MIXED"}, "mixed"},
		{"n-alloc", []string{"ALLOCATED"}, "allocated"},
		{"n-alloc-completing", []string{"ALLOCATED", "COMPLETING"}, "allocated+"},
		{"n-idle-completing", []string{"IDLE", "COMPLETING"}, "completing"},
		{"n-mixed-drain", []string{"MIXED", "DRAIN"}, "draining"},
		{"n-alloc-drain", []string{"ALLOCATED", "DRAIN"}, "draining"},
		{"n-idle-drain-cmpl", []string{"IDLE", "DRAIN", "COMPLETING"}, "draining"},
		{"n-idle-drain", []string{"IDLE", "DRAIN"}, "drained"},
		{"n-down-drain-noresp", []string{"DOWN", "DRAIN", "NOT_RESPONDING"}, "drained*"},
		{"n-idle-drain-maint", []string{"IDLE", "DRAIN", "MAINTENANCE"}, "drained$"},
		{"n-down", []string{"DOWN"}, "down"},
		{"n-down-noresp", []string{"DOWN", "NOT_RESPONDING"}, "down*"},
		{"n-idle-noresp", []string{"IDLE", "NOT_RESPONDING"}, "idle*"},
		{"n-idle-powered-down", []string{"IDLE", "POWERED_DOWN"}, "idle~"},
		{"n-idle-powering-up", []string{"IDLE", "POWERING_UP"}, "idle#"},
		{"n-idle-fail", []string{"IDLE", "FAIL"}, "fail"},
		{"n-mixed-fail", []string{"MIXED", "FAIL"}, "failing"},
		{"n-idle-maint", []string{"IDLE", "MAINTENANCE"}, "maint"},
		{"n-idle-reboot", []string{"IDLE", "REBOOT_REQUESTED"}, "reboot"},
		{"n-idle-reboot-issued", []string{"IDLE", "REBOOT_ISSUED"}, "reboot^"},
		{"n-mixed-reboot", []string{"MIXED", "REBOOT_REQUESTED"}, "mixed@"},
		{"n-idle-reserved", []string{"IDLE", "RESERVED"}, "reserved"},
		{"n-idle-planned", []string{"IDLE", "PLANNED"}, "planned"},
		{"n-mixed-planned", []string{"MIXED", "PLANNED"}, "mixed-"},
		{"n-invalid-reg", []string{"IDLE", "INVALID_REG"}, "inval"},
		{"n-unknown-cloud", []string{"UNKNOWN", "CLOUD"}, "cloud"},
		{"n-unknown", []string{"UNKNOWN"}, "unknown"},
		{"n-idle-fail-noresp", []string{"IDLE", "FAIL", "NOT_RESPONDING"}, "fail*"},
		{"n-alloc-fail-noresp", []string{"ALLOCATED", "FAIL", "NOT_RESPONDING"}, "failing*"},
		{"n-idle-maint-noresp", []string{"IDLE", "MAINTENANCE", "NOT_RESPONDING"}, "maint*"},
		{"n-idle-reboot-noresp", []string{"IDLE", "REBOOT_REQUESTED", "NOT_RESPONDING"}, "reboot*"},
		{"n-alloc-noresp", []string{"ALLOCATED", "NOT_RESPONDING"}, "allocated*"},
		{"n-idle-cmpl-pwr-up", []string{"IDLE", "COMPLETING", "POWERING_UP"}, "completing#"},
		{"n-idle-blocked", []string{"IDLE", "BLOCKED"}, "blocked"},
		{"n-unknown-resume", []string{"UNKNOWN", "RESUME"}, "resume"},
		{"n-unk-powered-down", []string{"UNKNOWN", "POWERED_DOWN"}, "powered_down"},
		{"n-unknown-undrain", []string{"UNKNOWN", "UNDRAIN"}, "undrain"},
		{"n-unknown-external", []string{"UNKNOWN", "EXTERNAL"}, "external"},
		{"n-down-maint", []string{"DOWN", "MAINTENANCE"}, "down$"},
		{"n-future", []string{"FUTURE"}, "future"},
		{"n-idle-pd-force", []string{"IDLE", "POWER_DOWN_FORCE"}, "idle~"},
		{"n-invalid", []string{"INVALID"}, "invalid"},
	}
	nodes := make([]fakeslurm.Node, 0, len(cases))
	for _, tc := range cases {
		n := idle(tc.name)
		n.State = tc.state
		nodes = append(nodes, n)
	}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: nodes})

	r := startSizedOn(t, cluster, screenWidth, len(cases)+12)
	r.waitFor(fmt.Sprintf("┌ Nodes %d ", len(cases)))
	r.fullScreen("nodes", "Nodes")

	for _, tc := range cases {
		if got := stateLabel(r.rowByName(tc.name).cells["STATE"]); got != tc.want {
			t.Errorf("%s %v: STATE = %q, want %q", tc.name, tc.state, got, tc.want)
		}
	}
}

func TestNodeColumnsShowUsageAndReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	drained := fakeslurm.Node{
		Name: "n02", State: []string{"MIXED", "DRAIN"},
		CPUs: 64, AllocCPUs: 48, RealMemory: 262144, AllocMemory: 184320,
		Partitions:      []string{"gpu", "cpu"},
		Gres:            "gpu:a100:4(S:0-1)",
		GresUsed:        "gpu:a100:1(IDX:0)",
		Reason:          "bad DIMM slot 3",
		ReasonSetByUser: "alice",
		ReasonChangedAt: fakeslurm.Time(now.Add(-(2*24*time.Hour + 3*time.Hour))),
	}
	small := fakeslurm.Node{
		Name: "n10", State: []string{"MIXED"},
		CPUs: 8, AllocCPUs: 2, RealMemory: 8192, AllocMemory: 512,
		Partitions: []string{"debug"},
	}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{small, drained}})

	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")

	want := map[string]string{
		"STATE": "◆ draining", "CPU": "48/64", "MEM(GiB)": "180/256",
		"GRES": "1/4", "REASON": "bad DIMM slot 3", "BY": "alice", "AGE": "2d3h",
	}
	row := r.rowByName("n02")
	for col, v := range want {
		if row.cells[col] != v {
			t.Errorf("n02 %s = %q, want %q", col, row.cells[col], v)
		}
	}
	smallRow := r.rowByName("n10")
	if got := smallRow.cells["MEM(GiB)"]; got != "0.5/8.0" {
		t.Errorf("n10 MEM(GiB) = %q, want 0.5/8.0", got)
	}
	if got, want := r.columnNames(), "NAME STATE CPU MEM(GiB) GRES BY AGE REASON"; got != want {
		t.Errorf("columns = %q, want %q", got, want)
	}
	for _, col := range []string{"CPU", "MEM(GiB)", "GRES"} {
		if !r.rightAligned(col) {
			t.Errorf("%s is not right-aligned\nscreen:\n%s", col, r.screen())
		}
	}
	for _, col := range []string{"GRES", "REASON", "BY", "AGE"} {
		if smallRow.cells[col] != "" {
			t.Errorf("n10 %s = %q, want blank", col, smallRow.cells[col])
		}
	}
}

func TestNodesSortBySeverityThenNaturallyByName(t *testing.T) {
	t.Parallel()
	r := startOnNodes(t,
		idle("n10"), idle("n2"),
		nodeIn("off", "IDLE", "POWERED_DOWN"),
		nodeIn("busy", "ALLOCATED"),
		nodeIn("half", "MIXED"),
		nodeIn("drn", "MIXED", "DRAIN"),
		nodeIn("dead", "DOWN", "NOT_RESPONDING"),
		nodeIn("dead2", "IDLE", "DRAIN", "NOT_RESPONDING"),
	)

	waitNames(r, "dead dead2 drn half busy n2 n10 off")
	r.waitFor("STATE ↑")
}

func TestNodeStateColourComesFromStateFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		state []string
		want  int
	}{
		{"n-down-drain", []string{"DOWN", "DRAIN"}, ansiRed},
		{"n-idle-noresp", []string{"IDLE", "NOT_RESPONDING"}, ansiRed},
		{"n-idle-fail", []string{"IDLE", "FAIL"}, ansiRed},
		{"n-mixed-drain", []string{"MIXED", "DRAIN"}, ansiYellow},
		{"n-idle", []string{"IDLE"}, -1},
		{"n-alloc", []string{"ALLOCATED"}, ansiGreen},
	}
	nodes := []fakeslurm.Node{nodeIn("a0", "DOWN")}
	for _, tc := range cases {
		nodes = append(nodes, nodeIn(tc.name, tc.state...))
	}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: nodes})

	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 7 ")

	for _, tc := range cases {
		_, style := r.stateCellStyle(tc.name)
		if tc.want < 0 && style.Fg != nil || tc.want >= 0 && !isANSI(style.Fg, tc.want) {
			t.Errorf("%s STATE colour = %v, want ANSI %d (-1 is the default colour)", tc.name, style.Fg, tc.want)
		}
	}
}

func TestUnknownNodeStateIsAnError(t *testing.T) {
	t.Parallel()
	n := idle("n-odd")
	n.State = []string{"IDLE", "SPARKLING"}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})
	cluster.ExpectOffSpec("nodes[0].state[1]")

	r := startProvided(t, cluster)

	r.waitPopupText("Refresh failed", `/slurm/v0.0.45/nodes: node "n-odd": state: unknown value "SPARKLING"`)
}

func TestErrorBaseStateIsAnError(t *testing.T) {
	t.Parallel()
	n := idle("n-err")
	n.State = []string{"ERROR"}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})

	r := startProvided(t, cluster)

	r.waitPopupText("Refresh failed", `node "n-err": state: unknown value "ERROR"`)
}

func TestReasonWithoutUserShowsBlankBy(t *testing.T) {
	t.Parallel()
	n := idle("n-anon")
	n.State = []string{"IDLE", "DRAIN"}
	n.Reason = "slurmctld reason"
	n.ReasonChangedAt = fakeslurm.Time(time.Date(2026, 11, 26, 11, 0, 0, 0, time.UTC))
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})

	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	row := r.rowByName("n-anon")
	if row.cells["REASON"] != "slurmctld reason" || row.cells["BY"] != "" || row.cells["AGE"] != "1h" {
		t.Fatalf("row = %v, want reason shown, BY blank, AGE 1h", row.cells)
	}
}

func TestInfiniteReasonTimeIsAnError(t *testing.T) {
	t.Parallel()
	n := idle("n-inf")
	n.Reason = "odd"
	n.ReasonChangedAt = fakeslurm.NoVal{Set: true, Infinite: true}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})

	r := startProvided(t, cluster)

	r.waitPopupText("Refresh failed", `node "n-inf": reason_changed_at: infinite`)
}

func TestReasonTimeInTheFutureShowsNegativeAge(t *testing.T) {
	t.Parallel()
	n := idle("n-skew")
	n.State = []string{"IDLE", "DRAIN"}
	n.Reason = "clock skew"
	n.ReasonSetByUser = "alice"
	n.ReasonChangedAt = fakeslurm.Time(time.Date(2026, 11, 26, 12, 0, 30, 0, time.UTC))
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})

	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	if got := r.rowByName("n-skew").cells["AGE"]; got != "-30s" {
		t.Fatalf("AGE = %q, want -30s", got)
	}
}

func TestGresShowsGPUsInUseOverGPUsConfigured(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, gres, used, want string
	}{
		{"n1", "gpu:a100:4(S:0-1)", "gpu:a100:1(IDX:0)", "1/4"},
		{"n2", "", "", ""},
		{"n3", "gpu:a100:2(S:0),gpu:h100:4(S:1)", "gpu:a100:1(IDX:0),gpu:h100:3(IDX:0,2-3)", "4/6"},
		{"n4", "gpu:2,shard:8", "gpu:0,shard:2", "0/2"},
		{"n5", "gpu", "gpu:1", "1/1"},
		{"n6", "shard:8", "shard:0", ""},
	}
	nodes := make([]fakeslurm.Node, 0, len(cases))
	for _, tc := range cases {
		n := idle(tc.name)
		n.Gres, n.GresUsed = tc.gres, tc.used
		nodes = append(nodes, n)
	}
	r := startOnNodes(t, nodes...)
	r.waitFor("┌ Nodes 6 ")

	for _, tc := range cases {
		if got := r.rowByName(tc.name).cells["GRES"]; got != tc.want {
			t.Errorf("%s gres %q used %q: GRES = %q, want %q", tc.name, tc.gres, tc.used, got, tc.want)
		}
	}
}

func TestGresThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()
	odd := idle("n7")
	odd.Gres = "gpu:a100:four"
	r := startOnNodes(t, idle("n1"), odd)

	r.waitFor(`node n7: gres "gpu:a100:four"`)
	if got := r.rowByName("n7").cells["GRES"]; got != "" {
		t.Errorf("n7 GRES = %q, want blank", got)
	}
}
