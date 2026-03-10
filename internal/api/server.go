package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/hookrelay/hookrelay/internal/middleware"
	"github.com/hookrelay/hookrelay/internal/store"
)

// Server holds the HTTP router and the backing store for the Dashboard API.
type Server struct {
	store  store.Store
	router chi.Router
}

// NewServer creates a new Server with all routes registered.
func NewServer(s store.Store, apiKey string) *Server {
	srv := &Server{store: s}

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// Health check endpoints (public, no auth)
	r.Get("/healthz", srv.handleHealthz)
	r.Get("/readyz", srv.handleReadyz)

	// Dashboard API routes (protected by API key)
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.Auth(apiKey))

		// Sources
		api.Post("/sources", srv.handleCreateSource)
		api.Get("/sources", srv.handleListSources)
		api.Get("/sources/{id}", srv.handleGetSource)
		api.Put("/sources/{id}", srv.handleUpdateSource)
		api.Delete("/sources/{id}", srv.handleDeleteSource)

		// Subscriptions
		api.Post("/subscriptions", srv.handleCreateSubscription)
		api.Get("/subscriptions", srv.handleListSubscriptions)
		api.Get("/subscriptions/{id}", srv.handleGetSubscription)
		api.Put("/subscriptions/{id}", srv.handleUpdateSubscription)
		api.Delete("/subscriptions/{id}", srv.handleDeleteSubscription)
		api.Get("/subscriptions/{id}/stats", srv.handleGetSubscriptionStats)

		// Events
		api.Get("/events", srv.handleListEvents)
		api.Get("/events/{id}", srv.handleGetEvent)
		api.Get("/events/{id}/deliveries", srv.handleGetEventDeliveries)
		api.Post("/events/{id}/replay", srv.handleReplayEvent)

		// Dead Letters
		api.Get("/dead-letters", srv.handleListDeadLetters)
		api.Post("/dead-letters/{id}/retry", srv.handleRetryDeadLetter)
		api.Post("/dead-letters/batch-retry", srv.handleBatchRetryDeadLetters)

		// Stats
		api.Get("/stats/overview", srv.handleOverviewStats)
		api.Get("/stats/throughput", srv.handleThroughput)
	})

	srv.router = r
	return srv
}

// Router returns the chi.Router so it can be mounted in an HTTP server.
func (s *Server) Router() chi.Router {
	return s.router
}

// handleHealthz returns 200 OK if the service is alive.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz returns 200 OK if the backing store is reachable.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "store not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
