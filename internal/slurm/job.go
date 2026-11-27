package slurm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Job struct {
	JobID            int64
	ArrayJobID       int64
	ArrayTaskID      int64
	HasArrayTaskID   bool
	ArrayTaskString  string
	HetJobID         int64
	HetJobOffset     int64
	State            JobState
	StateReason      string
	StateDescription string
	Partition        string
	Name             string
	User             string
	Nodes            string
	NodeCount        int64
	Priority         int64
	StartTime        time.Time
	EndTime          time.Time
	Account          string
	QOS              string
	SubmitTime       time.Time
	TimeLimit        TimeLimit
	ExitCode         ExitCode
	WorkingDirectory string
	StandardOutput   string
	Command          string
	Dependency       string
	TRESRequested    string
	ExpectedStart    time.Time
	Raw              json.RawMessage
}

type TimeLimit struct {
	Minutes   int64
	Set       bool
	Unlimited bool
}

func (l TimeLimit) Label() string {
	switch {
	case l.Unlimited:
		return "UNLIMITED"
	case !l.Set:
		return "Partition_Limit"
	}
	days, rest := l.Minutes/(24*60), l.Minutes%(24*60)
	clock := fmt.Sprintf("%02d:%02d:00", rest/60, rest%60)
	if days > 0 {
		return fmt.Sprintf("%d-%s", days, clock)
	}
	return clock
}

type ExitCode struct {
	ReturnCode    int64
	HasReturnCode bool
	Signal        int64
}

func (e ExitCode) Label() string {
	if !e.HasReturnCode {
		return ""
	}
	return fmt.Sprintf("%d:%d", e.ReturnCode, e.Signal)
}

type Jobs struct {
	Meta Meta
	Jobs []Job
}

func (j Job) ID() string {
	switch {
	case j.ArrayTaskString != "":
		return fmt.Sprintf("%d_[%s]", j.ArrayJobID, j.ArrayTaskString)
	case j.HasArrayTaskID:
		return fmt.Sprintf("%d_%d", j.ArrayJobID, j.ArrayTaskID)
	case j.HetJobID != 0:
		return fmt.Sprintf("%d+%d", j.HetJobID, j.HetJobOffset)
	}
	return strconv.FormatInt(j.JobID, 10)
}

func (j Job) NodeListOrReason() string {
	if !j.State.ShowsReason() {
		return j.Nodes
	}
	reason := j.StateDescription
	if reason == "" {
		reason = j.StateReason
	}
	if reason == "None" || reason == "" {
		return ""
	}
	return "(" + reason + ")"
}

func (j Job) Elapsed(now time.Time) (time.Duration, bool) {
	if j.State.Base == "PENDING" || j.StartTime.IsZero() {
		return 0, false
	}
	if j.State.Ended() {
		return j.EndTime.Sub(j.StartTime), true
	}
	return now.Sub(j.StartTime), true
}

func (j Job) Waited(now time.Time) (time.Duration, bool) {
	if j.State.Base != "PENDING" || j.SubmitTime.IsZero() {
		return 0, false
	}
	return now.Sub(j.SubmitTime), true
}

func (j Job) HasOwnTimeLimit() bool {
	return j.TimeLimit.Set || j.TimeLimit.Unlimited
}

func (j Job) TimeLeft(limit TimeLimit, now time.Time) (time.Duration, bool) {
	if j.State.Base == "PENDING" || j.State.Ended() || !limit.Set || limit.Unlimited {
		return 0, false
	}
	elapsed, ok := j.Elapsed(now)
	if !ok {
		return 0, false
	}
	return max(time.Duration(limit.Minutes)*time.Minute-elapsed, 0), true
}

type wireJob struct {
	JobID            int64    `json:"job_id"`
	ArrayJobID       noVal    `json:"array_job_id"`
	ArrayTaskID      noVal    `json:"array_task_id"`
	ArrayTaskString  string   `json:"array_task_string"`
	HetJobID         noVal    `json:"het_job_id"`
	HetJobOffset     noVal    `json:"het_job_offset"`
	JobState         []string `json:"job_state"`
	StateReason      string   `json:"state_reason"`
	StateDescription string   `json:"state_description"`
	Partition        string   `json:"partition"`
	Name             string   `json:"name"`
	UserName         string   `json:"user_name"`
	Nodes            string   `json:"nodes"`
	NodeCount        noVal    `json:"node_count"`
	StartTime        noVal    `json:"start_time"`
	EndTime          noVal    `json:"end_time"`
	Account          string   `json:"account"`
	QOS              string   `json:"qos"`
	SubmitTime       noVal    `json:"submit_time"`
	TimeLimit        noVal    `json:"time_limit"`
	Priority         noVal    `json:"priority"`
	ExitCode         struct {
		ReturnCode noVal `json:"return_code"`
		Signal     struct {
			ID noVal `json:"id"`
		} `json:"signal"`
	} `json:"exit_code"`
	CurrentWorkingDirectory string `json:"current_working_directory"`
	StandardOutput          string `json:"standard_output"`
	Command                 string `json:"command"`
	Dependency              string `json:"dependency"`
	TRESRequested           string `json:"tres_req_str"`
}

