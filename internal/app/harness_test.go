package app_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/app"
	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

var startTime = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

func providedToken(sun string, exp time.Time) string {
	part := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := part(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims := part(map[string]any{"exp": exp.Unix(), "iat": startTime.Unix(), "sun": sun})
	return header + "." + claims + "." + base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
}

var aliceToken = providedToken("alice", startTime.Add(time.Hour))

func configDir(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), []byte(yaml))
	return dir
}

const (
	screenWidth  = 160
	screenHeight = 50
	waitLimit    = 5 * time.Second
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type cookedOutput struct {
	mu   *sync.Mutex
	term *vt.SafeEmulator
	raw  *lockedBuffer
}

func (c cookedOutput) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.raw.Write(p); err != nil {
		return 0, err
	}
	if _, err := c.term.Write(bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))); err != nil {
		return 0, err
	}
	return len(p), nil
}

type run struct {
	t      *testing.T
	term   *vt.SafeEmulator
	termMu *sync.Mutex
	input  *io.PipeWriter
	stderr *lockedBuffer
	output *lockedBuffer
	clock  *clock.Fake
	width  int
	height int
	done   chan struct{}
	code   int
}

func start(t *testing.T, args []string, env map[string]string) *run {
	t.Helper()
	return startWithClock(t, clock.NewFake(startTime), args, env)
}

func startWithClock(t *testing.T, clk *clock.Fake, args []string, env map[string]string) *run {
	t.Helper()
	return startSized(t, clk, screenWidth, screenHeight, args, env)
}

func startSized(t *testing.T, clk *clock.Fake, width, height int, args []string, env map[string]string) *run {
	t.Helper()
	term := vt.NewSafeEmulator(width, height)
	inR, inW := io.Pipe()
	go func() {
		if _, err := io.Copy(inW, term); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			panic(fmt.Sprintf("copy terminal replies: %s", err))
		}
	}()
	r := &run{
		t:      t,
		term:   term,
		input:  inW,
		termMu: &sync.Mutex{},
		stderr: &lockedBuffer{},
		output: &lockedBuffer{},
		clock:  clk,
		width:  width,
		height: height,
		done:   make(chan struct{}),
	}
	fullEnv := map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "CLICOLOR_FORCE": "1"}
	_, hasDir := env["SLURMTUI_CONFIG_DIR"]
	_, hasXDG := env["XDG_CONFIG_HOME"]
	if !hasDir && !hasXDG {
		fullEnv["SLURMTUI_CONFIG_DIR"] = t.TempDir()
	}
	for k, v := range env {
		fullEnv[k] = v
	}
	go func() {
		defer close(r.done)
		r.code = app.Main(app.Process{
			Args:   args,
			Env:    fullEnv,
			In:     inR,
			Out:    cookedOutput{mu: r.termMu, term: term, raw: r.output},
			Err:    r.stderr,
			Clock:  r.clock,
			Width:  width,
			Height: height,
		})
	}()
	t.Cleanup(func() {
		select {
		case <-r.done:
		default:
			r.send("\x03")
			select {
			case <-r.done:
			case <-time.After(waitLimit):
				t.Errorf("slurmtui did not exit after ctrl-c")
			}
		}
		if err := inW.Close(); err != nil {
			t.Errorf("close input: %s", err)
		}
	})
	return r
}

