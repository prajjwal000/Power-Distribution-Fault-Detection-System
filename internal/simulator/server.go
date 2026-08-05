package simulator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	st          *SimulatorState
	clock       *Clock
	te          *TelemetryEngine
	broadcaster *Broadcaster
}

func NewServer(st *SimulatorState, clock *Clock, te *TelemetryEngine, broadcaster *Broadcaster) *Server {
	return &Server{st: st, clock: clock, te: te, broadcaster: broadcaster}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /clock", s.handleGetClock)
	mux.HandleFunc("POST /clock", s.handleSetClock)
	mux.HandleFunc("GET /sim/topology/tree", s.handleTopologyTree)
	mux.HandleFunc("GET /sim/faults", s.handleListFaults)
	mux.HandleFunc("POST /sim/faults", s.handleInjectFault)
	mux.HandleFunc("POST /sim/faults/{id}", s.handleRepairFault)
	mux.HandleFunc("POST /sim/faults/repair", s.handleRepairAll)
	mux.HandleFunc("GET /sim/events/stream", s.handleEventsStream)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	body, _ := json.Marshal(map[string]any{
		"service":    "simulator",
		"status":     "ok",
		"poles":      len(s.st.PoleByID),
		"devices":    len(s.st.Devices),
		"multiplier": s.clock.GetMultiplier(),
		"time":       time.Now().UTC().Format(time.RFC3339),
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *Server) handleGetClock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.clock.Response())
}

func (s *Server) handleSetClock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Multiplier *int   `json:"multiplier"`
		SimTime    *int64 `json:"sim_time"`
		Paused     *bool  `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Multiplier != nil {
		s.clock.SetMultiplier(*req.Multiplier)
	}
	if req.SimTime != nil {
		s.clock.SetSimTime(*req.SimTime)
	}
	if req.Paused != nil {
		if *req.Paused {
			s.clock.Pause()
		} else {
			s.clock.Resume()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.clock.Response())
}

type TopologyTree struct {
	Substations   any `json:"substations"`
	Feeders       any `json:"feeders"`
	Transformers  any `json:"transformers"`
	GTTopology    any `json:"gt_topology"`
	RegistryPoles any `json:"registry_poles"`
}

func (s *Server) handleTopologyTree(w http.ResponseWriter, r *http.Request) {
	gtNodes := make([]map[string]any, 0, len(s.st.GTPoles))
	for _, p := range s.st.GTPoles {
		node := map[string]any{
			"pole_id":        p.ID,
			"parent_id":      p.ParentPoleID,
			"dt_id":          p.DTID,
			"seq_on_line":    p.SeqOnLine,
			"is_branch_point": p.IsBranchPoint,
			"children":       s.st.Children[p.ID],
			"lat":            p.Lat,
			"lon":            p.Lon,
		}
		gtNodes = append(gtNodes, node)
	}

	regPoles := make([]map[string]any, 0, len(s.st.PoleByID))
	for _, p := range s.st.PoleByID {
		node := map[string]any{
			"id":        p.ID,
			"dt_id":     p.DTID,
			"feeder_id": p.FeederID,
			"lat":       p.Lat,
			"lon":       p.Lon,
			"pole_type": p.PoleType,
			"ward":      p.Ward,
		}
		if p.SeqOnLine != nil {
			node["seq_on_line"] = *p.SeqOnLine
		}
		if p.ParentPoleID != nil {
			node["parent_pole_id"] = *p.ParentPoleID
		}
		if p.Pincode != nil {
			node["pincode"] = *p.Pincode
		}
		if p.DeviceID != nil {
			node["device_id"] = *p.DeviceID
		}
		regPoles = append(regPoles, node)
	}

	dtStatuses := make([]map[string]any, 0)
	for _, t := range s.st.Transformers {
		hasTopo := false
		for _, p := range s.st.GTPoles {
			if p.DTID == t.ID && p.ParentPoleID != nil {
				hasTopo = true
				break
			}
		}
		dtStatuses = append(dtStatuses, map[string]any{
			"dt_id":        t.ID,
			"has_topology": hasTopo,
		})
	}

	tree := map[string]any{
		"substations":        s.st.Substations,
		"feeders":            s.st.Feeders,
		"transformers":       s.st.Transformers,
		"gt_topology":        gtNodes,
		"registry_poles":     regPoles,
		"dt_topology_status": dtStatuses,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tree)
}

func (s *Server) handleListFaults(w http.ResponseWriter, r *http.Request) {
	faults := s.te.ListFaults()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(faults)
}

func (s *Server) handleInjectFault(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID string `json:"parent_id"`
		ChildID  string `json:"child_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ParentID == "" || req.ChildID == "" {
		http.Error(w, "parent_id and child_id are required", http.StatusBadRequest)
		return
	}
	fault := s.te.InjectFault(req.ParentID, req.ChildID)
	if fault == nil {
		http.Error(w, "invalid fault target: parent->child not an edge in ground-truth topology", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(fault)
}

func (s *Server) handleRepairFault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.te.RepairFault(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRepairAll(w http.ResponseWriter, r *http.Request) {
	s.te.RepairAll()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch, cancel := s.broadcaster.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", data); writeErr != nil {
				return
			}
			flusher.Flush()
		}
	}
}
