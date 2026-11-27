package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/awalvie/slurmtui/internal/auth"
)

const apiPrefix = "/slurm/v0.0.45"

type Meta struct {
	Cluster string
	Version string
}

type Node struct {
	Name              string
	State             NodeState
	CPUs              int
	AllocCPUs         int
	RealMemory        int64
	AllocMemory       int64
	Partitions        []string
	Reason            string
	ReasonSetBy       string
	ReasonChangedAt   time.Time
	Features          []string
	Gres              string
	GresUsed          string
	SpecializedCPUs   string
	SpecializedMemory int64
	FreeMem           int64
	HasFreeMem        bool
	CPULoad           int
	BootTime          time.Time
	LastBusy          time.Time
	Raw               json.RawMessage
}

func (n Node) Usable() bool {
	drained := n.State.Has("DRAIN") && n.AllocCPUs == 0
	return !n.State.Down() && !drained && !n.State.Has("POWERED_DOWN")
}

type Nodes struct {
	Meta  Meta
	Nodes []Node
}

type Auth interface {
	Next() (auth.Credentials, error)
}

const protocolAuthenticationError = 1007

type refusalExplainer interface {
	ExplainRefusal() string
}

type Client struct {
	BaseURL string
	Auth    Auth
	HTTP    *http.Client
}

type wireMeta struct {
	Slurm struct {
		Version struct {
			Major string `json:"major"`
			Minor string `json:"minor"`
			Micro string `json:"micro"`
		} `json:"version"`
		Cluster string `json:"cluster"`
	} `json:"slurm"`
}

func (m wireMeta) meta() Meta {
	v := m.Slurm.Version
	return Meta{Cluster: m.Slurm.Cluster, Version: v.Major + "." + v.Minor + "." + v.Micro}
}

type noVal struct {
	Set      bool  `json:"set"`
	Infinite bool  `json:"infinite"`
	Number   int64 `json:"number"`
}

func (v noVal) time() (time.Time, error) {
	if v.Infinite {
		return time.Time{}, fmt.Errorf("infinite")
	}
	if !v.Set || v.Number == 0 {
		return time.Time{}, nil
	}
	return time.Unix(v.Number, 0), nil
}

func (v noVal) count(field string) (int64, bool, error) {
	if v.Infinite {
		return 0, false, fmt.Errorf("%s: infinite", field)
	}
	return v.Number, v.Set, nil
}

type wireNode struct {
	Name              string   `json:"name"`
	State             []string `json:"state"`
	CPUs              int      `json:"cpus"`
	AllocCPUs         int      `json:"alloc_cpus"`
	RealMemory        int64    `json:"real_memory"`
	AllocMemory       int64    `json:"alloc_memory"`
	Partitions        []string `json:"partitions"`
	Reason            string   `json:"reason"`
	ReasonSetByUser   string   `json:"reason_set_by_user"`
	ReasonChangedAt   noVal    `json:"reason_changed_at"`
	Features          []string `json:"features"`
	Gres              string   `json:"gres"`
	GresUsed          string   `json:"gres_used"`
	SpecializedCPUs   string   `json:"specialized_cpus"`
	SpecializedMemory int64    `json:"specialized_memory"`
	FreeMem           noVal    `json:"free_mem"`
	CPULoad           int      `json:"cpu_load"`
	BootTime          noVal    `json:"boot_time"`
	LastBusy          noVal    `json:"last_busy"`
}

func (n wireNode) node(raw json.RawMessage) (Node, error) {
	state, err := ParseNodeState(n.State)
	if err != nil {
		return Node{}, err
	}
	out := Node{
		Name:              n.Name,
		State:             state,
		CPUs:              n.CPUs,
		AllocCPUs:         n.AllocCPUs,
		RealMemory:        n.RealMemory,
		AllocMemory:       n.AllocMemory,
		Partitions:        n.Partitions,
		Reason:            n.Reason,
		ReasonSetBy:       n.ReasonSetByUser,
		Features:          n.Features,
		Gres:              n.Gres,
		GresUsed:          n.GresUsed,
		SpecializedCPUs:   n.SpecializedCPUs,
		SpecializedMemory: n.SpecializedMemory,
		CPULoad:           n.CPULoad,
		Raw:               raw,
	}
	if out.FreeMem, out.HasFreeMem, err = n.FreeMem.count("free_mem"); err != nil {
		return Node{}, err
	}
	times := []struct {
		field string
		v     noVal
		into  *time.Time
	}{
		{"reason_changed_at", n.ReasonChangedAt, &out.ReasonChangedAt},
		{"boot_time", n.BootTime, &out.BootTime},
		{"last_busy", n.LastBusy, &out.LastBusy},
	}
	for _, t := range times {
		if *t.into, err = t.v.time(); err != nil {
			return Node{}, fmt.Errorf("%s: %w", t.field, err)
		}
	}
	return out, nil
}

