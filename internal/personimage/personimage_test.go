package personimage

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"holodex/internal/model"
)

// webpBytes decodes a small, still WebP fixture. x/image/webp is decode-only (there
// is no WebP encoder in the stdlib or x/image), so the fixture is a fixed, known-good
// still image rather than one synthesized per test like jpegBytes/pngBytes. If this
// ever fails to decode, the fixture — not the code under test — is wrong.
func webpBytes(t *testing.T) []byte {
	t.Helper()
	// A real 16×16 still WebP (VP8), generated with `ffmpeg -f lavfi -i color=...
	// -c:v libwebp`. Base64 so the raw RIFF bytes don't have to live in the source as
	// an escaped blob.
	const fixture = "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoQABAAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=="
	raw, err := base64.StdEncoding.DecodeString(fixture)
	if err != nil {
		t.Fatalf("decode webp fixture base64: %v", err)
	}
	return raw
}

// jpegBytes encodes a solid w×h JPEG with a fake "EXIF-ish" trailing marker so a
// test can assert re-encoding drops anything that isn't pixels.
func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0x40, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0x10, uint8(x), uint8(y), 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source png: %v", err)
	}
	return buf.Bytes()
}

func TestNormalizeReencodesToJPEG(t *testing.T) {
	// A PNG in → a JPEG out (the format is normalized regardless of input type).
	out, w, h, err := Normalize(pngBytes(t, 120, 80), 0)
	if err != nil {
		t.Fatalf("normalize png: %v", err)
	}
	if w != 120 || h != 80 {
		t.Errorf("dims = %dx%d, want 120x80", w, h)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(out)); err != nil || format != "jpeg" {
		t.Errorf("output format = %q err=%v, want jpeg", format, err)
	}
}

func TestNormalizeAcceptsWebP(t *testing.T) {
	// F42: a still WebP is decoded and re-encoded to JPEG like any other input, so a
	// provider that serves WebP is no longer silently dropped at the decode step.
	out, w, h, err := Normalize(webpBytes(t), 0)
	if err != nil {
		t.Fatalf("normalize webp: %v", err)
	}
	if w != 16 || h != 16 {
		t.Errorf("dims = %dx%d, want 16x16", w, h)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(out)); err != nil || format != "jpeg" {
		t.Errorf("output format = %q err=%v, want jpeg", format, err)
	}
}

func TestNormalizeRejectsUndecodableWebP(t *testing.T) {
	// x/image/webp is still-image only, so an animated WebP (VP8X + ANIM/ANMF chunks)
	// does not decode — F41's fail-safe: the asset errors out and is skipped by the
	// caller, never stored, rather than crashing. A truncated WebP takes the same path.
	animated, err := base64.StdEncoding.DecodeString(
		"UklGRmQBAABXRUJQVlA4WAoAAAASAAAADwAADwAAQU5JTQYAAAD/////AABBTk1GSAAAAAAAAAAAAA8AAA8AAPoAAABWUDggMAAAANABAJ0BKhAAEAACADQloAJ0ugH4AAOwAP7w6Pf/ILlhdcjX/yA/5Af8gP/48gAAAEFOTUZIAAAAAAAAAAAADwAADwAA+gAAAFZQOCAwAAAA0AEAnQEqEAAQAAIANCWgAnS6AfgAA7AA/vDo9/8guWF1yNf/ID/kB/yA//jyAAAAQU5NRkgAAAAAAAAAAAAPAAAPAAD6AAAAVlA4IDAAAADQAQCdASoQABAAAgA0JaACdLoB+AADsAD+8Oj3/yC5YXXI1/8gP+QH/ID/+PIAAABBTk1GSAAAAAAAAAAAAA8AAA8AAPoAAABWUDggMAAAANABAJ0BKhAAEAACADQloAJ0ugH4AAOwAP7w6Pf/ILlhdcjX/yA/5Af8gP/48gAAAA==")
	if err != nil {
		t.Fatalf("decode animated webp fixture base64: %v", err)
	}
	for name, in := range map[string][]byte{
		"animated":  animated,
		"truncated": webpBytes(t)[:20],
	} {
		if _, _, _, err := Normalize(in, 0); err == nil {
			t.Errorf("%s webp: expected a decode error, got nil (should be skipped, not stored)", name)
		}
	}
}

func TestNormalizeStripsTrailingMetadata(t *testing.T) {
	// Append a junk "metadata" blob after the JPEG EOI. Re-encoding produces a clean
	// stream that no longer contains it — the metadata strip.
	src := jpegBytes(t, 64, 64)
	marker := []byte("EXIFGPS:secret-location-data")
	polluted := append(append([]byte{}, src...), marker...)

	out, _, _, err := Normalize(polluted, 0)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if bytes.Contains(out, marker) {
		t.Error("re-encoded image still contains the trailing metadata marker")
	}
}

