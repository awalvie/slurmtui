package ui

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/awalvie/slurmtui/internal/hostlist"
	"github.com/awalvie/slurmtui/internal/slurm"
)

type row struct {
	key      string
	name     string
	group    int
	priority int64
	cells    []cell
}

func (r row) texts() []string {
	out := make([]string, len(r.cells))
	for i, c := range r.cells {
		out[i] = c.text
	}
	return out
}

type table struct {
	rows []row
	err  error
}

type jobHosts struct {
	hosts []string
	err   error
}

func expandJobHosts(jobs []slurm.Job) map[string]jobHosts {
	out := make(map[string]jobHosts, len(jobs))
	for _, j := range jobs {
		hosts, err := hostlist.Expand(j.Nodes)
		if err != nil {
			err = fmt.Errorf("job %s: nodes %q: %w", j.ID(), j.Nodes, err)
		}
		out[j.ID()] = jobHosts{hosts: hosts, err: err}
	}
	return out
}

func partitionsByName(partitions []slurm.Partition) map[string]slurm.Partition {
	out := make(map[string]slurm.Partition, len(partitions))
	for _, p := range partitions {
		out[p.Name] = p
	}
	return out
}

type tableSpec struct {
	name      string
	scopeWord string
	rows      func(m Model, scope string) ([]row, error)
	facts     func(m Model, rows []row) []string
	right     func(m Model) string
	columns   []column
	dropOrder []string
	byState   func(a, b row) int
	byName    func(a, b row) int
	empty     string
}

func naturalByName(a, b row) int {
	return naturalCompare(a.name, b.name)
}

func newestIDFirst(a, b row) int {
	return naturalCompare(b.key, a.key)
}

func nodesBySeverity(a, b row) int {
	return cmp.Or(cmp.Compare(a.group, b.group), naturalByName(a, b))
}

func jobsByGroupAndPriority(a, b row) int {
	return cmp.Or(cmp.Compare(a.group, b.group), cmp.Compare(b.priority, a.priority), newestIDFirst(a, b))
}

func jobsByName(a, b row) int {
	return cmp.Or(naturalByName(a, b), newestIDFirst(a, b))
}

const (
	pendingGroup = iota
	runningGroup
	endedGroup
)

func jobGroup(s slurm.JobState) int {
	switch {
	case s.Pending():
		return pendingGroup
	case s.Ended():
		return endedGroup
	}
	return runningGroup
}

var nodeTable = tableSpec{
	name:      "Nodes",
	scopeWord: "of",
	rows:      Model.nodeRows,
	facts:     func(m Model, _ []row) []string { return nodeProblemFacts(m.nodes) },
	right:     func(m Model) string { return usableAllocation(m.nodes) },
	columns: []column{
		{name: "NAME", kind: fitColumn, size: 20},
		{name: "STATE", kind: fixedColumn, size: nodeStateWidth()},
		{name: "CPU", kind: fitColumn, right: true},
		{name: "MEM(GiB)", kind: fitColumn, right: true},
		{name: "GRES", kind: fitColumn, right: true},
		{name: "BY", kind: fitColumn, size: 12},
		{name: "AGE", kind: fitColumn},
		{name: "REASON", kind: flexColumn, size: 1},
	},
	dropOrder: []string{"BY", "AGE", "GRES"},
	byState:   nodesBySeverity,
	byName:    naturalByName,
	empty:     "no nodes",
}

var jobTable = tableSpec{
	name:      "Jobs",
	scopeWord: "on",
	rows:      Model.jobRows,
	facts:     func(_ Model, rows []row) []string { return jobGroupFacts(rows) },
	right:     func(Model) string { return "" },
	columns: []column{
		{name: "ID", kind: fitColumn, fromWindow: true},
		{name: "PARTITION", kind: fitColumn, size: 10},
		{name: "NAME", kind: flexColumn, size: 1},
		{name: "USER", kind: fitColumn, size: 12},
		{name: "STATE", kind: fixedColumn, size: jobStateWidth()},
		{name: "TIME", kind: fitColumn, right: true},
		{name: "TIME_LEFT", kind: fitColumn, right: true},
		{name: "NODES", kind: fitColumn, right: true},
		{name: "NODELIST(REASON)", kind: flexColumn, size: 3},
	},
	dropOrder: []string{"TIME_LEFT", "PARTITION", "NODES", "USER", "TIME"},
	byState:   jobsByGroupAndPriority,
	byName:    jobsByName,
	empty:     "no jobs",
}

func specOf(v view) tableSpec {
	if v == jobsView {
		return jobTable
	}
	return nodeTable
}

func gresCell(n slurm.Node) (string, error) {
	gpus, has, err := n.GPUs()
	if err != nil || !has {
		return "", err
	}
	return fmt.Sprintf("%d/%d", gpus.Used, gpus.Configured), nil
}

