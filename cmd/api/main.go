package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

type ingestStats struct {
	total uint64
	byType map[string]uint64
	mu     sync.Mutex
}

func (s *ingestStats) record(eventType string) {
	atomic.AddUint64(&s.total, 1)
	if eventType != "" {
		s.mu.Lock()
		s.byType[eventType]++
		s.mu.Unlock()
	}
}

func (s *ingestStats) snapshot() (uint64, map[string]uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := atomic.LoadUint64(&s.total)
	byType := make(map[string]uint64, len(s.byType))
	for k, v := range s.byType {
		byType[k] = v
	}
	return total, byType
}

func main() {
	cfg := readConfig()

	stats := &ingestStats{byType: make(map[string]uint64)}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			total, byType := stats.snapshot()
			if total == 0 {
				continue
			}
			log.Printf("[api] ingest: %d events in 10s %v", total, byType)
		}
	}()

	srv := &http.Server{
		Addr:    cfg.addr(),
		Handler: routes(cfg, stats),
	}

	// Graceful shutdown
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
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

type apiConfig struct {
	port         string
	databaseURL  string
	simulatorURL string
}

func readConfig() apiConfig {
	return apiConfig{
		port:         env("API_PORT", "8080"),
		databaseURL:  os.Getenv("DATABASE_URL"),
		simulatorURL: env("SIMULATOR_URL", "http://localhost:8081"),
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

// routes builds the service mux. Extracted for testing.
func routes(cfg apiConfig, stats *ingestStats) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz(cfg))
	mux.HandleFunc("POST /ingest", handleIngest(stats))
	return mux
}

func handleIngest(stats *ingestStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if evt, ok := event["event"].(string); ok {
			stats.record(evt)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
