package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

func (h *handler) describe(w http.ResponseWriter, r *http.Request) {
	resp := describeResponse{
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
			// non-canonical person field, rendered first-class via a field hint (F39).
			"known_for_department",
		},
		AssetKinds: []string{"photo"},
		// F39 (Holodex contract §4.7): presentation hints for our non-canonical keys,
		// so they render labeled and ordered with no per-operator config. Canonical
		// keys are omitted here — Holodex's registry owns those.
		FieldHints: map[string]fieldHint{
			"known_for_department": {Label: "Known for", Render: "text", Group: "attributes", Order: 10},
		},
	}
	// Advertise the bundled TMDB brand mark (HOLODEX-161), served by this sidecar at
	// /brand-icon.png. Its host is the one Holodex used to reach /describe (the request
	// Host) — i.e. the provider's own base host, always on Holodex's asset allowlist —
	// so it self-hosts with no operator config. An operator can override it with a
	// different raster on an allowlisted host via TMDB_BRAND_ICON_URL (e.g. a CDN copy).
	if u := strings.TrimSpace(os.Getenv("TMDB_BRAND_ICON_URL")); u != "" {
		resp.BrandIcon = &iconRef{URL: u}
	} else if r.Host != "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		resp.BrandIcon = &iconRef{URL: scheme + "://" + r.Host + "/brand-icon.png"}
	}
	writeJSON(w, http.StatusOK, resp)
}

// brandIcon serves the bundled TMDB brand mark (HOLODEX-161) that /describe advertises.
// A short cache is fine — Holodex downloads + self-hosts it once (ADR-059), so this is
// hit rarely (on describe-time relink), not on every page view.
func (h *handler) brandIcon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(brandIconPNG)
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
