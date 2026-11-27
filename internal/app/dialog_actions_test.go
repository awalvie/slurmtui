package app_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func (r *run) buttonFocused(label string) bool {
	lines := r.lines()
	for y, line := range lines {
		at := strings.Index(line, " "+label+" ")
		if at < 0 {
			continue
		}
		x := len([]rune(line[:at])) + 1
		if r.cellAt(x, y).Style.Attrs&uv.AttrReverse != 0 {
			return true
		}
	}
	return false
}

func (r *run) waitButton(label string) {
	r.t.Helper()
	r.waitUntil("focus on button "+label, func() bool { return r.buttonFocused(label) })
}

func TestDialogTitlesAndColoursNameTheType(t *testing.T) {
	cases := []struct {
		name   string
		node   string
		keys   string
		title  string
		colour int
	}{
		{"drain", "n02", "D", "Drain n02 on %s (lamp)", ansiYellow},
		{"edit reason", "n05", "D", "Edit reason of n05 on %s (lamp)", ansiYellow},
		{"resume", "n02", "U", "Resume n02 on %s (lamp)", ansiBlue},
		{"cancel", "n02", "\t" + ctrlD, "Cancel job 4851 on %s (lamp)", ansiRed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n02"), drained("n05", "bad fan", "bob", startTime)}, Jobs: []fakeslurm.Job{runningJob(4851)}})
			r := startProvided(t, cluster)
			r.waitFor("┌ Jobs 1 ")
			r.moveTo(tc.node)

			r.send(tc.keys)

			title := fmt.Sprintf(tc.title, contextOf(cluster))
			r.waitFor(title)
			b := r.dialogBox(title)
			if s := r.cellAt(b.x+2, b.y).Style; !isANSI(s.Fg, tc.colour) || s.Attrs&uv.AttrBold == 0 {
				t.Fatalf("title style = %+v, want ANSI %d bold", s, tc.colour)
			}
			if s := r.cellAt(b.x, b.y+1).Style; !isANSI(s.Fg, tc.colour) {
				t.Fatalf("border style = %+v, want ANSI %d", s, tc.colour)
			}
		})
	}
}

func TestButtonsAreCentredAndKeepHasTheFocus(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), drained("n02", "bad DIMM", "alice", startTime))

	r.send("U")

	title := "Resume n02 on " + contextOf(cluster) + " (lamp)"
	r.waitFor(title)
	r.waitButton("Keep")
	b := r.dialogBox(title)
	row := ""
	for _, l := range r.dialogRows(b) {
		if strings.Contains(l, " Resume ") && strings.Contains(l, " Keep") {
			row = l
		}
	}
	if !strings.Contains(row, " Resume      Keep") {
		t.Fatalf("buttons row = %q, want labels with 1 space each side, 4 spaces apart", row)
	}
	left := len(row) - len(strings.TrimLeft(row, " "))
	right := b.width - 2 - len([]rune(row)) - 1
	if left-right > 1 || right-left > 1 {
		t.Fatalf("buttons row %q is not centred: %d cells left, %d right", row, left, right)
	}
	if s := r.styleOf(" Resume  "); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("Resume button is not faint: %+v", s)
	}
	if s := r.styleOf(" Keep "); s.Attrs&uv.AttrBold == 0 {
		t.Fatalf("Keep button is not bold: %+v", s)
	}
}

func onNode(id int32, node string, limit time.Duration) fakeslurm.Job {
	j := runningJob(id)
	j.Nodes = node
	j.Name = fmt.Sprintf("job-%d", id)
	j.TimeLimit = fakeslurm.Number(int64((limit + time.Minute) / time.Minute))
	return j
}

func TestDrainListsTheJobsOnTheNodeAndWhenItIsEmpty(t *testing.T) {
	jobs := []fakeslurm.Job{onNode(10, "n01", 48*time.Hour)}
	for i := range 7 {
		jobs = append(jobs, onNode(int32(4800+i), "n02", time.Duration(i+1)*time.Hour))
	}
	jobs[7].TimeLimit = fakeslurm.Number(24*60 + 1)
	jobs[7].Name = "a-very-long-job-name"
	jobs = append(jobs, pendingJob(5000, "Priority"), endedJob(5001, "COMPLETED"))
	jobs[9].Nodes = "n02"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01"), idle("n02")}, Jobs: jobs})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 10 ")
	r.moveTo("n02")

	r.send("D")

	r.waitFor("No new jobs start on n02. Running jobs finish.")
	r.waitFor("7 jobs run here")
	r.waitMatch(`4806 +a-very-long-job… +alice +24h left`)
	r.waitMatch(`4805 +job-4805 +alice +6h left`)
	r.waitFor("and 3 more")
	r.waitFor("Empty by ~" + startTime.Add(24*time.Hour).Local().Format("Mon 15:04") + " (latest time limit)")
	b := r.dialogBox("Drain n02 on")
	listed := strings.Join(r.dialogRows(b), "\n")
	if strings.Contains(listed, "4802 ") || strings.Contains(listed, "5000") || strings.Contains(listed, "5001") {
		t.Fatalf("dialog lists more than 4 jobs, or a job that does not run\n%s", listed)
	}
}

