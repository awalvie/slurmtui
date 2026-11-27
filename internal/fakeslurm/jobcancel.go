package fakeslurm

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) RefuseJobCancels(reply []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelRefusal = &refusal{status: http.StatusOK, reply: reply}
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if s.refuseWith(w, &s.cancelRefusal) {
		return
	}
	id := r.PathValue("id")
	matches, ok := cancelMatcher(id)
	if !ok {
		http.Error(w, "fakeslurm: job id "+strconv.Quote(id)+" has a form the fake does not take", http.StatusBadRequest)
		return
	}
	cancelled := 0
	s.mu.Lock()
	for i, j := range s.cluster.Jobs {
		if matches(j) {
			s.cluster.Jobs[i].JobState = []string{"CANCELLED"}
			cancelled++
		}
	}
	s.mu.Unlock()
	status := []map[string]any{}
	if cancelled == 0 {
		status = append(status, map[string]any{
			"error":      map[string]any{"string": "Invalid job id specified", "code": 2017, "message": "Invalid job id specified"},
			"step_id":    id,
			"job_id":     NoVal{},
			"federation": map[string]any{"sibling": ""},
		})
	}
	s.reply(w, map[string]any{"status": status})
}

func cancelMatcher(id string) (func(Job) bool, bool) {
	if array, task, isTask := strings.Cut(id, "_"); isTask {
		a, aerr := strconv.ParseInt(array, 10, 64)
		t, terr := strconv.ParseInt(task, 10, 64)
		return func(j Job) bool {
			return j.ArrayJobID.Number == a && j.ArrayTaskID.Set && j.ArrayTaskID.Number == t
		}, aerr == nil && terr == nil
	}
	n, err := strconv.ParseInt(id, 10, 64)
	return func(j Job) bool {
		return int64(j.JobID) == n || j.ArrayJobID.Number == n || j.HetJobID.Number == n
	}, err == nil
}
