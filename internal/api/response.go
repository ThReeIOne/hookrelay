package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parsePagination extracts page and pageSize from query parameters.
// Defaults: page=1, pageSize=20, max pageSize=100.
func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20

	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}

	if v := r.URL.Query().Get("page_size"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize
}

// parseID extracts a chi URL parameter and parses it as int64.
func parseID(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
	if raw == "" {
		return 0, fmt.Errorf("missing parameter: %s", param)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid parameter %s: %w", param, err)
	}
	return id, nil
}
