package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func local(t time.Time) string {
	return t.Local().Format(expiryLayout)
}

func describedNode() fakeslurm.Node {
	return fakeslurm.Node{
		Name: "n02", State: []string{"MIXED", "DRAIN", "NOT_RESPONDING"},
		CPUs: 64, AllocCPUs: 48, RealMemory: 262144, AllocMemory: 184320,
		Partitions:        []string{"gpu", "cpu"},
		Reason:            "bad DIMM slot 3",
		ReasonSetByUser:   "alice",
		ReasonChangedAt:   fakeslurm.Time(startTime.Add(-3 * time.Hour)),
		Features:          []string{"avx512", "ib"},
		Gres:              "gpu:a100:4",
		SpecializedCPUs:   "0-1",
		SpecializedMemory: 2048,
		FreeMem:           fakeslurm.Number(43749),
		CPULoad:           108,
		BootTime:          fakeslurm.Time(startTime.Add(-48 * time.Hour)),
		LastBusy:          fakeslurm.Time(startTime.Add(-5 * time.Minute)),
	}
}

func describeRow(key, value string) string {
	return "│ " + fmt.Sprintf("%-16s", key) + value
}

func TestDescribeNodeShowsAllItsData(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01"), describedNode()}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")

	r.moveTo("n02")
	r.send("d")

	r.waitFor("┌ Describe · n02 ")
	for _, line := range []string{
		describeRow("Name", "n02"),
		describeRow("State", "✕ MIXED+DRAIN+NOT_RESPONDING"),
		describeRow("Reason", "bad DIMM slot 3"),
		describeRow("Reason by", "alice"),
		describeRow("Reason at", local(startTime.Add(-3*time.Hour))),
		describeRow("Partitions", "gpu,cpu"),
		"│ CPU",
		describeRow("Allocated", "48"),
		describeRow("Total", "64"),
		describeRow("Specialized", "0-1"),
		describeRow("Load", "1.08"),
		"│ Memory",
		describeRow("Allocated MiB", "184320"),
		describeRow("Total MiB", "262144"),
		describeRow("Free MiB", "43749"),
		describeRow("Features", "avx512,ib"),
		describeRow("Gres", "gpu:a100:4"),
		describeRow("Reboot pending", "false"),
		"│ Times",
		describeRow("Boot time", local(startTime.Add(-48*time.Hour))),
		describeRow("Last busy", local(startTime.Add(-5*time.Minute))),
	} {
		r.waitFor(line)
	}
}

func TestDescribeStylesKeysEmptyValuesStatesAndGroups(t *testing.T) {
	n := describedNode()
	n.Features = nil
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.send("d")

	r.waitFor(describeRow("Features", "—"))
	top := r.frameTop("Describe")
	if c := r.cellAt(gridLeft, top); !isANSI(c.Style.Fg, ansiBlue) || c.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("Describe in the title = %+v, want blue bold", c.Style)
	}
	if s := r.styleOf(" · n02"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("title separator is not faint: %+v", s)
	}
	if s := r.styleOf("Reason by"); !isANSI(s.Fg, ansiCyan) {
		t.Fatalf("key is not cyan: %+v", s)
	}
	if s := r.styleOf("—"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("empty value is not faint: %+v", s)
	}
	if s := r.styleOf("✕ MIXED"); !isANSI(s.Fg, ansiRed) {
		t.Fatalf("state value is not in its class colour: %+v", s)
	}
	lines := strings.Split(r.screen(), "\n")
	for y, line := range lines {
		if strings.HasPrefix(line, " │ CPU") {
			if strings.TrimSpace(strings.Trim(lines[y-1], " │")) != "" {
				t.Fatalf("no blank row above the CPU group: %q", lines[y-1])
			}
			if s := r.cellAt(gridLeft, y).Style; s.Attrs&uv.AttrBold == 0 {
				t.Fatalf("group name is not bold: %+v", s)
			}
		}
	}
}

func TestYShowsRawNodeJSONFormatted(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{describedNode()}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.send("y")

	r.waitFor("┌ JSON · n02 ")
	r.waitFor(`│   "reason_set_by_user": "alice",`)
	r.waitFor(`│   "cpu_load": 108,`)
}

func TestYShowsRawJobJSON(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{describedJob()})

	r.send("y")

	r.waitFor("┌ JSON · 1234 ")
	r.waitFor(`│   "job_id": 1234,`)
	r.send("G")
	r.waitFor(`│   "standard_output": "/home/bob/run/slurm-%j.out",`)
}

func TestDescribeUpdatesAtEachRefresh(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{describedNode()}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")
	r.send("d")
	r.waitFor(describeRow("Reason", "bad DIMM slot 3"))

	changed := describedNode()
	changed.Reason = "DIMM replaced"
	cluster.SetNodes([]fakeslurm.Node{changed})
	r.advance(5 * time.Second)

	r.waitFor(describeRow("Reason", "DIMM replaced"))
}

func TestRawJSONScrolls(t *testing.T) {
	nodes := make([]string, 60)
	for i := range nodes {
		nodes[i] = fmt.Sprintf("p%02d", i)
	}
	n := describedNode()
	n.Partitions = nodes
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{n}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")
	r.send("y")
	r.waitFor(`"p00"`)

	r.send("G")

	r.waitFor(`"p59"`)
	r.waitUntil("first lines scrolled away", func() bool { return !strings.Contains(r.screen(), `"p00"`) })
	r.send("g")
	r.waitFor(`"p00"`)
}

