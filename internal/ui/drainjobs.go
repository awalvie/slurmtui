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
		if c := j.State.Class(); c != slurm.JobRunning && c != slurm.JobCompleting {
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

type jobList struct {
	head []string
	rows []string
	tail []string
}

func (m Model) jobsOnNode(name string) jobList {
	now := m.clock.Now()
	on, problems := m.runningOn(name)
	if len(on) == 0 {
		return jobList{head: append(problems, dimStyle.Render("No jobs run on "+name+"."))}
	}
	list := jobList{head: append(problems, keyStyle.Render(jobCount(len(on))))}
	var latest time.Duration
	limited := true
	for _, j := range on {
		left, ok, err := m.timeLeft(j, now)
		if err != nil {
			list.head = append(list.head, badStyle.Render(err.Error()))
		}
		limited = limited && ok
		latest = max(latest, left)
		leftText := "no time limit"
		if ok {
			leftText = age(left) + " left"
		}
		list.rows = append(list.rows, jobRowIndent+fitLeft(j.ID(), jobIDWidth)+" "+fitLeft(j.Name, jobNameWidth)+" "+fitLeft(j.User, jobUserWidth)+" "+leftText)
	}
	if !limited {
		list.tail = []string{dimStyle.Render("Empty by: unknown (a job has no time limit)")}
		return list
	}
	list.tail = []string{dimStyle.Render("Empty by ~" + now.Add(latest).Local().Format(emptyByLayout) + " (latest time limit)")}
	return list
}

func (l jobList) shown(room int) []string {
	if len(l.rows) <= min(room, jobsListedOnNode) {
		return l.rows
	}
	keep := max(min(room-1, jobsListedOnNode), 0)
	return append(slices.Clone(l.rows[:keep]), jobRowIndent+dimStyle.Render(fmt.Sprintf("and %d more", len(l.rows)-keep)))
}
