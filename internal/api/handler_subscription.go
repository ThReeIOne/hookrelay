package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hookrelay/hookrelay/internal/store"
)

// subscriptionRequest is the JSON body for creating or updating a subscription.
type subscriptionRequest struct {
	Name           string            `json:"name"`
	SourceID       int64             `json:"source_id"`
	EventFilter    []string          `json:"event_filter"`
	TargetURL      string            `json:"target_url"`
	SigningSecret  string            `json:"signing_secret"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	Transform      map[string]any    `json:"transform"`
	MaxRetries     *int              `json:"max_retries"`
	TimeoutSeconds *int              `json:"timeout_seconds"`
	RateLimitRPS   *int              `json:"rate_limit_rps"`
	Active         *bool             `json:"active"`
}

// handleCreateSubscription creates a new subscription.
func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req subscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.SourceID == 0 {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}
	if req.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "target_url is required")
		return
	}

	sub := &store.Subscription{
		Name:          req.Name,
		SourceID:      req.SourceID,
		EventFilter:   req.EventFilter,
		TargetURL:     req.TargetURL,
		SigningSecret: req.SigningSecret,
		CustomHeaders: req.CustomHeaders,
		Transform:     req.Transform,
		Active:        true,
	}

	if req.MaxRetries != nil {
		sub.MaxRetries = *req.MaxRetries
	} else {
		sub.MaxRetries = 8 // default
	}

	if req.TimeoutSeconds != nil {
		sub.TimeoutSeconds = *req.TimeoutSeconds
	} else {
		sub.TimeoutSeconds = 30 // default
	}

	if req.RateLimitRPS != nil {
		sub.RateLimitRPS = *req.RateLimitRPS
	}

	created, err := s.store.CreateSubscription(r.Context(), sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// handleListSubscriptions returns subscriptions, optionally filtered by source_id.
func (s *Server) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	sourceIDStr := r.URL.Query().Get("source_id")

	if sourceIDStr != "" {
		sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid source_id")
			return
		}

		subs, err := s.store.ListSubscriptions(r.Context(), sourceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list subscriptions")
			return
		}

		writeJSON(w, http.StatusOK, subs)
		return
	}

	// No source_id filter: list all by iterating sources.
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	var allSubs []*store.Subscription
	for _, src := range sources {
		subs, err := s.store.ListSubscriptions(r.Context(), src.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list subscriptions")
			return
		}
		allSubs = append(allSubs, subs...)
	}

	if allSubs == nil {
		allSubs = []*store.Subscription{}
	}

	writeJSON(w, http.StatusOK, allSubs)
}

// handleGetSubscription returns a single subscription by ID.
func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sub, err := s.store.GetSubscription(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}

	writeJSON(w, http.StatusOK, sub)
}

// handleUpdateSubscription updates an existing subscription.
func (s *Server) handleUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetSubscription(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}

	var req subscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.SourceID != 0 {
		existing.SourceID = req.SourceID
	}
	if req.EventFilter != nil {
		existing.EventFilter = req.EventFilter
	}
	if req.TargetURL != "" {
		existing.TargetURL = req.TargetURL
	}
	if req.SigningSecret != "" {
		existing.SigningSecret = req.SigningSecret
	}
	if req.CustomHeaders != nil {
		existing.CustomHeaders = req.CustomHeaders
	}
	if req.Transform != nil {
		existing.Transform = req.Transform
	}
	if req.MaxRetries != nil {
		existing.MaxRetries = *req.MaxRetries
	}
	if req.TimeoutSeconds != nil {
		existing.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.RateLimitRPS != nil {
		existing.RateLimitRPS = *req.RateLimitRPS
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}

	if err := s.store.UpdateSubscription(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// handleDeleteSubscription soft-deletes a subscription by setting active=false.
func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetSubscription(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}

	existing.Active = false
	if err := s.store.UpdateSubscription(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate subscription")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

// handleGetSubscriptionStats returns delivery statistics for a subscription.
func (s *Server) handleGetSubscriptionStats(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify subscription exists.
	if _, err := s.store.GetSubscription(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}

	stats, err := s.store.GetSubscriptionStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get subscription stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
