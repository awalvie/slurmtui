package app_test

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	ansiRed        = 1
	ansiGreen      = 2
	ansiYellow     = 3
	ansiBlue       = 4
	ansiCyan       = 6
	ansiBrightCyan = 14
)

func nodeIn(name string, state ...string) fakeslurm.Node {
	n := idle(name)
	n.State = state
	return n
}

func jobIn(id int32, state ...string) fakeslurm.Job {
	j := runningJob(id)
	j.JobState = state
	return j
}

func classFixture() fakeslurm.Cluster {
	return fakeslurm.Cluster{
		Name: "lamp",
		Nodes: []fakeslurm.Node{
			nodeIn("a0", "DOWN"),
			nodeIn("n-down", "DOWN", "NOT_RESPONDING"),
			nodeIn("n-drain", "IDLE", "DRAIN"),
			nodeIn("n-mixed", "MIXED"),
			nodeIn("n-alloc", "ALLOCATED"),
			nodeIn("n-idle", "IDLE"),
			nodeIn("n-off", "IDLE", "POWERED_DOWN"),
		},
		Jobs: []fakeslurm.Job{
			jobIn(1099, "PENDING"),
			runningJob(1001),
			jobIn(1002, "PENDING"),
			jobIn(1003, "RUNNING", "COMPLETING"),
			jobIn(1004, "COMPLETED"),
			jobIn(1005, "FAILED"),
			jobIn(1006, "CANCELLED"),
		},
	}
}

func (r *run) stateCellStyle(name string) (string, uv.Style) {
	r.t.Helper()
	row := r.rowByName(name)
	x := r.columnX(row, "STATE")
	return row.cells["STATE"], r.cellAt(x, row.y).Style
}

func TestStateCellsShowGlyphAndClassColour(t *testing.T) {
	r := startProvided(t, fakeslurm.Start(t, classFixture()))
	r.waitFor("┌ Nodes 7 ")

	nodes := []struct {
		name, state string
		color       int
		bold, faint bool
	}{
		{"n-down", "✕ down*", ansiRed, true, false},
		{"n-drain", "◆ drained", ansiYellow, true, false},
		{"n-mixed", "◐ mixed", ansiGreen, false, false},
		{"n-alloc", "● allocated", ansiGreen, false, false},
		{"n-idle", "○ idle", -1, false, false},
		{"n-off", "◌ idle~", -1, false, true},
	}
	for _, n := range nodes {
		text, style := r.stateCellStyle(n.name)
		if text != n.state {
			t.Errorf("%s: STATE = %q, want %q", n.name, text, n.state)
		}
		if n.color >= 0 && !isANSI(style.Fg, n.color) {
			t.Errorf("%s: STATE colour = %v, want ANSI %d", n.name, style.Fg, n.color)
		}
		if n.color < 0 && style.Fg != nil {
			t.Errorf("%s: STATE colour = %v, want the default colour", n.name, style.Fg)
		}
		if got := style.Attrs&uv.AttrBold != 0; got != n.bold {
			t.Errorf("%s: STATE bold = %v, want %v", n.name, got, n.bold)
		}
		if got := style.Attrs&uv.AttrFaint != 0; got != n.faint {
			t.Errorf("%s: STATE faint = %v, want %v", n.name, got, n.faint)
		}
		row := r.rowByName(n.name)
		if fg := r.cellAt(gridLeft, row.y).Style.Fg; fg != nil {
			t.Errorf("%s: NAME colour = %v, want the default colour", n.name, fg)
		}
	}

	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 7 ")
	jobs := []struct {
		id, state   string
		color       int
		bold, faint bool
	}{
		{"1001", "▶ RUNNING", ansiGreen, false, false},
		{"1002", "◷ PENDING", -1, false, false},
		{"1003", "◼ COMPLETING", ansiCyan, false, false},
		{"1004", "✓ COMPLETED", -1, false, true},
		{"1005", "✕ FAILED", ansiRed, true, false},
		{"1006", "✕ CANCELLED", ansiRed, false, false},
	}
	for _, j := range jobs {
		text, style := r.stateCellStyle(j.id)
		if text != j.state {
			t.Errorf("job %s: STATE = %q, want %q", j.id, text, j.state)
		}
		if j.color >= 0 && !isANSI(style.Fg, j.color) {
			t.Errorf("job %s: STATE colour = %v, want ANSI %d", j.id, style.Fg, j.color)
		}
		if j.color < 0 && style.Fg != nil {
			t.Errorf("job %s: STATE colour = %v, want the default colour", j.id, style.Fg)
		}
		if got := style.Attrs&uv.AttrBold != 0; got != j.bold {
			t.Errorf("job %s: STATE bold = %v, want %v", j.id, got, j.bold)
		}
		if got := style.Attrs&uv.AttrFaint != 0; got != j.faint {
			t.Errorf("job %s: STATE faint = %v, want %v", j.id, got, j.faint)
		}
	}
	for id, ended := range map[string]bool{"1001": false, "1004": true, "1005": true, "1006": true} {
		row := r.rowByName(id)
		style := r.cellAt(gridLeft, row.y).Style
		if style.Fg != nil || (style.Attrs&uv.AttrFaint != 0) != ended {
			t.Errorf("job %s: ID style = %v faint %v, want the default colour, faint %v", id, style.Fg, style.Attrs&uv.AttrFaint != 0, ended)
		}
	}
}

