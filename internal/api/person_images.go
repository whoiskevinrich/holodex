package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
)

// Person images (F25, ADR-038). Public reads serve the on-disk JPEG for a filled
// role or a themed placeholder SVG for an empty one; owner-gated mutations
// upload/delete/promote/reorder. A role from a request is always validated against
// the enum (model.ValidPersonImageRole) — a filesystem path is only ever built from
// the server-assigned integer id, never a request value.

// mountPersonImages registers the owner-gated image mutations (ADR-038 F25). The
// public reads are mounted ungated in Mount; only the controls are gated.
func (h *Handlers) mountPersonImages(r chi.Router) {
	r.Post("/people/{id}/image", h.uploadPersonImage)
	r.Delete("/people/{id}/images/{imageId}", h.deletePersonImage)
	r.Post("/people/{id}/images/{imageId}/promote", h.promotePersonImage)
	r.Post("/people/{id}/images/reorder", h.reorderPersonImages)
}

// servePersonImageByRole serves a person's image for a core role (ADR-038 F25).
// A filled role streams the on-disk JPEG with a long immutable cache; an empty role
// (or any role with no stored image) falls back to the themed placeholder SVG built
// from the person's enriched gender. Unknown role → 400; unknown person → 404.
func (h *Handlers) servePersonImageByRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role := chi.URLParam(r, "role")
	if !model.ValidPersonImageRole(role) {
		writeError(w, http.StatusBadRequest, "invalid image role")
		return
	}
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	// The gallery has no single "role" image; only core roles fill a slot.
	if model.CorePersonImageRole(role) {
		img, err := h.repo.CorePersonImage(r.Context(), id, role)
		switch {
		case err == nil:
			h.servePersonImageFile(w, r, id, img.ID)
			return
		case !errors.Is(err, repo.ErrNotFound):
			h.fail(w, "core person image", err)
			return
		}
	}
	// Empty slot → themed placeholder.
	h.servePlaceholder(w, r, id, role)
}

// servePersonImageByID serves one gallery (or any) image of a person by its id
// (ADR-038 F25). 404 when the image does not belong to the person. There is no
// placeholder fallback for an id fetch — a missing id is genuinely not found.
func (h *Handlers) servePersonImageByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	imageID, ok := parseImageID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.GetPersonImage(r.Context(), id, imageID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}
		h.fail(w, "get person image", err)
		return
	}
	h.servePersonImageFile(w, r, id, imageID)
}

