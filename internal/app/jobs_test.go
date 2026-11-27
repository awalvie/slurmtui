package app_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func runningJob(id int32) fakeslurm.Job {
	return fakeslurm.Job{
		JobID:        id,
		ArrayJobID:   fakeslurm.Number(0),
		ArrayTaskID:  fakeslurm.NoVal{},
		HetJobID:     fakeslurm.Number(0),
		HetJobOffset: fakeslurm.Number(0),
		JobState:     []string{"RUNNING"},
		StateReason:  "None",
		Partition:    "debug",
		Name:         "sleep",
		UserName:     "alice",
		Nodes:        "n1",
		NodeCount:    fakeslurm.Number(1),
		StartTime:    fakeslurm.Time(startTime.Add(-time.Minute)),
		EndTime:      fakeslurm.Time(startTime.Add(time.Hour)),
	}
}

func TestJobsCommandShowsJobsAndNodesCommandGoesBack(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1")},
		Jobs:  []fakeslurm.Job{runningJob(1234), runningJob(1235)},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.fullScreen("jobs", "Jobs")

	r.waitFor("┌ Jobs 2 ")
	r.rowByName("1234")
	r.rowByName("1235")

	r.fullScreen("nodes", "Nodes")
	r.waitFor("┌ Nodes 1 ")
}

func startOnJobs(t *testing.T, jobs []fakeslurm.Job) (*run, *fakeslurm.Server) {
	t.Helper()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}, Jobs: jobs})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")
	r.fullScreen("jobs", "Jobs")
	r.waitFor(fmt.Sprintf("┌ Jobs %d ", len(jobs)))
	return r, cluster
}

func pendingJob(id int32, reason string) fakeslurm.Job {
	j := runningJob(id)
	j.JobState = []string{"PENDING"}
	j.StateReason = reason
	j.Nodes = ""
	j.StartTime = fakeslurm.Time(startTime.Add(time.Hour))
	return j
}

func endedJob(id int32, state ...string) fakeslurm.Job {
	j := runningJob(id)
	j.JobState = state
	j.StartTime = fakeslurm.Time(startTime.Add(-10 * time.Minute))
	j.EndTime = fakeslurm.Time(startTime.Add(-7 * time.Minute))
	return j
}

func TestShortViewCommands(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{runningJob(7)})

	r.fullScreen("no", "Nodes")
	r.waitFor("┌ Nodes 1 ")
	r.fullScreen("j", "Jobs")
	r.waitFor("┌ Jobs 1 ")
}

func TestJobColumnsFollowSqueue(t *testing.T) {
	job := runningJob(1234)
	job.Partition = "gpu"
	job.Name = "train"
	job.UserName = "bob"
	job.Nodes = "n[01-04]"
	job.NodeCount = fakeslurm.Number(4)
	job.StartTime = fakeslurm.Time(startTime.Add(-(4*time.Hour + 3*time.Minute)))
	job.TimeLimit = fakeslurm.Number(6 * 60)
	r, _ := startOnJobs(t, []fakeslurm.Job{job})

	want := map[string]string{
		"ID": "1234", "PARTITION": "gpu", "NAME": "train", "USER": "bob",
		"STATE": "▶ RUNNING", "TIME": "4h3m", "TIME_LEFT": "1h57m", "NODES": "4", "NODELIST(REASON)": "n[01-04]",
	}
	row := r.rowByName("1234")
	for col, v := range want {
		if row.cells[col] != v {
			t.Errorf("%s = %q, want %q", col, row.cells[col], v)
		}
	}
	if got, want := r.columnNames(), "ID PARTITION NAME USER STATE TIME TIME_LEFT NODES NODELIST(REASON)"; got != want {
		t.Errorf("columns = %q, want %q", got, want)
	}
	for _, col := range []string{"TIME", "TIME_LEFT", "NODES"} {
		if !r.rightAligned(col) {
			t.Errorf("%s is not right-aligned\nscreen:\n%s", col, r.screen())
		}
	}
}

