package app_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

const (
	readOnlyFlash = "read-only mode — action disabled"
	ctrlR         = "\x12"
)

func TestReadOnlyFlagShowsBadgeAfterContextName(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})

	r := start(t, []string{"--readonly", "--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("Context  " + contextOf(cluster) + "  [read-only]")
}

func readOnlyConfig(global, context, url string) string {
	yaml := "slurmtui:\n"
	if global != "" {
		yaml += "  readOnly: " + global + "\n"
	}
	yaml += "  contexts:\n    prod:\n      url: " + url + "\n      tokenEnv: SLURM_JWT\n"
	if context != "" {
		yaml += "      readOnly: " + context + "\n"
	}
	return yaml
}

func TestReadOnlyOrderIsFlagThenContextThenGlobal(t *testing.T) {
	cases := []struct {
		name     string
		global   string
		context  string
		flag     string
		readOnly bool
	}{
		{"default is write", "", "", "", false},
		{"global", "true", "", "", true},
		{"context over global", "true", "false", "", false},
		{"context", "", "true", "", true},
		{"write flag over context", "", "true", "--write", false},
		{"write flag over global", "true", "", "--write", false},
		{"readonly flag over context", "false", "false", "--readonly", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
			args := []string{"--context", "prod"}
			if c.flag != "" {
				args = append(args, c.flag)
			}
			r := start(t, args, map[string]string{
				"SLURMTUI_CONFIG_DIR": configDir(t, readOnlyConfig(c.global, c.context, cluster.URL)),
				"SLURM_JWT":           aliceToken,
			})
			r.waitFor("┌ Nodes 1 ")

			r.send("D")
			if c.readOnly {
				r.waitFor("Context  prod  [read-only]")
				r.waitFor(readOnlyFlash)
				return
			}
			r.waitFor("Drain n1 on prod (lamp)")
			if strings.Contains(r.screen(), "[read-only]") {
				t.Fatalf("header shows [read-only] in write mode\nscreen:\n%s", r.screen())
			}
		})
	}
}

func TestReadOnlyGlobalSettingAppliesToContextFromFlags(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})

	r := start(t, []string{"--url", cluster.URL}, map[string]string{
		"SLURMTUI_CONFIG_DIR": configDir(t, "slurmtui:\n  readOnly: true\n"),
		"SLURM_JWT":           aliceToken,
	})

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("[read-only]")
}

func TestReadOnlyAndWriteFlagsTogetherAreAStartupError(t *testing.T) {
	stderr := startupError(t, []string{"--readonly", "--write", "--url", "http://127.0.0.1:6820"}, map[string]string{"SLURM_JWT": aliceToken})

	wantStderr(t, stderr, "--readonly and --write")
}

func TestReadOnlySetupFormPingsAndOpensNodesInReadOnlyMode(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := start(t, []string{"--readonly"}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("Connect to Slurm")

	r.send("lab" + tab + cluster.URL + tab + tab + tab)
	r.waitFor("‹ Env var ›")
	r.send(tab + tab + enter)

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("Context  lab  [read-only]")
	r.send("D")
	r.waitFor(readOnlyFlash)
	if writes := writeRequests(cluster); len(writes) > 0 {
		t.Fatalf("read-only mode sent %d write requests: %+v", len(writes), writes)
	}
}

func TestReadOnlyHelpHidesActionKeys(t *testing.T) {
	for _, v := range []struct{ open, title string }{{"", "┌ Help "}, {"\t", "┌ Help "}} {
		cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}, Jobs: []fakeslurm.Job{runningJob(1234)}})
		r := startReadOnly(t, cluster)
		r.send(v.open + "?")
		r.waitFor(v.title)

		for _, hint := range []string{"Drain", "Resume", "Cancel", "ctrl-d"} {
			if strings.Contains(r.screen(), hint) {
				t.Fatalf("read-only help shows %q\nscreen:\n%s", hint, r.screen())
			}
		}
	}
}

func startReadOnly(t *testing.T, cluster *fakeslurm.Server) *run {
	t.Helper()
	r := start(t, []string{"--readonly", "--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})
	r.waitFor("┌ Nodes 1 ")
	return r
}

func writeRequests(cluster *fakeslurm.Server) []fakeslurm.Request {
	return requestsWhere(cluster, func(req fakeslurm.Request) bool { return req.Method != http.MethodGet })
}

func TestReadOnlyActionKeysOpenNoDialogAndSendNoWriteInAnyView(t *testing.T) {
	views := []struct {
		name  string
		open  string
		shown string
		bound map[string]bool
	}{
		{"nodes", "", "┌ Nodes 1 ", map[string]bool{"D": true, "U": true}},
		{"jobs", "\t", "nodes pane", map[string]bool{ctrlD: true}},
		{"node describe", "d", "Describe · n1 ", nil},
		{"node raw json", "y", "JSON · n1 ", nil},
		{"job describe", "\td", "Describe · 1234 ", nil},
		{"job raw json", "\ty", "JSON · 1234 ", nil},
		{"nodes help", "?", "┌ Help ", nil},
		{"jobs help", "\t?", "┌ Help ", nil},
	}
	for _, v := range views {
		for _, key := range []string{"D", "U", ctrlD} {
			t.Run(fmt.Sprintf("%s %q", v.name, key), func(t *testing.T) {
				cluster := fakeslurm.Start(t, fakeslurm.Cluster{
					Name:  "lamp",
					Nodes: []fakeslurm.Node{idle("n1")},
					Jobs:  []fakeslurm.Job{runningJob(1234)},
				})
				r := startReadOnly(t, cluster)
				r.send(v.open)
				r.waitFor(v.shown)

				r.send(key)
				if v.bound[key] {
					r.waitFor(readOnlyFlash)
				}
				r.send(enter + left + enter + "y" + enter)
				seen := len(cluster.Requests())
				r.send(ctrlR)
				r.waitUntil("a refresh after the keys", func() bool { return len(cluster.Requests()) > seen })

				for _, title := range []string{"Drain n1", "Resume n1", "Cancel job 1234"} {
					if strings.Contains(r.screen(), title) {
						t.Fatalf("read-only mode shows dialog %q\nscreen:\n%s", title, r.screen())
					}
				}
				if writes := writeRequests(cluster); len(writes) > 0 {
					t.Fatalf("read-only mode sent %d write requests: %+v", len(writes), writes)
				}
			})
		}
	}
}

func TestReadOnlyActionKeyInTheOtherPaneSaysReadOnly(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}, Jobs: []fakeslurm.Job{runningJob(1234)}})
	r := startReadOnly(t, cluster)

	r.send(ctrlD)
	r.waitFor(readOnlyFlash)
	r.send("\tD")
	r.waitFor(readOnlyFlash)

	if strings.Contains(r.screen(), "press tab") {
		t.Fatalf("read-only mode suggests a pane for a disabled action\nscreen:\n%s", r.screen())
	}
}
