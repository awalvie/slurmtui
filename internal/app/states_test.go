package app_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func startSizedOn(t *testing.T, cluster *fakeslurm.Server, width, height int) *run {
	t.Helper()
	return startSized(t, clock.NewFake(startTime), width, height, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURM_JWT": aliceToken})
}

func TestSmallTerminalShowsOnlyTheMinimumSize(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	const message = "terminal too small (80x24 minimum)"
	for _, size := range []struct{ width, height int }{{79, 24}, {80, 23}} {
		r := startSizedOn(t, cluster, size.width, size.height)
		r.waitUntil("the size message", func() bool { return strings.HasPrefix(r.screen(), "   "+message) })
		if s := strings.TrimSpace(r.screen()); s != message {
			t.Fatalf("at %dx%d the screen shows more than the message:\n%s", size.width, size.height, s)
		}
		if !isANSI(r.cellAt(gridLeft, 0).Style.Fg, ansiRed) {
			t.Fatalf("at %dx%d the message is not red", size.width, size.height)
		}
	}
}

func wideCluster() fakeslurm.Cluster {
	drained := nodeIn("gpu-node-07", "IDLE", "DRAIN")
	drained.CPUs, drained.AllocCPUs = 128, 128
	drained.RealMemory, drained.AllocMemory = 1024*1024, 1000*1024
	drained.Gres, drained.GresUsed = "gpu:a100:4", "gpu:a100:4"
	drained.Reason, drained.ReasonSetByUser = "GPU 2 fell off the bus (Xid 79); nvidia-smi hangs, ticket INC-20417", "slurmadmin"
	drained.ReasonChangedAt = fakeslurm.Time(startTime.Add(-(13*24 + 4) * time.Hour))
	job := runningJob(4851)
	job.Partition, job.Name, job.Nodes = "gpu", "train-resnet-imagenet-full", "gpu-node-[01-06,08-12]"
	return fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{drained}, Jobs: []fakeslurm.Job{job}}
}

func TestNarrowPanesRemoveColumnsInTheirOrder(t *testing.T) {
	cluster := fakeslurm.Start(t, wideCluster())
	for _, tc := range []struct {
		width, height int
		nodes, jobs   string
	}{
		{100, 30, "NAME STATE CPU MEM(GiB) GRES BY AGE REASON", "ID PARTITION NAME USER STATE TIME TIME_LEFT NODES NODELIST(REASON)"},
		{80, 24, "NAME STATE CPU MEM(GiB) GRES AGE REASON", "ID NAME USER STATE TIME NODES NODELIST(REASON)"},
	} {
		r := startSizedOn(t, cluster, tc.width, tc.height)
		r.waitFor("┌ Nodes 1 ")
		if got := r.columnNamesIn("Nodes"); got != tc.nodes {
			t.Errorf("at %dx%d Nodes columns = %q, want %q\nscreen:\n%s", tc.width, tc.height, got, tc.nodes, r.screen())
		}
		if reason := r.rowByName("gpu-node-07").cells["REASON"]; !strings.HasSuffix(reason, "…") {
			t.Errorf("at %dx%d REASON = %q, want it cut with …", tc.width, tc.height, reason)
		}
		r.send(":jobs\r")
		r.waitFor("┌ Jobs 1 ")
		if got := r.columnNamesIn("Jobs"); got != tc.jobs {
			t.Errorf("at %dx%d Jobs columns = %q, want %q\nscreen:\n%s", tc.width, tc.height, got, tc.jobs, r.screen())
		}
	}
}

func (r *run) waitEmptyRow(title, text string) {
	r.t.Helper()
	r.waitUntil("faint row "+text, func() bool {
		top := r.frameTop(title)
		return top >= 0 && r.isEmptyMessage(top+2, text) && strings.Contains(strings.Split(r.screen(), "\n")[top+2], "│ "+text+" ")
	})
}

