package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"power-fault-detector/internal/simulator"
)

func main() {
	cfg := readConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("[simulator] loading state from database...")
	st, err := simulator.LoadFromDB(ctx, pool)
	if err != nil {
		log.Fatalf("load simulator state: %v", err)
	}
	log.Printf("[simulator] loaded: %d poles, %d devices",
		len(st.PoleByID), len(st.Devices))

	clock := simulator.NewClock(cfg.clockMultiplier)

	ingest := simulator.NewIngestClient(cfg.apiURL)
	broadcaster := simulator.NewBroadcaster()

	fanout := func(event simulator.TelemetryEvent) {
		ingest.Emit(event)
		broadcaster.Publish(event)
	}

	te := simulator.NewTelemetryEngine(st, clock, fanout)
	te.Start()

	log.Println("[simulator] running backfill...")
	simulator.Backfill(st, clock, fanout)

	mux := http.NewServeMux()
	svr := simulator.NewServer(st, clock, te, broadcaster)
	svr.Register(mux)

	srv := &http.Server{Addr: cfg.addr(), Handler: mux}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[simulator] shutting down...")
		te.Stop()
		cancel()
		time.Sleep(500 * time.Millisecond)
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
	databaseURL     string
	clockMultiplier int
}

func readConfig() simConfig {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	return simConfig{
		port:            env("SIM_PORT", "8081"),
		apiURL:          env("API_URL", "http://localhost:8080"),
		databaseURL:     dbURL,
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
