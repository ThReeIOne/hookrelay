package api

import (
	"net/http"
	"time"
)

// handleOverviewStats returns dashboard-level aggregate statistics.
func (s *Server) handleOverviewStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetOverviewStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get overview stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleThroughput returns time-series throughput data.
// Supports query params: start, end (RFC3339), granularity (default "1h").
func (s *Server) handleThroughput(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	// Default: last 24 hours.
	start := now.Add(-24 * time.Hour)
	end := now

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start time, expected RFC3339")
			return
		}
		start = t
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end time, expected RFC3339")
			return
		}
		end = t
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "1h"
	}

	points, err := s.store.GetThroughput(r.Context(), start, end, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get throughput data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"start":       start,
		"end":         end,
		"granularity": granularity,
		"data":        points,
	})
}