type wireJobs struct {
	envelope
	Meta wireMeta          `json:"meta"`
	Jobs []json.RawMessage `json:"jobs"`
}

func (w wireJob) job() (Job, error) {
	state, err := ParseJobState(w.JobState)
	if err != nil {
		return Job{}, err
	}
	j := Job{
		JobID:            w.JobID,
		ArrayTaskString:  w.ArrayTaskString,
		State:            state,
		StateReason:      w.StateReason,
		StateDescription: w.StateDescription,
		Partition:        w.Partition,
		Name:             w.Name,
		User:             w.UserName,
		Nodes:            w.Nodes,
		Account:          w.Account,
		QOS:              w.QOS,
		TimeLimit:        timeLimitOf(w.TimeLimit),
		WorkingDirectory: w.CurrentWorkingDirectory,
		StandardOutput:   w.StandardOutput,
		Command:          w.Command,
		Dependency:       w.Dependency,
		TRESRequested:    w.TRESRequested,
	}
	counts := []struct {
		field string
		v     noVal
		into  *int64
		set   *bool
	}{
		{"array_job_id", w.ArrayJobID, &j.ArrayJobID, nil},
		{"array_task_id", w.ArrayTaskID, &j.ArrayTaskID, &j.HasArrayTaskID},
		{"het_job_id", w.HetJobID, &j.HetJobID, nil},
		{"het_job_offset", w.HetJobOffset, &j.HetJobOffset, nil},
		{"node_count", w.NodeCount, &j.NodeCount, nil},
		{"priority", w.Priority, &j.Priority, nil},
		{"exit_code.return_code", w.ExitCode.ReturnCode, &j.ExitCode.ReturnCode, &j.ExitCode.HasReturnCode},
		{"exit_code.signal.id", w.ExitCode.Signal.ID, &j.ExitCode.Signal, nil},
	}
	for _, c := range counts {
		n, set, err := c.v.count(c.field)
		if err != nil {
			return Job{}, err
		}
		if set {
			*c.into = n
		}
		if c.set != nil {
			*c.set = set
		}
	}
	if j.SubmitTime, err = w.SubmitTime.time(); err != nil {
		return Job{}, fmt.Errorf("submit_time: %w", err)
	}
	if state.Base == "PENDING" {
		if w.StartTime.Infinite {
			return j, nil
		}
		if j.ExpectedStart, err = w.StartTime.time(); err != nil {
			return Job{}, fmt.Errorf("start_time: %w", err)
		}
		return j, nil
	}
	if j.StartTime, err = w.StartTime.time(); err != nil {
		return Job{}, fmt.Errorf("start_time: %w", err)
	}
	if j.EndTime, err = w.EndTime.time(); err != nil {
		return Job{}, fmt.Errorf("end_time: %w", err)
	}
	if state.Ended() && j.EndTime.IsZero() && !j.StartTime.IsZero() {
		return Job{}, fmt.Errorf("end_time: not set on an ended job")
	}
	return j, nil
}

func (c *Client) Jobs(ctx context.Context) (Jobs, error) {
	var wire wireJobs
	if err := c.get(ctx, "/jobs", &wire); err != nil {
		return Jobs{}, err
	}
	url := c.url("/jobs")
	out := Jobs{Meta: wire.Meta.meta(), Jobs: make([]Job, 0, len(wire.Jobs))}
	for _, raw := range wire.Jobs {
		var w wireJob
		if err := decodeRecord(raw, &w); err != nil {
			return Jobs{}, fmt.Errorf("GET %s: job %d: %w", url, w.JobID, err)
		}
		j, err := w.job()
		if err != nil {
			return Jobs{}, fmt.Errorf("GET %s: job %d: %w", url, w.JobID, err)
		}
		j.Raw = raw
		out.Jobs = append(out.Jobs, j)
	}
	return out, nil
}
