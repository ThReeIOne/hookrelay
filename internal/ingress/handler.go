package ingress

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hookrelay/hookrelay/internal/metrics"
	"github.com/hookrelay/hookrelay/internal/router"
	"github.com/hookrelay/hookrelay/internal/store"
	"github.com/hookrelay/hookrelay/internal/verify"
)

// Handler handles inbound webhook events.
type Handler struct {
	store  store.Store
	router *router.Router
}

// NewHandler creates a new ingress handler.
func NewHandler(s store.Store, r *router.Router) *Handler {
	return &Handler{store: s, router: r}
}

// ServeHTTP handles POST /ingest/{sourceName}.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	sourceName := chi.URLParam(r, "sourceName")
	if sourceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing source name"})
		return
	}

	// 1. Find source
	source, err := h.store.GetSourceByName(r.Context(), sourceName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}
	if !source.Active {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "source is disabled"})
		return
	}

	// 2. Read raw body (1MB limit)
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	// 3. Verify signature
	headers := flattenHeaders(r.Header)
	v, err := verify.GetVerifier(source.VerifyType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unknown verify type"})
		return
	}
	if err := v.Verify(source.VerifyConfig, headers, rawBody); err != nil {
		metrics.EventsRejectedTotal.WithLabelValues("verify_failed").Inc()
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "signature verification failed"})
		return
	}

	// 4. Parse payload
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	// 5. Extract event type
	eventType := extractEventType(source, headers, payload)

	// 6. Extract idempotency key + dedup
	idempotencyKey := extractIdempotencyKey(source, headers, payload)
	var idempotencyKeyPtr *string
	if idempotencyKey != "" {
		idempotencyKeyPtr = &idempotencyKey
		existing, _ := h.store.FindEventByIdempotencyKey(r.Context(), source.ID, idempotencyKey)
		if existing != nil {
			metrics.EventsRejectedTotal.WithLabelValues("duplicate").Inc()
			writeJSON(w, http.StatusOK, map[string]any{
				"status":   "duplicate",
				"event_id": existing.ID,
			})
			return
		}
	}

	// 7. Store event
	event, err := h.store.CreateEvent(r.Context(), &store.Event{
		SourceID:       source.ID,
		SourceName:     source.Name,
		EventType:      eventType,
		IdempotencyKey: idempotencyKeyPtr,
		Headers:        headers,
		Payload:        payload,
		RawBody:        rawBody,
		RemoteAddr:     r.RemoteAddr,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store event"})
		return
	}

	// 8. Route → create deliveries
	deliveryCount, err := h.router.Route(r.Context(), event)
	if err != nil {
		slog.Error("routing failed", "event_id", event.ID, "error", err)
	}

	metrics.EventsReceivedTotal.WithLabelValues(source.Name).Inc()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "accepted",
		"event_id":   event.ID,
		"deliveries": deliveryCount,
	})
}

func extractEventType(source *store.Source, headers map[string]string, payload map[string]any) string {
	// Prefer header
	if source.EventTypeHeader != "" {
		if v, ok := headers[strings.ToLower(source.EventTypeHeader)]; ok && v != "" {
			return v
		}
	}
	// Fallback to payload path
	if source.EventTypePath != "" {
		if v := jsonPathGet(payload, source.EventTypePath); v != "" {
			return v
		}
	}
	return "unknown"
}

func extractIdempotencyKey(source *store.Source, headers map[string]string, payload map[string]any) string {
	// Prefer header
	if source.IdempotencyHeader != "" {
		if v, ok := headers[strings.ToLower(source.IdempotencyHeader)]; ok && v != "" {
			return v
		}
	}
	// Fallback to payload path
	if source.IdempotencyPath != "" {
		return jsonPathGet(payload, source.IdempotencyPath)
	}
	return ""
}

// flattenHeaders converts multi-value HTTP headers to single-value map (lowercase keys).
func flattenHeaders(h http.Header) map[string]string {
	flat := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			flat[strings.ToLower(k)] = v[0]
		}
	}
	return flat
}

// jsonPathGet extracts a value from a nested map using dot-separated path.
func jsonPathGet(data map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return v
	case float64:
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				strings.Replace(
					json.Number(strings.TrimRight(strings.TrimRight(
						func() string { b, _ := json.Marshal(v); return string(b) }(),
						"0"), ".")).String(),
					"e+", "e", 1),
				"E+", "E", 1),
			"0"), ".")
	default:
		if current != nil {
			b, err := json.Marshal(current)
			if err == nil {
				return string(b)
			}
		}
		return ""
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
