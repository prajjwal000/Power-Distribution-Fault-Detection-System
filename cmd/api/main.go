package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"power-fault-detector/internal/detect"
	"power-fault-detector/internal/ingestor"
)

func main() {
	cfg := readConfig()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("[api] connected to database")

	topo, err := detect.LoadTopology(ctx, conn)
	if err != nil {
		log.Fatalf("failed to load topology: %v", err)
	}
	log.Printf("[api] loaded topology: %d poles, %d transformers, %d feeders",
		len(topo.PoleByID), len(topo.TransformerByID), len(topo.FeederByID))

	// Fetch clock multiplier from simulator to convert detection window from sim time to wall time
	multiplier := fetchClockMultiplier(cfg.simulatorURL)
	log.Printf("[api] simulator clock multiplier: %.1fx", multiplier)

	engine := detect.NewEngine(topo)
	engine.Start()

	// Detection window: 60 simulator seconds -> wall clock seconds
	detectionWindowWallSecs := int(float64(60) / multiplier)
	if detectionWindowWallSecs < 1 {
		detectionWindowWallSecs = 1
	}
	log.Printf("[api] detection window: 60 sim seconds = %d wall seconds (at %.1fx)", detectionWindowWallSecs, multiplier)

	ingestCfg := ingestor.Config{
		DetectionWindowSecs: detectionWindowWallSecs,
	}
	ing := ingestor.NewIngestor(topo, engine.JobChannel(), ingestCfg)
	ing.StartStatsLogger()

	mux := http.NewServeMux()
	
	// API routes only - nginx handles proxying to simulator and static files
	mux.HandleFunc("GET /healthz", handleHealthz(cfg))
	mux.HandleFunc("POST /ingest", handleIngest(ing, engine))
	mux.HandleFunc("GET /tickets", handleGetTickets(engine))
	mux.HandleFunc("PATCH /tickets/", handlePatchTicket(engine))
	mux.HandleFunc("GET /tickets/stream", handleTicketsStream(engine))
	mux.HandleFunc("GET /network/inferred-topology", handleInferredTopology(topo))
	mux.HandleFunc("GET /stats", handleStats(ing, topo))

	srv := &http.Server{
		Addr:    cfg.addr(),
		Handler: mux,
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		engine.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("api listening on %s", cfg.addr())
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("api error: %v", err)
	}
	log.Println("api stopped")
}

func fetchClockMultiplier(simulatorURL string) float64 {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(simulatorURL + "/clock")
	if err != nil {
		log.Printf("[api] failed to fetch clock from simulator: %v, using default 30x", err)
		return 30.0
	}
	defer resp.Body.Close()

	var clock struct {
		Multiplier float64 `json:"multiplier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&clock); err != nil {
		log.Printf("[api] failed to decode clock response: %v, using default 30x", err)
		return 30.0
	}
	if clock.Multiplier <= 0 {
		return 30.0
	}
	return clock.Multiplier
}

type apiConfig struct {
	port         string
	databaseURL  string
	simulatorURL string
}

func readConfig() apiConfig {
	return apiConfig{
		port:         env("API_PORT", "8080"),
		databaseURL:  os.Getenv("DATABASE_URL"),
		simulatorURL: env("SIMULATOR_URL", "http://simulator:8081"),
	}
}

func (c apiConfig) addr() string {
	return ":" + c.port
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func handleIngest(ing *ingestor.Ingestor, engine *detect.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ev ingestor.TelemetryEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if ev.DeviceID == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}

		ing.ProcessEvent(ev)

		if ev.Event == "boot" || ev.Event == "power_restored" {
			pole, ok := ing.GetTopology().DeviceToPole[ev.DeviceID]
			if ok {
				engine.HandleRestoration(ev.DeviceID, pole.ID, pole.DTID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleGetTickets(engine *detect.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tickets := engine.GetTickets()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tickets)
	}
}

func handleTicketsStream(engine *detect.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := engine.Broadcaster().Subscribe()
		defer engine.Broadcaster().Unsubscribe(ch)

		ctx := r.Context()
		for {
			select {
			case update, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(update)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	}
}

func handleStats(ing *ingestor.Ingestor, topo *detect.TopologyIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := ing.GetStats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ingest": stats,
			"topology": map[string]int{
				"poles":        len(topo.PoleByID),
				"transformers": len(topo.TransformerByID),
				"feeders":      len(topo.FeederByID),
			},
		})
	}
}

func handleHealthz(cfg apiConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		status := "ok"
		detail := ""
		if _, err := pgx.Connect(ctx, cfg.databaseURL); err != nil {
			status = "db_unavailable"
			detail = err.Error()
		}

		body, _ := json.Marshal(map[string]any{
			"service": "api",
			"status":  status,
			"detail":  detail,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
		w.Header().Set("Content-Type", "application/json")
		if status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = w.Write(body)
	}
}

func handlePatchTicket(engine *detect.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		id := strings.TrimPrefix(path, "/tickets/")
		if id == "" {
			http.Error(w, "missing ticket id", http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		ticket := engine.GetTicket(id)
		if ticket == nil {
			http.Error(w, "ticket not found", http.StatusNotFound)
			return
		}

		switch req.Status {
		case "acknowledged":
			if ticket.Status != "active" {
				http.Error(w, "can only acknowledge active tickets", http.StatusBadRequest)
				return
			}
			ticket.Status = "acknowledged"
		case "resolved":
			if ticket.Status != "verified" && ticket.Status != "acknowledged" {
				http.Error(w, "can only resolve verified tickets", http.StatusBadRequest)
				return
			}
			now := time.Now()
			ticket.Status = "resolved"
			ticket.ResolvedAt = &now
		default:
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}

		engine.Broadcaster().Broadcast(detect.TicketUpdate{
			Type:   "ticket_updated",
			Ticket: *ticket,
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ticket)
	}
}

func handleInferredTopology(topo *detect.TopologyIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dtID := r.URL.Query().Get("dt_id")
		if dtID == "" {
			http.Error(w, "missing dt_id parameter", http.StatusBadRequest)
			return
		}

		inferred := detect.InferTopologyForDT(dtID, topo)

		edges := make([]map[string]any, len(inferred.Edges))
		for i, e := range inferred.Edges {
			edges[i] = map[string]any{
				"parent_id":  e.ParentID,
				"child_id":   e.ChildID,
				"distance_m": math.Round(e.Distance*100) / 100,
				"confidence": math.Round(e.Confidence*100) / 100,
				"method":     e.Method,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"dt_id":  dtID,
			"edges":  edges,
			"method": inferred.Method,
		})
	}
}