func TestCursorRowIsReversedWithoutStateColour(t *testing.T) {
	r := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("a0"), nodeIn("n-down", "DOWN")}}))
	r.waitFor("┌ Nodes 2 ")
	r.moveTo("n-down")

	row := r.rowByName("n-down")
	lead := r.cellAt(leadX, row.y)
	if !isANSI(lead.Style.Fg, ansiBlue) || lead.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("lead cell = %q %v, want a blue bold ▌", lead.Content, lead.Style)
	}
	_, style := r.stateCellStyle("n-down")
	if style.Fg != nil || style.Attrs&uv.AttrReverse == 0 || style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("STATE on the cursor row = %v, want reverse and bold in the default colours", style)
	}
	for x := gridLeft; x < r.width-2; x++ {
		if r.cellAt(x, row.y).Style.Attrs&uv.AttrReverse == 0 {
			t.Fatalf("cursor row is not reversed at x=%d", x)
		}
	}
}

func TestFramesAreSquareAndOnTheGrid(t *testing.T) {
	r := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n01"), idle("n02"), idle("n03")}}))
	r.waitFor("┌ Nodes 3 ")

	top, bottom := r.frameRows("Nodes")
	right := r.width - 2
	corners := map[[2]int]string{{1, top}: "┌", {right, top}: "┐", {1, bottom}: "└", {right, bottom}: "┘"}
	for at, want := range corners {
		if got := r.cellAt(at[0], at[1]).Content; got != want {
			t.Errorf("cell %v = %q, want %q", at, got, want)
		}
	}
	if got := r.cellAt(right+1, top).Content; got != "" && got != " " {
		t.Errorf("frame goes past 1 column before the right edge: %q", got)
	}
	header := r.rowByName("n01")
	lines := strings.Split(r.screen(), "\n")
	if !strings.HasPrefix(string([]rune(lines[top+1])[gridLeft:]), "NAME") {
		t.Errorf("column header row does not start in column 3: %q", lines[top+1])
	}
	if x := r.columnX(header, "NAME"); x != gridLeft {
		t.Errorf("first cell starts in column %d, want 3", x)
	}
	if !strings.HasPrefix(lines[0], "   Context") {
		t.Errorf("header does not start in column 3: %q", lines[0])
	}
	for x := gridLeft; x < gridLeft+4; x++ {
		if r.cellAt(x, top+1).Style.Attrs&uv.AttrBold == 0 {
			t.Fatalf("column header row is not bold at x=%d", x)
		}
	}
}

