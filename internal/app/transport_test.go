package app_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

func TestPlainHTTPToARemoteHostIsAStartupError(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"ip", "http://10.0.0.5:6820"},
		{"name", "http://slurm.example:6820"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := startupError(t, []string{"--url", tc.url}, map[string]string{"SLURM_JWT": aliceToken})

			wantStderr(t, stderr, "--url "+tc.url, "http is allowed only to a loopback host")
		})
	}
}

func TestRemoteHTTPURLInConfigFileNamesTheLine(t *testing.T) {
	dir := configDir(t, tlsContext("http://slurm.example:6820", ""))

	stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})

	wantStderr(t, stderr, "line 4: slurmtui.contexts.prod.url: http://slurm.example:6820: http is allowed only to a loopback host")
}

func tlsContext(url, caFile string) string {
	out := `slurmtui:
  contexts:
    prod:
      url: ` + url + `
      tokenEnv: SLURM_JWT
`
	if caFile != "" {
		out += "      caFile: " + caFile + "\n"
	}
	return out
}

func TestHTTPSWithoutCAFileRejectsASelfSignedServer(t *testing.T) {
	cluster := fakeslurm.StartTLS(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := configDir(t, tlsContext(cluster.URL, ""))

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})

	r.waitFor("certificate")
	r.waitFor(cluster.URL + "/slurm/v0.0.45/nodes")
	if n := len(cluster.Requests()); n != 0 {
		t.Fatalf("server got %d requests, want 0", n)
	}
}

func TestHTTPSWithCAFileTrustsTheServer(t *testing.T) {
	cluster := fakeslurm.StartTLS(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	dir := configDir(t, tlsContext(cluster.URL, "ca.pem"))
	writeFile(t, filepath.Join(dir, "ca.pem"), cluster.CertPEM)

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})

	r.waitFor("┌ Nodes 1 ")
}

func TestCAFilePathExpandsTilde(t *testing.T) {
	cluster := fakeslurm.StartTLS(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "ca.pem"), cluster.CertPEM)
	dir := configDir(t, tlsContext(cluster.URL, "~/ca.pem"))

	r := start(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "HOME": home, "SLURM_JWT": aliceToken})

	r.waitFor("┌ Nodes 1 ")
}

func TestRedirectIsRefusedAndTheTokenStaysHome(t *testing.T) {
	target := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	r := start(t, []string{"--url", redirector.URL}, map[string]string{"SLURM_JWT": aliceToken})

	r.waitFor("redirect to " + target.URL + "/slurm/v0.0.45/nodes refused")
	if n := len(target.Requests()); n != 0 {
		t.Fatalf("redirect target got %d requests, want 0: %+v", n, target.Requests())
	}
	if strings.Contains(r.screen(), aliceToken) {
		t.Fatalf("screen shows the token:\n%s", r.screen())
	}
}

func TestCAFileWithoutACertificateIsAStartupError(t *testing.T) {
	dir := configDir(t, tlsContext("https://127.0.0.1:1", "ca.pem"))
	writeFile(t, filepath.Join(dir, "ca.pem"), []byte("not a certificate\n"))

	stderr := startupError(t, nil, map[string]string{"SLURMTUI_CONFIG_DIR": dir, "SLURM_JWT": aliceToken})

	wantStderr(t, stderr, `context "prod": caFile: `+filepath.Join(dir, "ca.pem")+": no PEM certificate found")
}
