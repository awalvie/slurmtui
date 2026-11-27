package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

const expiryLayout = "2006-01-02 15:04:05 -07:00"

func twoContexts(prodURL, labURL string) string {
	return `slurmtui:
  contexts:
    prod:
      url: ` + prodURL + `
      tokenEnv: SLURM_JWT_PROD
    lab:
      url: ` + labURL + `
      tokenEnv: SLURM_JWT_LAB
`
}

func TestContextFlagSelectsContextAndItsOwnToken(t *testing.T) {
	prod := fakeslurm.Start(t, fakeslurm.Cluster{Name: "prod-cluster", Nodes: []fakeslurm.Node{idle("p1")}})
	lab := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lab-cluster", Nodes: []fakeslurm.Node{idle("l1"), idle("l2")}})
	bobToken := providedToken("bob", startTime.Add(time.Hour))
	dir := configDir(t, twoContexts(prod.URL, lab.URL))

	r := start(t, []string{"--context", "lab"}, map[string]string{
		"SLURMTUI_CONFIG_DIR": dir,
		"SLURM_JWT_PROD":      aliceToken,
		"SLURM_JWT_LAB":       bobToken,
		"SLURM_JWT":           aliceToken,
	})

	r.waitFor("┌ Nodes 2 ")
	r.waitFor("Context  lab")
	r.waitFor("Cluster  lab-cluster")
	r.waitFor("User     bob")
	if n := len(prod.Requests()); n != 0 {
		t.Fatalf("prod got %d requests, want 0", n)
	}
	req := lab.Requests()[0]
	if got := req.Header.Get("X-SLURM-USER-TOKEN"); got != bobToken {
		t.Fatalf("X-SLURM-USER-TOKEN is not the lab token")
	}
	if got := req.Header.Get("X-SLURM-USER-NAME"); got != "bob" {
		t.Fatalf("X-SLURM-USER-NAME = %q, want bob", got)
	}
}

func TestOneContextIsSelectedWithoutContextFlag(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := configDir(t, "slurmtui:\n  contexts:\n    only:\n      url: "+cluster.URL+"\n      tokenEnv: MY_TOKEN\n")

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "MY_TOKEN": aliceToken})

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("Context  only")
}

func TestConfigFileIsInXDGConfigHome(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	xdg := t.TempDir()
	if err := os.Mkdir(filepath.Join(xdg, "slurmtui"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "slurmtui:\n  contexts:\n    xdg:\n      url: " + cluster.URL + "\n      tokenEnv: SLURM_JWT\n"
	writeFile(t, filepath.Join(xdg, "slurmtui", "config.yaml"), []byte(config))

	r := start(t, nil, map[string]string{"XDG_CONFIG_HOME": xdg, "SLURM_JWT": aliceToken})

	r.waitFor("Context  xdg")
	r.waitFor("┌ Nodes 1 ")
}

func TestFlagsWithoutConfigFileNameTheContextAfterTheHost(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	host := strings.TrimPrefix(cluster.URL, "http://")

	r := start(t, []string{"--url", cluster.URL}, map[string]string{"SLURM_JWT": aliceToken})

	r.waitFor("Context  " + host)
	r.waitFor("User     alice")
	r.waitFor("┌ Nodes 1 ")
}

func TestManyContextsWithoutContextFlagFail(t *testing.T) {
	dir := configDir(t, twoContexts("http://127.0.0.1:1", "http://127.0.0.1:2"))

	stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir})

	wantStderr(t, stderr, "--context is required", "lab, prod")
}

func TestUnknownContextFails(t *testing.T) {
	dir := configDir(t, twoContexts("http://127.0.0.1:1", "http://127.0.0.1:2"))

	stderr := startupError(t, []string{"--context", "staging"}, map[string]string{"SLURMTUI_CONFIG_DIR": dir})

	wantStderr(t, stderr, `--context "staging"`, "lab, prod")
}

func TestContextFlagWithoutConfigFileFails(t *testing.T) {
	stderr := startupError(t, []string{"--context", "prod"}, nil)

	wantStderr(t, stderr, `--context "prod"`)
}

func TestContextNeedsExactlyOneTokenSource(t *testing.T) {
	cases := map[string]string{
		"none": "slurmtui:\n  contexts:\n    prod:\n      url: http://127.0.0.1:1\n      user: alice\n",
		"two":  "slurmtui:\n  contexts:\n    prod:\n      url: http://127.0.0.1:1\n      user: alice\n      tokenEnv: SLURM_JWT\n      keyFile: keys/prod.key\n",
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, config), "SLURM_JWT": aliceToken})

			wantStderr(t, stderr, "slurmtui.contexts.prod", "exactly one of keyFile or tokenEnv")
		})
	}
}

func TestUserThatDiffersFromTokenSunFails(t *testing.T) {
	config := "slurmtui:\n  contexts:\n    prod:\n      url: http://127.0.0.1:1\n      user: bob\n      tokenEnv: SLURM_JWT\n"

	t.Run("config", func(t *testing.T) {
		stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, config), "SLURM_JWT": aliceToken})
		wantStderr(t, stderr, `user "bob"`, `sun "alice"`)
	})
	t.Run("flag", func(t *testing.T) {
		stderr := startupError(t, []string{"--url", "http://127.0.0.1:1", "--user", "bob"}, map[string]string{"SLURM_JWT": aliceToken})
		wantStderr(t, stderr, `user "bob"`, `sun "alice"`)
	})
}

func TestConfigUserThatMatchesSunIsAccepted(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	config := "slurmtui:\n  contexts:\n    prod:\n      url: " + cluster.URL + "\n      user: alice\n      tokenEnv: SLURM_JWT\n"

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, config), "SLURM_JWT": aliceToken})

	r.waitFor("┌ Nodes 1 ")
	r.waitFor("User     alice")
}