func TestTitlesNameCountScopeAndFilter(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{
		Name:  "lamp",
		Nodes: []fakeslurm.Node{idle("n01"), idle("n02"), idle("n10")},
		Jobs:  []fakeslurm.Job{runningJob(1), runningJob(2), runningJob(3), runningJob(4), runningJob(5)},
	})
	jobs := []fakeslurm.Job{runningJob(1), runningJob(2), runningJob(3), runningJob(4), runningJob(5)}
	for i := range jobs {
		jobs[i].Nodes = "n01"
		if i < 3 {
			jobs[i].Nodes = "n02"
		}
	}
	cluster.SetJobs(jobs)
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 3 ─")

	top := r.frameTop("Nodes")
	name := r.cellAt(3, top)
	if name.Content != "N" || !isANSI(name.Style.Fg, ansiBlue) || name.Style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("title name = %q %v, want blue bold", name.Content, name.Style)
	}
	if count := r.cellAt(9, top); count.Content != "3" || count.Style.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("title count = %q %v, want a faint 3", count.Content, count.Style)
	}

	r.send("/n0\r")
	r.waitFor("┌ Nodes 2 · /n0 ─")
	if filter := r.cellAt(13, top); filter.Content != "/" || !isANSI(filter.Style.Fg, ansiCyan) {
		t.Fatalf("filter = %q %v, want cyan", filter.Content, filter.Style)
	}
	if sep := r.cellAt(11, top); sep.Content != "·" || sep.Style.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("separator = %q %v, want faint", sep.Content, sep.Style)
	}
	r.send("/n1\r")
	r.waitFor("┌ Nodes 1 · /n1 ─")

	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 5 · 5 running ─")
	r.fullScreen("nodes", "Nodes")
	r.waitFor("┌ Nodes 3 ─")
	r.send("j\r")
	r.waitFor("┌ Jobs 3 · on n02 · 3 running ─")
}

func TestNodesTitleCountsProblemsAndAllocationOfUsableNodes(t *testing.T) {
	node := func(name string, allocCPUs int, allocMem int64, state ...string) fakeslurm.Node {
		n := nodeIn(name, state...)
		n.AllocCPUs, n.AllocMemory = allocCPUs, allocMem
		return n
	}
	r := startOnNodes(t,
		node("n1", 0, 0, "IDLE", "DRAIN"),
		node("n2", 32, 65536, "MIXED", "DRAIN"),
		node("n3", 0, 0, "DOWN", "NOT_RESPONDING"),
		node("n4", 0, 0, "IDLE", "POWERED_DOWN"),
		node("n5", 16, 32768, "MIXED"),
		node("n6", 0, 0, "IDLE"),
	)

	r.waitFor("┌ Nodes 6 · ◆ 1 draining · ◆ 1 drained · ✕ 1 down ─")
	r.waitFor("─ cpu 25% · mem 12% ┐")
	top := r.frameTop("Nodes")
	line := []rune(strings.Split(r.screen(), "\n")[top])
	for x, want := range map[string]int{"◆": ansiYellow, "✕": ansiRed} {
		at := slices.Index(line, []rune(x)[0])
		c := r.cellAt(at, top)
		if !isANSI(c.Style.Fg, want) || c.Style.Attrs&uv.AttrBold == 0 {
			t.Errorf("%s in the title = %v, want ANSI %d bold", x, c.Style, want)
		}
	}
	cpu := r.cellAt(slices.Index(line, 'c'), top)
	if cpu.Style.Attrs&uv.AttrFaint == 0 {
		t.Errorf("right text style = %v, want faint", cpu.Style)
	}
}

func TestJobsTitleCountsRunningAndPendingRows(t *testing.T) {
	r, _ := startOnJobs(t, []fakeslurm.Job{
		runningJob(1),
		jobIn(2, "CANCELLED", "COMPLETING"),
		pendingJob(3, "Priority"),
		pendingJob(4, "Priority"),
		endedJob(5, "COMPLETED"),
	})

	r.waitFor("┌ Jobs 5 · 2 running · 2 pending ─")
	r.send("/^1\r")
	r.waitFor("┌ Jobs 1 · 1 running · /^1 ─")
}

