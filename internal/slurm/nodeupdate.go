package slurm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type NodeStateChange string

const (
	Drain  NodeStateChange = "DRAIN"
	Resume NodeStateChange = "RESUME"
)

type NodeUpdate struct {
	State  NodeStateChange
	Reason string
}

type wireNodeUpdate struct {
	State  []NodeStateChange `json:"state"`
	Reason string            `json:"reason,omitempty"`
}

func (c *Client) UpdateNode(ctx context.Context, name string, update NodeUpdate) error {
	body, err := json.Marshal(wireNodeUpdate{State: []NodeStateChange{update.State}, Reason: update.Reason})
	if err != nil {
		return fmt.Errorf("encode update of node %q: %w", name, err)
	}
	var reply envelope
	return c.do(ctx, http.MethodPost, "/node/"+url.PathEscape(name), body, &reply)
}