func (m Model) nodeCells(n slurm.Node) ([]cell, error) {
	gres, err := gresCell(n)
	text := func(s string) cell { return cell{text: s, style: textStyle} }
	return []cell{
		text(n.Name),
		nodeStateCell(n.State),
		text(fmt.Sprintf("%d/%d", n.AllocCPUs, n.CPUs)),
		text(gib(n.AllocMemory) + "/" + gib(n.RealMemory)),
		text(gres),
		text(n.ReasonSetBy),
		text(m.reasonAge(n)),
		text(n.Reason),
	}, err
}

func (m Model) timeLimit(j slurm.Job) (slurm.TimeLimit, error) {
	if j.HasOwnTimeLimit() || !j.State.CanRunOut() {
		return j.TimeLimit, nil
	}
	p, ok := m.partitions[j.Partition]
	if !ok {
		return slurm.TimeLimit{}, fmt.Errorf("job %s: partition %q is not in the partition list", j.ID(), j.Partition)
	}
	return p.JobTimeLimit(), nil
}

func (m Model) timeLeft(j slurm.Job, now time.Time) (time.Duration, bool, error) {
	limit, err := m.timeLimit(j)
	if err != nil {
		return 0, false, err
	}
	left, ok := j.TimeLeft(limit, now)
	return left, ok, nil
}

func timeLeftText(j slurm.Job, limit slurm.TimeLimit, now time.Time) string {
	if limit.Unlimited && j.State.CanRunOut() {
		return "UNLIMITED"
	}
	if d, ok := j.TimeLeft(limit, now); ok {
		return age(d)
	}
	return ""
}

func (m Model) timeLeftText(j slurm.Job, now time.Time) string {
	limit, err := m.timeLimit(j)
	if err != nil {
		return err.Error()
	}
	return timeLeftText(j, limit, now)
}

func (m Model) jobCells(j slurm.Job) ([]cell, error) {
	now := m.clock.Now()
	base := textStyle
	if j.State.Ended() {
		base = dimStyle
	}
	text := func(s string) cell { return cell{text: s, style: base} }
	elapsed := text("")
	if d, ok := j.Elapsed(now); ok {
		elapsed = text(age(d))
	}
	if d, ok := j.Waited(now); ok {
		elapsed = cell{text: age(d), style: dimStyle}
	}
	limit, err := m.timeLimit(j)
	return []cell{
		text(j.ID()),
		text(j.Partition),
		text(j.Name),
		text(j.User),
		jobStateCell(j.State),
		elapsed,
		text(timeLeftText(j, limit, now)),
		text(strconv.FormatInt(j.NodeCount, 10)),
		text(j.NodeListOrReason()),
	}, err
}

func (m Model) nodeRows(scope string) ([]row, error) {
	inJob := map[string]bool{}
	if scope != "" {
		h, ok := m.hosts[scope]
		if !ok {
			return nil, fmt.Errorf("job %s: not in the job list", scope)
		}
		if h.err != nil {
			return nil, h.err
		}
		for _, host := range h.hosts {
			inJob[host] = true
		}
	}
	var problem error
	rows := make([]row, 0, len(m.nodes))
	for _, n := range m.nodes {
		if scope != "" && !inJob[n.Name] {
			continue
		}
		cells, err := m.nodeCells(n)
		problem = cmp.Or(problem, err)
		rows = append(rows, row{key: n.Name, name: n.Name, group: int(n.State.Class()), cells: cells})
	}
	return rows, problem
}

func (m Model) jobRows(scope string) ([]row, error) {
	var problem error
	rows := make([]row, 0, len(m.jobs))
	for _, j := range m.jobs {
		if scope != "" {
			h := m.hosts[j.ID()]
			if h.err != nil {
				problem = cmp.Or(problem, h.err)
				continue
			}
			if !slices.Contains(h.hosts, scope) {
				continue
			}
		}
		cells, err := m.jobCells(j)
		problem = cmp.Or(problem, err)
		rows = append(rows, row{key: j.ID(), name: j.Name, group: jobGroup(j.State), priority: j.Priority, cells: cells})
	}
	return rows, problem
}

func sortRows(spec tableSpec, rows []row, by sortKey, reverse bool) {
	compare := spec.byState
	if by == sortByName {
		compare = spec.byName
	}
	slices.SortStableFunc(rows, compare)
	if reverse {
		slices.Reverse(rows)
	}
}

func filterRows(rows []row, filter string) ([]row, error) {
	if filter == "" {
		return rows, nil
	}
	expr, invert := filter, false
	if rest, ok := strings.CutPrefix(filter, "!"); ok {
		expr, invert = strings.TrimLeft(rest, " "), true
	}
	rx, err := regexp.Compile("(?i)" + expr)
	if err != nil {
		return nil, fmt.Errorf("filter %q: %w", filter, err)
	}
	kept := make([]row, 0, len(rows))
	for _, r := range rows {
		if rx.MatchString(strings.Join(r.texts(), " ")) != invert {
			kept = append(kept, r)
		}
	}
	return kept, nil
}