func TestTallDrainDialogShrinksTheJobListAndKeepsItsButtons(t *testing.T) {
	var jobs []fakeslurm.Job
	for i := range 10 {
		jobs = append(jobs, onNode(int32(4800+i), "n01", time.Hour))
	}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01")}, Jobs: jobs})
	r := startSizedOn(t, cluster, 80, 24)
	r.waitFor("┌ Jobs 10 ")
	bodyTop := r.mustFrameTop("Nodes")

	r.send("D" + "\r")

	r.waitFor("Reason is required")
	b := r.dialogBox("Drain n01 on")
	r.checkMarginAround(b, bodyTop)
	rows := strings.Join(r.dialogRows(b), "\n")
	for _, text := range []string{"10 jobs run here", "4809", "more", "Empty by", "▸ Reason", " Drain ", "Keep", "Reason is required"} {
		if !strings.Contains(rows, text) {
			t.Fatalf("tall dialog has no %q\n%s", text, r.screen())
		}
	}
	r.send(escKey)
	r.waitGone("Drain n01 on")
}

func TestDrainSaysWhenTheNodeHasNoJobsOrAJobHasNoLimit(t *testing.T) {
	unlimited := onNode(7, "n01", time.Hour)
	unlimited.TimeLimit = fakeslurm.NoVal{Infinite: true}
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01"), idle("n04")}, Jobs: []fakeslurm.Job{unlimited}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 1 ")

	r.moveTo("n01")
	r.send("D")
	r.waitFor("1 job runs here")
	r.waitFor("Empty by: unknown (a job has no time limit)")
	r.send(escKey)
	r.waitGone("Drain n01 on")
	r.moveTo("n04")
	r.send("D")
	r.waitFor("No jobs run on n04.")
}

func TestDrainUsesThePartitionLimitOfAJobWithNoOwnLimit(t *testing.T) {
	job := onNode(7, "n01", time.Hour)
	job.TimeLimit = fakeslurm.NoVal{}
	job.Partition = "gpu"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:       "lamp",
		Nodes:      []fakeslurm.Node{idle("n01")},
		Jobs:       []fakeslurm.Job{job},
		Partitions: []fakeslurm.Partition{{Name: "gpu", MaxTime: fakeslurm.Number(240)}},
	})
	r := startProvided(t, cluster)
	r.waitFor("┌ Jobs 1 ")

	r.send("D")

	r.waitMatch(`7 +job-7 +alice +3h59m left`)
	r.waitFor("Empty by ~" + startTime.Add(239*time.Minute).Local().Format("Mon 15:04") + " (latest time limit)")
}

func TestJobInAPartitionThatSlurmDoesNotListIsAnError(t *testing.T) {
	job := onNode(7, "n01", time.Hour)
	job.TimeLimit = fakeslurm.NoVal{}
	job.Partition = "gone"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01")}, Jobs: []fakeslurm.Job{job}})
	r := startProvided(t, cluster)

	r.waitFor(`job 7: partition "gone" is not in the partition list`)
}

func TestDOnADrainedNodeEditsItsReason(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), drained("n05", "bad DIMM", "bob", startTime))

	r.send("D")

	r.waitFor("Edit reason of n05 on " + contextOf(cluster) + " (lamp)")
	r.waitFor("The node stays drained. Only the reason changes.")
	r.waitFor("▸ Reason  bad DIMM█")
	r.waitMatch(`│\s+ Save \s+ Keep \s+│`)
	r.send(strings.Repeat("\x7f", 4) + "fan\r")
	checkBody(t, r.waitNodeUpdate(cluster), "/slurm/v0.0.45/node/n05", `{"state":["DRAIN"],"reason":"bad fan"}`)
	r.waitFor("reason of n05 updated")
}

func TestEnterOnAnUnchangedReasonSendsNothing(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), drained("n05", "bad DIMM", "bob", startTime))
	r.send("D")
	r.waitFor("▸ Reason  bad DIMM█")

	r.send("\r")

	r.waitFor("reason unchanged")
	r.waitGone("Edit reason of n05")
	if got := nodeUpdates(cluster); len(got) != 0 {
		t.Fatalf("enter on an unchanged reason sent %d node updates", len(got))
	}
}

