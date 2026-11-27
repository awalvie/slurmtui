package ui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

type hint struct {
	key   string
	label string
}

type place int

const (
	nodesPane place = iota
	jobsPane
	bothPanes
	anywhere
)

var paneTitles = map[place]string{nodesPane: "Nodes", jobsPane: "Jobs"}

type binding struct {
	keys      []string
	shown     string
	hintKey   string
	place     place
	help      string
	otherPane string
	writes    bool
	hint      func(m Model, f frame) string
	do        func(m Model, key string) (Model, tea.Cmd)
}

func (b binding) in(kind view) bool {
	switch b.place {
	case anywhere:
		return true
	case bothPanes:
		return kind == nodesView || kind == jobsView
	case nodesPane:
		return kind == nodesView
	}
	return kind == jobsView
}

func (b binding) hintKeyText() string {
	if b.hintKey != "" {
		return b.hintKey
	}
	return b.shown
}

func just(f func(Model) Model) func(Model, string) (Model, tea.Cmd) {
	return func(m Model, _ string) (Model, tea.Cmd) { return f(m), nil }
}

func label(text string) func(Model, frame) string {
	return func(Model, frame) string { return text }
}

var bindings = []binding{
	{keys: []string{"j", "down", "k", "up"}, shown: "j/k ↑/↓", hintKey: "j/k", place: bothPanes, help: "Move the cursor", hint: label("move"),
		do: func(m Model, key string) (Model, tea.Cmd) {
			if key == "k" || key == "up" {
				return m.moveCursor(-1), nil
			}
			return m.moveCursor(1), nil
		}},
	{keys: []string{"enter"}, shown: "enter", place: nodesPane, help: "Show the jobs on the node", hint: label("jobs"), do: just(Model.enterOnNode)},
	{keys: []string{"enter"}, shown: "enter", place: jobsPane, help: "Describe the job", do: just(Model.describe)},
	{keys: []string{"d"}, shown: "d", place: bothPanes, help: "Describe", hint: label("describe"), do: just(Model.describe)},
	{keys: []string{"D"}, shown: "D", place: nodesPane, help: "Drain, or edit the reason of a drained node", otherPane: "works on a node", writes: true, hint: Model.drainHint, do: just(Model.openDrain)},
	{keys: []string{"U"}, shown: "U", place: nodesPane, help: "Resume the node", otherPane: "works on a node", writes: true, hint: Model.resumeHint, do: just(Model.openResume)},
	{keys: []string{"ctrl+d"}, shown: "ctrl-d", place: jobsPane, help: "Cancel the job", otherPane: "cancels a job", writes: true, hint: label("cancel"), do: just(Model.openCancel)},
	{keys: []string{"o"}, shown: "o", place: jobsPane, help: "Show the nodes of the job", otherPane: "shows the nodes of a job", do: just(Model.nodesOfJob)},
	{keys: []string{"g", "G"}, shown: "g/G", place: bothPanes, help: "First or last row",
		do: func(m Model, key string) (Model, tea.Cmd) {
			if key == "G" {
				return m.cursorTo(-1), nil
			}
			return m.cursorTo(0), nil
		}},
	{keys: []string{"N", "shift+n"}, shown: "shift-n", place: bothPanes, help: "Sort by name, again to reverse", do: just(func(m Model) Model { return m.sortedBy(sortByName) })},
	{keys: []string{"S", "shift+s"}, shown: "shift-s", place: bothPanes, help: "Sort by state, again to reverse", do: just(func(m Model) Model { return m.sortedBy(sortByState) })},
	{keys: []string{"y"}, shown: "y", place: bothPanes, help: "Raw JSON", do: just(func(m Model) Model { return m.openDetail(jsonView) })},
	{keys: []string{"/"}, shown: "/", place: bothPanes, help: "Filter (regex, ! inverts)", hint: Model.filterHint, do: just(Model.startFilter)},
	{keys: []string{"esc"}, shown: "esc", place: anywhere, help: "Clear the filter, then the scope, then back", hint: Model.escHint, do: just(Model.back)},
	{keys: []string{"tab", "shift+tab"}, shown: "tab", place: bothPanes, help: "Other pane", hint: Model.tabHint, do: Model.tab},
	{keys: []string{"ctrl+r"}, shown: "ctrl-r", place: anywhere, help: "Refresh now", do: func(m Model, _ string) (Model, tea.Cmd) { return m.refresh() }},
	{keys: []string{"q"}, shown: "q", place: anywhere, help: "Quit; in describe, JSON and help, back", hint: Model.quitHint, do: Model.quitOrBack},
	{keys: []string{"?"}, shown: "?", place: anywhere, hint: label("more"), do: just(Model.toggleHelp)},
	{keys: []string{":"}, shown: ":", place: anywhere, do: just(Model.startCommand)},
}

