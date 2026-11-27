package ui

import (
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/awalvie/slurmtui/internal/slurm"
)

type stateClass struct {
	glyph string
	style lipgloss.Style
}

func (c stateClass) cell(label string) string {
	return c.glyph + " " + label
}

type nodeClass int

const (
	nodeDown nodeClass = iota
	nodeDrain
	nodeMixed
	nodeAllocated
	nodeIdle
	nodeOther
)

var nodeClasses = map[nodeClass]stateClass{
	nodeDown:      {glyph: "✕", style: badStyle.Bold(true)},
	nodeDrain:     {glyph: "◆", style: warnStyle.Bold(true)},
	nodeMixed:     {glyph: "◐", style: okStyle},
	nodeAllocated: {glyph: "●", style: okStyle},
	nodeIdle:      {glyph: "○", style: textStyle},
	nodeOther:     {glyph: "◌", style: dimStyle},
}

var flagsThatMakeIdleOther = []string{
	"POWERED_DOWN", "POWERING_UP", "POWERING_DOWN", "MAINTENANCE",
	"REBOOT_REQUESTED", "REBOOT_ISSUED", "RESERVED",
}

func classOfNode(s slurm.NodeState) nodeClass {
	switch {
	case s.Down():
		return nodeDown
	case s.Has("DRAIN"):
		return nodeDrain
	case s.Base == "MIXED":
		return nodeMixed
	case s.Base == "ALLOCATED" || s.Has("COMPLETING"):
		return nodeAllocated
	case s.Base == "IDLE" && !slices.ContainsFunc(flagsThatMakeIdleOther, s.Has):
		return nodeIdle
	}
	return nodeOther
}

type jobClass int

const (
	jobPending jobClass = iota
	jobRunning
	jobCompleting
	jobCompleted
	jobFailed
	jobStopped
)

var jobClasses = map[jobClass]stateClass{
	jobPending:    {glyph: "◷", style: textStyle},
	jobRunning:    {glyph: "▶", style: okStyle},
	jobCompleting: {glyph: "◼", style: accentStyle},
	jobCompleted:  {glyph: "✓", style: dimStyle},
	jobFailed:     {glyph: "✕", style: badStyle.Bold(true)},
	jobStopped:    {glyph: "✕", style: badStyle},
}

func classOfJob(s slurm.JobState) jobClass {
	switch {
	case s.Has("COMPLETING") || s.Has("STAGE_OUT"):
		return jobCompleting
	case s.Base == "PENDING":
		return jobPending
	case s.Base == "RUNNING" || s.Base == "SUSPENDED":
		return jobRunning
	case s.Base == "COMPLETED":
		return jobCompleted
	case s.Base == "FAILED" || s.Base == "NODE_FAIL" || s.Base == "BOOT_FAIL" || s.Base == "OUT_OF_MEMORY":
		return jobFailed
	}
	return jobStopped
}

func nodeStateCell(s slurm.NodeState) cell {
	c := nodeClasses[classOfNode(s)]
	return cell{text: c.cell(s.Label()), style: c.style}
}

func jobStateCell(s slurm.JobState) cell {
	c := jobClasses[classOfJob(s)]
	return cell{text: c.cell(s.Label()), style: c.style}
}

const glyphAndSpace = 2

func nodeStateWidth() int {
	return glyphAndSpace + slurm.WidestNodeLabel()
}

func jobStateWidth() int {
	return glyphAndSpace + slurm.WidestJobLabel()
}
