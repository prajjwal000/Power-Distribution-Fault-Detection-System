package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	cfg := readConfig()
	srv := &http.Server{
		Addr:    cfg.addr(),
		Handler: routes(cfg),
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
func routes(cfg apiConfig) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz(cfg))
	mux.HandleFunc("POST /ingest", handleIngest())
	return mux
}

func handleIngest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if poleID, ok := event["pole_id"].(string); ok {
			if evt, ok := event["event"].(string); ok {
				log.Printf("[api] ingest: pole=%s event=%s", poleID, evt)
			}
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
