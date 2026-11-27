package ui

import (
	"fmt"
	"strings"

	"github.com/awalvie/slurmtui/internal/slurm"
)

func nodeProblemFacts(nodes []slurm.Node) []string {
	var draining, drained, down int
	for _, n := range nodes {
		switch classOfNode(n.State) {
		case nodeDown:
			down++
		case nodeDrain:
			if n.State.Draining() {
				draining++
			} else {
				drained++
			}
		}
	}
	var facts []string
	add := func(count int, class nodeClass, label string) {
		if count > 0 {
			c := nodeClasses[class]
			facts = append(facts, c.style.Bold(true).Render(fmt.Sprintf("%s %d %s", c.glyph, count, label)))
		}
	}
	add(draining, nodeDrain, "draining")
	add(drained, nodeDrain, "drained")
	add(down, nodeDown, "down")
	return facts
}

func percentOf(used, total int64) string {
	return fmt.Sprintf("%d%%", used*100/total)
}

func usableAllocation(nodes []slurm.Node) string {
	var allocCPUs, cpus, allocMem, mem int64
	for _, n := range nodes {
		if !n.Usable() {
			continue
		}
		allocCPUs += int64(n.AllocCPUs)
		cpus += int64(n.CPUs)
		allocMem += n.AllocMemory
		mem += n.RealMemory
	}
	var parts []string
	if cpus > 0 {
		parts = append(parts, "cpu "+percentOf(allocCPUs, cpus))
	}
	if mem > 0 {
		parts = append(parts, "mem "+percentOf(allocMem, mem))
	}
	if len(parts) == 0 {
		return ""
	}
	return dimStyle.Render(strings.Join(parts, factSeparator))
}

func jobGroupFacts(rows []row) []string {
	var running, pending int
	for _, r := range rows {
		switch r.group {
		case runningGroup:
			running++
		case pendingGroup:
			pending++
		}
	}
	var facts []string
	if running > 0 {
		facts = append(facts, dimStyle.Render(fmt.Sprintf("%d running", running)))
	}
	if pending > 0 {
		facts = append(facts, dimStyle.Render(fmt.Sprintf("%d pending", pending)))
	}
	return facts
}
