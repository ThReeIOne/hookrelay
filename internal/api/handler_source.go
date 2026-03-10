package api

import (
	"encoding/json"
	"net/http"

	"github.com/hookrelay/hookrelay/internal/store"
)

// sourceRequest is the JSON body for creating or updating a source.
type sourceRequest struct {
	Name              string         `json:"name"`
	VerifyType        string         `json:"verify_type"`
	VerifyConfig      map[string]any `json:"verify_config"`
	EventTypePath     string         `json:"event_type_path"`
	EventTypeHeader   string         `json:"event_type_header"`
	IdempotencyPath   string         `json:"idempotency_path"`
	IdempotencyHeader string         `json:"idempotency_header"`
	Description       string         `json:"description"`
}

// handleCreateSource creates a new webhook source.
func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req sourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	src := &store.Source{
		Name:              req.Name,
		VerifyType:        req.VerifyType,
		VerifyConfig:      req.VerifyConfig,
		EventTypePath:     req.EventTypePath,
		EventTypeHeader:   req.EventTypeHeader,
		IdempotencyPath:   req.IdempotencyPath,
		IdempotencyHeader: req.IdempotencyHeader,
		Description:       req.Description,
		Active:            true,
	}

	created, err := s.store.CreateSource(r.Context(), src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// handleListSources returns all sources.
func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	writeJSON(w, http.StatusOK, sources)
}

// handleGetSource returns a single source by ID.
func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	src, err := s.store.GetSource(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	writeJSON(w, http.StatusOK, src)
}

// handleUpdateSource updates an existing source.
func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetSource(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	var req sourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.VerifyType != "" {
		existing.VerifyType = req.VerifyType
	}
	if req.VerifyConfig != nil {
		existing.VerifyConfig = req.VerifyConfig
	}
	if req.EventTypePath != "" {
		existing.EventTypePath = req.EventTypePath
	}
	if req.EventTypeHeader != "" {
		existing.EventTypeHeader = req.EventTypeHeader
	}
	if req.IdempotencyPath != "" {
		existing.IdempotencyPath = req.IdempotencyPath
	}
	if req.IdempotencyHeader != "" {
		existing.IdempotencyHeader = req.IdempotencyHeader
	}
	if req.Description != "" {
		existing.Description = req.Description
	}

	if err := s.store.UpdateSource(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// handleDeleteSource soft-deletes a source by setting active=false.
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetSource(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	existing.Active = false
	if err := s.store.UpdateSource(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate source")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}
