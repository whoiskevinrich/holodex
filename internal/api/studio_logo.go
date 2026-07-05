package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/studioimage"
)

// Self-hosted studio logo (HOLODEX-130, ADR-056). The studio `logo` field stays a
// resolver field; downstream of resolution Holodex keeps an on-disk, normalized copy
// of whatever URL it currently RESOLVES to and serves that copy from its own origin —
// so viewers never hotlink the provider CDN. RelinkStudioLogo is the sole writer of
// the studio_logos cache (the derived-cache twin of RelinkVideoStudios, ADR-053),
// re-synced on every trigger that can move the resolved logo.

// setStudioLogoURL fills LogoURL from the cached logo version (ADR-056), pointing at
// the served route on our own origin. Zero version → empty (the SPA renders the
// monogram). Mirrors setThumbnailURL.
func setStudioLogoURL(s *model.Studio) {
	if s == nil || s.LogoVersion == 0 {
		return
	}
	s.LogoURL = fmt.Sprintf("/api/v1/studios/%d/logo?v=%d", s.ID, s.LogoVersion)
}

// serveStudioLogo streams a studio's on-disk logo JPEG with a long immutable cache
// (ADR-056). The ?v={id} the model emits changes when the logo is replaced, so a
// stale image is never pinned. No placeholder: an absent logo is 404 and the SPA
// falls back to the F38 monogram. Public read, like every other studio read.
func (h *Handlers) serveStudioLogo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.studioLogoDir == "" {
		writeError(w, http.StatusNotFound, "logo not available")
		return
	}
	logo, err := h.repo.GetStudioLogo(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no logo")
			return
		}
		h.fail(w, "get studio logo", err)
		return
	}
	f, err := os.Open(studioimage.ImagePath(h.studioLogoDir, id, logo.ID))
	if err != nil {
		writeError(w, http.StatusNotFound, "logo not available")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "logo not available")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// relinkStudioLogo re-syncs a studio's logo cache, best-effort: a failure is logged,
// never failing the user action that triggered it (the enrich/decision write already
// committed; the cache self-heals on the next trigger or the boot backfill). Mirrors
// relinkStudios.
func (h *Handlers) relinkStudioLogo(ctx context.Context, studioID int64) {
	if err := h.RelinkStudioLogo(ctx, studioID); err != nil {
		h.log.Warn("relink studio logo", "studio", studioID, "err", err)
	}
}

// relinkStudioLogoIfLogo re-syncs only when the mutated field is `logo` — so a
// description/country/website decision doesn't pay for a resolve+maybe-fetch. Mirrors
// relinkIfStudio.
func (h *Handlers) relinkStudioLogoIfLogo(ctx context.Context, studioID int64, canonical string) {
	if strings.EqualFold(strings.TrimSpace(canonical), model.StudioLogoField) {
		h.relinkStudioLogo(ctx, studioID)
	}
}

// RelinkStudioLogo makes the studio_logos cache match the studio's RESOLVED `logo`
// field (ADR-056). It is the single entry point behind every logo trigger (enrich
// apply/clear, logo decision set/clear, boot backfill). Idempotent and safe to call
// redundantly: it skips the download when the cached source_url already matches. Only
// PROVIDER-sourced logos are cached — a record (blank-pin) or manual logo has no
// provider allowlist to fetch through, so it clears the cache (the monogram shows).
func (h *Handlers) RelinkStudioLogo(ctx context.Context, studioID int64) error {
	if h.studioLogoDir == "" || h.enrich == nil {
		return nil // storage or enrichment not wired → nothing to cache
	}
	s, err := h.repo.GetStudio(ctx, studioID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return h.clearStudioLogo(ctx, studioID) // studio gone → drop cache + file
		}
		return err
	}

	url, provider := h.resolvedStudioLogo(ctx, studioID, s)
	if url == "" {
		return h.clearStudioLogo(ctx, studioID) // blank-pin / no provider logo → hide
	}

	// Idempotency: skip the fetch when the cache already tracks this exact URL.
	existing, err := h.repo.GetStudioLogo(ctx, studioID)
	switch {
	case err == nil && existing.SourceURL == url:
		return nil
	case err != nil && !errors.Is(err, repo.ErrNotFound):
		return err
	}

	raw, err := h.enrich.FetchAsset(ctx, provider, url)
	if err != nil {
		return fmt.Errorf("fetch studio logo: %w", err)
	}
	norm, iw, ih, err := personimage.Normalize(raw, h.studioLogoMaxDim)
	if err != nil {
		return fmt.Errorf("normalize studio logo: %w", err)
	}
	newID, err := h.repo.ReplaceStudioLogo(ctx, repo.StudioLogoInsert{
		StudioID: studioID, SourceURL: url, Provider: provider,
		Width: iw, Height: ih, ByteSize: len(norm),
	})
	if err != nil {
		return err
	}
	if err := studioimage.Store(h.studioLogoDir, studioID, newID, norm); err != nil {
		// Roll back the row so the index never points at a file that isn't there.
		_ = h.repo.DeleteStudioLogo(ctx, studioID)
		return fmt.Errorf("store studio logo: %w", err)
	}
	// Remove the superseded file (best-effort; a left-behind file is harmless).
	if existing.ID != 0 && existing.ID != newID {
		_ = studioimage.Remove(h.studioLogoDir, studioID, existing.ID)
	}
	return nil
}

// clearStudioLogo removes a studio's logo cache row and file (best-effort on the file).
func (h *Handlers) clearStudioLogo(ctx context.Context, studioID int64) error {
	existing, err := h.repo.GetStudioLogo(ctx, studioID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil // nothing cached
	}
	if err != nil {
		return err
	}
	if err := h.repo.DeleteStudioLogo(ctx, studioID); err != nil {
		return err
	}
	_ = studioimage.Remove(h.studioLogoDir, studioID, existing.ID)
	return nil
}

// resolvedStudioLogo returns the studio's resolved logo URL and the winning provider,
// or ("","") when there is no cacheable logo. Only a provider-sourced logo is
// cacheable: a record/file baseline (there is none for logo) or a manual literal has
// no provider allowlist to fetch through — fetching an arbitrary owner-typed URL
// server-side would be an SSRF vector — so those clear the cache instead.
func (h *Handlers) resolvedStudioLogo(ctx context.Context, studioID int64, s *model.Studio) (url, provider string) {
	for _, f := range h.resolveStudio(ctx, studioID, s) {
		if !strings.EqualFold(f.Canonical, model.StudioLogoField) {
			continue
		}
		if len(f.Values) == 0 || strings.TrimSpace(f.Values[0]) == "" {
			return "", ""
		}
		ns := strings.SplitN(f.WinningSource, ":", 2)[0]
		switch ns {
		case "", "record", "file", "manual":
			return "", "" // not provider-sourced → no safe fetch path
		default:
			return strings.TrimSpace(f.Values[0]), ns
		}
	}
	return "", ""
}
