package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/awalvie/slurmtui/internal/slurm"
)

const (
	flashDuration = 6 * time.Second
	readOnlyFlash = "read-only mode — action disabled"
)

type flash struct {
	text  string
	level flashLevel
	at    time.Time
}

type actionDoneMsg struct {
	info string
	err  error
}

func (m Model) target() string {
	if m.meta.Cluster == "" {
		return m.session.Context
	}
	return fmt.Sprintf("%s (%s)", m.session.Context, m.meta.Cluster)
}

func (m Model) node(name string) (slurm.Node, bool) {
	at := slices.IndexFunc(m.nodes, func(n slurm.Node) bool { return n.Name == name })
	if at < 0 {
		return slurm.Node{}, false
	}
	return m.nodes[at], true
}

func (m Model) job(id string) (slurm.Job, bool) {
	at := slices.IndexFunc(m.jobs, func(j slurm.Job) bool { return j.ID() == id })
	if at < 0 {
		return slurm.Job{}, false
	}
	return m.jobs[at], true
}

func (m Model) selectedNode() (slurm.Node, bool) {
	r, ok := m.selectedRow()
	if !ok {
		return slurm.Node{}, false
	}
	return m.node(r.key)
}

func (m Model) selectedJob() (slurm.Job, bool) {
	r, ok := m.selectedRow()
	if !ok {
		return slurm.Job{}, false
	}
	return m.job(r.key)
}

func runAction(info string, act func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		if err := act(context.Background()); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{info: info}
	}
}

func updateNode(source Source, name string, update slurm.NodeUpdate, info string) tea.Cmd {
	return runAction(info, func(ctx context.Context) error {
		return source.UpdateNode(ctx, name, update)
	})
}

func (m Model) openDrain() Model {
	n, ok := m.selectedNode()
	if !ok {
		return m
	}
	name, source := n.Name, m.source
	w := drainWords(n)
	m.dialog = newConfirm(confirmSpec{
		title:  fmt.Sprintf("%s %s on %s", w.title, name, m.target()),
		colour: warnStyle,
		body:   []string{w.first, ""},
		jobs:   m.jobsOnNode(name),
		field:  &confirmField{label: "Reason", value: w.reason, required: true, mustChange: w.editsReason},
		action: w.action,
		kept:   keptFlash(name),
		submit: func(reason string) tea.Cmd {
			return updateNode(source, name, slurm.NodeUpdate{State: slurm.Drain, Reason: reason}, w.done)
		},
	})
	return m
}

type drainDialogWords struct {
	title       string
	first       string
	reason      string
	action      string
	done        string
	editsReason bool
}

func drainWords(n slurm.Node) drainDialogWords {
	if n.State.InDrain() {
		return drainDialogWords{
			title:       "Edit reason of",
			first:       "The node stays drained. Only the reason changes.",
			reason:      n.Reason,
			action:      "Save",
			done:        "reason of " + n.Name + " updated",
			editsReason: true,
		}
	}
	return drainDialogWords{
		title:  "Drain",
		first:  "No new jobs start on " + n.Name + ". Running jobs finish.",
		action: "Drain",
		done:   "drain of " + n.Name + " accepted",
	}
}

func keptFlash(target string) string {
	return "kept " + target + " as it was"
}

func (m Model) openResume() Model {
	n, ok := m.selectedNode()
	if !ok {
		return m
	}
	name, source := n.Name, m.source
	body := []string{name + " takes new jobs again. Its reason is cleared.", ""}
	if n.Reason == "" {
		body = append(body, dimStyle.Render("No reason is set."))
	} else {
		by := n.ReasonSetBy
		if by == "" {
			by = "an unknown user"
		}
		set := "Set by " + by
		if a := m.reasonAge(n); a != "" {
			set += ", " + a + " ago"
		}
		body = append(body, "Reason: "+n.Reason, dimStyle.Render(set))
	}
	m.dialog = newConfirm(confirmSpec{
		title:  fmt.Sprintf("Resume %s on %s", name, m.target()),
		colour: focusStyle,
		body:   body,
		action: "Resume",
		kept:   keptFlash(name),
		submit: func(string) tea.Cmd {
			return updateNode(source, name, slurm.NodeUpdate{State: slurm.Resume}, "resume of "+name+" accepted")
		},
	})
	return m
}

func (m Model) openCancel() Model {
	j, ok := m.selectedJob()
	if !ok {
		return m
	}
	target, source := j.CancelTarget(), m.source
	body := []string{"Slurm stops job " + target.ID + ". Its work is lost."}
	if target.Scope != "" {
		body = append(body, "This stops "+target.Scope+".")
	}
	m.dialog = newConfirm(confirmSpec{
		title:  fmt.Sprintf("Cancel job %s on %s", target.ID, m.target()),
		colour: badStyle,
		body:   body,
		action: "Cancel job",
		kept:   keptFlash("job " + target.ID),
		submit: func(string) tea.Cmd {
			return runAction("cancel of job "+target.ID+" accepted", func(ctx context.Context) error {
				return source.CancelJob(ctx, target.ID)
			})
		},
	})
	return m
}

func (m Model) reasonAge(n slurm.Node) string {
	if n.Reason == "" || n.ReasonChangedAt.IsZero() {
		return ""
	}
	return age(m.clock.Now().Sub(n.ReasonChangedAt))
}

func (m Model) dialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	next, outcome, cmd := m.dialog.key(msg)
	switch outcome {
	case stillOpen:
		m.dialog = &next
	case keptAsItWas:
		m.dialog = nil
		return m.say(flashInfo, next.spec.kept), nil
	case keptUnchanged:
		m.dialog = nil
		return m.say(flashInfo, strings.ToLower(next.spec.field.label)+" unchanged"), nil
	case submitted:
		m.dialog = nil
	}
	return m, cmd
}

func (m Model) actionDone(msg actionDoneMsg) (Model, tea.Cmd) {
	m = m.say(flashInfo, msg.info)
	if msg.err != nil {
		m = m.say(flashError, msg.err.Error())
	}
	if m.fetching {
		m.refreshQueued = true
		return m, nil
	}
	return m.refresh()
}

func (m Model) say(level flashLevel, text string) Model {
	m.flash = &flash{text: text, level: level, at: m.clock.Now()}
	return m
}

func (m Model) expireFlash() Model {
	if m.flash != nil && m.clock.Now().Sub(m.flash.at) >= flashDuration {
		m.flash = nil
	}
	return m
}

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
		limit, err := m.timeLimit(j)
		if err != nil {
			list.head = append(list.head, badStyle.Render(err.Error()))
		}
		left, ok := j.TimeLeft(limit, now)
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