type wireError struct {
	Description string `json:"description"`
	Error       string `json:"error"`
	ErrorNumber int    `json:"error_number"`
}

type envelope struct {
	Errors []wireError `json:"errors"`
}

func (e envelope) refusedToken() bool {
	for _, w := range e.Errors {
		if w.ErrorNumber == protocolAuthenticationError {
			return true
		}
	}
	return false
}

func (e envelope) err() error {
	if len(e.Errors) == 0 {
		return nil
	}
	parts := make([]string, 0, len(e.Errors))
	for _, w := range e.Errors {
		switch {
		case w.Description != "" && w.Error != "" && w.Error != w.Description:
			parts = append(parts, w.Description+": "+w.Error)
		case w.Description != "":
			parts = append(parts, w.Description)
		case w.Error != "":
			parts = append(parts, w.Error)
		default:
			parts = append(parts, fmt.Sprintf("error %d", w.ErrorNumber))
		}
	}
	return fmt.Errorf("slurm: %s", strings.Join(parts, "; "))
}

type wireNodes struct {
	envelope
	Meta  wireMeta          `json:"meta"`
	Nodes []json.RawMessage `json:"nodes"`
}

func (c *Client) Nodes(ctx context.Context) (Nodes, error) {
	var wire wireNodes
	if err := c.get(ctx, "/nodes", &wire); err != nil {
		return Nodes{}, err
	}
	url := c.url("/nodes")
	out := Nodes{Meta: wire.Meta.meta(), Nodes: make([]Node, 0, len(wire.Nodes))}
	for _, raw := range wire.Nodes {
		var n wireNode
		if err := decodeRecord(raw, &n); err != nil {
			return Nodes{}, fmt.Errorf("GET %s: node %q: %w", url, n.Name, err)
		}
		node, err := n.node(raw)
		if err != nil {
			return Nodes{}, fmt.Errorf("GET %s: node %q: %w", url, n.Name, err)
		}
		out.Nodes = append(out.Nodes, node)
	}
	return out, nil
}

type reply interface {
	err() error
}

func (c *Client) url(path string) string {
	return c.BaseURL + apiPrefix + path
}

func (c *Client) explainRefusal() (string, bool) {
	explainer, ok := c.Auth.(refusalExplainer)
	if !ok {
		return "", false
	}
	return explainer.ExplainRefusal(), true
}

func (c *Client) get(ctx context.Context, path string, into reply) error {
	return c.do(ctx, http.MethodGet, path, nil, into)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, into reply) (err error) {
	url := c.url(path)
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	creds, err := c.Auth.Next()
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	req.Header.Set("X-SLURM-USER-NAME", creds.User)
	req.Header.Set("X-SLURM-USER-TOKEN", string(creds.Token))
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("%s %s: close body: %w", method, url, cerr)
		}
	}()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("%s %s: %w", method, url, redirectRefused(resp))
	}
	if resp.StatusCode != http.StatusOK {
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if rerr != nil {
			return fmt.Errorf("%s %s: HTTP %d: read body: %w", method, url, resp.StatusCode, rerr)
		}
		var env envelope
		if json.Unmarshal(body, &env) == nil {
			if why, ok := c.explainRefusal(); ok && env.refusedToken() {
				return fmt.Errorf("%s %s: %s", method, url, why)
			}
			if eerr := env.err(); eerr != nil {
				return fmt.Errorf("%s %s: HTTP %d: %w", method, url, resp.StatusCode, eerr)
			}
			return fmt.Errorf("%s %s: HTTP %d", method, url, resp.StatusCode)
		}
		text, _, _ := strings.Cut(strings.TrimSpace(string(body)), "\n")
		if len(text) > 200 {
			text = text[:200] + "…"
		}
		return fmt.Errorf("%s %s: HTTP %d: %s", method, url, resp.StatusCode, text)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("%s %s: decode: %w", method, url, err)
	}
	if err := into.err(); err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	return nil
}