func TestTimeLeftIsTheLimitMinusTheElapsedTime(t *testing.T) {
	withLimit := func(j fakeslurm.Job, limit fakeslurm.NoVal) fakeslurm.Job {
		j.TimeLimit = limit
		return j
	}
	fourHours := fakeslurm.Number(240)
	running := withLimit(runningJob(1), fourHours)
	running.StartTime = fakeslurm.Time(startTime.Add(-40 * time.Minute))
	unlimited := withLimit(runningJob(2), fakeslurm.NoVal{Infinite: true})
	partitionLimit := withLimit(runningJob(3), fakeslurm.NoVal{})
	partitionLimit.Partition = "gpu"
	partitionDefault := withLimit(runningJob(9), fakeslurm.NoVal{})
	partitionDefault.Partition = "short"
	unlimitedPartition := withLimit(runningJob(10), fakeslurm.NoVal{})
	pending := withLimit(pendingJob(4, "Priority"), fourHours)
	ended := withLimit(endedJob(5, "COMPLETED"), fourHours)
	overdue := withLimit(runningJob(6), fakeslurm.Number(10))
	overdue.StartTime = fakeslurm.Time(startTime.Add(-time.Hour))
	suspended := withLimit(jobIn(7, "SUSPENDED"), fakeslurm.Number(30))
	completing := withLimit(jobIn(8, "RUNNING", "COMPLETING"), fakeslurm.Number(30))
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n1")},
		Jobs:  []fakeslurm.Job{running, unlimited, partitionLimit, pending, ended, overdue, suspended, completing, partitionDefault, unlimitedPartition},
		Partitions: []fakeslurm.Partition{
			fakeslurm.DebugPartition,
			{Name: "gpu", MaxTime: fourHours},
			{Name: "short", DefaultTime: fakeslurm.Number(30), MaxTime: fourHours},
		},
	})
	r := startProvided(t, cluster)
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 10 ")

	want := map[string]string{"1": "3h20m", "2": "UNLIMITED", "3": "3h59m", "4": "", "5": "", "6": "0s", "7": "29m", "8": "29m", "9": "29m", "10": "UNLIMITED"}
	for id, v := range want {
		if got := r.rowByName(id).cells["TIME_LEFT"]; got != v {
			t.Errorf("job %s TIME_LEFT = %q, want %q", id, got, v)
		}
	}
}

func TestJobIDUsesSqueueForm(t *testing.T) {
	plain := runningJob(1234)

	arrayMeta := pendingJob(1300, "Resources")
	arrayMeta.ArrayJobID = fakeslurm.Number(1300)
	arrayMeta.ArrayTaskString = "3-100"

	splitTask := runningJob(1305)
	splitTask.ArrayJobID = fakeslurm.Number(1300)
	splitTask.ArrayTaskID = fakeslurm.Number(5)

	lastTask := runningJob(1400)
	lastTask.ArrayJobID = fakeslurm.Number(1400)
	lastTask.ArrayTaskID = fakeslurm.Number(100)

	hetComponent := runningJob(1501)
	hetComponent.HetJobID = fakeslurm.Number(1500)
	hetComponent.HetJobOffset = fakeslurm.Number(1)

	r, _ := startOnJobs(t, []fakeslurm.Job{plain, arrayMeta, splitTask, lastTask, hetComponent})

	for _, id := range []string{"1234", "1300_[3-100]", "1300_5", "1400_100", "1500+1"} {
		r.rowByName(id)
	}
}

