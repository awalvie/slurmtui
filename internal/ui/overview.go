package ui

import "fmt"

const (
	nodesShareOfBody = 2
	bodyShares       = 5
	minNodesPane     = 5
	paneChrome       = 3
)

func (m Model) scopeJobs() Model {
	r, ok := m.selectedRow()
	if !ok {
		return m
	}
	m.jobPane.scope = r.key
	m.focus = jobsView
	m = m.withCursor(m.jobPane, m.table(m.jobPane).rows, 0)
	return m.say(flashInfo, fmt.Sprintf("Jobs shows %s only · esc shows all jobs", r.key))
}

func nodesPaneHeight(rows, body int) int {
	return min(max(rows, 1)+paneChrome, max(body*nodesShareOfBody/bodyShares, minNodesPane))
}

func (m Model) overview(width, height int) ([]string, error) {
	nodes, jobs := m.table(m.nodePane), m.table(m.jobPane)
	nodesHeight := nodesPaneHeight(len(nodes.rows), height)
	gap := m.blankUnderHeader()
	top := m.paneOf(m.nodePane, nodes, m.focus == nodesView)
	top.mark = m.jobPane.scope
	lines := top.render(width, nodesHeight)
	for range gap {
		lines = append(lines, "")
	}
	lines = append(lines, m.paneOf(m.jobPane, jobs, m.focus == jobsView).render(width, height-nodesHeight-gap)...)
	if nodes.err != nil {
		return lines, nodes.err
	}
	return lines, jobs.err
}
