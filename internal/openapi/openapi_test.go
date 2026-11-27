package openapi_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/openapi"
)

func spec(t *testing.T) *openapi.Spec {
	t.Helper()
	s, err := openapi.Slurm()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func checkFails(t *testing.T, body, want string) {
	t.Helper()
	err := spec(t).CheckResponse("GET", "/slurm/v0.0.45/nodes", 200, []byte(body))
	if err == nil {
		t.Fatalf("reply passed the check, want error with %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

const lastUpdate = `"last_update":{"set":true,"infinite":false,"number":1}`

func TestRealSlurmRepliesMatchSpec(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata")
	index, err := os.ReadFile(filepath.Join(dir, "http-status.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for i, line := range strings.Split(strings.TrimSpace(string(index)), "\n")[1:] {
		cols := strings.Split(line, "\t")
		if len(cols) != 5 {
			t.Fatalf("http-status.tsv line %d: want 5 columns, got %q", i+2, line)
		}
		file, method, path := cols[0], cols[2], strings.SplitN(cols[3], "?", 2)[0]
		status, err := strconv.Atoi(cols[1])
		if err != nil {
			t.Fatalf("http-status.tsv line %d: %s", i+2, err)
		}
		body, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(file, ".json") || len(body) == 0 {
			continue
		}
		t.Run(file, func(t *testing.T) {
			if err := spec(t).CheckResponse(method, path, status, body); err != nil {
				t.Error(err)
			}
		})
		checked++
	}
	if checked < 15 {
		t.Fatalf("checked %d recorded replies, want at least 15", checked)
	}
}

func TestNodesReplyWithFieldOfIncorrectTypeFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"name":"n1","cpus":"64"}],`+lastUpdate+`}`, `nodes[0].cpus: want integer, got string "64"`)
}

func TestNodesReplyWithoutRequiredFieldFails(t *testing.T) {
	checkFails(t, `{"nodes":[]}`, `last_update: required field is missing`)
}

func TestFailedReplyMayLeaveOutRequiredFields(t *testing.T) {
	body := `{"errors":[{"description":"d","error_number":1,"error":"e","source":"s"}],"warnings":[]}`
	if err := spec(t).CheckResponse("GET", "/slurm/v0.0.45/nodes", 500, []byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestNodesReplyWithFieldNotInSpecFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"name":"n1","cpu_count":4}],`+lastUpdate+`}`, `nodes[0].cpu_count: field is not in the spec`)
}

func TestNodesReplyWithUnknownNodeStateFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"name":"n1","state":["IDEL"]}],`+lastUpdate+`}`, `nodes[0].state[0]: "IDEL" is not one of the spec values`)
}

func TestNodesReplyWithNullFieldFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"name":null}],`+lastUpdate+`}`, `nodes[0].name: want string, got null`)
}

func TestInt32FieldOutOfRangeFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"cpus":4294967296}],`+lastUpdate+`}`, `nodes[0].cpus: 4294967296 is outside the int32 range`)
}

func TestFractionInIntegerFieldFails(t *testing.T) {
	checkFails(t, `{"nodes":[{"cpus":1.5}],`+lastUpdate+`}`, `nodes[0].cpus: want integer, got number 1.5`)
}

func TestReplyForPathWithParameterUsesTemplate(t *testing.T) {
	err := spec(t).CheckResponse("DELETE", "/slurm/v0.0.45/job/1234_5", 200, []byte(`{"status":[{"bogus":1}]}`))
	if err == nil || !strings.Contains(err.Error(), "DELETE /slurm/v0.0.45/job/1234_5 200: status[0]") {
		t.Fatalf("want error that names status[0], got %v", err)
	}
}

func TestReplyForPathNotInSpecFails(t *testing.T) {
	err := spec(t).CheckResponse("GET", "/slurm/v0.0.45/nosuch", 200, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "spec has no path that matches") {
		t.Fatalf("want unknown path error, got %v", err)
	}
}
