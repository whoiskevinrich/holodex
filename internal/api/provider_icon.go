package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/personimage"
	"holodex/internal/providericon"
	"holodex/internal/repo"
)

// Self-hosted provider brand icon (HOLODEX-134, ADR-059). A metadata provider may
// advertise a `brand_icon` in its /describe manifest; downstream Holodex keeps an
// on-disk, normalized copy and serves it from its own origin, so the SPA can render a
// provider glyph in place of the repeated "from <provider>" provenance text without
// hotlinking the provider's CDN. RelinkProviderIcon is the sole writer of the
// provider_icons cache — the per-provider analogue of RelinkStudioLogo (ADR-057) — but
// triggered at boot and on config-reload rather than per enrich, because a brand mark
// is static per provider.

// providerInfo is the SPA's view of an enabled provider: its name, entity types, and —
// when a brand icon is cached — the served icon URL. IconURL is omitted (SPA renders a
// monogram) when no icon is cached. Shared by the owner /enrich/sources list and the
// public /providers directory.
type providerInfo struct {
	Name        string   `json:"name"`
	EntityTypes []string `json:"entity_types"`
	IconURL     string   `json:"icon_url,omitempty"`
}

// providerIconURL is the served route for a provider's cached icon, cache-busted by the
// provider_icons row id. The name is path-escaped defensively; provider ids are
// lowercase registry strings, but the escape keeps a stray character from breaking out
// of the path segment.
func providerIconURL(name string, id int64) string {
	return fmt.Sprintf("/api/v1/providers/%s/icon?v=%d", url.PathEscape(name), id)
}

// providerInfos lists the enabled providers with their icon URLs (ADR-059), the shared
// body of both the owner /enrich/sources response and the public /providers directory.
// A missing icon table read degrades to no icon URLs (monograms), never an error.
func (h *Handlers) providerInfos(ctx context.Context) []providerInfo {
	if h.enrich == nil {
		return []providerInfo{}
	}
	srcs := h.enrich.Sources()
	icons := map[string]int64{}
	if h.providerIconDir != "" {
		if rows, err := h.repo.ListProviderIcons(ctx); err != nil {
			h.log.Warn("provider icons: list for directory", "err", err)
		} else {
			for _, row := range rows {
				icons[row.Provider] = row.ID
			}
		}
	}
	out := make([]providerInfo, 0, len(srcs))
	for _, s := range srcs {
		pi := providerInfo{Name: s.Name, EntityTypes: s.EntityTypes}
		if id, ok := icons[s.Name]; ok {
			pi.IconURL = providerIconURL(s.Name, id)
		}
		out = append(out, pi)
	}
	return out
}

// listProviders is the PUBLIC provider directory (ADR-059 §5): the visitor read path
// for provenance badges + the website label, which render for everyone but cannot reach
// the owner-gated /enrich/sources. It exposes only provider name / entity types / icon
// URL — provider names are already visitor-visible via the provenance line, so this
// leaks no identity the page doesn't already show. No secrets, no base_url, no config.
func (h *Handlers) listProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": h.providerInfos(r.Context())})
}

// serveProviderIcon streams a provider's on-disk brand icon JPEG with a long immutable
// cache (ADR-059). The ?v={id} the directory emits changes when the icon is replaced,
// so a stale image is never pinned. No placeholder: an absent icon is 404 and the SPA
// falls back to a monogram. Public read, like the studio logo and thumbnails. The
// {name} selects a row; the on-disk path is built from the row id, never the name.
func (h *Handlers) serveProviderIcon(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" || h.providerIconDir == "" {
		writeError(w, http.StatusNotFound, "icon not available")
		return
	}
	icon, err := h.repo.GetProviderIcon(r.Context(), name)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no icon")
			return
		}
		h.fail(w, "get provider icon", err)
		return
	}
	f, err := os.Open(providericon.ImagePath(h.providerIconDir, icon.ID))
	if err != nil {
		writeError(w, http.StatusNotFound, "icon not available")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "icon not available")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// RefreshProviderIcons re-syncs every enabled provider's brand icon from its /describe
