package fakeslurm

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/openapi"
)

const api = "/slurm/v0.0.45"

type Version struct {
	Major string `json:"major"`
	Minor string `json:"minor"`
	Micro string `json:"micro"`
}

type NoVal struct {
	Set      bool  `json:"set"`
	Infinite bool  `json:"infinite"`
	Number   int64 `json:"number"`
}

func Time(t time.Time) NoVal {
	return NoVal{Set: true, Number: t.Unix()}
}

type Node struct {
	Name              string   `json:"name"`
	State             []string `json:"state"`
	CPUs              int      `json:"cpus"`
	AllocCPUs         int      `json:"alloc_cpus"`
	RealMemory        int64    `json:"real_memory"`
	AllocMemory       int64    `json:"alloc_memory"`
	Partitions        []string `json:"partitions"`
	Reason            string   `json:"reason"`
	ReasonSetByUser   string   `json:"reason_set_by_user"`
	ReasonChangedAt   NoVal    `json:"reason_changed_at"`
	Features          []string `json:"features,omitzero"`
	Gres              string   `json:"gres,omitempty"`
	GresUsed          string   `json:"gres_used,omitempty"`
	SpecializedCPUs   string   `json:"specialized_cpus,omitempty"`
	SpecializedMemory int64    `json:"specialized_memory,omitempty"`
	FreeMem           NoVal    `json:"free_mem,omitzero"`
	CPULoad           int32    `json:"cpu_load,omitempty"`
	BootTime          NoVal    `json:"boot_time,omitzero"`
	LastBusy          NoVal    `json:"last_busy,omitzero"`
}

func Number(n int64) NoVal {
	return NoVal{Set: true, Number: n}
}

type Job struct {
	JobID                   int32     `json:"job_id"`
	ArrayJobID              NoVal     `json:"array_job_id"`
	ArrayTaskID             NoVal     `json:"array_task_id"`
	ArrayTaskString         string    `json:"array_task_string"`
	HetJobID                NoVal     `json:"het_job_id"`
	HetJobOffset            NoVal     `json:"het_job_offset"`
	JobState                []string  `json:"job_state"`
	StateReason             string    `json:"state_reason"`
	StateDescription        string    `json:"state_description"`
	Partition               string    `json:"partition"`
	Name                    string    `json:"name"`
	UserName                string    `json:"user_name"`
	Nodes                   string    `json:"nodes"`
	NodeCount               NoVal     `json:"node_count"`
	StartTime               NoVal     `json:"start_time"`
	EndTime                 NoVal     `json:"end_time"`
	Account                 string    `json:"account,omitempty"`
	QOS                     string    `json:"qos,omitempty"`
	SubmitTime              NoVal     `json:"submit_time,omitzero"`
	TimeLimit               NoVal     `json:"time_limit,omitzero"`
	Priority                NoVal     `json:"priority,omitzero"`
	ExitCode                *ExitCode `json:"exit_code,omitempty"`
	CurrentWorkingDirectory string    `json:"current_working_directory,omitempty"`
	StandardOutput          string    `json:"standard_output,omitempty"`
	Command                 string    `json:"command,omitempty"`
	Dependency              string    `json:"dependency,omitempty"`
	TRESRequested           string    `json:"tres_req_str,omitempty"`
}

type Signal struct {
	ID   NoVal  `json:"id"`
	Name string `json:"name"`
}

type ExitCode struct {
	Status     []string `json:"status"`
	ReturnCode NoVal    `json:"return_code"`
	Signal     Signal   `json:"signal"`
}

type Partition struct {
	Name        string
	DefaultTime NoVal
	MaxTime     NoVal
}

type Cluster struct {
	Name       string
	Version    Version
	Nodes      []Node
	Jobs       []Job
	Partitions []Partition
}

var DebugPartition = Partition{Name: "debug", MaxTime: NoVal{Infinite: true}}

type Request struct {
	Method string
	Query  string
	Path   string
	Header http.Header
	Body   []byte
}

type Server struct {
	URL      string
	CertPEM  []byte
	mu       sync.Mutex
	cluster  Cluster
	requests []Request
	status   int
	errors   []string
	hold     chan struct{}
	offSpec  string
	signed   *signedTokens
	claims   []Claims

	nodeFields map[int]map[string]any

	nodeRefusal   *refusal
	cancelRefusal *refusal
}

func Start(t testing.TB, cluster Cluster) *Server {
	t.Helper()
	s, handler := newServer(t, cluster)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

func StartTLS(t testing.TB, cluster Cluster) *Server {
	t.Helper()
	s, handler := newServer(t, cluster)
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	s.CertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	return s
}

func newServer(t testing.TB, cluster Cluster) (*Server, http.Handler) {
	t.Helper()
	spec, err := openapi.Slurm()
	if err != nil {
		t.Fatalf("fakeslurm: load the slurmrestd OpenAPI spec: %s", err)
	}
	s := &Server{cluster: cluster, status: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+api+"/nodes", s.nodes)
	mux.HandleFunc("GET "+api+"/jobs", s.jobs)
	mux.HandleFunc("POST "+api+"/node/{name}", s.updateNode)
	mux.HandleFunc("DELETE "+api+"/job/{id}", s.cancelJob)
	mux.HandleFunc("GET "+api+"/ping", s.ping)
	mux.HandleFunc("GET "+api+"/partitions", s.partitions)
	return s, s.record(s.checkAgainstSpec(t, spec, s.checkSignedToken(mux)))
}

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

func (s *Server) SetNodes(nodes []Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cluster.Nodes = nodes
}

func (s *Server) SetJobs(jobs []Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cluster.Jobs = jobs
}

func (s *Server) SetStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *Server) SetErrors(descriptions ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = descriptions
}

