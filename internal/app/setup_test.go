package app_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	tab       = "\t"
	shiftTab  = "\x1b[Z"
	enter     = "\r"
	left      = "\x1b[D"
	right     = "\x1b[C"
	clearLine = "\x15"
)

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %s", err)
	}
	return string(b)
}

func wantNoDir(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want it not to exist", dir, err)
	}
}

func wantMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %s", path, err)
	}
	if info.Mode() != want {
		t.Fatalf("mode of %s = %s, want %s", path, info.Mode(), want)
	}
}

func TestSetupFormWithEnvVarSavesContextAndOpensNodes(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := filepath.Join(t.TempDir(), "slurmtui")
	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("lab" + tab + cluster.URL + tab + tab + tab)
	r.waitFor("‹ Env var ›")
	r.send(tab + tab + enter)

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("Context  lab")
	want := "slurmtui:\n  contexts:\n    lab:\n      url: " + cluster.URL + "\n      tokenEnv: SLURM_JWT\n"
	if got := readConfig(t, dir); got != want {
		t.Fatalf("config.yaml =\n%s\nwant\n%s", got, want)
	}
	wantMode(t, dir, 0o700|os.ModeDir)
	wantMode(t, filepath.Join(dir, "config.yaml"), 0o600)
}

func TestSetupFormOpensWithFlagValuesFilledIn(t *testing.T) {
	r := start(t, []string{"--url", "http://127.0.0.1:6820", "--user", "alice"}, nil)

	r.waitFor("Connect to Slurm")
	r.waitFor("▸ Context name  127.0.0.1:6820█")
	r.waitFor("  URL           http://127.0.0.1:6820")
	r.waitFor("  User          alice")
	r.waitFor("‹ Key file ›")
}

