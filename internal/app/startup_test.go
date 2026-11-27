package app_test

import (
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func TestColonQAndCtrlCQuit(t *testing.T) {
	t.Parallel()
	for name, keys := range map[string]string{":q": ":q\r", "ctrl-c": "\x03"} {
		t.Run(name, func(t *testing.T) {
			cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n01", State: []string{"IDLE"}}}})
			r := start(t, []string{"--url", cluster.URL, "--user", "alice"}, map[string]string{"SLURM_JWT": aliceToken})
			r.waitFor("┌ Nodes 1 ")

			r.send(keys)

			if code := r.waitExit(); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
		})
	}
}

func TestHelpFlagPrintsTheFlagsAndExitsZero(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			r := start(t, []string{flag}, nil)

			if code := r.waitExit(); code != 0 {
				t.Fatalf("exit code = %d, want 0\nstderr:\n%s", code, r.stderr.String())
			}
			wantStderr(t, r.stderr.String(), "-url", "-readonly")
			if strings.Contains(r.stderr.String(), "help requested") {
				t.Fatalf("stderr = %q, want no error", r.stderr.String())
			}
		})
	}
}
