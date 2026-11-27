package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	focusColor  = ansi.Blue
	okColor     = ansi.Green
	warnColor   = ansi.Yellow
	badColor    = ansi.Red
	accentColor = ansi.Cyan
)

const (
	logoShineColor  = ansi.BrightWhite
	logoCanColor    = ansi.White
	logoShadeColor  = ansi.BrightBlack
	logoBandColor   = ansi.BrightCyan
	logoBandShade   = ansi.Cyan
	logoLabelColor  = ansi.Blue
	logoNodeColor   = ansi.BrightGreen
	logoBubbleColor = ansi.Cyan
)

var (
	textStyle   = lipgloss.NewStyle()
	dimStyle    = lipgloss.NewStyle().Faint(true)
	keyStyle    = lipgloss.NewStyle().Bold(true)
	focusStyle  = lipgloss.NewStyle().Foreground(focusColor)
	titleStyle  = focusStyle.Bold(true)
	okStyle     = lipgloss.NewStyle().Foreground(okColor)
	warnStyle   = lipgloss.NewStyle().Foreground(warnColor)
	badStyle    = lipgloss.NewStyle().Foreground(badColor)
	accentStyle = lipgloss.NewStyle().Foreground(accentColor)
	cursorStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
)