func TestStateChangeKeepsColumnPositions(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1"), idle("n2")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 2 ")
	positions := func() map[string]int {
		row := r.rowByName("n2")
		out := map[string]int{}
		for _, c := range []string{"NAME", "CPU", "MEM(GiB)"} {
			out[c] = r.columnX(row, c)
		}
		return out
	}
	before := positions()

	drained := nodeIn("n1", "IDLE", "DRAIN")
	drained.Reason = "x"
	cluster.SetNodes([]fakeslurm.Node{drained, idle("n2")})
	r.advance(5 * time.Second)
	r.waitFor("◆ drained")

	after := positions()
	for c, x := range before {
		if after[c] != x {
			t.Errorf("%s moved from x=%d to x=%d after a state change", c, x, after[c])
		}
	}
}

func TestIDIsAsWideAsTheIDsOnTheScreen(t *testing.T) {
	var jobs []fakeslurm.Job
	for i := range int32(70) {
		id := 1069 - i
		if i == 60 {
			j := runningJob(id)
			j.ArrayJobID = fakeslurm.Number(int64(id))
			j.ArrayTaskString = "1-99999"
			jobs = append(jobs, j)
			continue
		}
		jobs = append(jobs, runningJob(id))
	}
	r, _ := startOnJobs(t, jobs)
	idWidth := func() int {
		lines := strings.Split(r.screen(), "\n")
		header := []rune(lines[r.frameTop("Jobs")+1])
		return strings.Index(string(header[gridLeft:]), "PARTITION") - 2
	}
	if got := idWidth(); got != 4 {
		t.Fatalf("ID width with the array job off the screen = %d, want 4\nscreen:\n%s", got, r.screen())
	}

	r.send("G")
	r.waitFor("1009_[1-99999]")
	if got := idWidth(); got != 14 {
		t.Fatalf("ID width with the array job on the screen = %d, want 14\nscreen:\n%s", got, r.screen())
	}
}

func TestLongCellsAreCutInsideTheFrame(t *testing.T) {
	long := strings.Repeat("r", 200)
	node := nodeIn("n1", "IDLE", "DRAIN")
	node.Reason = long
	job := jobIn(7, "PENDING")
	job.StateReason = long
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{node}, Jobs: []fakeslurm.Job{job}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")
	r.checkFrameEdges("Nodes")
	if reason := r.rowByName("n1").cells["REASON"]; !strings.HasSuffix(reason, "r…") {
		t.Fatalf("REASON = %q, want a cut cell that ends with …", reason)
	}

	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 1 ")
	r.checkFrameEdges("Jobs")
	if where := r.rowByName("7").cells["NODELIST(REASON)"]; !strings.HasSuffix(where, "r…") {
		t.Fatalf("NODELIST(REASON) = %q, want a cut cell that ends with …", where)
	}
}

func (r *run) checkFrameEdges(title string) {
	r.t.Helper()
	top := r.frameTop(title)
	lines := strings.Split(r.screen(), "\n")
	for y := top + 1; !strings.HasPrefix(lines[y], " └"); y++ {
		if r.cellAt(frameLeft, y).Content != "│" || r.cellAt(r.width-2, y).Content != "│" {
			r.t.Fatalf("line %d does not start and end with │ at the frame edges: %q", y, lines[y])
		}
	}
}

func TestSortArrowFollowsTheSortedColumn(t *testing.T) {
	r := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}}))
	r.waitFor("┌ Nodes 1 ")
	r.waitFor("STATE ↑")
	r.send("S")
	r.waitFor("STATE ↓")
	r.send("N")
	r.waitFor("NAME ↑")
}