func TestProvidedTokenThatIsNotAJWTFails(t *testing.T) {
	stderr := startupError(t, []string{"--url", "http://127.0.0.1:1"}, map[string]string{"SLURM_JWT": "not-a-jwt-secret"})

	wantStderr(t, stderr, "SLURM_JWT")
	if strings.Contains(stderr, "not-a-jwt-secret") {
		t.Fatalf("stderr shows the token: %q", stderr)
	}
}

func TestMissingTokenEnvNamesTheVariable(t *testing.T) {
	dir := configDir(t, twoContexts("http://127.0.0.1:1", "http://127.0.0.1:2"))

	stderr := startupError(t, []string{"--context", "lab"}, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})

	wantStderr(t, stderr, "SLURM_JWT_LAB is not set")
}

func TestHeaderShowsProvidedTokenExpiry(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})

	startProvided(t, cluster).waitFor("Expires  " + startTime.Add(time.Hour).Local().Format("15:04"))
}

func TestExpiredProvidedTokenStopsRefresh(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	expiry := startTime.Add(7 * time.Second)
	r := start(t, []string{"--url", cluster.URL}, map[string]string{"SLURM_JWT": providedToken("alice", expiry)})
	r.waitFor("┌ Nodes 1 ")

	r.advance(5 * time.Second)
	r.waitUntil("second nodes request", func() bool { return nodeRequests(cluster) == 2 })
	r.advance(5 * time.Second)

	want := "SLURM_JWT expired at " + expiry.Local().Format(expiryLayout) + "; get a new token and restart"
	r.waitFor(want)
	r.rowByName("n1")
	r.advance(5 * time.Second)
	r.advance(5 * time.Second)
	r.send("\x12")
	r.waitFor(want)
	if got := nodeRequests(cluster); got != 2 {
		t.Fatalf("nodes requests = %d, want 2", got)
	}
	if strings.Contains(r.screen(), "GET ") {
		t.Fatalf("flash line has more than the expiry message:\n%s", r.screen())
	}
}

func TestExpiredMessageUsesTheContextTokenEnv(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := configDir(t, twoContexts("http://127.0.0.1:1", cluster.URL))
	expiry := startTime.Add(-time.Minute)

	r := start(t, []string{"--context", "lab"}, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT_LAB": providedToken("alice", expiry)})

	r.waitFor("SLURM_JWT_LAB expired at " + expiry.Local().Format(expiryLayout) + "; get a new token and restart")
	if got := nodeRequests(cluster); got != 0 {
		t.Fatalf("nodes requests = %d, want 0", got)
	}
}

func TestRefreshRateFromConfigAndFlagHasPriority(t *testing.T) {
	config := func(url string) string {
		return "slurmtui:\n  refreshRate: 3\n  contexts:\n    prod:\n      url: " + url + "\n      tokenEnv: SLURM_JWT\n"
	}

	t.Run("config", func(t *testing.T) {
		cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
		r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, config(cluster.URL)), "SLURM_JWT": aliceToken})
		r.waitFor("┌ Nodes 1 ")

		cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
		r.advance(3 * time.Second)

		r.waitFor("┌ Nodes 2 ")
	})
	t.Run("flag", func(t *testing.T) {
		cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
		r := start(t, []string{"--refresh", "10"}, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, config(cluster.URL)), "SLURM_JWT": aliceToken})
		r.waitFor("┌ Nodes 1 ")

		cluster.SetNodes([]fakeslurm.Node{idle("n1"), idle("n2")})
		r.advance(3 * time.Second)
		r.advance(3 * time.Second)
		r.waitFor("● 6s")
		if strings.Contains(r.screen(), "┌ Nodes 2 ") {
			t.Fatalf("refreshed at the config rate, not the flag rate")
		}
		r.advance(4 * time.Second)
		r.waitFor("┌ Nodes 2 ")
	})
}

func TestIncorrectConfigFileNamesTheField(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   []string
	}{
		{"bad yaml", "slurmtui:\n  contexts: [\n", []string{"config.yaml"}},
		{"unknown top field", "kubectl: {}\n", []string{"kubectl", "unknown field"}},
		{"unknown context field", "slurmtui:\n  contexts:\n    prod:\n      url: http://127.0.0.1:1\n      tokenEnv: SLURM_JWT\n      password: x\n", []string{"slurmtui.contexts.prod.password", "unknown field"}},
		{"refresh rate type", "slurmtui:\n  refreshRate: fast\n", []string{"slurmtui.refreshRate"}},
		{"refresh rate too low", "slurmtui:\n  refreshRate: 1\n", []string{"slurmtui.refreshRate", "at least 2"}},
		{"read only type", "slurmtui:\n  readOnly: maybe\n", []string{"slurmtui.readOnly"}},
		{"empty token env", "slurmtui:\n  contexts:\n    prod:\n      url: http://127.0.0.1:1\n      tokenEnv: \"\"\n", []string{"slurmtui.contexts.prod.tokenEnv", "empty"}},
		{"contexts not a map", "slurmtui:\n  contexts: [prod]\n", []string{"slurmtui.contexts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, tc.config), "SLURM_JWT": aliceToken})

			wantStderr(t, stderr, tc.want...)
		})
	}
}

func TestEmptyConfigFileWithFlagsWorks(t *testing.T) {
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})

	r := start(t, []string{"--url", cluster.URL}, map[string]string{"SLURMTUI_CONFIG_DIR": configDir(t, ""), "SLURM_JWT": aliceToken})

	r.waitFor("┌ Nodes 1 ")
}