func TestSetupFormShowsAFailedFieldCheckAndStaysOpen(t *testing.T) {
	r := start(t, nil, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("lab" + tab + "http://slurm.example:6820" + shiftTab + shiftTab + shiftTab + enter)

	r.waitFor("URL: http://slurm.example:6820: http is allowed only to a")
	r.waitFor("loopback host")
	r.waitFor("Connect to Slurm")
}

func TestSetupFormShowsAPingFailureAndStaysOpen(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	cluster.SetErrors("Unable to contact slurmctld")
	dir := filepath.Join(t.TempDir(), "slurmtui")
	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("lab" + tab + cluster.URL + shiftTab + shiftTab + shiftTab + enter)

	r.waitFor("Unable to contact slurmctld")
	r.waitFor("Connect to Slurm")
	wantNoDir(t, dir)
	paths := []string{}
	for _, req := range cluster.Requests() {
		paths = append(paths, req.Path)
	}
	if want := "/slurm/v0.0.45/ping"; len(paths) != 1 || paths[0] != want {
		t.Fatalf("requests = %v, want [%s]", paths, want)
	}

	cluster.SetErrors()
	cluster.SetStatus(http.StatusInternalServerError)
	r.send(enter)
	r.waitFor("HTTP 500")

	cluster.SetStatus(http.StatusOK)
	r.send(enter)
	r.waitFor("┌ Nodes 1 ")
}

func TestSetupFormShowsATLSFailure(t *testing.T) {
	cluster := fakeslurm.StartTLS(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "ca.pem"), cluster.CertPEM)
	r := start(t, nil, map[string]string{"HOME": home, "SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("lab" + tab + cluster.URL + shiftTab + shiftTab + shiftTab + enter)
	r.waitFor("certificate")

	r.send(tab + tab + tab + tab + "~/ca.pem" + shiftTab + shiftTab + shiftTab + shiftTab + enter)
	r.waitFor("┌ Nodes 1 ")
}

func TestSetupFormWithKeyFileSavesOnlyThePath(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	clk := clock.NewFake(startTime)
	cluster.RequireSignedTokens(fakeClusterKey, clk)
	home := t.TempDir()
	keyFileIn(t, home, "prod.key", 0o400)
	dir := filepath.Join(t.TempDir(), "slurmtui")
	r := startWithClock(t, clk, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "HOME": home})
	r.waitFor("Connect to Slurm")

	r.send(shiftTab + shiftTab + shiftTab + "~/prod.key" + tab + enter)

	r.waitFor("┌ Nodes 1 ")
	want := "slurmtui:\n  contexts:\n    " + strings.TrimPrefix(cluster.URL, "http://") + ":\n      url: " + cluster.URL + "\n      user: alice\n      keyFile: ~/prod.key\n"
	got := readConfig(t, dir)
	if got != want {
		t.Fatalf("config.yaml =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, strings.TrimSpace(string(fakeClusterKey))) {
		t.Fatalf("config.yaml contains the key:\n%s", got)
	}
}

func TestSetupFormExplainsARefusedSignedToken(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	clk := clock.NewFake(startTime)
	cluster.RequireSignedTokens([]byte("another-cluster-key"), clk)
	home := t.TempDir()
	keyFileIn(t, home, "prod.key", 0o400)
	dir := filepath.Join(t.TempDir(), "slurmtui")
	r := startWithClock(t, clk, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "HOME": home})
	r.waitFor("Connect to Slurm")

	r.send(shiftTab + shiftTab + shiftTab + "~/prod.key" + tab + enter)

	r.waitFor("refused the signed token")
	r.waitFor("Connect to Slurm")
	wantNoDir(t, dir)
}

const commentedConfig = `# Clusters I use.
slurmtui:
  refreshRate: 7 # seconds
  contexts:
    # The production cluster.
    prod:
      url: https://slurm.example:6820
      tokenEnv: SLURM_JWT_PROD
`

func TestSetupFormAddsAContextAndKeepsTheRestOfTheFile(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := configDir(t, commentedConfig)
	r := start(t, []string{"--url", cluster.URL}, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT_LAB": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send(clearLine + "prod" + tab + tab + tab + tab + right + tab + clearLine + "SLURM_JWT_LAB" + tab + enter)
	r.waitFor(`Context name: "prod" is already a context`)

	r.send(tab + tab + clearLine + "lab" + shiftTab + shiftTab + enter)
	r.waitFor("┌ Nodes 1 ")
	r.waitFor("Context  lab")

	want := commentedConfig + "    lab:\n      url: " + cluster.URL + "\n      tokenEnv: SLURM_JWT_LAB\n"
	if got := readConfig(t, dir); got != want {
		t.Fatalf("config.yaml =\n%s\nwant\n%s", got, want)
	}
}

func TestSetupFormWithPastedTokenSavesNothing(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := filepath.Join(t.TempDir(), "slurmtui")
	r := start(t, []string{"--url", cluster.URL}, map[string]string{"SLURMTUI_CONFIG_DIR": dir})
	r.waitFor("Connect to Slurm")

	r.send(shiftTab + shiftTab + shiftTab + shiftTab + left)
	r.waitFor("‹ Paste ›")
	r.waitFor("A pasted token is used for this session only.")
	r.send(tab + aliceToken)
	r.waitFor(fmt.Sprintf("Token         [hidden, %d characters]", len(aliceToken)))
	if strings.Contains(r.screen(), aliceToken[:20]) {
		t.Fatalf("screen shows the pasted token:\n%s", r.screen())
	}

	r.send(tab + enter)

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("User     alice")
	wantNoDir(t, dir)
}

func TestSetupFormQuitAndCtrlCExitWithZero(t *testing.T) {
	for name, keys := range map[string]string{"quit button": shiftTab + enter, "ctrl-c": "\x03"} {
		t.Run(name, func(t *testing.T) {
			r := start(t, nil, nil)
			r.waitFor("Connect to Slurm")

			r.send(keys)

			if code := r.waitExit(); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
		})
	}
}

func TestSavedContextOpensNodesOnTheNextStart(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := filepath.Join(t.TempDir(), "slurmtui")
	env := map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken}
	first := start(t, nil, env)
	first.waitFor("Connect to Slurm")
	first.send("lab" + tab + cluster.URL + shiftTab + shiftTab + shiftTab + enter)
	first.waitFor("┌ Nodes 1 ")
	first.send(":q\r")
	if code := first.waitExit(); code != 0 {
		t.Fatalf("first exit code = %d, want 0", code)
	}
	if strings.Contains(readConfig(t, dir), aliceToken) {
		t.Fatalf("config.yaml contains the token:\n%s", readConfig(t, dir))
	}

	second := start(t, nil, env)

	second.waitFor("┌ Nodes 1 ")
	if strings.Contains(second.screen(), "Connect to Slurm") {
		t.Fatalf("second start shows the setup form:\n%s", second.screen())
	}
}

func TestSetupFormIsABlueFrameWithFormHints(t *testing.T) {
	r := start(t, nil, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	b := r.dialogBox("Connect to Slurm")
	if b.width != 64 || b.x != (r.width-64)/2 {
		t.Fatalf("form at x %d, width %d; want width 64 in the centre", b.x, b.width)
	}
	if s := r.cellAt(b.x, b.y).Style; !isANSI(s.Fg, ansiBlue) {
		t.Fatalf("form border = %+v, want blue", s)
	}
	if x, _ := r.offsetOf("▸ Context name"); x != b.x+3 {
		t.Fatalf("focused field at column %d, want %d (2 cells inside the border)\n%s", x, b.x+3, r.screen())
	}
	if s := r.styleOf("▸ Context name"); !isANSI(s.Fg, ansiCyan) {
		t.Fatalf("▸ is not cyan: %+v", s)
	}
	if s := r.styleOf("█"); !isANSI(s.Fg, ansiCyan) {
		t.Fatalf("cursor is not cyan: %+v", s)
	}
	if s := r.styleOf("‹ Env var ›"); s.Attrs&uv.AttrFaint == 0 {
		t.Fatalf("‹ is not faint: %+v", s)
	}
	if !strings.HasPrefix(r.bottomLine(), " tab next · shift-tab previous · ←/→ change · enter select") {
		t.Fatalf("bottom line = %q, want the form hints", r.bottomLine())
	}

	r.send("lab" + tab + "http://slurm.example:6820" + shiftTab + shiftTab + shiftTab + enter)

	r.waitFor("URL: http://slurm.example:6820")
	after := r.dialogBox("Connect to Slurm")
	if after.y != b.y || after.height <= b.height {
		t.Fatalf("form with an error is rows %d to %d, want top row %d and more rows than %d\n%s", after.y, after.y+after.height-1, b.y, b.height, r.screen())
	}
	if x, _ := r.offsetOf("URL: http"); x != b.x+3 {
		t.Fatalf("error at column %d, want %d\n%s", x, b.x+3, r.screen())
	}
	rows := r.dialogRows(after)
	buttons, problem := -1, -1
	for i, l := range rows {
		if strings.Contains(l, "Test and save") {
			buttons = i
		}
		if strings.Contains(l, "URL: http") && problem < 0 {
			problem = i
		}
	}
	if problem < buttons {
		t.Fatalf("error row %d is not under the buttons row %d\n%s", problem, buttons, r.screen())
	}
	if s := r.styleOf("URL: http"); !isANSI(s.Fg, ansiRed) {
		t.Fatalf("error is not red: %+v", s)
	}
}

func TestSetupFormTypesQ(t *testing.T) {
	r := start(t, nil, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("q")

	r.waitFor("▸ Context name  q█")
}