func TestHeaderInfoRows(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Version: fakeslurm.Version{Major: "26", Minor: "05", Micro: "5"}, Nodes: []fakeslurm.Node{idle("n1")}})
	r := start(t, []string{"--url", cluster.URL, "--readonly"}, map[string]string{"SLURM_JWT": providedToken("root", startTime.Add(time.Hour))})
	r.waitFor("┌ Nodes 1 ")

	lines := strings.Split(r.screen(), "\n")
	expires := startTime.Add(time.Hour).Local().Format("15:04")
	want := []string{"   Context  " + contextOf(cluster) + "  [read-only]", "   Cluster  lamp", "   User     root", "   Slurm    26.05.5", "   Expires  " + expires}
	for y, w := range want {
		if !strings.HasPrefix(lines[y], w) {
			t.Errorf("header row %d = %q, want it to start with %q", y, lines[y], w)
		}
		if r.cellAt(gridLeft, y).Style.Attrs&uv.AttrFaint == 0 {
			t.Errorf("label on row %d is not faint", y)
		}
	}
	root := r.cellAt(12, 2).Style
	if !isANSI(root.Fg, ansiRed) || root.Attrs&uv.AttrBold == 0 {
		t.Errorf("User root = %v, want red bold", root)
	}
	ro := r.cellAt(strings.Index(lines[0], "[read-only]"), 0).Style
	if !isANSI(ro.Fg, ansiYellow) || ro.Attrs&uv.AttrBold == 0 {
		t.Errorf("[read-only] = %v, want yellow bold", ro)
	}
}

func TestHeaderShowsNoExpiresWithSignedToken(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSigned(t, cluster, "alice")
	r.waitFor("┌ Nodes 1 ")
	if line := strings.Split(r.screen(), "\n")[4]; strings.TrimSpace(line) != "" && strings.Contains(line, "Expires") {
		t.Fatalf("Expires row = %q, want blank", line)
	}
}

func hintColumn(t *testing.T, line string) int {
	t.Helper()
	at := strings.Index(line, "j/k")
	if at < 0 {
		t.Fatalf("no hint grid in %q", line)
	}
	return len([]rune(line[:at]))
}

func (r *run) headerGrid(x int) []string {
	var grid []string
	for _, l := range strings.Split(r.screen(), "\n")[:5] {
		row := []rune(l)
		grid = append(grid, strings.TrimRight(string(row[min(x, len(row)):min(x+48, len(row))]), " "))
	}
	return grid
}

func wantGrid(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("hint grid:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHintGridShowsKeysOfTheView(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{nodeIn("n1", "IDLE", "DRAIN")}, Jobs: []fakeslurm.Job{runningJob(1)}})
	r := startSigned(t, cluster, "alice")
	r.waitFor("┌ Nodes 1 ")

	x := hintColumn(t, strings.Split(r.screen(), "\n")[0])
	if x != 40 {
		t.Errorf("hint grid starts at %d, want 40", x)
	}
	wantGrid(t, r.headerGrid(x), []string{
		"j/k   move           U     resume",
		"enter jobs           /     filter",
		"d     describe       tab   jobs pane",
		"D     edit reason    ?     more",
		"",
	})
	if key := r.cellAt(x, 0).Style; key.Attrs&uv.AttrBold == 0 {
		t.Errorf("hint key is not bold")
	}
	if label := r.cellAt(x+6, 0).Style; label.Attrs&uv.AttrFaint == 0 {
		t.Errorf("hint label is not faint")
	}

	r.send("\t")
	r.waitFor("nodes pane")
	if got := hintColumn(t, strings.Split(r.screen(), "\n")[0]); got != x {
		t.Errorf("jobs hint grid starts at %d, want %d as for nodes", got, x)
	}
	wantGrid(t, r.headerGrid(x), []string{
		"j/k    move          /      filter",
		"d      describe      tab    nodes pane",
		"ctrl-d cancel        ?      more",
		"",
		"",
	})
}

func TestHintGridHidesActionsInReadOnlyMode(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{nodeIn("n1", "IDLE", "DRAIN")}, Jobs: []fakeslurm.Job{runningJob(1)}})
	r := start(t, []string{"--url", cluster.URL, "--user", "alice", "--readonly"}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")
	header := strings.Join(strings.Split(r.screen(), "\n")[:5], "\n")
	for _, h := range []string{"D ", "U ", "edit reason", "resume"} {
		if strings.Contains(header, h) {
			t.Errorf("read-only header shows %q:\n%s", h, header)
		}
	}
	r.fullScreen("jobs", "Jobs")
	r.waitFor("┌ Jobs 1 ")
	if header := strings.Join(strings.Split(r.screen(), "\n")[:5], "\n"); strings.Contains(header, "ctrl-d") {
		t.Errorf("read-only header shows ctrl-d:\n%s", header)
	}
}