func (s *Server) Hold() func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	hold := make(chan struct{})
	s.hold = hold
	return func() {
		s.mu.Lock()
		s.hold = nil
		s.mu.Unlock()
		close(hold)
	}
}

func (s *Server) record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		s.mu.Lock()
		s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
		s.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) SetNodeField(index int, field string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nodeFields == nil {
		s.nodeFields = map[int]map[string]any{}
	}
	if s.nodeFields[index] == nil {
		s.nodeFields[index] = map[string]any{}
	}
	s.nodeFields[index][field] = value
}

func withFields(n Node, fields map[string]any) any {
	if len(fields) == 0 {
		return n
	}
	data, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	for k, v := range fields {
		out[k] = v
	}
	return out
}

func (s *Server) ExpectOffSpec(field string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offSpec = field
}

func (s *Server) checkAgainstSpec(t testing.TB, spec *openapi.Spec, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := httptest.NewRecorder()
		next.ServeHTTP(reply, r)
		s.mu.Lock()
		offSpec := s.offSpec
		s.mu.Unlock()
		if !strings.HasPrefix(offSpec, path.Base(r.URL.Path)+"[") {
			offSpec = ""
		}
		err := spec.CheckResponse(r.Method, r.URL.Path, reply.Code, reply.Body.Bytes())
		var fieldErr *openapi.FieldError
		switch {
		case offSpec != "" && err == nil:
			t.Errorf("fakeslurm: %s %s: want it to break the spec at %s, but it matches the spec", r.Method, r.URL.Path, offSpec)
		case offSpec != "" && errors.As(err, &fieldErr) && fieldErr.Field == offSpec:
		case err != nil:
			t.Errorf("fakeslurm: reply does not match the slurmrestd v0.0.45 spec: %s", err)
			http.Error(w, fmt.Sprintf("fakeslurm: reply does not match the spec: %s", err), http.StatusInternalServerError)
			return
		}
		for k, v := range reply.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(reply.Code)
		if _, err := w.Write(reply.Body.Bytes()); err != nil {
			t.Errorf("fakeslurm: write reply: %s", err)
		}
	})
}

func (s *Server) meta() map[string]any {
	return map[string]any{
		"slurm": map[string]any{
			"version": s.cluster.Version,
			"release": s.cluster.Version.Major + "." + s.cluster.Version.Minor + "." + s.cluster.Version.Micro,
			"cluster": s.cluster.Name,
		},
	}
}

func withEmptyListsAsSlurmSends(n Node) Node {
	if n.State == nil {
		n.State = []string{}
	}
	if n.Partitions == nil {
		n.Partitions = []string{}
	}
	return n
}

func (s *Server) nodes(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	hold := s.hold
	s.mu.Unlock()
	if hold != nil {
		<-hold
	}
	s.mu.Lock()
	nodes := make([]any, len(s.cluster.Nodes))
	for i, n := range s.cluster.Nodes {
		nodes[i] = withFields(withEmptyListsAsSlurmSends(n), s.nodeFields[i])
	}
	s.mu.Unlock()
	s.reply(w, map[string]any{"nodes": nodes, "last_update": NoVal{Set: true}})
}

func (s *Server) ping(w http.ResponseWriter, _ *http.Request) {
	s.reply(w, map[string]any{"pings": []map[string]any{{
		"hostname":   "ctld",
		"responding": true,
		"latency":    695,
		"primary":    true,
		"status":     "No error",
	}}})
}

func (s *Server) jobs(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	jobs := make([]Job, len(s.cluster.Jobs))
	for i, j := range s.cluster.Jobs {
		if j.JobState == nil {
			j.JobState = []string{}
		}
		jobs[i] = j
	}
	s.mu.Unlock()
	s.reply(w, map[string]any{"jobs": jobs, "last_update": NoVal{Set: true}, "last_backfill": NoVal{Set: true}})
}

func (s *Server) partitions(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	partitions := s.cluster.Partitions
	if partitions == nil {
		partitions = []Partition{DebugPartition}
	}
	out := make([]map[string]any, len(partitions))
	for i, p := range partitions {
		out[i] = map[string]any{
			"name":     p.Name,
			"defaults": map[string]any{"time": p.DefaultTime},
			"maximums": map[string]any{"time": p.MaxTime},
		}
	}
	s.mu.Unlock()
	s.reply(w, map[string]any{"partitions": out, "last_update": NoVal{Set: true}})
}

func (s *Server) reply(w http.ResponseWriter, body map[string]any) {
	s.mu.Lock()
	status := s.status
	errs := []map[string]any{}
	for _, d := range s.errors {
		errs = append(errs, map[string]any{"description": d, "error_number": 1, "error": "Unspecified error", "source": "fake"})
	}
	body["meta"] = s.meta()
	body["errors"] = errs
	body["warnings"] = []any{}
	s.mu.Unlock()
	writeJSON(w, status, body)
}

type refusal struct {
	status int
	reply  []byte
}

func (s *Server) refuseWith(w http.ResponseWriter, refusals **refusal) bool {
	s.mu.Lock()
	refused := *refusals
	s.mu.Unlock()
	if refused == nil {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(refused.status)
	if _, err := w.Write(refused.reply); err != nil {
		panic(err)
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}
