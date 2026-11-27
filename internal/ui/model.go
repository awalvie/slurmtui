package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/slurm"
)

type Source interface {
	Nodes(ctx context.Context) (slurm.Nodes, error)
	Jobs(ctx context.Context) (slurm.Jobs, error)
	Partitions(ctx context.Context) ([]slurm.Partition, error)
	UpdateNode(ctx context.Context, name string, update slurm.NodeUpdate) error
	CancelJob(ctx context.Context, id string) error
}

type view string

const (
	overviewView view = "overview"
	nodesView    view = "nodes"
	jobsView     view = "jobs"
	helpView     view = "help"
	describeView view = "describe"
	jsonView     view = "json"
)

var viewCommands = map[string]view{
	"nodes": nodesView, "no": nodesView,
	"jobs": jobsView, "j": jobsView,
	"overview": overviewView, "ov": overviewView,
}

type Session struct {
	Context  string
	User     string
	Expires  time.Time
	ReadOnly bool
}

type Model struct {
	source       Source
	session      Session
	halted       bool
	clock        clock.Clock
	interval     time.Duration
	meta         slurm.Meta
	nodes        []slurm.Node
	jobs         []slurm.Job
	partitions   map[string]slurm.Partition
	hosts        map[string]jobHosts
	stack        []frame
	nodePane     frame
	jobPane      frame
	focus        view
	err          error
	width        int
	height       int
	command      *string
	filtering    bool
	filterBefore string
	fetching     bool
	lastOK       time.Time
	cursors      map[string]cursorAt
	built        map[frame]table

	dialog        *confirm
	flash         *flash
	refreshQueued bool
}

func New(source Source, session Session, clk clock.Clock, interval time.Duration) Model {
	return Model{source: source, session: session, clock: clk, interval: interval, fetching: true, stack: []frame{{kind: overviewView}}, nodePane: frame{kind: nodesView}, jobPane: frame{kind: jobsView}, focus: nodesView}
}

type refreshTickMsg struct{}

type footerTickMsg struct{}

func (m Model) after(d time.Duration, msg tea.Msg) tea.Cmd {
	ch := m.clock.After(d)
	return func() tea.Msg {
		<-ch
		return msg
	}
}

func (m Model) refresh() (Model, tea.Cmd) {
	if m.fetching || m.halted {
		return m, nil
	}
	m.fetching = true
	return m, m.fetch
}

type snapshotMsg struct {
	nodes      slurm.Nodes
	jobs       slurm.Jobs
	partitions []slurm.Partition
	err        error
}

func (m Model) fetch() tea.Msg {
	ctx := context.Background()
	nodes, err := m.source.Nodes(ctx)
	if err != nil {
		return snapshotMsg{err: err}
	}
	jobs, err := m.source.Jobs(ctx)
	if err != nil {
		return snapshotMsg{err: err}
	}
	partitions, err := m.source.Partitions(ctx)
	if err != nil {
		return snapshotMsg{err: err}
	}
	return snapshotMsg{nodes: nodes, jobs: jobs, partitions: partitions}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetch, m.after(m.interval, refreshTickMsg{}), m.after(time.Second, footerTickMsg{}))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case refreshTickMsg:
		next := m.after(m.interval, refreshTickMsg{})
		if m.dialog != nil {
			return m, next
		}
		m, cmd := m.refresh()
		return m, tea.Batch(next, cmd)
	case footerTickMsg:
		return m.expireFlash(), m.after(time.Second, footerTickMsg{})
	case actionDoneMsg:
		return m.actionDone(msg)
	case snapshotMsg:
		m.fetching = false
		if m.refreshQueued {
			m.refreshQueued = false
			return m.refresh()
		}
		var expired auth.ExpiredError
		if errors.As(msg.err, &expired) {
			m.halted = true
			m.err = expired
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.lastOK = m.clock.Now()
		m.meta = msg.nodes.Meta
		m.nodes = slices.Clone(msg.nodes.Nodes)
		m.jobs = slices.Clone(msg.jobs.Jobs)
		m.hosts = expandJobHosts(m.jobs)
		m.partitions = partitionsByName(msg.partitions)
		m = m.anchorCursors()
	case tea.PasteMsg:
		if m.dialog != nil {
			pasted := m.dialog.typed(singleLine(msg.Content))
			m.dialog = &pasted
		}
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.dialog != nil {
		return m.dialogKey(msg)
	}
	if m.command != nil {
		switch key {
		case "enter":
			cmd := *m.command
			m.command = nil
			if cmd == "q" || cmd == "quit" {
				return m, tea.Quit
			}
			v, ok := viewCommands[cmd]
			if !ok {
				return m.say(flashWarning, ":"+cmd+" is not a command · ? lists the commands"), nil
			}
			m = m.show(v)
		case "esc":
			m.command = nil
		case "backspace":
			text := dropLastRune(*m.command)
			m.command = &text
		default:
			text := *m.command + msg.Text
			m.command = &text
		}
		return m, nil
	}
	if m.filtering {
		return m.filterKey(msg), nil
	}
	return m.paneKey(key)
}

const (
	minWidth         = 80
	minHeight        = 24
	airyHeight       = 30
	minBodyHeight    = 7
	bottomLineHeight = 1
)

func (m Model) size() (int, int) {
	return screenSize(m.width, m.height)
}

func screenSize(width, height int) (int, int) {
	if width == 0 || height == 0 {
		return minWidth, minHeight
	}
	return width, height
}

func altView(screen string) tea.View {
	v := tea.NewView(screen)
	v.AltScreen = true
	return v
}

func (m Model) bodyHeight() int {
	_, height := m.size()
	return max(height-headerRows-m.blankUnderHeader()-bottomLineHeight, minBodyHeight)
}

func (m Model) blankUnderHeader() int {
	if _, height := m.size(); height >= airyHeight {
		return 1
	}
	return 0
}

const tooSmallText = "terminal too small (80x24 minimum)"

func (m Model) View() tea.View {
	width, height := m.size()
	if width < minWidth || height < minHeight {
		return altView(badStyle.Render(tooSmallText))
	}
	base := m.withBuiltTables()
	lines := base.header(width)
	for range m.blankUnderHeader() {
		lines = append(lines, "")
	}
	body, problem := base.body(width-2, m.bodyHeight())
	for _, l := range body {
		lines = append(lines, frameIndent+l)
	}
	for len(lines) < height-bottomLineHeight {
		lines = append(lines, "")
	}
	lines = append(lines[:height-bottomLineHeight], m.bottomLine(width, problem))
	screen := strings.Join(lines, "\n")
	if m.dialog != nil {
		screen = overlaid(lines, m.dialog.view(width), width, headerRows+m.blankUnderHeader(), m.bodyHeight())
	}
	return altView(screen)
}