func TestLogoShowsOnlyWhenItFits(t *testing.T) {
	logoRow := " ▄▄█▄▄"
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})

	wide := startProvided(t, cluster)
	wide.waitFor("┌ Nodes 1 ")
	line := strings.Split(wide.screen(), "\n")[0]
	if !strings.HasSuffix(line, logoRow) {
		t.Fatalf("logo row = %q, want the can lid and tab", line)
	}
	frameRight := wide.width - 2
	for y := 1; y < 5; y++ {
		if c := wide.cellAt(frameRight, y); c.Content != "█" && c.Content != "▀" {
			t.Fatalf("logo row %d ends with %q at the frame edge, want the can\n%s", y, c.Content, wide.screen())
		}
		if c := wide.cellAt(frameRight+1, y); c.Content != " " && c.Content != "" {
			t.Fatalf("logo row %d goes past the frame edge", y)
		}
	}
	if band := wide.cellAt(frameRight-3, 2); band.Content != "█" || !isANSI(band.Style.Fg, ansiBrightCyan) {
		t.Fatalf("logo band = %q %+v, want a bright cyan █", band.Content, band.Style)
	}

	contextIn := func(name string) string {
		return configDir(t, fmt.Sprintf("slurmtui:\n  contexts:\n    %s:\n      url: %s\n      tokenEnv: SLURM_JWT\n", name, cluster.URL))
	}
	mid := startSized(t, clock.NewFake(startTime), 100, 30, nil, map[string]string{"SLURM_JWT": aliceToken, "SLURMTUI_CONFIG_DIR": contextIn(strings.Repeat("c", 40))})
	mid.waitFor("┌ Nodes 1 ")
	if s := mid.screen(); strings.Contains(s, logoRow) || !strings.Contains(s, "j/k") {
		t.Fatalf("at 100 columns, want no logo and the hint grid:\n%s", s)
	}

	narrow := startSized(t, clock.NewFake(startTime), 80, 24, nil, map[string]string{"SLURM_JWT": aliceToken, "SLURMTUI_CONFIG_DIR": contextIn(strings.Repeat("c", 40))})
	narrow.waitFor("┌ Nodes 1 ")
	s := narrow.screen()
	if strings.Contains(s, "j/k") || !strings.Contains(s, "Context  "+strings.Repeat("c", 40)) || !strings.Contains(s, "Cluster  lamp") {
		t.Fatalf("at 80 columns with a long Context, want the info rows and no hint grid:\n%s", s)
	}
}

func (r *run) bottomLine() string {
	return strings.Split(r.screen(), "\n")[r.height-1]
}

func (r *run) flashStyle(text string) uv.Style {
	r.t.Helper()
	line := r.bottomLine()
	at := strings.Index(line, text)
	if at < 0 {
		r.t.Fatalf("bottom line %q has no %q", line, text)
	}
	return r.cellAt(len([]rune(line[:at])), r.height-1).Style
}

func TestBottomLineShowsFreshnessAndFlash(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	release := cluster.Hold()
	r := startProvided(t, cluster)
	r.waitUntil("○ no data on the bottom line", func() bool { return strings.HasSuffix(r.bottomLine(), "○ no data") })
	release()
	r.waitFor("┌ Nodes 1 ")
	r.advance(3 * time.Second)
	r.waitUntil("● 3s on the bottom line", func() bool { return strings.HasSuffix(r.bottomLine(), "● 3s") })
	dot := r.cellAt(r.width-6, r.height-1)
	if dot.Content != "●" || !isANSI(dot.Style.Fg, ansiGreen) {
		t.Fatalf("freshness dot = %q %v, want a green ●", dot.Content, dot.Style)
	}

	r.send("D")
	r.waitFor("Reason")
	r.send("bad fan\r")
	r.waitFor("drain of n1 accepted")
	if !isANSI(r.flashStyle("drain of n1").Fg, ansiGreen) {
		t.Fatalf("info flash is not green: %q", r.bottomLine())
	}

	cluster.SetStatus(http.StatusInternalServerError)
	r.advance(5 * time.Second)
	r.waitUntil("stale freshness", func() bool { return strings.HasSuffix(r.bottomLine(), "○ stale 5s") })
	r.advance(5 * time.Second)
	r.waitUntil("stale freshness", func() bool { return strings.HasSuffix(r.bottomLine(), "○ stale 10s") })
	r.waitFor("HTTP 500")
	if !isANSI(r.flashStyle("HTTP 500").Fg, ansiRed) {
		t.Fatalf("error flash is not red: %q", r.bottomLine())
	}
	if !strings.Contains(r.bottomLine(), "HTTP 500  ○ stale 10s") {
		t.Fatalf("flash is not left of the freshness: %q", r.bottomLine())
	}
}

