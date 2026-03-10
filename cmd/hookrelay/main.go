package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hookrelay/hookrelay/internal/api"
	"github.com/hookrelay/hookrelay/internal/config"
	"github.com/hookrelay/hookrelay/internal/delivery"
	"github.com/hookrelay/hookrelay/internal/ingress"
	"github.com/hookrelay/hookrelay/internal/ratelimit"
	"github.com/hookrelay/hookrelay/internal/router"
	"github.com/hookrelay/hookrelay/internal/store"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	var logLevel slog.Level
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(handler))

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL store
	connMaxLifetime, _ := time.ParseDuration(cfg.Database.ConnMaxLifetime)
	if connMaxLifetime == 0 {
		connMaxLifetime = 5 * time.Minute
	}
	pgStore, err := store.NewPostgresStore(
		ctx,
		cfg.Database.DSN,
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		connMaxLifetime,
	)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Initialize rate limiter
	rateLimiter := ratelimit.NewMemoryRateLimiter()

	// Initialize router
	eventRouter := router.NewRouter(pgStore)

	// Initialize delivery engine
	pollInterval, _ := time.ParseDuration(cfg.Delivery.PollInterval)
	if pollInterval == 0 {
		pollInterval = time.Second
	}
	engine := delivery.NewEngine(
		pgStore,
		rateLimiter,
		cfg.Delivery.Workers,
		pollInterval,
		cfg.Delivery.BatchSize,
	)

	// Initialize ingress handler
	ingressHandler := ingress.NewHandler(pgStore, eventRouter)

	// Initialize API server
	apiServer := api.NewServer(pgStore, cfg.API.Key)

	// Build main router
	mux := chi.NewRouter()

	// Ingress endpoint
	mux.Post("/ingest/{sourceName}", ingressHandler.ServeHTTP)

	// Health endpoints
	mux.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pgStore.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		mux.Handle(cfg.Metrics.Path, promhttp.Handler())
	}

	// Mount API routes
	mux.Mount("/", apiServer.Router())

	// Parse server timeouts
	readTimeout, _ := time.ParseDuration(cfg.Server.ReadTimeout)
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout, _ := time.ParseDuration(cfg.Server.WriteTimeout)
	if writeTimeout == 0 {
		writeTimeout = 30 * time.Second
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	// Start delivery engine
	engine.Start(ctx)
	slog.Info("delivery engine started", "workers", cfg.Delivery.Workers)

	// Start HTTP server in goroutine
	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutdown signal received")

	// Graceful shutdown
	cancel()
	engine.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
