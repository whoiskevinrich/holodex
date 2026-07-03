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
		EntityTypes:     []string{"person", "video", "studio"},
		IDNamespaces:    []string{"tmdb", "imdb"},
		Fields: []string{
			// person fields
			"bio", "birthdate", "nationality", "deathdate", "website", "aliases",
			// video/film fields
			"title", "overview", "release_date", "runtime", "genres", "tagline", "homepage",
			"original_language", "original_title", "status", "imdb_id", "poster_url",
			"actors", "director", "studio",
			// studio-entity fields (F38 S3). logo is an image_url field (the video
			// poster_url pattern), not a downloaded asset — the F25 image store is not
			// generalized to studios here (spec Non-Goal / P2-3). website is shared.
			"description", "country", "logo",
		},
		AssetKinds: []string{"photo"},
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
	if !isSupportedEntity(req.EntityType) {
		writeJSON(w, http.StatusOK, map[string]any{"candidates": []any{}})
		return
	}
	candidates, err := h.tmdb.resolve(r.Context(), req.Hint, req.EntityType)
	if err != nil {
		h.log.Warn("resolve failed", "entity_type", req.EntityType, "err", err)
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
	if !isSupportedEntity(req.EntityType) {
		http.Error(w, "unsupported entity type", http.StatusBadRequest)
		return
	}
	result, err := h.tmdb.enrich(r.Context(), req.ExternalID, req.EntityType)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Warn("enrich failed", "entity_type", req.EntityType, "external_id", req.ExternalID, "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// isSupportedEntity reports whether this provider handles the entity type. Keep in
// sync with describe's EntityTypes and tmdbClient.resolve/enrich dispatch.
func isSupportedEntity(entityType string) bool {
	switch entityType {
	case "person", "video", "studio":
		return true
	}
	return false
}

func decode(body io.ReadCloser, v any) error {
	return json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