func TestReadOnlyKeyFlashesAWarning(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := start(t, []string{"--url", cluster.URL, "--user", "alice", "--readonly"}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")
	r.send("D")
	r.waitFor("action disabled")
	if !isANSI(r.flashStyle("read-only mode").Fg, ansiYellow) {
		t.Fatalf("warning flash is not yellow: %q", r.bottomLine())
	}
}

func TestFilterPromptIsOnTheBottomLine(t *testing.T) {
	r := startProvided(t, fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}}))
	r.waitFor("┌ Nodes 1 ")
	r.send("/n")
	r.waitUntil("the prompt", func() bool { return strings.HasPrefix(r.bottomLine(), "   /n█  enter keep · esc undo") })
	if c := r.cellAt(5, r.height-1); c.Content != "█" || !isANSI(c.Style.Fg, ansiCyan) {
		t.Fatalf("prompt cursor = %q %v, want a cyan █", c.Content, c.Style)
	}
	r.send(escKey + ":jo")
	r.waitUntil("the command prompt", func() bool { return strings.HasPrefix(r.bottomLine(), "   :jo█  enter run · esc cancel") })
	if strings.Contains(r.screen(), "<nodes>") {
		t.Fatalf("screen still shows crumbs:\n%s", r.screen())
	}
}

func TestBodyHeightFollowsTheTerminal(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	tall := startProvided(t, cluster)
	tall.waitFor("┌ Nodes 1 ")
	if top := tall.frameTop("Nodes"); top != 6 {
		t.Fatalf("at 50 rows the frame starts on row %d, want 6 (one blank row under the header)", top)
	}

	small := startSized(t, clock.NewFake(startTime), 80, 24, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURM_JWT": aliceToken})
	small.waitFor("┌ Nodes 1 ")
	small.fullScreen("nodes", "Nodes")
	lines := strings.Split(small.screen(), "\n")
	top := small.frameTop("Nodes")
	bottom := top
	for !strings.HasPrefix(lines[bottom], " └") {
		bottom++
	}
	if top != 5 || bottom-top+1 != 24-6 {
		t.Fatalf("at 80x24 the frame is rows %d to %d, want 5 to %d\nscreen:\n%s", top, bottom, 5+24-6-1, small.screen())
	}
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

func TestOutputUsesOnlyANSIColours(t *testing.T) {
	cluster := fakeslurm.Start(t, classFixture())
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 7 ")
	for _, step := range []struct{ keys, want string }{
		{"j", "┌ Nodes 7 "}, {"d", "Describe"}, {escKey + "y", "JSON"}, {escKey + "?", "┌ Help"}, {escKey + "D", "Drain"},
		{escKey + "\t", "nodes pane"}, {"/x", "esc undo"}, {escKey + "\x04", "Cancel"},
	} {
		r.send(step.keys)
		r.waitFor(step.want)
	}
	for _, m := range sgr.FindAllStringSubmatch(r.output.String(), -1) {
		params := strings.FieldsFunc(m[1], func(c rune) bool { return c == ';' || c == ':' })
		for i, p := range params {
			if (p == "38" || p == "48" || p == "58") && i+1 < len(params) && (params[i+1] == "5" || params[i+1] == "2") {
				t.Fatalf("output has a 256-colour or truecolor sequence %q", m[0])
			}
		}
	}
}
