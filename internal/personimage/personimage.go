// Package personimage is the storage + security spine for per-person images (F25,
// ADR-038): on-disk path layout, untrusted-bytes normalization, atomic writes, and
// the themed placeholder served when a role is empty.
//
// Unlike the thumbnail pipeline (ADR-009) there is no ffmpeg/exiftool and no
// background queue: Normalize is fast, synchronous, and stdlib-only, so the API
// handler normalizes inline and writes the file directly — simpler, and it avoids a
// queue whose only job would be a single re-encode. The disk write is still
// atomic-ish (temp file + rename) like the thumbnail manager so a crash mid-write
// never leaves a torn JPEG that the serving handler would hand a client.
//
// Normalize is the metadata-strip: every accepted upload is re-encoded with the
// stdlib encoders (JPEG, or PNG when the image is not opaque — ADR-097), which carry
// no EXIF/XMP/ICC — so an image's embedded GPS, camera, or other PII never reaches
// disk, and an SVG/polyglot can't be stored (stdlib refuses to decode it as a raster
// image).
package personimage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/image/draw"

	// Register the decoders Normalize sniffs for — jpeg/png/gif from the stdlib, all
	// re-encoded to jpeg or png on the way in. webp (F42) comes from
	// golang.org/x/image/webp, which is decode-only and still-image only: an animated
	// or otherwise unsupported webp fails to decode and its asset is skipped
	// (fail-safe), never stored.
	_ "image/gif"

	_ "golang.org/x/image/webp"

	"holodex/internal/entityimage"
)