func TestEmptyPanesSayWhyTheyHaveNoRows(t *testing.T) {
	empty := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp"}))
	empty.waitFor("┌ Nodes 0 ─")
	empty.waitEmptyRow("Nodes", "no nodes")

	r := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01"), idle("n02")}}))
	r.waitFor("┌ Nodes 2 ")
	r.send("/x\r")
	r.waitFor("┌ Nodes 0 · /x ─")
	r.waitEmptyRow("Nodes", "no nodes match /x")

	r.send(escKey)
	r.waitFor("┌ Nodes 2 ")
	r.moveTo("n02")
	r.send("\r")
	r.waitFor("┌ Jobs 0 · on n02 ─")
	r.waitEmptyRow("Jobs", "no jobs on n02")
}

func TestLongEmptyRowIsCutAndKeepsTheRightMargin(t *testing.T) {
	r := startSizedOn(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01")}}), 80, 24)
	r.waitFor("┌ Nodes 1 ")
	filter := strings.Repeat("abcdefghij", 8)
	r.send("/" + filter + "\r")
	r.waitMatch(`no nodes match /[a-j]{20,}…`)

	top := r.mustFrameTop("Nodes")
	line := []rune(strings.Split(r.screen(), "\n")[top+2])
	margin := r.width - 3
	if got := string(line[margin-1]); got != "…" {
		t.Fatalf("empty row = %q, want it cut with … before the margin", string(line))
	}
	if got := r.cellAt(margin, top+2).Content; got != " " && got != "" {
		t.Fatalf("cell %d of the empty row = %q, want the 1-cell margin blank", margin, got)
	}
}

func TestFieldOfTheWrongTypeIsAnErrorThatNamesFieldAndValue(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	cluster.ExpectOffSpec("nodes[0].cpus")
	cluster.SetNodeField(0, "cpus", "sixty-four")

	r := startProvided(t, cluster)

	r.waitFor(`/slurm/v0.0.45/nodes: node "n1": cpus: "sixty-four" is not int`)
}

const noVisibleDelay = 100 * time.Millisecond * raceSlowdown

func TestJobsPaneStaysUsableWith5000Jobs(t *testing.T) {
	jobs := make([]fakeslurm.Job, 5000)
	for i := range jobs {
		jobs[i] = runningJob(int32(10000 + i))
		jobs[i].Name = fmt.Sprintf("job-%d", i)
	}
	r, _ := startOnJobs(t, jobs)
	r.waitSelected("14999")

	const moves = 20
	began := time.Now()
	for i := 1; i <= moves; i++ {
		r.send("j")
		r.waitSelected(strconv.Itoa(14999 - i))
	}
	perMove := time.Since(began) / moves
	if perMove > noVisibleDelay {
		t.Errorf("one cursor move takes %s, want at most %s", perMove, noVisibleDelay)
	}

	began = time.Now()
	r.send("/^14321 \r")
	r.waitFor("┌ Jobs 1 · 1 running · /^14321  ─")
	if took := time.Since(began); took > noVisibleDelay*time.Duration(len("/^14321 \r")) {
		t.Errorf("the filter takes %s, want at most %s for each key", took, noVisibleDelay)
	}
}

func TestNodesRightTextGoesWhenTheTitleFillsTheFrame(t *testing.T) {
	r := startSizedOn(t, fakeslurm.Start(t, wideCluster()), 80, 24)
	r.waitFor("─ cpu 100% · mem 97% ┐")
	r.send("/gpu-node-07|" + strings.Repeat("x", 30) + "\r")
	r.waitFor("/gpu-node-07|" + strings.Repeat("x", 30) + " ─")
	if top := strings.Split(r.screen(), "\n")[r.frameTop("Nodes")]; strings.Contains(top, "cpu") || !strings.HasSuffix(top, "─┐") {
		t.Fatalf("title row = %q, want no right text", top)
	}
	r.send(escKey + "/gpu-node-07|" + strings.Repeat("y", 90) + "\r")
	r.waitFor("yyy…")
	if top := []rune(strings.Split(r.screen(), "\n")[r.frameTop("Nodes")]); len(top) != r.width-1 || top[len(top)-1] != '┐' {
		t.Fatalf("title row = %q, want the title cut inside the frame", string(top))
	}
}