func describedJob() fakeslurm.Job {
	j := runningJob(1234)
	j.Name = "train"
	j.UserName = "bob"
	j.Account = "physics"
	j.QOS = "high"
	j.JobState = []string{"RUNNING", "COMPLETING"}
	j.StateDescription = "waiting for epilog"
	j.Partition = "gpu"
	j.Nodes = "n[01-02]"
	j.SubmitTime = fakeslurm.Time(startTime.Add(-2 * time.Hour))
	j.StartTime = fakeslurm.Time(startTime.Add(-time.Hour))
	j.EndTime = fakeslurm.Time(startTime.Add(25 * time.Hour))
	j.TimeLimit = fakeslurm.Number(26*60 + 30)
	j.ExitCode = &fakeslurm.ExitCode{
		Status:     []string{"SIGNALED"},
		ReturnCode: fakeslurm.Number(0),
		Signal:     fakeslurm.Signal{ID: fakeslurm.Number(9), Name: "KILL"},
	}
	j.CurrentWorkingDirectory = "/home/bob/run"
	j.StandardOutput = "/home/bob/run/slurm-%j.out"
	j.Command = "/home/bob/run/train.sh"
	j.Priority = fakeslurm.Number(4294)
	return j
}

func TestDescribeJobShowsAllItsData(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{describedJob()})

	r.send("d")

	r.waitFor("┌ Describe · 1234 ")
	for _, line := range []string{
		describeRow("ID", "1234"),
		describeRow("Name", "train"),
		describeRow("User", "bob"),
		describeRow("Account", "physics"),
		describeRow("QOS", "high"),
		describeRow("State", "◼ RUNNING+COMPLETING (waiting for epilog)"),
		describeRow("Reason", "—"),
		describeRow("Priority", "4294"),
		describeRow("Dependency", "—"),
		describeRow("Partition", "gpu"),
		describeRow("Nodes", "n[01-02]"),
		describeRow("Node count", "1"),
		describeRow("Requested TRES", "—"),
		"│ Times",
		describeRow("Submit", local(startTime.Add(-2*time.Hour))),
		describeRow("Waiting", "—"),
		describeRow("Expected start", "—"),
		describeRow("Start", local(startTime.Add(-time.Hour))),
		describeRow("End", local(startTime.Add(25*time.Hour))),
		describeRow("Elapsed", "1h"),
		describeRow("Time limit", "1-02:30:00"),
		describeRow("Time left", "25h"),
		"│ Files",
		describeRow("Exit code", "0:9"),
		describeRow("Work dir", "/home/bob/run"),
		describeRow("StdOut", "/home/bob/run/slurm-%j.out"),
		describeRow("Command", "/home/bob/run/train.sh"),
	} {
		r.waitFor(line)
	}
}

func TestEnterOnJobDescribesIt(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{describedJob()})

	r.send("\r")

	r.waitFor("┌ Describe · 1234 ")
}

func TestJobTimeLimitForms(t *testing.T) {
	cases := []struct {
		limit fakeslurm.NoVal
		want  string
	}{
		{fakeslurm.Number(90), "01:30:00"},
		{fakeslurm.NoVal{Infinite: true}, "UNLIMITED"},
		{fakeslurm.NoVal{}, "Partition_Limit"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			j := runningJob(7)
			j.TimeLimit = c.limit
			r, _ := startOnJobs(t, []fakeslurm.Job{j})
			r.send("d")
			r.waitFor(describeRow("Time limit", c.want))
		})
	}
}

func TestDescribePendingJobShowsDependencyAndRequestedTRES(t *testing.T) {
	j := pendingJob(9, "Dependency")
	j.Dependency = "afterok:8(unfulfilled)"
	j.TRESRequested = "cpu=4,mem=8G,node=1,billing=4"
	r, _ := startOnJobs(t, []fakeslurm.Job{j})

	r.send("d")

	r.waitFor(describeRow("Dependency", "afterok:8(unfulfilled)"))
	r.waitFor(describeRow("Requested TRES", "cpu=4,mem=8G,node=1,billing=4"))
}

func TestDescribePendingJobWithNoExpectedStartShowsADash(t *testing.T) {
	j := pendingJob(9, "Priority")
	j.StartTime = fakeslurm.NoVal{Set: false, Infinite: true}
	r, _ := startOnJobs(t, []fakeslurm.Job{j})

	r.send("d")

	r.waitFor(describeRow("Expected start", "—"))
}

func TestDescribePendingJobShowsWaitAndExpectedStart(t *testing.T) {
	j := pendingJob(9, "Priority")
	j.SubmitTime = fakeslurm.Time(startTime.Add(-5 * time.Minute))
	r, _ := startOnJobs(t, []fakeslurm.Job{j})

	r.send("d")

	r.waitFor(describeRow("Reason", "Priority"))
	r.waitFor(describeRow("Waiting", "5m"))
	r.waitFor(describeRow("Expected start", local(startTime.Add(time.Hour))))
	r.waitFor(describeRow("Start", "—"))
}

func TestRawJSONColoursKeysStringsNumbersAndPunctuation(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{describedNode()}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.send("y")

	r.waitFor(`"reason_set_by_user": "alice",`)
	for _, c := range []struct {
		text   string
		colour int
		faint  bool
	}{
		{`"reason_set_by_user"`, ansiCyan, false},
		{`"alice"`, ansiGreen, false},
		{`108,`, ansiYellow, false},
		{`: "alice"`, 0, true},
		{`{`, 0, true},
	} {
		s := r.styleOf(c.text)
		if c.faint && (s.Attrs&uv.AttrFaint == 0 || s.Fg != nil) {
			t.Errorf("%s = %+v, want faint", c.text, s)
		}
		if !c.faint && !isANSI(s.Fg, c.colour) {
			t.Errorf("%s = %+v, want ANSI %d", c.text, s, c.colour)
		}
	}
}