// servePersonImageFile streams the on-disk JPEG for a (person, image) pair with an
// immutable long-lived cache: the URL carries the image id (and the caller appends
// ?v=), and a replaced core slot gets a NEW id, so a stale image is never pinned.
// A missing file is a 404 (the row outlived its bytes — shouldn't happen, but fail
// closed rather than serve garbage).
func (h *Handlers) servePersonImageFile(w http.ResponseWriter, r *http.Request, personID, imageID int64) {
	if h.personImageDir == "" {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	f, err := os.Open(personimage.ImagePath(h.personImageDir, personID, imageID))
	if err != nil {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// servePlaceholder writes the themed placeholder SVG for an empty role. The skin
// comes from ?skin= (defaulting to the app default); the gender bucket from the
// person's enriched "gender" field (neutral when absent). It is not cached
// aggressively — a later upload fills the slot and the role URL then resolves to the
// real file — so a short cache keeps the empty state cheap without pinning it.
func (h *Handlers) servePlaceholder(w http.ResponseWriter, r *http.Request, personID int64, role string) {
	skin := strings.TrimSpace(r.URL.Query().Get("skin"))
	if skin == "" {
		skin = h.defaultSkin
	}
	svg := personimage.Placeholder(skin, role, h.personGender(r, personID))
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(svg)
}

// personGender reads the person's enriched "gender" field (if any provider supplied
// one) for the placeholder silhouette. Empty when no enrichment is wired or stored —
// the placeholder resolver maps that to the neutral bucket.
func (h *Handlers) personGender(r *http.Request, personID int64) string {
	if h.enrich == nil {
		return ""
	}
	fields, err := h.enrich.Fields(r.Context(), model.EnrichEntityPerson, personID)
	if err != nil {
		// A real lookup error (vs. "no gender stored") still fails soft to the neutral
		// placeholder, but log it so a systemic enrichment-store fault is traceable
		// rather than every placeholder silently going neutral.
		h.log.Warn("placeholder gender lookup failed", "person", personID, "err", err)
		return ""
	}
	for _, f := range fields {
		if strings.EqualFold(f.Canonical, "gender") && len(f.Values) > 0 {
			return f.Values[0]
		}
	}
	return ""
}

// uploadPersonImage ingests a multipart upload (`image` file + `role` field),
// normalizes the bytes (metadata strip + bomb guard), stores them on disk, and
// records the row (ADR-038 F25). 400 on a bad role / missing field / undecodable
// image; 409 when the gallery is full; 201 with {id, version} on success.
func (h *Handlers) uploadPersonImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.personImageDir == "" {
		writeError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return
	}
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}

	// Cap the whole request body before parsing so a hostile upload can't exhaust
	// memory/disk; ParseMultipartForm buffers small parts in memory and spills the
	// rest to temp files, all bounded by this reader.
	r.Body = http.MaxBytesReader(w, r.Body, h.personImageMaxBytes)
	if err := r.ParseMultipartForm(h.personImageMaxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or oversized upload")
		return
	}

	role := strings.TrimSpace(r.FormValue("role"))
	if !model.ValidPersonImageRole(role) {
		writeError(w, http.StatusBadRequest, "invalid image role")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer file.Close()
	raw, err := readAllLimited(file, h.personImageMaxBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}

	// allow_over_cap lets the owner deliberately exceed the gallery cap (F25). It is
	// honored only here, on the owner-gated upload path — enrichment never sets it.
	overCap, _ := strconv.ParseBool(r.FormValue("allow_over_cap"))
	h.storeNormalized(w, r, id, role, model.PersonImageSourceUpload, "", "", overCap, raw)
}

// storeNormalized normalizes raw bytes, inserts the row, and writes the file —
// the shared tail of upload and promote. It maps a full gallery to 409 and an
// undecodable/oversize image to 400, and emits 201 {id, version} on success.
// overCap bypasses the gallery cap (owner over-cap upload); it is ignored for core
// roles, which are never capped.
func (h *Handlers) storeNormalized(w http.ResponseWriter, r *http.Request, personID int64, role, source, provider, externalID string, overCap bool, raw []byte) {
	norm, iw, ih, err := personimage.Normalize(raw, h.personImageMaxDim)
	if err != nil {
		h.log.Warn("person image normalize failed", "person", personID, "err", err)
		writeError(w, http.StatusBadRequest, "unsupported or invalid image")
		return
	}
	imgID, err := h.repo.InsertPersonImage(r.Context(), repo.PersonImageInsert{
		PersonID: personID,
		Role:     role,
		Source:   source,
		Provider: provider,
		ExternalID: externalID,
		Width:    iw,
		Height:   ih,
		ByteSize:     len(norm),
		OverCap:  overCap,
	})
	if errors.Is(err, repo.ErrGalleryFull) {
		writeError(w, http.StatusConflict, "gallery is full")
		return
	}
	if err != nil {
		h.fail(w, "insert person image", err)
		return
	}
	if err := personimage.Store(h.personImageDir, personID, imgID, norm); err != nil {
		// Roll back the row so the index never points at a file that isn't there.
		_ = h.repo.DeletePersonImage(r.Context(), personID, imgID)
		h.fail(w, "store person image", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": imgID, "version": imgID})
}

// deletePersonImage removes one of a person's images: the row, then the file
// (best-effort — a left-behind file is harmless, the index is the source of truth).
// 404 when the image does not belong to the person.
func (h *Handlers) deletePersonImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	imageID, ok := parseImageID(w, r)
	if !ok {
		return
	}
	switch err := h.repo.DeletePersonImage(r.Context(), id, imageID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "image not found")
		return
	case err != nil:
		h.fail(w, "delete person image", err)
		return
	}
	if h.personImageDir != "" {
		_ = personimage.Remove(h.personImageDir, id, imageID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// promotePersonImage copies an existing image (typically a gallery item) into a
// core slot (ADR-038 F25): it re-reads the stored file, re-normalizes it, and
// inserts a new row with source=promoted into the target role — replacing whatever
// filled that slot. The original gallery item is left in place. Body: {role}.
func (h *Handlers) promotePersonImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.personImageDir == "" {
		writeError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return
	}
	imageID, ok := parseImageID(w, r)
	if !ok {
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	role := strings.TrimSpace(body.Role)
	if !model.CorePersonImageRole(role) {
		writeError(w, http.StatusBadRequest, "promote target must be a core role")
		return
	}
	// The source image must belong to the person (scoped lookup); the file path is
	// then built from the server-stored id, never a request value.
	if _, err := h.repo.GetPersonImage(r.Context(), id, imageID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}
		h.fail(w, "promote: get image", err)
		return
	}
	raw, err := os.ReadFile(personimage.ImagePath(h.personImageDir, id, imageID))
	if err != nil {
		h.fail(w, "promote: read image", err)
		return
	}
	// Promote always targets a core role (validated above), which is never gallery-capped.
	h.storeNormalized(w, r, id, role, model.PersonImageSourcePromoted, "", "", false, raw)
}

// reorderPersonImages sets the gallery order from a body list of image ids
// (ADR-038 F25). Ids not belonging to the person are ignored. 200 with the new
// image set so the caller can refresh without a second round-trip.
func (h *Handlers) reorderPersonImages(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Order []int64 `json:"order"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.repo.ReorderGallery(r.Context(), id, body.Order); err != nil {
		h.fail(w, "reorder gallery", err)
		return
	}
	set, err := h.repo.PersonImageSet(r.Context(), id)
	if err != nil {
		h.fail(w, "person image set", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": set})
}

// personImageSet reads a person's image read model for the (ungated) detail view —
// per-role presence + version + ordered gallery. Returns the zero set (empty roles +
// gallery) on error so the detail response is always well-formed.
func (h *Handlers) personImageSet(r *http.Request, id int64) model.PersonImageSet {
	set, err := h.repo.PersonImageSet(r.Context(), id)
	if err != nil {
		h.log.Warn("read person image set", "id", id, "err", err)
		return model.PersonImageSet{Roles: map[string]model.PersonImageSlot{}, Gallery: []model.PersonImage{}}
	}
	return set
}

// readAllLimited reads up to limit+1 bytes and errors if the source exceeds limit,
// so an over-cap multipart part is rejected rather than silently truncated. The
// request body is already MaxBytesReader-capped; this is the per-file backstop.
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("image exceeds size limit")
	}
	return data, nil
}

// parseImageID reads the {imageId} path param as a positive integer.
func parseImageID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	imageID, err := strconv.ParseInt(chi.URLParam(r, "imageId"), 10, 64)
	if err != nil || imageID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid image id")
		return 0, false
	}
	return imageID, true
}
