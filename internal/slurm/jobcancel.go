package slurm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type CancelTarget struct {
	ID    string
	Scope string
}

func (j Job) CancelTarget() CancelTarget {
	switch {
	case j.HasArrayTaskID:
		return CancelTarget{ID: fmt.Sprintf("%d_%d", j.ArrayJobID, j.ArrayTaskID)}
	case j.ArrayTaskString != "":
		id := strconv.FormatInt(j.JobID, 10)
		return CancelTarget{ID: id, Scope: "all tasks of array " + id}
	case j.HetJobID != 0:
		id := strconv.FormatInt(j.HetJobID, 10)
		return CancelTarget{ID: id, Scope: "all components of het job " + id}
	}
	return CancelTarget{ID: strconv.FormatInt(j.JobID, 10)}
}

type wireCancelStatus struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	StepID string `json:"step_id"`
}

type wireCancelReply struct {
	envelope
	Status []wireCancelStatus `json:"status"`
}

func (r wireCancelReply) err() error {
	if err := r.envelope.err(); err != nil {
		return err
	}
	var parts []string
	for _, s := range r.Status {
		if s.Error.Code != 0 {
			parts = append(parts, fmt.Sprintf("job %s: %s (error %d)", s.StepID, s.Error.Message, s.Error.Code))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("slurm: %s", strings.Join(parts, "; "))
}

func (c *Client) CancelJob(ctx context.Context, id string) error {
	var reply wireCancelReply
	return c.do(ctx, http.MethodDelete, "/job/"+url.PathEscape(id), nil, &reply)
}