func (r *run) screen() string {
	r.termMu.Lock()
	defer r.termMu.Unlock()
	lines := make([]string, 0, r.height)
	for y := range r.height {
		var line strings.Builder
		for x := range r.width {
			cell := r.term.CellAt(x, y)
			if cell == nil || cell.Content == "" {
				line.WriteByte(' ')
				continue
			}
			line.WriteString(cell.Content)
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.Join(lines, "\n")
}

func (r *run) waitFor(text string) {
	r.t.Helper()
	r.waitUntil(fmt.Sprintf("%q on the screen", text), func() bool { return strings.Contains(r.screen(), text) })
}

const escKey = "\x1b[27u"

func (r *run) send(text string) {
	if _, err := io.WriteString(r.input, text); err != nil {
		r.t.Fatalf("send %q: %s", text, err)
	}
}

func (r *run) waitExit() int {
	r.t.Helper()
	select {
	case <-r.done:
		return r.code
	case <-time.After(waitLimit):
		r.t.Fatalf("slurmtui did not exit\nscreen:\n%s", r.screen())
		return 0
	}
}

const (
	frameLeft = 1
	leadX     = 2
	gridLeft  = 3
)

type tableRow struct {
	cells map[string]string
	ends  map[string]int
	y     int
}

func (r *run) cellAt(x, y int) uv.Cell {
	r.termMu.Lock()
	defer r.termMu.Unlock()
	cell := r.term.CellAt(x, y)
	if cell == nil {
		return uv.Cell{}
	}
	return *cell
}

func isANSI(c color.Color, index int) bool {
	b, ok := c.(ansi.BasicColor)
	return ok && int(b) == index
}

func (r *run) frameTop(title string) int {
	for y, line := range strings.Split(r.screen(), "\n") {
		if strings.HasPrefix(line, " ┌ "+title) {
			return y
		}
	}
	return -1
}

func (r *run) table() []tableRow {
	r.t.Helper()
	return r.tableAt(r.tableTop())
}

func (r *run) tableTop() int {
	r.t.Helper()
	first := -1
	for _, title := range []string{"Nodes", "Jobs"} {
		top := r.frameTop(title)
		if top < 0 {
			continue
		}
		if isANSI(r.cellAt(gridLeft, top).Style.Fg, ansiBlue) {
			return top
		}
		if first < 0 {
			first = top
		}
	}
	if first < 0 {
		r.t.Fatalf("no Nodes or Jobs frame\nscreen:\n%s", r.screen())
	}
	return first
}

func (r *run) mustFrameTop(title string) int {
	r.t.Helper()
	top := r.frameTop(title)
	if top < 0 {
		r.t.Fatalf("no %s frame\nscreen:\n%s", title, r.screen())
	}
	return top
}

func (r *run) frameRows(title string) (int, int) {
	r.t.Helper()
	top := r.mustFrameTop(title)
	lines := strings.Split(r.screen(), "\n")
	bottom := top
	for !strings.HasPrefix(lines[bottom], " └") {
		bottom++
	}
	return top, bottom
}

func (r *run) pane(title string) []tableRow {
	r.t.Helper()
	return r.tableAt(r.mustFrameTop(title))
}

type tableColumn struct {
	name       string
	start, end int
	headerEnd  int
}

func lastText(line []rune, start, end int) int {
	for x := end; x > start; x-- {
		if line[x-1] != ' ' {
			return x
		}
	}
	return 0
}

func (r *run) tableAt(top int) []tableRow {
	r.t.Helper()
	_, rows := r.tableLayoutAt(top)
	return rows
}

func (r *run) tableLayout() ([]tableColumn, []tableRow) {
	r.t.Helper()
	return r.tableLayoutAt(r.tableTop())
}

func (r *run) columnNames() string {
	r.t.Helper()
	return r.columnNamesAt(r.tableTop())
}

func (r *run) columnNamesIn(title string) string {
	r.t.Helper()
	return r.columnNamesAt(r.mustFrameTop(title))
}

func (r *run) columnNamesAt(top int) string {
	r.t.Helper()
	columns, _ := r.tableLayoutAt(top)
	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = c.name
	}
	return strings.Join(names, " ")
}

func (r *run) rightAligned(name string) bool {
	r.t.Helper()
	columns, rows := r.tableLayout()
	for _, c := range columns {
		if c.name != name {
			continue
		}
		for _, row := range rows {
			if row.ends[name] != 0 && row.ends[name] != c.end {
				return false
			}
		}
		return c.headerEnd == c.end
	}
	r.t.Fatalf("no column %s\nscreen:\n%s", name, r.screen())
	return false
}

func (r *run) tableLayoutAt(top int) ([]tableColumn, []tableRow) {
	r.t.Helper()
	lines := strings.Split(r.screen(), "\n")
	padded := func(y int) []rune {
		line := []rune(lines[y])
		for len(line) < r.width {
			line = append(line, ' ')
		}
		return line
	}
	header := padded(top + 1)
	if header[frameLeft] != '│' {
		r.t.Fatalf("no column header row under the frame title on row %d\nscreen:\n%s", top, r.screen())
	}
	right := r.width - 2
	var body [][]rune
	var ys []int
	for y := top + 2; y < len(lines); y++ {
		line := padded(y)
		if line[frameLeft] != '│' {
			break
		}
		text := strings.TrimSpace(string(line[gridLeft:right]))
		if text == "" || r.isEmptyMessage(y, text) {
			continue
		}
		body = append(body, line)
		ys = append(ys, y)
	}
	blank := func(x int) bool {
		if x >= right {
			return true
		}
		if header[x] != ' ' {
			return false
		}
		for _, line := range body {
			if line[x] != ' ' {
				return false
			}
		}
		return true
	}
	var columns []tableColumn
	for x := gridLeft; x < right; {
		if blank(x) {
			x++
			continue
		}
		start := x
		for x < right && (!blank(x) || !blank(x+1)) {
			x++
		}
		name := strings.TrimSpace(string(header[start:x]))
		name = strings.TrimSuffix(strings.TrimSuffix(name, " ↑"), " ↓")
		if name == "" && len(columns) > 0 {
			columns[len(columns)-1].end = x
			continue
		}
		columns = append(columns, tableColumn{name: name, start: start, end: x})
	}
	for i := range columns {
		columns[i].headerEnd = lastText(header, columns[i].start, columns[i].end)
	}
	rows := make([]tableRow, 0, len(body))
	for i, line := range body {
		row := tableRow{cells: map[string]string{}, ends: map[string]int{}, y: ys[i]}
		for _, c := range columns {
			row.cells[c.name] = strings.TrimSpace(string(line[c.start:c.end]))
			row.ends[c.name] = lastText(line, c.start, c.end)
		}
		rows = append(rows, row)
	}
	return columns, rows
}

func (r *run) isEmptyMessage(y int, text string) bool {
	faint := r.cellAt(gridLeft, y).Style.Attrs&uv.AttrFaint != 0
	return faint && (strings.HasPrefix(text, "no nodes") || strings.HasPrefix(text, "no jobs"))
}

func stateLabel(cell string) string {
	_, label, _ := strings.Cut(cell, " ")
	return label
}

func (row tableRow) key() string {
	if id := row.cells["ID"]; id != "" {
		return id
	}
	return row.cells["NAME"]
}

func (r *run) rowByName(name string) tableRow {
	r.t.Helper()
	_, row := r.rowIndex(r.table(), name)
	return row
}

func (r *run) rowByNameIn(title, name string) tableRow {
	r.t.Helper()
	_, row := r.rowIndex(r.pane(title), name)
	return row
}

func (r *run) rowIndex(rows []tableRow, name string) (int, tableRow) {
	r.t.Helper()
	for i, row := range rows {
		if row.key() == name {
			return i, row
		}
	}
	r.t.Fatalf("no row %q\nscreen:\n%s", name, r.screen())
	return 0, tableRow{}
}

func (r *run) columnX(row tableRow, name string) int {
	r.t.Helper()
	line := []rune(strings.Split(r.screen(), "\n")[row.y])
	value := []rune(row.cells[name])
	if len(value) == 0 {
		r.t.Fatalf("row on line %d has no %s cell", row.y, name)
	}
	for x := gridLeft; x+len(value) <= len(line); x++ {
		if string(line[x:x+len(value)]) == string(value) {
			return x
		}
	}
	r.t.Fatalf("no %q on line %d", row.cells[name], row.y)
	return 0
}

const steadyTimers = 2

func (r *run) advance(d time.Duration) {
	r.t.Helper()
	r.waitUntil("slurmtui to arm its timers", func() bool { return r.clock.Waiters() >= steadyTimers })
	r.clock.Advance(d)
}

func (r *run) waitUntil(what string, cond func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for !cond() {
		if time.Now().After(deadline) {
			r.t.Fatalf("timed out waiting for %s\nscreen:\n%s\nstderr:\n%s", what, r.screen(), r.stderr.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *run) selected() string {
	r.t.Helper()
	var rows []tableRow
	for _, title := range []string{"Nodes", "Jobs"} {
		if r.frameTop(title) >= 0 {
			rows = append(rows, r.pane(title)...)
		}
	}
	for _, row := range rows {
		lead := r.cellAt(leadX, row.y)
		first := r.cellAt(gridLeft, row.y)
		if lead.Content != "▌" || first.Style.Attrs&uv.AttrReverse == 0 {
			continue
		}
		return row.key()
	}
	return ""
}

func (r *run) moveTo(name string) {
	r.t.Helper()
	i, _ := r.rowIndex(r.table(), name)
	r.send(strings.Repeat("j", i))
	r.waitSelected(name)
}

func (r *run) waitSelected(name string) {
	r.t.Helper()
	r.waitUntil("selection on "+name, func() bool { return r.selected() == name })
}

func startupError(t *testing.T, args []string, env map[string]string) string {
	t.Helper()
	r := start(t, args, env)
	if code := r.waitExit(); code == 0 {
		t.Fatalf("exit code = 0, want non-zero\nscreen:\n%s", r.screen())
	}
	return r.stderr.String()
}

func wantStderr(t *testing.T, stderr string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(stderr, p) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, p)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %s", path, err)
	}
}

func (r *run) fullScreen(command, title string) {
	r.t.Helper()
	other := map[string]string{"Nodes": "Jobs", "Jobs": "Nodes"}[title]
	r.send(":" + command + "\r")
	r.waitUntil(title+" at full screen", func() bool { return r.frameTop(title) >= 0 && r.frameTop(other) < 0 })
}

func (r *run) offsetOf(text string) (int, int) {
	r.t.Helper()
	for y, line := range strings.Split(r.screen(), "\n") {
		if at := strings.Index(line, text); at >= 0 {
			return len([]rune(line[:at])), y
		}
	}
	r.t.Fatalf("screen has no %q\n%s", text, r.screen())
	return 0, 0
}

func (r *run) styleOf(text string) uv.Style {
	r.t.Helper()
	return r.cellAt(r.offsetOf(text)).Style
}

func requestsWhere(cluster *fakeslurm.Server, keep func(fakeslurm.Request) bool) []fakeslurm.Request {
	var out []fakeslurm.Request
	for _, req := range cluster.Requests() {
		if keep(req) {
			out = append(out, req)
		}
	}
	return out
}

func (r *run) waitOneRequest(cluster *fakeslurm.Server, what string, keep func(fakeslurm.Request) bool) fakeslurm.Request {
	r.t.Helper()
	r.waitUntil("a "+what+" request", func() bool { return len(requestsWhere(cluster, keep)) > 0 })
	got := requestsWhere(cluster, keep)
	if len(got) != 1 {
		r.t.Fatalf("got %d %s requests, want 1: %+v", len(got), what, got)
	}
	return got[0]
}

func checkBody(t *testing.T, req fakeslurm.Request, wantPath, wantBody string) {
	t.Helper()
	if req.Path != wantPath {
		t.Errorf("path = %s, want %s", req.Path, wantPath)
	}
	var got, want any
	if err := json.Unmarshal(req.Body, &got); err != nil {
		t.Fatalf("body %q is not JSON: %s", req.Body, err)
	}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("want body %q is not JSON: %s", wantBody, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %s, want %s", req.Body, wantBody)
	}
}

func startSizedOn(t *testing.T, cluster *fakeslurm.Server, width, height int) *run {
	t.Helper()
	return startSized(t, clock.NewFake(startTime), width, height, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURM_JWT": aliceToken})
}

func startProvided(t *testing.T, cluster *fakeslurm.Server) *run {
	t.Helper()
	return startSizedOn(t, cluster, screenWidth, screenHeight)
}

func startReadOnly(t *testing.T, cluster *fakeslurm.Server) *run {
	t.Helper()
	r := start(t, []string{"--readonly", "--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")
	return r
}
