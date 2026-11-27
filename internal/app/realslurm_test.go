//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
)

const realAPI = "/slurm/v0.0.45"

type realCluster struct {
	t     *testing.T
	url   string
	user  string
	token string
}

func connectRealCluster(t *testing.T) *realCluster {
	t.Helper()
	c := &realCluster{t: t}
	for name, into := range map[string]*string{
		"SLURMTUI_TEST_URL":  &c.url,
		"SLURMTUI_TEST_USER": &c.user,
		"SLURM_JWT":          &c.token,
	} {
		*into = os.Getenv(name)
		if *into == "" {
			t.Fatalf("%s is not set; run the integration tests with `just cluster-test`", name)
		}
	}
	return c
}

func (c *realCluster) start() *run {
	c.t.Helper()
	return startWithClock(c.t, clock.NewFake(time.Now()), []string{"--url", c.url, "--user", c.user}, map[string]string{"SLURM_JWT": c.token})
}

func (c *realCluster) call(method, path string, body, reply any) {
	c.t.Helper()
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("encode %s %s body: %s", method, path, err)
		}
		payload = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.url+realAPI+path, payload)
	if err != nil {
		c.t.Fatalf("build %s %s: %s", method, path, err)
	}
	req.Header.Set("X-SLURM-USER-NAME", c.user)
	req.Header.Set("X-SLURM-USER-TOKEN", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %s", method, path, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.t.Errorf("close %s %s reply: %s", method, path, err)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read %s %s reply: %s", method, path, err)
	}
	var envelope struct {
		Errors []struct {
			Error       string `json:"error"`
			Description string `json:"description"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		c.t.Fatalf("%s %s: HTTP %d, reply is not JSON: %s\n%s", method, path, resp.StatusCode, err, raw)
	}
	if resp.StatusCode != http.StatusOK || len(envelope.Errors) > 0 {
		c.t.Fatalf("%s %s: HTTP %d, errors %+v", method, path, resp.StatusCode, envelope.Errors)
	}
	if reply != nil {
		if err := json.Unmarshal(raw, reply); err != nil {
			c.t.Fatalf("decode %s %s reply: %s", method, path, err)
		}
	}
}

type realNode struct {
	Name  string   `json:"name"`
	State []string `json:"state"`
}

func (c *realCluster) nodes() []realNode {
	c.t.Helper()
	var reply struct {
		Nodes []realNode `json:"nodes"`
	}
	c.call(http.MethodGet, "/nodes", nil, &reply)
	return reply.Nodes
}

func (c *realCluster) nodeNotDrained() realNode {
	c.t.Helper()
	nodes := c.nodes()
	for _, n := range nodes {
		if !slices.Contains(n.State, "DRAIN") {
			return n
		}
	}
	c.t.Fatalf("every node is drained, so no node is free for this test: %+v", nodes)
	return realNode{}
}

func (c *realCluster) resume(node string) {
	c.t.Helper()
	c.call(http.MethodPost, "/node/"+node, map[string]any{"state": []string{"RESUME"}}, nil)
}

func (c *realCluster) drainUntilCleanup(node, reason string) {
	c.t.Helper()
	c.call(http.MethodPost, "/node/"+node, map[string]any{"state": []string{"DRAIN"}, "reason": reason}, nil)
	c.t.Cleanup(func() { c.resume(node) })
}

func (c *realCluster) resumeAtCleanupIfDrained(node string) {
	c.t.Helper()
	c.t.Cleanup(func() {
		for _, n := range c.nodes() {
			if n.Name == node && slices.Contains(n.State, "DRAIN") {
				c.resume(node)
			}
		}
	})
}

const uniqueDigits = 6

func uniqueReason() string {
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	return "it-" + id[len(id)-uniqueDigits:]
}

type realNumber struct {
	Set    bool `json:"set"`
	Number int  `json:"number"`
}

type realJob struct {
	JobID       int        `json:"job_id"`
	JobState    []string   `json:"job_state"`
	Nodes       string     `json:"nodes"`
	ArrayJobID  realNumber `json:"array_job_id"`
	ArrayTaskID realNumber `json:"array_task_id"`
	HetJobID    realNumber `json:"het_job_id"`
}

func (c *realCluster) submitUntilCleanup(job map[string]any) int {
	c.t.Helper()
	body := map[string]any{
		"script":                    "#!/bin/sh\nsleep 600",
		"current_working_directory": "/tmp",
		"environment":               []string{"PATH=/bin:/usr/bin"},
	}
	for k, v := range job {
		body[k] = v
	}
	var reply struct {
		JobID int `json:"job_id"`
	}
	c.call(http.MethodPost, "/job/submit", map[string]any{"job": body}, &reply)
	if reply.JobID == 0 {
		c.t.Fatalf("submit %v: reply has no job_id", body)
	}
	c.t.Cleanup(func() {
		c.call(http.MethodDelete, fmt.Sprintf("/job/%d", reply.JobID), nil, nil)
	})
	return reply.JobID
}

func (c *realCluster) runningPlainJobs() []realJob {
	c.t.Helper()
	var reply struct {
		Jobs []realJob `json:"jobs"`
	}
	c.call(http.MethodGet, "/jobs", nil, &reply)
	var running []realJob
	for _, j := range reply.Jobs {
		plain := j.ArrayJobID.Number == 0 && !j.ArrayTaskID.Set && j.HetJobID.Number == 0
		if plain && slices.Equal(j.JobState, []string{"RUNNING"}) {
			running = append(running, j)
		}
	}
	return running
}
