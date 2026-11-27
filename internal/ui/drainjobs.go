package ui

import (
	"fmt"
	"slices"
	"time"

	"github.com/awalvie/slurmtui/internal/slurm"
)

const (
	jobsListedOnNode = 4
	jobIDWidth       = 8
	jobNameWidth     = 16
	jobUserWidth     = 6
	jobRowIndent     = "  "
	emptyByLayout    = "Mon 15:04"
)

func (m Model) runningOn(name string) ([]slurm.Job, []string) {
	var on []slurm.Job
	var problems []string
	for _, j := range m.jobs {
		if c := classOfJob(j.State); c != jobRunning && c != jobCompleting {
			continue
		}
		h := m.hosts[j.ID()]
		if h.err != nil {
			problems = append(problems, badStyle.Render(h.err.Error()))
			continue
		}
		if slices.Contains(h.hosts, name) {
			on = append(on, j)
		}
	}
	slices.SortFunc(on, func(a, b slurm.Job) int { return naturalCompare(b.ID(), a.ID()) })
	return on, problems
}

func jobCount(n int) string {
	if n == 1 {
		return "1 job runs here"
	}
	return fmt.Sprintf("%d jobs run here", n)
}

func (m Model) jobsOnNode(name string) []string {
	now := m.clock.Now()
	on, problems := m.runningOn(name)
	if len(on) == 0 {
		return append(problems, dimStyle.Render("No jobs run on "+name+"."))
	}
	lines := append(problems, keyStyle.Render(jobCount(len(on))))
	var latest time.Duration
	limited := true
	for i, j := range on {
		left, ok, err := m.timeLeft(j, now)
		if err != nil {
			lines = append(lines, badStyle.Render(err.Error()))
		}
		limited = limited && ok
		latest = max(latest, left)
		if i >= jobsListedOnNode {
			continue
		}
		leftText := "no time limit"
		if ok {
			leftText = age(left) + " left"
		}
		lines = append(lines, jobRowIndent+fitLeft(j.ID(), jobIDWidth)+" "+fitLeft(j.Name, jobNameWidth)+" "+fitLeft(j.User, jobUserWidth)+" "+leftText)
	}
	if more := len(on) - jobsListedOnNode; more > 0 {
		lines = append(lines, jobRowIndent+dimStyle.Render(fmt.Sprintf("and %d more", more)))
	}
	if !limited {
		return append(lines, dimStyle.Render("Empty by: unknown (a job has no time limit)"))
	}
	return append(lines, dimStyle.Render("Empty by ~"+now.Add(latest).Local().Format(emptyByLayout)+" (latest time limit)"))
}