func TestNormalizeRejectsNonImage(t *testing.T) {
	for _, in := range [][]byte{
		[]byte("not an image at all"),
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`),
		{}, // empty
	} {
		if _, _, _, err := Normalize(in, 0); err == nil {
			t.Errorf("expected error for non-image input %q", string(in))
		}
	}
}

func TestNormalizeRejectsDecompressionBomb(t *testing.T) {
	// A tiny PNG whose header CLAIMS huge dimensions: DecodeConfig reads the claimed
	// size and the bomb guard rejects it before any pixel allocation. We forge the
	// IHDR width/height of a real small PNG to 100000×100000.
	small := pngBytes(t, 4, 4)
	bomb := forgePNGDims(small, 100000, 100000)
	if _, _, _, err := Normalize(bomb, 0); err == nil {
		t.Error("expected decompression-bomb rejection for oversized declared dimensions")
	}
}

// forgePNGDims rewrites the IHDR width/height (bytes 16..24) of a PNG. The CRC will
// be wrong, but DecodeConfig reads the dimensions from IHDR before validating the
// full image, which is exactly the pre-decode guard we want to exercise.
func forgePNGDims(p []byte, w, h uint32) []byte {
	out := append([]byte{}, p...)
	put := func(off int, v uint32) {
		out[off] = byte(v >> 24)
		out[off+1] = byte(v >> 16)
		out[off+2] = byte(v >> 8)
		out[off+3] = byte(v)
	}
	put(16, w)
	put(20, h)
	return out
}

func TestNormalizeRejectsOversizePerSide(t *testing.T) {
	// 12001×4 is well under the 60 MP total-pixel cap but over the 12000-px-per-side
	// limit, so it isolates the per-side guard (the decompression-bomb test trips both).
	wide := forgePNGDims(pngBytes(t, 4, 4), 12001, 4)
	if _, _, _, err := Normalize(wide, 0); err == nil {
		t.Error("expected rejection for a dimension over the per-side limit")
	}
}

func TestNormalizeDownscales(t *testing.T) {
	out, w, h, err := Normalize(jpegBytes(t, 1000, 500), 200)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if w != 200 || h != 100 {
		t.Errorf("downscaled dims = %dx%d, want 200x100 (aspect preserved)", w, h)
	}
	if cfg, _, _ := image.DecodeConfig(bytes.NewReader(out)); cfg.Width != 200 {
		t.Errorf("encoded width = %d, want 200", cfg.Width)
	}
}

func TestGenderBucket(t *testing.T) {
	cases := map[string]string{
		"male":        BucketMale,
		"Female":      BucketFemale,
		"  MAN  ":     BucketMale,
		"woman":       BucketFemale,
		"nonbinary":   BucketNeutral,
		"unknown":     BucketNeutral,
		"":            BucketNeutral,
		"genderfluid": BucketNeutral,
	}
	for in, want := range cases {
		if got := GenderBucket(in); got != want {
			t.Errorf("GenderBucket(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlaceholderResolution(t *testing.T) {
	// Pure + deterministic: identical inputs → identical bytes.
	a := Placeholder("cinematheque", model.PersonImageHeadshot, "male")
	b := Placeholder("cinematheque", model.PersonImageHeadshot, "male")
	if !bytes.Equal(a, b) {
		t.Error("Placeholder is not deterministic for identical inputs")
	}

	// Themed with CONCRETE per-skin colors resolved server-side (ADR-038): the SVG is
	// served standalone via <img>, an isolated document that can't read the page's
	// CSS variables, so bare var(--…) would render un-themed black. Each skin's accent
	// must appear; switching skins must change the bytes; an unknown skin defaults to
	// Cinémathèque.
	if strings.Contains(string(a), "var(--") {
		t.Error("placeholder must not rely on CSS var() — it is served standalone via <img>")
	}
	cine := string(Placeholder("cinematheque", model.PersonImageHeadshot, "male"))
	broad := string(Placeholder("broadcast", model.PersonImageHeadshot, "male"))
	brut := string(Placeholder("brutalist", model.PersonImageHeadshot, "male"))
	if !strings.Contains(cine, "#e8a33d") {
		t.Error("cinematheque placeholder should carry the ember accent #e8a33d")
	}
	if !strings.Contains(broad, "#36e0d0") {
		t.Error("broadcast placeholder should carry the cyan accent #36e0d0")
	}
	if !strings.Contains(brut, "#d6ff3f") {
		t.Error("brutalist placeholder should carry the lime accent #d6ff3f")
	}
	if cine == broad || cine == brut {
		t.Error("placeholder should differ per skin")
	}
	if Placeholder("nonsense-skin", model.PersonImageHeadshot, "male") == nil ||
		!bytes.Equal(Placeholder("nonsense-skin", model.PersonImageHeadshot, "male"), []byte(cine)) {
		t.Error("unknown skin should default to cinematheque")
	}

	// Role-shaped viewBox: square headshot, wide banner, tall poster.
	if !strings.Contains(string(Placeholder("", model.PersonImageHeadshot, "")), "viewBox=\"0 0 400 400\"") {
		t.Error("headshot should be 1:1 (400x400)")
	}
	if !strings.Contains(string(Placeholder("", model.PersonImageBanner, "")), "viewBox=\"0 0 1600 900\"") {
		t.Error("banner should be 16:9 (1600x900)")
	}
	if !strings.Contains(string(Placeholder("", model.PersonImagePoster, "")), "viewBox=\"0 0 400 600\"") {
		t.Error("poster should be 2:3 (400x600)")
	}
}
