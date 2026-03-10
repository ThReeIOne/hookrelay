package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/hookrelay/hookrelay/internal/store"
)

// handleListEvents returns a paginated, filtered list of events.
// Supports query params: source, event_type, start, end, page, page_size.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	filter := store.EventFilter{
		SourceName: r.URL.Query().Get("source"),
		EventType:  r.URL.Query().Get("event_type"),
		Page:       page,
		PageSize:   pageSize,
	}

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start time, expected RFC3339")
			return
		}
		filter.Start = &t
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end time, expected RFC3339")
			return
		}
		filter.End = &t
	}

	events, total, err := s.store.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events":    events,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// handleGetEvent returns a single event by ID.
func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event, err := s.store.GetEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	writeJSON(w, http.StatusOK, event)
}

// handleGetEventDeliveries returns all deliveries for a given event.
func (s *Server) handleGetEventDeliveries(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify event exists.
	if _, err := s.store.GetEvent(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	deliveries, err := s.store.ListDeliveriesByEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deliveries")
		return
	}

	writeJSON(w, http.StatusOK, deliveries)
}

// replayRequest is the optional JSON body for replaying an event.
type replayRequest struct {
	SubscriptionIDs []int64 `json:"subscription_ids"`
}

// handleReplayEvent re-creates delivery records for an existing event.
// If subscription_ids are provided in the body, only those subscriptions receive
// the replay. Otherwise, all active subscriptions for the event's source are used.
func (s *Server) handleReplayEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event, err := s.store.GetEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	// Parse optional body.
	var req replayRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	var subs []*store.Subscription

	if len(req.SubscriptionIDs) > 0 {
		// Use the specified subscriptions.
		for _, subID := range req.SubscriptionIDs {
			sub, err := s.store.GetSubscription(r.Context(), subID)
			if err != nil {
				writeError(w, http.StatusNotFound, "subscription not found")
				return
			}
			subs = append(subs, sub)
		}
	} else {
		// Use all active subscriptions for the event's source.
		activeSubs, err := s.store.ListActiveSubscriptions(r.Context(), event.SourceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list active subscriptions")
			return
		}
		subs = activeSubs
	}

	// Create delivery records.
	created := 0
	for _, sub := range subs {
		_, err := s.store.CreateDelivery(r.Context(), &store.Delivery{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			Status:         store.StatusPending,
			MaxRetries:     sub.MaxRetries,
			NextAttemptAt:  nil, // nil = deliver immediately
		})
		if err != nil {
			slog.Error("replay: failed to create delivery",
				"event_id", event.ID,
				"sub_id", sub.ID,
				"error", err,
			)
			continue
		}
		created++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "replayed",
		"event_id":   event.ID,
		"deliveries": created,
	})
}
