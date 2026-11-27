package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

const ctrlD = "\x04"

func TestCancelOpensDialogWithKeepFocused(t *testing.T) {
	t.Parallel()
	r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(1234)})
	r.waitSelected("1234")

	r.send(ctrlD)

	r.waitFor("Cancel job 1234 on " + contextOf(cluster) + " (lamp)")
	r.waitButton("Keep")
}

func arrayTask(id, arrayID, task int32) fakeslurm.Job {
	j := pendingJob(id, "JobHeldUser")
	j.ArrayJobID, j.ArrayTaskID = fakeslurm.Number(int64(arrayID)), fakeslurm.Number(int64(task))
	return j
}

func arrayMeta(id int32, tasks string) fakeslurm.Job {
	j := pendingJob(id, "JobHeldUser")
	j.ArrayJobID, j.ArrayTaskString = fakeslurm.Number(int64(id)), tasks
	return j
}

func hetComponent(id, hetID, offset int32) fakeslurm.Job {
	j := runningJob(id)
	j.HetJobID, j.HetJobOffset = fakeslurm.Number(int64(hetID)), fakeslurm.Number(int64(offset))
	return j
}

func isJobCancel(req fakeslurm.Request) bool {
	return req.Method == http.MethodDelete
}

func jobCancels(cluster *fakeslurm.Server) []fakeslurm.Request {
	return requestsWhere(cluster, isJobCancel)
}

func (r *run) waitJobCancel(cluster *fakeslurm.Server) fakeslurm.Request {
	r.t.Helper()
	return r.waitOneRequest(cluster, "job cancel", isJobCancel)
}

func (r *run) confirmCancel() {
	r.t.Helper()
	r.waitButton("Keep")
	r.send("\x1b[D")
	r.waitButton("Cancel job")
	r.send("\r")
}

func TestCancelTargetFollowsArrayAndHetRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		job   fakeslurm.Job
		row   string
		title string
		scope string
		path  string
	}{
		{"array task", arrayTask(1240, 1234, 5), "1234_5", "Cancel job 1234_5 on", "", "/slurm/v0.0.45/job/1234_5"},
		{"last array task keeps the array ID", arrayTask(1234, 1234, 7), "1234_7", "Cancel job 1234_7 on", "", "/slurm/v0.0.45/job/1234_7"},
		{"array meta record", arrayMeta(1234, "3-100"), "1234_[3-100]", "Cancel job 1234 on", "all tasks of array 1234", "/slurm/v0.0.45/job/1234"},
		{"het component", hetComponent(1301, 1300, 1), "1300+1", "Cancel job 1300 on", "all components of het job 1300", "/slurm/v0.0.45/job/1300"},
		{"plain job", runningJob(1234), "1234", "Cancel job 1234 on", "", "/slurm/v0.0.45/job/1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, cluster := startOnJobs(t, []fakeslurm.Job{tc.job})
			r.waitSelected(tc.row)

			r.send(ctrlD)
			r.waitFor(tc.title)
			if tc.scope != "" {
				r.waitFor(tc.scope)
			} else if strings.Contains(r.screen(), "all ") {
				t.Errorf("dialog names a wider scope for %s\n%s", tc.row, r.screen())
			}
			r.confirmCancel()

			req := r.waitJobCancel(cluster)
			if req.Path != tc.path || req.Query != "" {
				t.Errorf("request = DELETE %s?%s, want DELETE %s with no query", req.Path, req.Query, tc.path)
			}
		})
	}
}

func TestCancelFlashesAcceptedAndRefreshesAtOnce(t *testing.T) {
	t.Parallel()
	r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(1234)})
	r.waitSelected("1234")

	r.send(ctrlD)
	r.confirmCancel()

	r.waitJobCancel(cluster)
	r.waitGone("Cancel job 1234 on")
	r.waitFor("cancel of job 1234 accepted")
	r.waitUntil("1234 cancelled in the table", func() bool { return r.rowByName("1234").cells["STATE"] == "✕ CANCELLED" })
}

func TestCancelKeepClosesWithoutRequest(t *testing.T) {
	t.Parallel()
	r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(1234)})
	r.waitSelected("1234")

	r.send(ctrlD)
	r.waitButton("Keep")
	r.send("\r")
	r.waitGone("Cancel job 1234 on")
	r.send(ctrlD)
	r.waitButton("Keep")
	r.send(escKey)
	r.waitGone("Cancel job 1234 on")

	if got := jobCancels(cluster); len(got) != 0 {
		t.Fatalf("Keep sent %d cancel requests", len(got))
	}
}

func TestCancelDialogFixesTargetAndPausesRefresh(t *testing.T) {
	t.Parallel()
	r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(1234)})
	r.waitSelected("1234")
	r.send(ctrlD)
	r.waitFor("Cancel job 1234 on")
	before := nodeRequests(cluster)

	cluster.SetJobs([]fakeslurm.Job{runningJob(1235)})
	r.advance(5 * time.Second)
	r.advance(5 * time.Second)

	if got := nodeRequests(cluster); got != before {
		t.Fatalf("refreshed %d times while the dialog was open", got-before)
	}
	r.confirmCancel()
	if req := r.waitJobCancel(cluster); req.Path != "/slurm/v0.0.45/job/1234" {
		t.Errorf("cancel path = %s, want the job the dialog opened on", req.Path)
	}
}
