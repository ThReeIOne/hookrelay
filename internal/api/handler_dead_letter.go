package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hookrelay/hookrelay/internal/store"
)

// handleListDeadLetters returns a paginated list of dead-letter deliveries.
// Supports query params: subscription_id, page, page_size.
func (s *Server) handleListDeadLetters(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	filter := store.DeliveryFilter{
		Page:     page,
		PageSize: pageSize,
	}

	// Dead letters always have status = StatusDeadLetter.
	deadLetterStatus := store.StatusDeadLetter
	filter.Status = &deadLetterStatus

	if subIDStr := r.URL.Query().Get("subscription_id"); subIDStr != "" {
		subID, err := strconv.ParseInt(subIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid subscription_id")
			return
		}
		filter.SubscriptionID = subID
	}

	deliveries, total, err := s.store.ListDeadLetters(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list dead letters")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dead_letters": deliveries,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// handleRetryDeadLetter resets a single dead-letter delivery for reprocessing.
func (s *Server) handleRetryDeadLetter(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.RetryDeadLetter(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retry dead letter")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
}

// batchRetryRequest is the JSON body for batch-retrying dead letters.
type batchRetryRequest struct {
	DeliveryIDs []int64 `json:"delivery_ids"`
}

// handleBatchRetryDeadLetters resets multiple dead-letter deliveries for reprocessing.
func (s *Server) handleBatchRetryDeadLetters(w http.ResponseWriter, r *http.Request) {
	var req batchRetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.DeliveryIDs) == 0 {
		writeError(w, http.StatusBadRequest, "delivery_ids is required and must not be empty")
		return
	}

	if err := s.store.BatchRetryDeadLetters(r.Context(), req.DeliveryIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to batch retry dead letters")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "retrying",
		"count":  len(req.DeliveryIDs),
	})
}