// and prunes icons for providers no longer enabled (ADR-059). It is the boot backfill +
// config-reload entry point, best-effort per provider — a failed describe/fetch/store is
// logged and skipped, never failing boot or the reload. Runs off the request path
// (spawn it in a goroutine); provider icons are static, so this is the only refresh
// point besides a fresh install's first boot.
func (h *Handlers) RefreshProviderIcons(ctx context.Context) {
	if h.providerIconDir == "" || h.enrich == nil {
		return
	}
	enabled := map[string]struct{}{}
	for _, s := range h.enrich.Sources() {
		enabled[s.Name] = struct{}{}
		if err := h.RelinkProviderIcon(ctx, s.Name); err != nil {
			h.log.Warn("relink provider icon", "provider", s.Name, "err", err)
		}
	}
	// Prune orphans: a provider removed from the registry keeps no cached icon (there is
	// no FK cascade — provider_icons is keyed by name, ADR-059 §2).
	rows, err := h.repo.ListProviderIcons(ctx)
	if err != nil {
		h.log.Warn("provider icon reconcile: list", "err", err)
		return
	}
	for _, row := range rows {
		if _, ok := enabled[row.Provider]; ok {
			continue
		}
		if err := h.repo.DeleteProviderIcon(ctx, row.Provider); err != nil {
			h.log.Warn("provider icon reconcile: delete", "provider", row.Provider, "err", err)
			continue
		}
		_ = providericon.Remove(h.providerIconDir, row.ID)
	}
}

// RelinkProviderIcon makes the provider_icons cache match the provider's advertised
// /describe `brand_icon` (ADR-059). Idempotent and safe to call redundantly: it reads
// the manifest, and skips the download when the cached source_url already matches the
// advertised URL. An absent/empty brand_icon clears the cache (the SPA shows a
// monogram). Errors bubble to the best-effort caller (RefreshProviderIcons), which logs
// and moves on.
func (h *Handlers) RelinkProviderIcon(ctx context.Context, provider string) error {
	if h.providerIconDir == "" || h.enrich == nil {
		return nil // storage or enrichment not wired → nothing to cache
	}
	m, err := h.enrich.DescribeProvider(ctx, provider)
	if err != nil {
		return fmt.Errorf("describe provider %q: %w", provider, err)
	}
	iconURL := ""
	if m.BrandIcon != nil {
		iconURL = strings.TrimSpace(m.BrandIcon.URL)
	}
	if iconURL == "" {
		return h.clearProviderIcon(ctx, provider) // provider advertises none → hide
	}

	// Idempotency: skip the fetch when the cache already tracks this exact URL.
	existing, err := h.repo.GetProviderIcon(ctx, provider)
	switch {
	case err == nil && existing.SourceURL == iconURL:
		return nil
	case err != nil && !errors.Is(err, repo.ErrNotFound):
		return err
	}

	raw, err := h.enrich.FetchAsset(ctx, provider, iconURL)
	if err != nil {
		return fmt.Errorf("fetch provider icon: %w", err)
	}
	norm, iw, ih, err := personimage.Normalize(raw, h.providerIconMaxDim)
	if err != nil {
		return fmt.Errorf("normalize provider icon: %w", err)
	}
	newID, err := h.repo.ReplaceProviderIcon(ctx, repo.ProviderIconInsert{
		Provider: provider, SourceURL: iconURL, Width: iw, Height: ih, ByteSize: len(norm),
	})
	if err != nil {
		return err
	}
	if err := providericon.Store(h.providerIconDir, newID, norm); err != nil {
		// Roll back the row so the index never points at a file that isn't there.
		_ = h.repo.DeleteProviderIcon(ctx, provider)
		return fmt.Errorf("store provider icon: %w", err)
	}
	// Remove the superseded file (best-effort; a left-behind file is harmless).
	if existing.ID != 0 && existing.ID != newID {
		_ = providericon.Remove(h.providerIconDir, existing.ID)
	}
	return nil
}

// clearProviderIcon removes a provider's icon cache row and file (best-effort on the
// file). Idempotent — clearing an absent icon is a no-op success.
func (h *Handlers) clearProviderIcon(ctx context.Context, provider string) error {
	existing, err := h.repo.GetProviderIcon(ctx, provider)
	if errors.Is(err, repo.ErrNotFound) {
		return nil // nothing cached
	}
	if err != nil {
		return err
	}
	if err := h.repo.DeleteProviderIcon(ctx, provider); err != nil {
		return err
	}
	_ = providericon.Remove(h.providerIconDir, existing.ID)
	return nil
}