func (m Model) table(f frame) table {
	if t, ok := m.built[f]; ok {
		return t
	}
	return m.buildTable(f)
}

func (m Model) withBuiltTables() Model {
	frames := []frame{m.top()}
	if m.top().kind == overviewView {
		frames = []frame{m.nodePane, m.jobPane}
	}
	built := make(map[frame]table, len(frames))
	for _, f := range frames {
		if f.kind == nodesView || f.kind == jobsView {
			built[f] = m.buildTable(f)
		}
	}
	m.built = built
	return m
}

func (m Model) buildTable(f frame) table {
	spec := specOf(f.kind)
	rows, err := spec.rows(m, f.scope)
	sortRows(spec, rows, f.sortBy, f.reverse)
	kept, filterErr := filterRows(rows, f.filter)
	if filterErr != nil {
		return table{err: filterErr}
	}
	return table{rows: kept, err: err}
}

func (f frame) sortColumn() string {
	if f.sortBy == sortByName {
		return "NAME"
	}
	return "STATE"
}

func (f frame) scopeFact(spec tableSpec) string {
	if f.scope == "" {
		return ""
	}
	return spec.scopeWord + " " + f.scope
}

func (f frame) emptyText(spec tableSpec) string {
	text := spec.empty
	if f.scope != "" {
		text += " " + f.scopeFact(spec)
	}
	if f.filter != "" {
		text += " match /" + f.filter
	}
	return text
}

type pane struct {
	spec    tableSpec
	title   paneTitle
	right   string
	rows    []row
	cursor  int
	focused bool
	sortCol string
	reverse bool
	empty   string
	mark    string
}

func (m Model) paneOf(f frame, t table, focused bool) pane {
	spec := specOf(f.kind)
	title := paneTitle{name: spec.name, count: len(t.rows), focused: focused, scope: f.scopeFact(spec), filter: f.filter, facts: spec.facts(m, t.rows)}
	return pane{
		spec:    spec,
		title:   title,
		right:   spec.right(m),
		rows:    t.rows,
		cursor:  m.cursorIn(f, t.rows),
		focused: focused,
		sortCol: f.sortColumn(),
		reverse: f.reverse,
		empty:   f.emptyText(spec),
	}
}

func windowStart(cursor, total, visible int) int {
	if visible <= 0 || total <= visible {
		return 0
	}
	return min(max(0, cursor-visible+1), total-visible)
}

func (p pane) columns() []column {
	arrow := " ↑"
	if p.reverse {
		arrow = " ↓"
	}
	cols := slices.Clone(p.spec.columns)
	for i := range cols {
		if cols[i].name == p.sortCol {
			cols[i].arrow = arrow
		}
	}
	return cols
}

func (p pane) render(width, height int) []string {
	inner := width - 2
	visible := max(height-3, 0)
	start := windowStart(p.cursor, len(p.rows), visible)
	end := min(len(p.rows), start+visible)
	all := make([][]cell, len(p.rows))
	for i, r := range p.rows {
		all[i] = r.cells
	}
	l := layoutColumns(p.columns(), p.spec.dropOrder, all, all[start:end], inner)
	headers := make([]string, len(l.columns))
	for i, c := range l.columns {
		headers[i] = alignCell(c.header(), l.widths[i], c.right)
	}
	lines := []string{" " + keyStyle.Render(strings.Join(headers, columnGap))}
	if len(p.rows) == 0 {
		lines = append(lines, " "+dimStyle.Render(fitLeft(p.empty, inner-2)))
	}
	for i := start; i < end; i++ {
		lines = append(lines, p.rowLine(p.rows[i], l, i == p.cursor, inner))
	}
	return box{title: p.title.render(), right: p.right, width: width, height: height, focused: p.focused, lines: lines}.render()
}

func alignCell(text string, width int, right bool) string {
	if right {
		return fitRight(text, width)
	}
	return fitLeft(text, width)
}

func (p pane) rowLine(r row, l layout, atCursor bool, inner int) string {
	lit := atCursor && p.focused
	lead := " "
	switch {
	case lit:
		lead = titleStyle.Render("▌")
	case atCursor:
		lead = dimStyle.Render("▌")
	case r.key == p.mark:
		lead = dimStyle.Render("▸")
	}
	parts := make([]string, len(l.keep))
	for k, i := range l.keep {
		text := alignCell(r.cells[i].text, l.widths[k], l.columns[k].right)
		if lit {
			parts[k] = text
			continue
		}
		parts[k] = r.cells[i].style.Render(text)
	}
	if lit {
		return lead + cursorStyle.Render(fitLeft(strings.Join(parts, columnGap), inner-1))
	}
	return lead + strings.Join(parts, columnGap)
}