func (m Model) bindingFor(key string, kind view) (binding, bool) {
	for _, b := range bindings {
		if slices.Contains(b.keys, key) && b.in(kind) {
			return b, true
		}
	}
	return binding{}, false
}

func (m Model) otherPaneBinding(key string) (binding, bool) {
	if m.top().kind != overviewView {
		return binding{}, false
	}
	other := map[view]view{nodesView: jobsView, jobsView: nodesView}[m.focus]
	b, ok := m.bindingFor(key, other)
	if !ok || b.otherPane == "" {
		return binding{}, false
	}
	return b, true
}

func (m Model) paneKey(key string) (Model, tea.Cmd) {
	kind := m.current().kind
	if b, ok := m.bindingFor(key, kind); ok && b.place == anywhere {
		return b.do(m, key)
	}
	switch {
	case kind == helpView:
		return m.helpKey(key), nil
	case isDetail(kind):
		return m.scrollKey(key, len(m.detailLines(m.top())), m.detailRoom()), nil
	}
	b, inPane := m.bindingFor(key, kind)
	other, inOtherPane := m.otherPaneBinding(key)
	switch {
	case m.session.ReadOnly && (inPane && b.writes || !inPane && inOtherPane && other.writes):
		return m.say(flashWarning, readOnlyFlash), nil
	case inPane:
		return b.do(m, key)
	case inOtherPane:
		return m.say(flashWarning, keyName(key)+" "+other.otherPane+" · press tab to focus "+paneTitles[other.place]), nil
	}
	return m.unknownKey(key), nil
}

func (m Model) hints() []hint {
	f := m.current()
	if isDetail(f.kind) {
		return []hint{{"j/k", "scroll"}, {"esc", "back"}, {"?", "more"}}
	}
	if f.kind == helpView {
		return helpHints
	}
	var out []hint
	for _, b := range bindings {
		if b.hint == nil || !b.in(f.kind) || b.writes && m.session.ReadOnly {
			continue
		}
		if text := b.hint(m, f); text != "" {
			out = append(out, hint{b.hintKeyText(), text})
		}
	}
	return out
}

func helpGroupsFromBindings() []helpGroup {
	groups := []helpGroup{{name: "Nodes pane"}, {name: "Jobs pane"}, {name: "Both panes"}}
	at := map[place]int{nodesPane: 0, jobsPane: 1, bothPanes: 2, anywhere: 2}
	for _, b := range bindings {
		if b.help == "" {
			continue
		}
		g := &groups[at[b.place]]
		g.keys = append(g.keys, keyHelp{key: b.shown, action: b.help, writes: b.writes})
	}
	return append(groups, fixedHelpGroups...)
}

func (m Model) drainHint(frame) string {
	n, ok := m.selectedNode()
	if ok && n.State.InDrain() {
		return "edit reason"
	}
	return "drain"
}

func (m Model) resumeHint(frame) string {
	n, ok := m.selectedNode()
	if ok && (n.State.Resumable()) {
		return "resume"
	}
	return ""
}

func (m Model) filterHint(f frame) string {
	if f.filter != "" || f.kind == jobsView && f.scope != "" {
		return ""
	}
	return "filter"
}

func (m Model) escHint(f frame) string {
	switch {
	case f.filter != "":
		return "clear filter"
	case f.kind == jobsView && f.scope != "":
		return "all jobs"
	}
	return ""
}

func (m Model) quitHint(f frame) string {
	if f.kind == nodesView || f.kind == jobsView {
		return "quit"
	}
	return ""
}

func (m Model) quitOrBack(string) (Model, tea.Cmd) {
	switch kind := m.top().kind; {
	case kind == helpView:
		return m.toggleHelp(), nil
	case isDetail(kind):
		return m.pop(), nil
	}
	return m, tea.Quit
}

func (m Model) tabHint(f frame) string {
	if m.top().kind != overviewView {
		return ""
	}
	if f.kind == jobsView {
		return "nodes pane"
	}
	return "jobs pane"
}

func (m Model) tab(key string) (Model, tea.Cmd) {
	if m.top().kind != overviewView {
		return m.unknownKey(key), nil
	}
	return m.switchPane(), nil
}

func (m Model) enterOnNode() Model {
	if m.top().kind == overviewView {
		return m.scopeJobs()
	}
	return m.pushSelected(frame{kind: jobsView})
}

func (m Model) describe() Model {
	return m.openDetail(describeView)
}

func (m Model) nodesOfJob() Model {
	return m.pushSelected(frame{kind: nodesView})
}

func (m Model) toggleHelp() Model {
	if m.top().kind == helpView {
		return m.back()
	}
	return m.push(frame{kind: helpView, scope: string(m.current().kind)})
}

func (m Model) startCommand() Model {
	empty := ""
	m.command = &empty
	return m
}

func (m Model) startFilter() Model {
	m.filtering = true
	f := m.current()
	m.filterBefore = f.filter
	f.filter = ""
	return m.withCurrent(f)
}
