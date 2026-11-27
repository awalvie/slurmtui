package fakeslurm

import (
	"encoding/json"
	"net/http"
	"slices"
)

func (s *Server) RefuseNodeUpdates(status int, reply []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodeRefusal = &refusal{status: status, reply: reply}
}

type nodeUpdate struct {
	State  []string `json:"state"`
	Reason string   `json:"reason"`
}

func (s *Server) updateNode(w http.ResponseWriter, r *http.Request) {
	if s.refuseWith(w, &s.nodeRefusal) {
		return
	}
	var update nodeUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "fakeslurm: node update body: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")
	s.mu.Lock()
	at := slices.IndexFunc(s.cluster.Nodes, func(n Node) bool { return n.Name == name })
	if at >= 0 {
		s.cluster.Nodes[at] = applyNodeUpdate(s.cluster.Nodes[at], update, r.Header.Get("X-SLURM-USER-NAME"))
	}
	s.mu.Unlock()
	if at < 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors":   []map[string]any{{"description": "Failure to update node " + name, "error_number": 2018, "error": "Invalid node name specified", "source": "_update_node"}},
			"warnings": []any{},
		})
		return
	}
	s.reply(w, map[string]any{})
}

func applyNodeUpdate(n Node, update nodeUpdate, user string) Node {
	if update.Reason != "" {
		n.Reason = update.Reason
		n.ReasonSetByUser = user
	}
	for _, state := range update.State {
		switch state {
		case "DRAIN":
			if !slices.Contains(n.State, "DRAIN") {
				n.State = append(slices.Clone(n.State), "DRAIN")
			}
		case "RESUME":
			n.State = slices.DeleteFunc(slices.Clone(n.State), func(f string) bool { return f == "DRAIN" })
			n.Reason, n.ReasonSetByUser, n.ReasonChangedAt = "", "", NoVal{}
		}
	}
	return n
}