// Hash is the content identity of a stored image (F34/ADR-050): the hex sha256 of
// the NORMALIZED bytes, so two ingests that re-encode to the same bytes collide
// regardless of their source URL, provider, or original container. Computed over
// Normalize's output (not the raw download), so EXIF/ICC noise in the source can't
// defeat the match.
func Hash(normalized []byte) string {
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

// Decompression-bomb and output bounds (ADR-038 F25). These guard a single decode
// of untrusted bytes: the config is read BEFORE the full decode so a tiny file
// claiming 100000×100000 is rejected without ever allocating the pixels.
const (
	maxDimension = 12000            // reject if width or height exceeds this (any side)
	maxPixels    = 60 * 1000 * 1000 // reject if width*height exceeds ~60 MP
	jpegQuality  = 85               // re-encode quality
)

// Find is the on-disk location for a person's image (ADR-038/ADR-014):
// {dir}/{personID}/{imageID}.jpg or .png (ADR-097). The id is server-assigned (an
// integer), never a request value, so traversal is structurally impossible. A missing
// file errors with os.ErrNotExist. Delegates to internal/entityimage (HOLODEX-286),
// shared with studioimage/filmimage.
func Find(dir string, personID, imageID int64) (string, error) {
	return entityimage.Find(dir, personID, imageID)
}

// Normalize sniffs, decodes, and re-encodes untrusted image bytes to a clean JPEG,
// or a clean PNG when the decoded image has any transparency (ADR-038 F25 — the
// metadata strip + decompression-bomb guard; ADR-097 — alpha survives so a logo
// keeps its transparent background). It returns the re-encoded bytes and the stored
// dimensions; entityimage.Ext tells the two formats apart. Steps:
//
//  1. DecodeConfig (cheap, no pixel allocation) → reject oversize dims/area before
//     the full decode, closing the decompression-bomb vector.
//  2. Decode → reject non-images / polyglots / SVG (stdlib has no SVG decoder).
//  3. Downscale to maxOutDimension if larger (cheap nearest-neighbour; portraits are
//     small and this is not a quality-critical path).
//  4. Re-encode with the stdlib encoder — PNG if the image is not opaque, JPEG
//     otherwise — which carries none of the source's EXIF/XMP/ICC metadata.
//
// maxOutDimension<=0 means "don't downscale" (only the bomb guard applies).
func Normalize(input []byte, maxOutDimension int) (out []byte, w, h int, err error) {
	return normalize(input, maxOutDimension, true)
}

// NormalizeJPEG is Normalize for a consumer whose storage is JPEG-only (the video
// poster, which is written into the ADR-009 thumbnail pipeline's {id}.jpg slots):
// identical hardening, but a transparent image is flattened rather than kept as PNG.
func NormalizeJPEG(input []byte, maxOutDimension int) (out []byte, w, h int, err error) {
	return normalize(input, maxOutDimension, false)
}

func normalize(input []byte, maxOutDimension int, keepAlpha bool) (out []byte, w, h int, err error) {
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

	out, err = encode(img, keepAlpha && !opaque(img))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	return out, b.Dx(), b.Dy(), nil
}

// encode is the spine's encoder: PNG (lossless, alpha intact) when asPNG, else JPEG
// at jpegQuality. Neither stdlib encoder writes EXIF/XMP/ICC.
func encode(img image.Image, asPNG bool) ([]byte, error) {
	var buf bytes.Buffer
	if asPNG {
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	} else if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// opaque reports whether every pixel is fully opaque. Every stdlib and x/image
// decoder returns a type with an Opaque method (a full-image scan for the alpha
// formats, a constant true for YCbCr/Gray); an unknown type is treated as opaque so
// the pre-ADR-097 JPEG path stays the fallback.
func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return true
}

// downscale shrinks img so its longest side is at most maxSide, preserving aspect
// ratio, via nearest-neighbour sampling (stdlib-only; quality is adequate for the
// ingest cap's mild reductions). The served width variants downscale much further and
// use x/image/draw CatmullRom instead (Variant). Images already within bounds are
// returned unchanged.
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

// Tiers are the served width variants, ascending (HOLODEX-332). A tier is the output
// width in px; the height follows the aspect ratio.
var Tiers = []int{160, 320, 640}

// SnapWidth maps a request's ?w= to a tier: a non-tier value snaps up to the next
// tier. Empty, non-numeric, zero, negative and anything above the largest tier return
// 0, meaning "serve the original" — so a crafted w can never mint a new file.
func SnapWidth(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	for _, t := range Tiers {
		if n <= t {
			return t
		}
	}
	return 0
}

// Variant returns the path to serve for a tier of a stored original (a path Find
// returned), generating the variant on its first request (HOLODEX-332): decode,
// downscale to width with CatmullRom, re-encode, and write it atomically beside the
// original. An original at or below width is returned as is and no file is written.
//
// The variant keeps the original's format: a stored .png is the alpha path, so its
// variant stays PNG even if the downscale happened to blend every edge opaque. That
// keeps the cached name deterministic from the original alone. The original is our own
// normalized output, bounded by the ingest bomb guard, so it is decoded without one.
func Variant(original string, width int) (string, error) {
	dst := entityimage.VariantPath(original, width)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	f, err := os.Open(original)
	if err != nil {
		return "", fmt.Errorf("open person image: %w", err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("decode person image config: %w", err)
	}
	if cfg.Width <= width {
		return original, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind person image: %w", err)
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode person image: %w", err)
	}
	h := max((cfg.Height*width+cfg.Width/2)/cfg.Width, 1)
	scaled := image.NewRGBA(image.Rect(0, 0, width, h))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Src, nil)
	data, err := encode(scaled, filepath.Ext(original) == ".png")
	if err != nil {
		return "", err
	}
	if err := entityimage.StoreVariant(dst, data); err != nil {
		return "", err
	}
	return dst, nil
}

// Store writes normalized JPEG/PNG bytes to their id-named path under {dir}/{personID},
// creating the per-person subdir, via a temp file + rename so a reader never sees a
// torn file (mirrors the thumbnail manager's atomic write). The caller has already
// inserted the DB row, so imageID is the authoritative, server-assigned name.
func Store(dir string, personID, imageID int64, data []byte) error {
	return entityimage.Store(dir, personID, imageID, data)
}

// Remove deletes a stored image file. A missing file is not an error (the row may
// have outlived its bytes, or a prior delete was interrupted) — the DB row is the
// source of truth and is removed separately.
func Remove(dir string, personID, imageID int64) error {
	return entityimage.Remove(dir, personID, imageID)
}