func TestJobStateFlagHasPriorityOverBase(t *testing.T) {
	cases := []struct {
		id    int32
		state []string
		want  string
	}{
		{1, []string{"CANCELLED", "COMPLETING"}, "COMPLETING"},
		{2, []string{"RUNNING", "CONFIGURING"}, "CONFIGURING"},
		{3, []string{"PENDING", "REQUEUED"}, "REQUEUED"},
		{4, []string{"COMPLETED", "STAGE_OUT"}, "STAGE_OUT"},
		{5, []string{"RUNNING", "COMPLETING", "SIGNALING"}, "COMPLETING"},
		{6, []string{"FAILED", "LAUNCH_FAILED"}, "FAILED"},
		{7, []string{"OUT_OF_MEMORY"}, "OUT_OF_MEMORY"},
		{8, []string{"SUSPENDED"}, "SUSPENDED"},
	}
	jobs := make([]fakeslurm.Job, 0, len(cases))
	for _, tc := range cases {
		jobs = append(jobs, endedJob(tc.id, tc.state...))
	}
	r, _ := startOnJobs(t, jobs)

	for _, tc := range cases {
		if got := stateLabel(r.rowByName(fmt.Sprint(tc.id)).cells["STATE"]); got != tc.want {
			t.Errorf("%v: STATE = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestNodeListShowsReasonForPendingAndFailedJobs(t *testing.T) {
	pending := pendingJob(1, "Priority")
	described := pendingJob(2, "Resources")
	described.StateDescription = "waiting for n1"
	failed := endedJob(3, "FAILED")
	failed.StateReason = "NonZeroExitCode"
	timedOut := endedJob(4, "TIMEOUT")
	timedOut.StateReason = "TimeLimit"
	completingFailed := endedJob(5, "FAILED", "COMPLETING")
	completingFailed.StateReason = "NonZeroExitCode"
	completingFailed.Nodes = "n7"
	cancelled := endedJob(6, "CANCELLED")
	cancelled.Nodes = "n8"
	noReason := pendingJob(7, "None")

	r, _ := startOnJobs(t, []fakeslurm.Job{pending, described, failed, timedOut, completingFailed, cancelled, noReason})

	want := map[string]string{
		"1": "(Priority)", "2": "(waiting for n1)", "3": "(NonZeroExitCode)",
		"4": "(TimeLimit)", "5": "n7", "6": "n8", "7": "",
	}
	for id, v := range want {
		if got := r.rowByName(id).cells["NODELIST(REASON)"]; got != v {
			t.Errorf("job %s NODELIST(REASON) = %q, want %q", id, got, v)
		}
	}
}

func TestJobTimeIsElapsedOrTheWaitOfAPendingJob(t *testing.T) {
	running := runningJob(1)
	running.StartTime = fakeslurm.Time(startTime.Add(-90 * time.Second))
	pending := pendingJob(2, "Priority")
	pending.SubmitTime = fakeslurm.Time(startTime.Add(-30 * time.Second))
	ended := endedJob(3, "COMPLETED")
	r, _ := startOnJobs(t, []fakeslurm.Job{running, pending, ended})
	r.moveTo("3")

	want := map[string]string{"1": "90s", "2": "30s", "3": "3m"}
	for id, v := range want {
		if got := r.rowByName(id).cells["TIME"]; got != v {
			t.Errorf("job %s TIME = %q, want %q", id, got, v)
		}
	}
	faint := func(id string) bool {
		row := r.rowByName(id)
		return r.cellAt(r.columnX(row, "TIME"), row.y).Style.Attrs&uv.AttrFaint != 0
	}
	if !faint("2") || faint("1") {
		t.Errorf("TIME faint: pending %v, running %v; want only the pending wait faint", faint("2"), faint("1"))
	}

	r.advance(5 * time.Second)
	r.waitUntil("running TIME and pending wait to grow", func() bool {
		return r.rowByName("1").cells["TIME"] == "95s" && r.rowByName("2").cells["TIME"] == "35s"
	})
	if got := r.rowByName("3").cells["TIME"]; got != "3m" {
		t.Errorf("ended job TIME = %q after refresh, want 3m", got)
	}
}

func TestEndedJobsAreDim(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{
		runningJob(1),
		pendingJob(2, "Priority"),
		endedJob(3, "COMPLETED"),
		endedJob(4, "CANCELLED", "COMPLETING"),
	})

	want := map[string]bool{"1": false, "2": false, "3": true, "4": false}
	for id, faint := range want {
		row := r.rowByName(id)
		if got := r.cellAt(gridLeft, row.y).Style.Attrs&uv.AttrFaint != 0; got != faint {
			t.Errorf("job %s faint = %v, want %v", id, got, faint)
		}
	}
}

func TestJobsSortPendingByPriorityThenRunningThenEnded(t *testing.T) {
	pending := func(id int32, priority int64) fakeslurm.Job {
		j := pendingJob(id, "Priority")
		j.Priority = fakeslurm.Number(priority)
		return j
	}
	r, _ := startOnJobs(t, []fakeslurm.Job{
		endedJob(5, "COMPLETED"),
		pending(9, 100),
		runningJob(3),
		endedJob(8, "CANCELLED", "COMPLETING"),
		runningJob(12),
		pending(10, 100),
		pending(13, 0),
		pending(4, 500),
		jobIn(2, "SUSPENDED"),
		endedJob(11, "FAILED"),
	})

	waitNames(r, "4 10 9 13 12 8 3 2 11 5")
	r.waitFor("STATE ↑")
}

func TestCursorFollowsJobAcrossRefresh(t *testing.T) {
	r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(3), runningJob(2), runningJob(1)})
	r.waitSelected("3")
	r.send("j")
	r.waitSelected("2")

	cluster.SetJobs([]fakeslurm.Job{runningJob(4), runningJob(3), runningJob(2), runningJob(1)})
	r.advance(5 * time.Second)
	r.waitFor("┌ Jobs 4 ")
	r.waitSelected("2")

	r.fullScreen("no", "Nodes")
	r.waitFor("┌ Nodes 1 ")
	r.waitSelected("n1")
}

func TestUnknownJobStateIsAnError(t *testing.T) {
	odd := runningJob(42)
	odd.JobState = []string{"RUNNING", "SPARKLING"}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}, Jobs: []fakeslurm.Job{odd}})
	cluster.ExpectOffSpec("jobs[0].job_state[1]")

	r := startProvided(t, cluster)

	r.waitFor(`/slurm/v0.0.45/jobs: job 42: job_state: unknown value "SPARKLING"`)
}

func TestJobsAreFetchedWithNodesEachRefresh(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetJobs([]fakeslurm.Job{runningJob(1)})
	r.advance(5 * time.Second)
	r.waitUntil("a second jobs request", func() bool { return requestsTo(cluster, "/slurm/v0.0.45/jobs") == 2 })

	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 1 ")
}

func requestsTo(cluster *fakeslurm.Server, path string) int {
	return len(requestsWhere(cluster, func(req fakeslurm.Request) bool { return req.Path == path }))
}
