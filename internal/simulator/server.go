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
	mux.HandleFunc("POST /sim/noise", s.handleInjectNoise)
	mux.HandleFunc("GET /scheduled-outages", s.handleScheduledOutages)
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
		Type     string `json:"type"`
		ParentID string `json:"parent_id"`
		ChildID  string `json:"child_id"`
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "type is required (span|dt|feeder)", http.StatusBadRequest)
		return
	}

	var fault *Fault
	switch req.Type {
	case "span":
		if req.ParentID == "" || req.ChildID == "" {
			http.Error(w, "span fault requires parent_id and child_id", http.StatusBadRequest)
			return
		}
		fault = s.te.InjectFault(req.ParentID, req.ChildID)
	case "dt":
		if req.TargetID == "" {
			http.Error(w, "dt fault requires target_id", http.StatusBadRequest)
			return
		}
		fault = s.te.InjectDT(req.TargetID)
	case "feeder":
		if req.TargetID == "" {
			http.Error(w, "feeder fault requires target_id", http.StatusBadRequest)
			return
		}
		fault = s.te.InjectFeeder(req.TargetID)
	default:
		http.Error(w, "unknown fault type: "+req.Type, http.StatusBadRequest)
		return
	}

	if fault == nil {
		// Could be invalid target or duplicate — check for existing fault
		for _, f := range s.te.ListFaults() {
			if (req.Type == "span" && f.Target == req.ParentID+"->"+req.ChildID) ||
				(req.Type == "dt" && f.Target == req.TargetID) ||
				(req.Type == "feeder" && f.Target == req.TargetID) {
				http.Error(w, "fault already active on this target", http.StatusConflict)
				return
			}
		}
		http.Error(w, "invalid fault target", http.StatusBadRequest)
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

func (s *Server) handleInjectNoise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind     string `json:"kind"`      // device_death | duplicate | stale_replay
		DeviceID string `json:"device_id"` // optional: specific device, or random if empty
		Count    int    `json:"count"`     // optional: number of devices/events
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}

	var err error
	switch req.Kind {
	case "device_death":
		err = s.te.InjectDeviceDeath(req.DeviceID, req.Count)
	case "duplicate":
		err = s.te.InjectDuplicateEvent(req.DeviceID, req.Count)
	case "stale_replay":
		err = s.te.InjectStaleReplay(req.DeviceID, req.Count)
	default:
		http.Error(w, "unknown noise kind: "+req.Kind, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScheduledOutages(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		http.Error(w, "from and to query parameters required", http.StatusBadRequest)
		return
	}

	fromTime, err := time.Parse(time.RFC3339, from)
	if err != nil {
		http.Error(w, "invalid from time: "+err.Error(), http.StatusBadRequest)
		return
	}
	toTime, err := time.Parse(time.RFC3339, to)
	if err != nil {
		http.Error(w, "invalid to time: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate deterministic mock outages based on the time range
	// In a real system this would come from a database
	outages := s.generateMockOutages(fromTime, toTime)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(outages)
}

func (s *Server) generateMockOutages(from, to time.Time) []ScheduledOutage {
	var outages []ScheduledOutage
	// Deterministic: use feeder/DT IDs from state, hash the time range
	// to decide which outages exist in this window

	// For demo: create 2-3 outages per day in the range
	days := int(to.Sub(from).Hours() / 24)
	if days < 1 {
		days = 1
	}

	for d := 0; d < days; d++ {
		dayStart := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, d)

		// Feeder outage ~1 per day
		if len(s.st.Feeders) > 0 {
			feeder := s.st.Feeders[d%len(s.st.Feeders)]
			start := dayStart.Add(time.Duration(10+d*7)*time.Hour + time.Duration(d*13)*time.Minute)
			end := start.Add(time.Duration(2+d*3)*time.Hour)
			if start.Before(to) && end.After(from) {
				// 1 in 10 is "cancelled" (don't include)
				feederHash := int(feeder.ID[0]) + int(feeder.ID[len(feeder.ID)-1])
				if (d*7+feederHash)%10 != 0 {
					// 20-40 min overrun
					overrun := time.Duration(20 + (d*11)%21) * time.Minute
					outages = append(outages, ScheduledOutage{
						ID:      fmt.Sprintf("SO-%s-%03d", dayStart.Format("2006-01-02"), d*10+feederHash),
						Scope:   "feeder",
						TargetID: feeder.ID,
						Start:   start.Format(time.RFC3339),
						End:     end.Add(overrun).Format(time.RFC3339),
						Reason:  "Planned maintenance",
					})
				}
			}
		}

		// DT outage ~1 per day
		if len(s.st.Transformers) > 0 {
			dt := s.st.Transformers[d%len(s.st.Transformers)]
			start := dayStart.Add(time.Duration(14+d*5)*time.Hour)
			end := start.Add(time.Duration(1+d)*time.Hour)
			if start.Before(to) && end.After(from) {
				dtHash := int(dt.ID[0]) + int(dt.ID[len(dt.ID)-1])
				if (d*5+dtHash)%10 != 0 {
					overrun := time.Duration(20 + (d*7)%21) * time.Minute
					outages = append(outages, ScheduledOutage{
						ID:      fmt.Sprintf("SO-%s-%03d", dayStart.Format("2006-01-02"), d*10+dtHash+100),
						Scope:   "dt",
						TargetID: dt.ID,
						Start:   start.Format(time.RFC3339),
						End:     end.Add(overrun).Format(time.RFC3339),
						Reason:  "Load shedding",
					})
				}
			}
		}
	}

	return outages
}

// ScheduledOutage represents a planned outage from the mock feed.
type ScheduledOutage struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`     // feeder | dt
	TargetID  string `json:"target_id"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Reason    string `json:"reason"`
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
