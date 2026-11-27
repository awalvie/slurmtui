package ui

import (
	"maps"
	"slices"
)

type sortKey string

const (
	sortByState sortKey = ""
	sortByName  sortKey = "name"
)

type frame struct {
	kind    view
	of      view
	scope   string
	filter  string
	sortBy  sortKey
	reverse bool
}

func (f frame) id() string {
	return string(f.kind) + "(" + f.scope + ")"
}

type cursorAt struct {
	index int
	key   string
}

func (m Model) top() frame {
	return m.stack[len(m.stack)-1]
}

func (m Model) withTop(f frame) Model {
	stack := slices.Clone(m.stack)
	stack[len(stack)-1] = f
	m.stack = stack
	return m
}

func (m Model) push(f frame) Model {
	m.stack = append(slices.Clone(m.stack), f)
	return m
}

func (m Model) pop() Model {
	m.stack = slices.Clone(m.stack[:len(m.stack)-1])
	return m
}

func (m Model) current() frame {
	switch {
	case m.top().kind != overviewView:
		return m.top()
	case m.focus == jobsView:
		return m.jobPane
	}
	return m.nodePane
}

func (m Model) withCurrent(f frame) Model {
	switch {
	case m.top().kind != overviewView:
		return m.withTop(f)
	case m.focus == jobsView:
		m.jobPane = f
	default:
		m.nodePane = f
	}
	return m
}

func (m Model) show(v view) Model {
	m.stack = []frame{{kind: overviewView}}
	if v != overviewView {
		m.stack = append(m.stack, frame{kind: v})
	}
	return m
}

func (m Model) back() Model {
	f := m.current()
	if f.filter != "" {
		f.filter = ""
		return m.withCurrent(f)
	}
	if len(m.stack) > 1 {
		return m.pop()
	}
	if m.focus == jobsView && m.jobPane.scope != "" {
		m.jobPane.scope = ""
		return m.say(flashInfo, "Jobs shows all jobs")
	}
	return m.say(flashWarning, "esc: nothing to clear · no filter and no node scope")
}

func (m Model) sortedBy(key sortKey) Model {
	f := m.current()
	f.reverse = f.sortBy == key && !f.reverse
	f.sortBy = key
	return m.withCurrent(f)
}

func (m Model) cursorIn(f frame, rows []row) int {
	at := m.cursors[f.id()]
	if i := slices.IndexFunc(rows, func(r row) bool { return r.key == at.key }); i >= 0 {
		return i
	}
	return max(min(at.index, len(rows)-1), 0)
}

func (m Model) withCursor(f frame, rows []row, i int) Model {
	i = max(min(i, len(rows)-1), 0)
	at := cursorAt{index: i}
	if i < len(rows) {
		at.key = rows[i].key
	}
	return m.setCursor(f, at)
}

func (m Model) setCursor(f frame, at cursorAt) Model {
	cursors := maps.Clone(m.cursors)
	if cursors == nil {
		cursors = map[string]cursorAt{}
	}
	cursors[f.id()] = at
	m.cursors = cursors
	return m
}

func (m Model) moveCursor(delta int) Model {
	f := m.current()
	rows := m.table(f).rows
	return m.withCursor(f, rows, m.cursorIn(f, rows)+delta)
}

func (m Model) cursorTo(i int) Model {
	f := m.current()
	rows := m.table(f).rows
	if i < 0 {
		i = len(rows) - 1
	}
	return m.withCursor(f, rows, i)
}

func (m Model) selectedRow() (row, bool) {
	f := m.current()
	rows := m.table(f).rows
	if len(rows) == 0 {
		return row{}, false
	}
	return rows[m.cursorIn(f, rows)], true
}

func (m Model) pushSelected(f frame) Model {
	r, ok := m.selectedRow()
	if !ok {
		return m
	}
	f.scope = r.key
	return m.push(f)
}

func (m Model) anchorCursors() Model {
	frames := map[string]frame{}
	for _, f := range append([]frame{m.nodePane, m.jobPane}, m.stack...) {
		if f.kind == nodesView || f.kind == jobsView {
			frames[f.id()] = f
		}
	}
	for _, f := range frames {
		rows := m.table(f).rows
		m = m.withCursor(f, rows, m.cursorIn(f, rows))
	}
	return m
}
