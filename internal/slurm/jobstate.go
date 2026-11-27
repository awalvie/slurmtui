package slurm

import (
	"fmt"
	"slices"
)

var jobBases = []string{
	"PENDING", "RUNNING", "SUSPENDED", "COMPLETED", "CANCELLED", "FAILED",
	"TIMEOUT", "NODE_FAIL", "PREEMPTED", "BOOT_FAIL", "DEADLINE", "OUT_OF_MEMORY",
}

var jobFlagsByPriority = []string{
	"COMPLETING", "STAGE_OUT", "CONFIGURING", "EXPEDITING", "RESIZING",
	"REQUEUED", "REQUEUE_FED", "REQUEUE_HOLD", "SPECIAL_EXIT", "STOPPED",
	"REVOKED", "RESV_DEL_HOLD", "SIGNALING",
}

var jobFlagsNotShown = []string{"LAUNCH_FAILED", "RECONFIG_FAIL", "POWER_UP_NODE"}

var jobBasesThatShowReason = []string{"PENDING", "TIMEOUT", "OUT_OF_MEMORY", "DEADLINE", "FAILED"}

var jobBasesStillActive = []string{"PENDING", "RUNNING", "SUSPENDED"}

type JobState struct {
	Base  string
	Flags []string
}

func ParseJobState(values []string) (JobState, error) {
	var s JobState
	for _, v := range values {
		switch {
		case slices.Contains(jobBases, v):
			if s.Base != "" {
				return JobState{}, fmt.Errorf("job_state: two base states in %q", values)
			}
			s.Base = v
		case slices.Contains(jobFlagsByPriority, v), slices.Contains(jobFlagsNotShown, v):
			s.Flags = append(s.Flags, v)
		default:
			return JobState{}, fmt.Errorf("job_state: unknown value %q", v)
		}
	}
	if s.Base == "" {
		return JobState{}, fmt.Errorf("job_state: no base state in %q", values)
	}
	return s, nil
}

func (s JobState) has(flag string) bool {
	return slices.Contains(s.Flags, flag)
}

type JobClass int

const (
	JobPending JobClass = iota
	JobRunning
	JobCompleting
	JobCompleted
	JobFailed
	JobStopped
)

var jobBasesThatFail = []string{"FAILED", "NODE_FAIL", "BOOT_FAIL", "OUT_OF_MEMORY"}

func (s JobState) Class() JobClass {
	switch {
	case s.has("COMPLETING") || s.has("STAGE_OUT"):
		return JobCompleting
	case s.Pending():
		return JobPending
	case s.Base == "RUNNING" || s.Base == "SUSPENDED":
		return JobRunning
	case s.Base == "COMPLETED":
		return JobCompleted
	case slices.Contains(jobBasesThatFail, s.Base):
		return JobFailed
	}
	return JobStopped
}

func (s JobState) Pending() bool {
	return s.Base == "PENDING"
}

func (s JobState) CanRunOut() bool {
	return !s.Pending() && !s.Ended()
}

func (s JobState) Label() string {
	for _, f := range jobFlagsByPriority {
		if s.has(f) {
			return f
		}
	}
	return s.Base
}

func (s JobState) ShowsReason() bool {
	if s.has("COMPLETING") {
		return false
	}
	return s.has("STAGE_OUT") || slices.Contains(jobBasesThatShowReason, s.Base)
}

func (s JobState) Ended() bool {
	return !slices.Contains(jobBasesStillActive, s.Base) && !s.has("COMPLETING") && !s.has("STAGE_OUT")
}

func WidestJobLabel() int {
	widest := 0
	for _, l := range slices.Concat(jobBases, jobFlagsByPriority) {
		widest = max(widest, len(l))
	}
	return widest
}
