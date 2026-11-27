package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type cell struct {
	text  string
	style lipgloss.Style
}

type columnKind int

const (
	fixedColumn columnKind = iota
	fitColumn
	flexColumn
)

type column struct {
	name       string
	arrow      string
	kind       columnKind
	size       int
	right      bool
	fromWindow bool
}

const columnGap = "  "

func (c column) header() string {
	return c.name + c.arrow
}

func (c column) headerWidth() int {
	return ansi.StringWidth(c.header())
}

func naturalWidths(cols []column, all, window [][]cell) []int {
	out := make([]int, len(cols))
	for i, c := range cols {
		out[i] = c.headerWidth()
		rows := all
		if c.fromWindow {
			rows = window
		}
		for _, r := range rows {
			out[i] = max(out[i], ansi.StringWidth(r[i].text))
		}
	}
	return out
}

func columnWidths(cols []column, natural []int, budget int) []int {
	widths := make([]int, len(cols))
	var flex []int
	for i, c := range cols {
		switch c.kind {
		case fixedColumn:
			widths[i] = max(c.size, c.headerWidth())
		case fitColumn:
			widths[i] = natural[i]
			if c.size > 0 {
				widths[i] = min(widths[i], max(c.size, c.headerWidth()))
			}
		case flexColumn:
			flex = append(flex, i)
			continue
		}
		budget -= widths[i]
	}
	return shareFlex(cols, natural, widths, flex, budget)
}

func weightOf(cols []column, among []int) int {
	total := 0
	for _, i := range among {
		total += cols[i].size
	}
	return total
}

func shareFlex(cols []column, natural, widths, flex []int, budget int) []int {
	active := flex
	for len(active) > 0 {
		weight := weightOf(cols, active)
		full := -1
		for _, i := range active {
			if budget*cols[i].size/weight >= natural[i] {
				full = i
				break
			}
		}
		if full < 0 {
			left := budget
			for _, i := range active {
				widths[i] = max(budget*cols[i].size/weight, cols[i].headerWidth())
				left -= widths[i]
			}
			widths[active[len(active)-1]] += max(left, 0)
			return widths
		}
		widths[full] = natural[full]
		budget -= natural[full]
		rest := make([]int, 0, len(active)-1)
		for _, i := range active {
			if i != full {
				rest = append(rest, i)
			}
		}
		active = rest
	}
	if budget <= 0 || len(flex) == 0 {
		return widths
	}
	weight, left := weightOf(cols, flex), budget
	for k, i := range flex {
		add := budget * cols[i].size / weight
		if k == len(flex)-1 {
			add = left
		}
		widths[i] += add
		left -= add
	}
	return widths
}

func tableBudget(inner, columns int) int {
	const lead, margin = 1, 1
	return inner - lead - margin - len(columnGap)*(columns-1)
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func project(cols []column, rows [][]cell, keep []int) ([]column, [][]cell) {
	outCols := make([]column, 0, len(keep))
	for _, i := range keep {
		outCols = append(outCols, cols[i])
	}
	outRows := make([][]cell, 0, len(rows))
	for _, r := range rows {
		kept := make([]cell, 0, len(keep))
		for _, i := range keep {
			kept = append(kept, r[i])
		}
		outRows = append(outRows, kept)
	}
	return outCols, outRows
}

type layout struct {
	columns []column
	widths  []int
	keep    []int
}

func layoutColumns(cols []column, dropOrder []string, all, window [][]cell, inner int) layout {
	keep := make([]int, len(cols))
	for i := range keep {
		keep[i] = i
	}
	measure := func() layout {
		c, a := project(cols, all, keep)
		_, w := project(cols, window, keep)
		return layout{columns: c, widths: columnWidths(c, naturalWidths(c, a, w), tableBudget(inner, len(c))), keep: keep}
	}
	l := measure()
	for _, name := range dropOrder {
		if sum(l.widths) <= tableBudget(inner, len(l.columns)) {
			break
		}
		for k, i := range keep {
			if cols[i].name == name {
				keep = append(keep[:k:k], keep[k+1:]...)
				break
			}
		}
		l = measure()
	}
	return l
}
