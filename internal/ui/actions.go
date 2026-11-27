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
	if n.State.Has("DRAIN") {
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

func (m Model) expireFlash() Model {
	if m.flash != nil && m.clock.Now().Sub(m.flash.at) >= flashDuration {
		m.flash = nil
	}
	return m
}
