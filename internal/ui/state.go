package ui

import (
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

var nodeClasses = map[slurm.NodeClass]stateClass{
	slurm.NodeDown:      {glyph: "✕", style: badStyle.Bold(true)},
	slurm.NodeDrain:     {glyph: "◆", style: warnStyle.Bold(true)},
	slurm.NodeMixed:     {glyph: "◐", style: okStyle},
	slurm.NodeAllocated: {glyph: "●", style: okStyle},
	slurm.NodeIdle:      {glyph: "○", style: textStyle},
	slurm.NodeOther:     {glyph: "◌", style: dimStyle},
}

var jobClasses = map[slurm.JobClass]stateClass{
	slurm.JobPending:    {glyph: "◷", style: textStyle},
	slurm.JobRunning:    {glyph: "▶", style: okStyle},
	slurm.JobCompleting: {glyph: "◼", style: accentStyle},
	slurm.JobCompleted:  {glyph: "✓", style: dimStyle},
	slurm.JobFailed:     {glyph: "✕", style: badStyle.Bold(true)},
	slurm.JobStopped:    {glyph: "✕", style: badStyle},
}

func nodeStateCell(s slurm.NodeState) cell {
	c := nodeClasses[s.Class()]
	return cell{text: c.cell(s.Label()), style: c.style}
}

func jobStateCell(s slurm.JobState) cell {
	c := jobClasses[s.Class()]
	return cell{text: c.cell(s.Label()), style: c.style}
}