func TestPasteGoesIntoTheReasonField(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))
	r.send("D")
	r.waitFor("▸ Reason  █")

	r.send("bad \x1b[200~DIMM slot 3\x1b[201~")
	r.waitFor("▸ Reason  bad DIMM slot 3█")
	r.send("\t\r")

	checkBody(t, r.waitNodeUpdate(cluster), "/slurm/v0.0.45/node/n02", `{"state":["DRAIN"],"reason":"bad DIMM slot 3"}`)
}

func TestDialogTitleLeavesOutAnUnknownClusterName(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Nodes: []fakeslurm.Node{idle("n01")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	r.send("D")

	r.waitFor("Drain n01 on " + contextOf(cluster) + " ─")
	if strings.Contains(r.screen(), "()") {
		t.Fatalf("dialog title shows an empty cluster name\nscreen:\n%s", r.screen())
	}
}

func TestDOnADownNodeWithoutDrainOpensDrain(t *testing.T) {
	down := nodeIn("n02", "DOWN", "NOT_RESPONDING")
	down.Reason = "Not responding"
	cluster, r := startOnSecondNode(t, idle("n01"), down)

	r.send("D")

	r.waitFor("Drain n02 on " + contextOf(cluster) + " (lamp)")
}

func TestResumeShowsTheReasonWithItsUserAndAge(t *testing.T) {
	_, r := startOnSecondNode(t, idle("n01"), drained("n02", "bad DIMM", "alice", startTime.Add(-(5*time.Hour+3*time.Minute))))

	r.send("U")

	r.waitFor("n02 takes new jobs again. Its reason is cleared.")
	r.waitFor("Reason: bad DIMM")
	r.waitFor("Set by alice, 5h3m ago")
}

func TestUOnAnIdleNodeOpensResumeAndSlurmRefuses(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("ghost3"))
	cluster.RefuseNodeUpdates(http.StatusOK, recordedReply(t, "node-resume-invalid-transition-reply.json"))

	r.send("U")
	r.waitFor("Resume ghost3 on")
	r.send("\x1b[D\r")

	r.waitFor("Invalid node state transition")
	if !isANSI(r.flashStyle("Failure to update").Fg, ansiRed) {
		t.Fatalf("refusal is not red: %q", r.bottomLine())
	}
}

func TestCtrlDOnAnEndedJobOpensCancelAndShowsTheReply(t *testing.T) {
	r, cluster := startOnJobs(t, []fakeslurm.Job{endedJob(1234, "COMPLETED")})
	cluster.RefuseJobCancels(recordedReply(t, "job-cancel-finished-reply.json"))
	r.waitSelected("1234")

	r.send(ctrlD)
	r.waitFor("Slurm stops job 1234. Its work is lost.")
	r.send("\x1b[D\r")

	r.waitFor("Job/step already completing or completed")
}

func TestCancelNamesAllTasksOrComponents(t *testing.T) {
	for _, tc := range []struct {
		job  fakeslurm.Job
		row  string
		want string
	}{
		{arrayMeta(1234, "3-100"), "1234_[3-100]", "This stops all tasks of array 1234."},
		{hetComponent(1301, 1300, 1), "1300+1", "This stops all components of het job 1300."},
	} {
		t.Run(tc.row, func(t *testing.T) {
			r, _ := startOnJobs(t, []fakeslurm.Job{tc.job})
			r.waitSelected(tc.row)
			r.send(ctrlD)
			r.waitFor(tc.want)
		})
	}
}

func TestEmptyReasonIsRefusedAndEscKeepsTheNode(t *testing.T) {
	cluster, r := startOnSecondNode(t, idle("n01"), idle("n02"))
	r.send("D")
	r.waitFor("▸ Reason")

	r.send("\r")

	r.waitFor("Reason is required")
	b := r.dialogBox("Drain n02 on")
	buttons, problem := -1, -1
	for i, l := range r.dialogRows(b) {
		if strings.Contains(l, " Drain ") {
			buttons = i
		}
		if strings.Contains(l, "Reason is required") {
			problem = i
		}
	}
	if problem < buttons {
		t.Fatalf("error row %d is not under the buttons row %d\n%s", problem, buttons, r.screen())
	}
	if !isANSI(r.styleOf("Reason is required").Fg, ansiRed) {
		t.Fatalf("Reason is required is not red")
	}
	r.send(escKey)
	r.waitFor("kept n02 as it was")
	if got := nodeUpdates(cluster); len(got) != 0 {
		t.Fatalf("esc sent %d node updates", len(got))
	}
}
