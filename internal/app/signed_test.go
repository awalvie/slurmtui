package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

var fakeClusterKey = []byte("fake-cluster-key-for-tests-only\n")

func keyFileIn(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("make key dir: %s", err)
	}
	if err := os.WriteFile(path, fakeClusterKey, 0o600); err != nil {
		t.Fatalf("write key: %s", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod key: %s", err)
	}
	return path
}

func signedContext(url, user, keyFile string) string {
	return "slurmtui:\n  contexts:\n    prod:\n      url: " + url + "\n      user: " + user + "\n      keyFile: " + keyFile + "\n"
}

func startSigned(t *testing.T, cluster *fakeslurm.Server, user string) *run {
	t.Helper()
	clk := clock.NewFake(startTime)
	cluster.RequireSignedTokens(fakeClusterKey, clk)
	dir := configDir(t, signedContext(cluster.URL, user, "keys/prod.key"))
	keyFileIn(t, dir, "keys/prod.key", 0o400)
	return startWithClock(t, clk, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir})
}

func TestSignedTokenFromRelativeKeyFileIsAccepted(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSigned(t, cluster, "alice")

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("User     alice")
	if strings.Contains(r.screen(), "Expires") {
		t.Fatalf("header shows an expiry for a Signed token:\n%s", r.screen())
	}

	claims := cluster.SignedClaims()
	if len(claims) != 3 {
		t.Fatalf("signed requests = %d, want 3 (nodes, jobs and partitions)", len(claims))
	}
	want := fakeslurm.Claims{Sun: "alice", Iat: startTime.Unix(), Exp: startTime.Add(5 * time.Minute).Unix()}
	for _, c := range claims {
		if c != want {
			t.Fatalf("claims = %+v, want %+v", c, want)
		}
	}
}

func TestSignedTokenIsNewForEachRequest(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSigned(t, cluster, "alice")
	r.waitFor("┌ Nodes 1 ")

	r.advance(5 * time.Second)
	r.waitUntil("signed requests of the second refresh", func() bool { return len(cluster.SignedClaims()) == 6 })

	second := cluster.SignedClaims()[3]
	if want := startTime.Add(5 * time.Second).Unix(); second.Iat != want {
		t.Fatalf("second iat = %d, want %d", second.Iat, want)
	}
	if want := startTime.Add(5*time.Second + 5*time.Minute).Unix(); second.Exp != want {
		t.Fatalf("second exp = %d, want %d", second.Exp, want)
	}
}

func TestHeaderShowsRootInRedBold(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSigned(t, cluster, "root")
	r.waitFor("User     root")

	style := r.cellAt(11, 2).Style
	if !isANSI(style.Fg, ansiRed) || style.Attrs&uv.AttrBold == 0 {
		t.Fatalf("root = %v, want red bold", style)
	}
}

func TestHeaderShowsOtherUsersInTheDefaultColor(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startSigned(t, cluster, "alice")
	r.waitFor("User     alice")

	if style := r.cellAt(11, 2).Style; style.Fg != nil {
		t.Fatalf("alice = %v, want the default colour", style)
	}
}

func TestRejectedSignedTokenSaysToCheckTheClock(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	clusterClock := clock.NewFake(startTime.Add(6 * time.Minute))
	cluster.RequireSignedTokens(fakeClusterKey, clusterClock)
	dir := configDir(t, signedContext(cluster.URL, "alice", "prod.key"))
	keyFileIn(t, dir, "prod.key", 0o400)

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir})

	r.waitFor("/nodes: the cluster refused the signed token; check that this computer's clock is not slow")
}

func TestKeyFilePathExpandsHome(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	clk := clock.NewFake(startTime)
	cluster.RequireSignedTokens(fakeClusterKey, clk)
	home := t.TempDir()
	keyFileIn(t, home, "secrets/prod.key", 0o400)
	dir := configDir(t, signedContext(cluster.URL, "alice", "~/secrets/prod.key"))

	r := startWithClock(t, clk, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "HOME": home})

	r.waitFor("┌ Nodes 1 ")
}

func TestKeyFileThroughSymlinkIsAccepted(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	clk := clock.NewFake(startTime)
	cluster.RequireSignedTokens(fakeClusterKey, clk)
	target := keyFileIn(t, t.TempDir(), "agenix/prod.key", 0o400)
	dir := configDir(t, signedContext(cluster.URL, "alice", "prod.key"))
	if err := os.Symlink(target, filepath.Join(dir, "prod.key")); err != nil {
		t.Fatalf("symlink: %s", err)
	}

	r := startWithClock(t, clk, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir})

	r.waitFor("┌ Nodes 1 ")
}

func TestUnsafeKeyFileFailsAtStartup(t *testing.T) {
	cases := []struct {
		name string
		make func(t *testing.T, dir string) string
		want string
	}{
		{"group readable", func(t *testing.T, dir string) string {
			return keyFileIn(t, dir, "prod.key", 0o440)
		}, "mode 0440 gives access to group or others"},
		{"other readable", func(t *testing.T, dir string) string {
			return keyFileIn(t, dir, "prod.key", 0o604)
		}, "mode 0604 gives access to group or others"},
		{"empty", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "prod.key")
			if err := os.WriteFile(path, nil, 0o400); err != nil {
				t.Fatal(err)
			}
			return path
		}, "is empty"},
		{"directory", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "prod.key")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			return path
		}, "is not a regular file"},
		{"fifo", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "prod.key")
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}, "is not a regular file"},
		{"missing", func(t *testing.T, dir string) string {
			return filepath.Join(dir, "prod.key")
		}, "no such file"},
		{"owned by another user", func(t *testing.T, dir string) string {
			if os.Getuid() == 0 {
				t.Skip("root owns every candidate file")
			}
			path := filepath.Join(dir, "prod.key")
			if err := os.Symlink("/etc/passwd", path); err != nil {
				t.Fatal(err)
			}
			return path
		}, "not by the current user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.make(t, t.TempDir())
			configs := configDir(t, signedContext("http://127.0.0.1:1", "alice", path))

			stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configs})

			wantStderr(t, stderr, `context "prod": keyFile: key file `+path+": ", tc.want)
			if strings.Contains(stderr, string(fakeClusterKey)) {
				t.Fatalf("stderr shows the key: %q", stderr)
			}
		})
	}
}

func TestKeyFileWithTildeUserFails(t *testing.T) {
	dir := configDir(t, signedContext("http://127.0.0.1:1", "alice", "~bob/prod.key"))

	stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir})

	wantStderr(t, stderr, `context "prod": keyFile: ~bob/prod.key: only ~ and ~/ expand, not ~user`)
}
