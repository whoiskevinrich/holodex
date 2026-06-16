// Package personimage is the storage + security spine for per-person images (F24,
// ADR-037): on-disk path layout, untrusted-bytes normalization, atomic writes, and
// the themed placeholder served when a role is empty.
//
// Unlike the thumbnail pipeline (ADR-009) there is no ffmpeg/exiftool and no
// background queue: Normalize is fast, synchronous, and stdlib-only, so the API
// handler normalizes inline and writes the file directly — simpler, and it avoids a
// queue whose only job would be a single re-encode. The disk write is still
// atomic-ish (temp file + rename) like the thumbnail manager so a crash mid-write
// never leaves a torn JPEG that the serving handler would hand a client.
//
// Normalize is the metadata-strip: every accepted upload is re-encoded to JPEG with
// stdlib image/jpeg, which carries no EXIF/XMP/ICC — so an image's embedded GPS,
// camera, or other PII never reaches disk, and an SVG/polyglot can't be stored
// (stdlib refuses to decode it as a raster image).
package personimage

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"

	// Register the decoders Normalize sniffs for. webp is intentionally omitted:
	// golang.org/x/image is not a project dependency, so accepted inputs are
	// jpeg/png/gif — all re-encoded to jpeg on the way in.
	_ "image/gif"
	_ "image/png"
)

// Decompression-bomb and output bounds (ADR-037 F24). These guard a single decode
// of untrusted bytes: the config is read BEFORE the full decode so a tiny file
// claiming 100000×100000 is rejected without ever allocating the pixels.
const (
	maxDimension = 12000            // reject if width or height exceeds this (any side)
	maxPixels    = 60 * 1000 * 1000 // reject if width*height exceeds ~60 MP
	jpegQuality  = 85               // re-encode quality
)

// ImagePath is the on-disk location for a person's image (ADR-037/ADR-014):
// {dir}/{personID}/{imageID}.jpg. The id is server-assigned (an integer), never a
// request value, so traversal is structurally impossible. The per-person subdir is
// NOT created here — callers that write use Store, which creates it.
func ImagePath(dir string, personID, imageID int64) string {
	return filepath.Join(dir, strconv.FormatInt(personID, 10), strconv.FormatInt(imageID, 10)+".jpg")
}

// personDir is the per-person subdir under the image root.
func personDir(dir string, personID int64) string {
	return filepath.Join(dir, strconv.FormatInt(personID, 10))
}

// Normalize sniffs, decodes, and re-encodes untrusted image bytes to a clean JPEG
// (ADR-037 F24 — the metadata strip + decompression-bomb guard). It returns the
// re-encoded bytes and the stored dimensions. Steps:
//
//  1. DecodeConfig (cheap, no pixel allocation) → reject oversize dims/area before
//     the full decode, closing the decompression-bomb vector.
//  2. Decode → reject non-images / polyglots / SVG (stdlib has no SVG decoder).
//  3. Downscale to maxOutDimension if larger (cheap nearest-neighbour; portraits are
//     small and this is not a quality-critical path).
//  4. Re-encode to JPEG, which carries none of the source's EXIF/XMP/ICC metadata.
//
// maxOutDimension<=0 means "don't downscale" (only the bomb guard applies).
func Normalize(input []byte, maxOutDimension int) (out []byte, w, h int, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, 0, 0, fmt.Errorf("image has non-positive dimensions")
	}
	if cfg.Width > maxDimension || cfg.Height > maxDimension {
		return nil, 0, 0, fmt.Errorf("image too large: %dx%d exceeds %d px per side", cfg.Width, cfg.Height, maxDimension)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, 0, 0, fmt.Errorf("image too large: %dx%d exceeds %d total pixels", cfg.Width, cfg.Height, maxPixels)
	}

	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}

	if maxOutDimension > 0 {
		img = downscale(img, maxOutDimension)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}
	b := img.Bounds()
	return buf.Bytes(), b.Dx(), b.Dy(), nil
}

// downscale shrinks img so its longest side is at most maxSide, preserving aspect
// ratio, via nearest-neighbour sampling (stdlib-only; quality is adequate for small
// portraits and avoids a golang.org/x/image dependency). Images already within
// bounds are returned unchanged.
func downscale(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= maxSide && sh <= maxSide {
		return img
	}
	dw, dh := sw, sh
	if sw >= sh {
		dw = maxSide
		dh = sh * maxSide / sw
	} else {
		dh = maxSide
		dw = sw * maxSide / sh
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + y*sh/dh
		for x := 0; x < dw; x++ {
			sx := b.Min.X + x*sw/dw
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// Store writes normalized JPEG bytes to ImagePath(dir, personID, imageID),
// creating the per-person subdir, via a temp file + rename so a reader never sees a
// torn file (mirrors the thumbnail manager's atomic write). The caller has already
// inserted the DB row, so imageID is the authoritative, server-assigned name.
func Store(dir string, personID, imageID int64, data []byte) error {
	if err := os.MkdirAll(personDir(dir, personID), 0o755); err != nil {
		return fmt.Errorf("create person image dir: %w", err)
	}
	dst := ImagePath(dir, personID, imageID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write person image: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename person image: %w", err)
	}
	return nil
}

// Remove deletes a stored image file. A missing file is not an error (the row may
// have outlived its bytes, or a prior delete was interrupted) — the DB row is the
// source of truth and is removed separately.
func Remove(dir string, personID, imageID int64) error {
	err := os.Remove(ImagePath(dir, personID, imageID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove person image: %w", err)
	}
	return nil
}
