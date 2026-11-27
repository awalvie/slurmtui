package fakeslurm_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
)

type recordingT struct {
	testing.TB
	mu     sync.Mutex
	errors []string
}

func (r *recordingT) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingT) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errors...)
}

func getNodes(t *testing.T, s *fakeslurm.Server) int {
	t.Helper()
	resp, err := http.Get(s.URL + "/slurm/v0.0.45/nodes")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestReplyThatDoesNotMatchSpecFailsTheTest(t *testing.T) {
	rt := &recordingT{TB: t}
	s := fakeslurm.Start(rt, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n1", State: []string{"IDEL"}}}})
	if status := getNodes(t, s); status != http.StatusInternalServerError {
		t.Errorf("status %d, want %d", status, http.StatusInternalServerError)
	}
	errs := rt.recorded()
	if len(errs) != 1 || !strings.Contains(errs[0], `nodes[0].state[0]: "IDEL" is not one of the spec values`) {
		t.Fatalf("recorded errors %q, want one that names nodes[0].state[0]", errs)
	}
}

func TestExpectedOffSpecReplyIsServed(t *testing.T) {
	rt := &recordingT{TB: t}
	s := fakeslurm.Start(rt, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n1", State: []string{"SPARKLING"}}}})
	s.ExpectOffSpec("nodes[0].state[0]")
	if status := getNodes(t, s); status != http.StatusOK {
		t.Errorf("status %d, want %d", status, http.StatusOK)
	}
	if errs := rt.recorded(); len(errs) != 0 {
		t.Fatalf("recorded errors %q, want none", errs)
	}
}

func TestExpectedOffSpecReplyMustBreakSpecAtThatField(t *testing.T) {
	rt := &recordingT{TB: t}
	s := fakeslurm.Start(rt, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n1", State: []string{"IDLE"}}}})
	s.ExpectOffSpec("nodes[0].state[0]")
	getNodes(t, s)
	errs := rt.recorded()
	if len(errs) != 1 || !strings.Contains(errs[0], `want it to break the spec at nodes[0].state[0], but it matches the spec`) {
		t.Fatalf("recorded errors %q, want one about the expected off-spec field", errs)
	}
}

func TestReplyThatMatchesSpecPasses(t *testing.T) {
	rt := &recordingT{TB: t}
	s := fakeslurm.Start(rt, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n1", State: []string{"IDLE"}}}})
	if status := getNodes(t, s); status != http.StatusOK {
		t.Errorf("status %d, want %d", status, http.StatusOK)
	}
	if errs := rt.recorded(); len(errs) != 0 {
		t.Fatalf("recorded errors %q, want none", errs)
	}
}

func TestFailedReplyMatchesSpec(t *testing.T) {
	rt := &recordingT{TB: t}
	s := fakeslurm.Start(rt, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{{Name: "n1", State: []string{"IDLE"}}}})
	s.SetStatus(http.StatusInternalServerError)
	s.SetErrors("Unable to contact slurm controller")
	getNodes(t, s)
	if errs := rt.recorded(); len(errs) != 0 {
		t.Fatalf("recorded errors %q, want none", errs)
	}
}
