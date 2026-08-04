package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	cfg := readConfig()
	srv := &http.Server{
		Addr:    cfg.addr(),
		Handler: routes(cfg),
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("simulator listening on %s", cfg.addr())
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("simulator error: %v", err)
	}
	log.Println("simulator stopped")
}

type simConfig struct {
	port            string
	apiURL          string
	clockMultiplier int
}

func readConfig() simConfig {
	return simConfig{
		port:            env("SIM_PORT", "8081"),
		apiURL:          env("API_URL", "http://localhost:8080"),
		clockMultiplier: intEnv("CLOCK_MULTIPLIER", 30),
	}
}

func (c simConfig) addr() string { return ":" + c.port }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func routes(cfg simConfig) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz(cfg))
	return mux
}

type healthResponse struct {
	Service    string `json:"service"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	Multiplier int    `json:"multiplier,omitempty"`
	Time       string `json:"time"`
}

func handleHealthz(cfg simConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(healthResponse{
			Service:    "simulator",
			Status:     "ok",
			Multiplier: cfg.clockMultiplier,
			Time:       time.Now().UTC().Format(time.RFC3339),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}
