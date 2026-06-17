package personimage

import (
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Gender buckets for placeholder silhouettes (ADR-038 F25). Only three buckets so
// the asset set stays tiny; everything unknown collapses to neutral. The bucket
// only nudges the silhouette's shoulder width — it is never asserted as fact about
// a person, just a visual default behind a missing photo.
const (
	BucketMale    = "male"
	BucketFemale  = "female"
	BucketNeutral = "neutral"
)

// GenderBucket maps a raw gender string (provider/enrichment-sourced, free-form) to
// one of the three buckets. male/female map through; nonbinary/unknown/"" and any
// unrecognized value map to neutral. Case- and space-insensitive; pure.
func GenderBucket(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "m", "male", "man", "boy":
		return BucketMale
	case "f", "female", "woman", "girl":
		return BucketFemale
	default:
		return BucketNeutral
	}
}

// placeholderDims gives the viewBox for each role's aspect ratio.
func placeholderDims(role string) (w, h int) {
	switch role {
	case model.PersonImageBanner:
		return 1600, 900 // 16:9
	case model.PersonImagePoster:
		return 400, 600 // 2:3
	default: // headshot + extra + unknown → square avatar
		return 400, 400
	}
}

// shoulderWidth nudges the silhouette by gender bucket (fraction of the figure box).
func shoulderWidth(bucket string) float64 {
	switch bucket {
	case BucketMale:
		return 0.46
	case BucketFemale:
		return 0.40
	default:
		return 0.43
	}
}

// skinPalette is the concrete subset of an ADR-021 skin's tokens the placeholder
// needs. Colors are resolved SERVER-SIDE (not CSS `var(--…)`) because the SVG is
// served standalone via <img src> — an isolated document that does NOT inherit the
// page's [data-theme] variables, so a bare var() would render as un-themed black.
// The `?skin=` query param the handler passes drives the lookup. Keep these in sync
// with the SKIN TOKENS block in web/src/app.css.
type skinPalette struct {
	surface2 string // --surface-2: silhouette field
	muted    string // --muted: the head/torso fill
	accent   string // --accent: baseline flourish
	rule     string // --rule: frame border
}

var skinPalettes = map[string]skinPalette{
	"cinematheque": {surface2: "#181310", muted: "#9b9082", accent: "#e8a33d", rule: "#2a2622"},
	"broadcast":    {surface2: "#0a0e1f", muted: "#6f7da6", accent: "#36e0d0", rule: "#1a2240"},
	"brutalist":    {surface2: "#111111", muted: "#8a8a8a", accent: "#d6ff3f", rule: "#333333"},
}

// paletteFor resolves a skin name to its palette, defaulting to Cinémathèque (the
// app default, ADR-021) for an empty or unknown skin.
func paletteFor(skin string) skinPalette {
	if p, ok := skinPalettes[strings.ToLower(strings.TrimSpace(skin))]; ok {
		return p
	}
	return skinPalettes["cinematheque"]
}

// Placeholder builds a deterministic, themed SVG silhouette for an empty role
// (ADR-038 F25). It is pure (same inputs → identical bytes) and resolves concrete
// per-skin colors from `skin` (see skinPalette) so the empty state is correctly
// themed even though the SVG is served standalone via <img>. The silhouette is
// role-shaped: a centered head+torso scaled to the role's aspect box, with a subtle
// accent baseline so it reads as "person, no photo yet".
func Placeholder(skin, role, genderBucket string) []byte {
	pal := paletteFor(skin)
	w, h := placeholderDims(role)
	bucket := GenderBucket(genderBucket)

	// Figure geometry, centered. The head is a circle; the torso a rounded shoulder
	// arc. Coordinates are in viewBox units so the SVG scales to any rendered size.
	cx := float64(w) / 2
	// Anchor the figure toward the lower-centre so the head sits in the frame across
	// all three aspect ratios.
	headR := minF(float64(w), float64(h)) * 0.16
	headCy := float64(h)*0.5 - headR*0.6
	shoulderHalf := float64(w) * shoulderWidth(bucket) / 2
	torsoTop := headCy + headR*1.2
	torsoBottom := float64(h)*0.5 + minF(float64(w), float64(h))*0.30

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" role="img" aria-label="No %s image">`, w, h, role)
	// Background fills with the surface token; a thin accent baseline rule reads under
	// the silhouette so the empty state is recognizably themed, not blank.
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, w, h, pal.surface2)
	// Torso: a rounded shoulder shape rising to the head.
	fmt.Fprintf(&b,
		`<path d="M %.1f %.1f Q %.1f %.1f %.1f %.1f L %.1f %.1f Q %.1f %.1f %.1f %.1f Z" fill="%s" opacity="0.55"/>`,
		cx-shoulderHalf, torsoBottom,
		cx-shoulderHalf, torsoTop, cx, torsoTop,
		cx+shoulderHalf, torsoTop,
		cx+shoulderHalf, torsoTop, cx+shoulderHalf, torsoBottom,
		pal.muted,
	)
	// Head circle.
	fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" opacity="0.55"/>`, cx, headCy, headR, pal.muted)
	// Accent baseline rule for a themed flourish.
	fmt.Fprintf(&b, `<rect x="0" y="%d" width="%d" height="3" fill="%s" opacity="0.6"/>`, h-3, w, pal.accent)
	// Subtle frame so it doesn't bleed into a same-colored container.
	fmt.Fprintf(&b, `<rect x="0.5" y="0.5" width="%.1f" height="%.1f" fill="none" stroke="%s" stroke-width="1" opacity="0.5"/>`, float64(w)-1, float64(h)-1, pal.rule)
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
