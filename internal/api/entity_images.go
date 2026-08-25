package api

import (
	"net/http"
	"os"

	"holodex/internal/personimage"
)

// Shared mechanics for the entity-image handlers (Person F25/ADR-038, Studio
// F51/ADR-079, Film F56/ADR-086; HOLODEX-286) — serving a stored file is
// byte-for-byte identical across all three, so servePersonImageFile also calls
// serveEntityImageFile. parseImageUpload is used only by Studio/Film: Person's
// upload path additionally handles the gallery cap, over-cap override, and
// dedup-by-hash in person_images.go, which are not part of this simple "replace
// the one row for a role" shape, so folding that one in would trade a real
// behavioral difference for a smaller diff.

// parseImageUpload parses a capped multipart image upload (the "image" file field),
// normalizes the bytes (metadata strip + bomb guard), and returns the normalized
// JPEG + dimensions. It writes the appropriate error response itself and returns
// ok=false when the request was rejected — the caller just returns on !ok.
func (h *Handlers) parseImageUpload(w http.ResponseWriter, r *http.Request, maxBytes int64, maxDim int, kind string, entityID int64, role string) (norm []byte, iw, ih int, ok bool) {
	// Cap the whole request body before parsing so a hostile upload can't exhaust
	// memory/disk (mirrors uploadPersonImage).
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or oversized upload")
		return nil, 0, 0, false
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return nil, 0, 0, false
	}
	defer file.Close()
	raw, err := readAllLimited(file, maxBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read image")
		return nil, 0, 0, false
	}
	norm, iw, ih, err = personimage.Normalize(raw, maxDim)
	if err != nil {
		h.log.Warn(kind+" image normalize failed", kind, entityID, "role", role, "err", err)
		writeError(w, http.StatusBadRequest, "unsupported or invalid image")
		return nil, 0, 0, false
	}
	return norm, iw, ih, true
}

// serveEntityImageFile streams an on-disk image JPEG with a long immutable cache,
// 404 on any failure to open or stat it. Distinct from handlers.go's
// (h *Handlers) serveImageFile, an unrelated older helper for video thumbnail/poster
// candidate-fallback serving with different cache/visibility semantics.
func serveEntityImageFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
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
