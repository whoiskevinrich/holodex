package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const providerVersion = "1.0.0"

type handler struct {
	tmdb *tmdbClient
	log  *slog.Logger
}

func newHandler(tmdb *tmdbClient, log *slog.Logger) *handler {
	return &handler{tmdb: tmdb, log: log}
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"provider": "tmdb",
		"version":  providerVersion,
	})
}

func (h *handler) describe(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, describeResponse{
		Provider:        "tmdb",
		Version:         providerVersion,
		ProtocolVersion: 1,
		EntityTypes:     []string{"person"},
		IDNamespaces:    []string{"tmdb", "imdb"},
		Fields:          []string{"bio", "birthdate", "nationality", "deathdate", "website", "aliases"},
		AssetKinds:      []string{"photo"},
	})
}

type resolveRequest struct {
	EntityType string   `json:"entity_type"`
	Hint       hintBody `json:"hint"`
}

type hintBody struct {
	Query       string   `json:"query"`
	ExternalIDs []string `json:"external_ids"`
}

func (h *handler) resolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := decode(r.Body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.EntityType != "person" {
		writeJSON(w, http.StatusOK, map[string]any{"candidates": []any{}})
		return
	}
	candidates, err := h.tmdb.resolve(r.Context(), req.Hint)
	if err != nil {
		h.log.Warn("resolve failed", "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates})
}

type enrichRequest struct {
	EntityType string `json:"entity_type"`
	ExternalID string `json:"external_id"`
}

func (h *handler) enrich(w http.ResponseWriter, r *http.Request) {
	var req enrichRequest
	if err := decode(r.Body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.EntityType != "person" {
		http.Error(w, "unsupported entity type", http.StatusBadRequest)
		return
	}
	result, err := h.tmdb.enrich(r.Context(), req.ExternalID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Warn("enrich failed", "external_id", req.ExternalID, "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decode(body io.ReadCloser, v any) error {
	return json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
