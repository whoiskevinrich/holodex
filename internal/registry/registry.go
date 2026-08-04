// Package registry is the canonical field registry for Holodex enrichment and
// metadata mapping (F27). It is the single source of truth for known field names,
// their default display labels, render hints, and descriptions.
//
// Every field that a metadata provider, file-tag mapping, or resolver may produce
// should have an entry here. Unknown field keys still work — they resolve with a
// title-cased label and no display hint — but registered fields get accurate labels
// and correct render behaviour (e.g. poster_url renders as <img>, not as text).
//
// The registry is intentionally code-level (not YAML) because it is a schema
// contract between the provider protocol, the resolver, and the SPA renderer.
// Operators extend display via metadata-mappings.yaml label overrides; they do not
// need to touch this file.
package registry

import "strings"

// FieldDef is one entry in the canonical field registry.
type FieldDef struct {
	// Canonical is the stable key used in entity_enrichment and mappings config.
	Canonical string
	// Label is the default human-readable display label.
	Label string
	// Display hints how the SPA should render this field's value:
	//   ""           — inline text (default)
	//   "long_text"  — block paragraph
	//   "image_url"  — render as <img src=…>
	//   "url"        — render as a link (opens in a new tab)
	Display string
	// Description documents the field for operators and provider authors.
	Description string
	// Computed marks a derived field genre (F45, ADR-063): source-less, read-only,
	// computed-on-read from other resolved fields by the resolver's Derive post-pass.
	// A computed field is never stored, never adoptable/curatable, and never produced
	// by a provider or file — it is the fact a human reads off the data (e.g. Age).
	Computed bool
	// DependsOn lists the canonical inputs a computed field's formula reads (e.g.
	// ["birthdate"]). It is the declarative contract the Derive pass uses to gather
	// inputs and to name the transitive "calculated from …" provenance. Empty for a
	// non-computed field.
	DependsOn []string
}

// KnownFields is the full canonical field registry. Order is documentation order;
// lookup is by Canonical key via Lookup().
var KnownFields = []FieldDef{
	// ---- Video / film fields ----
	{
		Canonical:   "title",
		Label:       "Title",
		Display:     "",
		Description: "Display title of the video. Prefer a provider value over the filename-derived title.",
	},
	{
		Canonical:   "original_title",
		Label:       "Original Title",
		Display:     "",
		Description: "Title in the original language when it differs from the primary title.",
	},
	{
		Canonical:   "overview",
		Label:       "Overview",
		Display:     "long_text",
		Description: "Plot summary or description. Trimmed to ≤4000 chars at a sentence boundary.",
	},
	{
		Canonical:   "tagline",
		Label:       "Tagline",
		Display:     "",
		Description: "Short marketing tagline.",
	},
	{
		Canonical:   "release_date",
		Label:       "Released",
		Display:     "",
		Description: "Release date in YYYY-MM-DD format.",
	},
	{
		Canonical:   "runtime",
		Label:       "Runtime (min)",
		Display:     "",
		Description: "Runtime in minutes (integer string).",
	},
	{
		Canonical:   "genres",
		Label:       "Genres",
		Display:     "",
		Description: "Genre list. Multi-valued; each value is one genre name.",
	},
	{
		Canonical:   "status",
		Label:       "Status",
		Display:     "",
		Description: "Release status (e.g. Released, Post Production, In Production).",
	},
	{
		Canonical:   "original_language",
		Label:       "Language",
		Display:     "",
		Description: "ISO 639-1 language code of the original language.",
	},
	{
		Canonical:   "homepage",
		Label:       "Website",
		Display:     "url",
		Description: "Official website URL for the film. Rendered as a link (opens in a new tab).",
	},
	{
		Canonical:   "imdb_id",
		Label:       "IMDb",
		Display:     "",
		Description: "IMDb title identifier (tt… format).",
	},
	{
		Canonical:   "poster_url",
		Label:       "Poster",
		Display:     "image_url",
		Description: "Poster image URL. Must be on an operator-allowlisted CDN host (ADR-039). Rendered as <img>.",
	},

	// ---- Person fields ----
	{
		Canonical:   "bio",
		Label:       "Bio",
		Display:     "long_text",
		Description: "Biography or description. Trimmed to ≤4000 chars at a sentence boundary.",
	},
	{
		Canonical:   "birthdate",
		Label:       "Born",
		Display:     "",
		Description: "Birth date in YYYY-MM-DD format.",
	},
	{
		Canonical:   "deathdate",
		Label:       "Died",
		Display:     "",
		Description: "Death date in YYYY-MM-DD format. Omitted when the person is living.",
	},
	{
		Canonical:   "nationality",
		Label:       "Nationality",
		Display:     "",
		Description: "Place of birth or nationality string as provided by the source.",
	},
	{
		Canonical:   "website",
		Label:       "Website",
		Display:     "url",
		Description: "Personal or professional website URL. Rendered as a link (opens in a new tab).",
	},
	{
		Canonical:   "aliases",
		Label:       "Aliases",
		Display:     "",
		Description: "Alternate names or pseudonyms. Multi-valued.",
	},
	{
		Canonical:   "photo",
		Label:       "Photo",
		Display:     "image_url",
		Description: "Portrait image. Delivered as an asset (not a field value) by providers that support it.",
	},

	// ---- Derived / computed person fields (F45, ADR-063) ----
	// Source-less, read-only, computed-on-read by resolver.Derive from the fields in
	// DependsOn. age and age_at_death are mutually exclusive on a person: a living
	// person shows a running age, a deceased one shows age-at-death instead.
	{
		Canonical:   "age",
		Label:       "Age",
		Display:     "",
		Description: "Current age in whole years, computed on read from birthdate (floor(now − birthdate)). Only shown for a living person (no deathdate). Never stored.",
		Computed:    true,
		DependsOn:   []string{"birthdate"},
	},
	{
		Canonical:   "age_at_death",
		Label:       "Age at death",
		Display:     "",
		Description: "Age in whole years at the time of death (floor(deathdate − birthdate)). Replaces the running age for a deceased person. Never stored.",
		Computed:    true,
		DependsOn:   []string{"birthdate", "deathdate"},
	},

	// ---- Studio-entity fields (F38 S3) ----
	{
		Canonical:   "description",
		Label:       "Description",
		Display:     "long_text",
		Description: "Studio description or summary. Trimmed to ≤4000 chars at a sentence boundary.",
	},
	{
		Canonical:   "country",
		Label:       "Country",
		Display:     "",
		Description: "Origin country of the studio (ISO 3166-1 code as provided by the source).",
	},
	// "logo" was a plain image_url field through F38 (ADR-057); retired in F51
	// (ADR-079) — the studio logo (plus icon/poster) is now a downloaded asset in
	// studio_images, delivered like a person photo, not a resolved field value.

	// ---- File-metadata fields (examples; operators add more via metadata-mappings.yaml) ----
	{
		Canonical:   "actors",
		Label:       "Actors",
		Display:     "",
		Description: "Cast members. Multi-valued; each value is one performer's name. Written as a comma-delimited Artist tag.",
	},
	{
		Canonical:   "studio",
		Label:       "Studio",
		Display:     "",
		Description: "Production company, publisher, or label. Typically sourced from Publisher/Label/Studio file tags.",
	},
	{
		Canonical:   "collection",
		Label:       "Collection",
		Display:     "",
		Description: "Album or collection name. Typically sourced from the Album file tag.",
	},
	{
		Canonical:   "director",
		Label:       "Director",
		Display:     "",
		Description: "Director(s). Multi-valued.",
	},
}

// Render modes (the FieldDef.Display vocabulary). "" is inline text; the rest are
// explicit. `chips` (F39) is a read-only pill list used by auto-registered
// multi-valued non-canonical fields. A provider hint (ADR-056) may only *suggest*
// one of these for a non-canonical key; an unknown value normalizes to text.
const (
	DisplayText     = ""
	DisplayLongText = "long_text"
	DisplayURL      = "url"
	DisplayImageURL = "image_url"
	DisplayChips    = "chips"
)

// NormalizeDisplay coerces an untrusted render-mode string to the known vocabulary,
// defaulting to inline text. Used on the provider-hint ingest path (F39).
func NormalizeDisplay(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case DisplayLongText:
		return DisplayLongText
	case DisplayURL:
		return DisplayURL
	case DisplayImageURL:
		return DisplayImageURL
	case DisplayChips:
		return DisplayChips
	default:
		return DisplayText
	}
}

// Ordering groups for auto-registered non-canonical fields (F39, ADR-056). They
// sort a field into a coarse band *after* the canonical fields; within a band a
// numeric order then the key break ties. `extended` is the default (lowest).
const (
	GroupPrimary    = "primary"
	GroupAttributes = "attributes"
	GroupExtended   = "extended"
)

// NormalizeGroup coerces an untrusted ordering-group string to the known vocabulary,
// defaulting to the lowest band (extended).
func NormalizeGroup(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case GroupPrimary:
		return GroupPrimary
	case GroupAttributes:
		return GroupAttributes
	default:
		return GroupExtended
	}
}

// GroupRank ranks a (normalized) ordering group for sorting: primary < attributes
// < extended. Unknown groups rank as extended.
func GroupRank(group string) int {
	switch group {
	case GroupPrimary:
		return 0
	case GroupAttributes:
		return 1
	default:
		return 2
	}
}

// index is built once at init time for O(1) lookup.
var index map[string]FieldDef

func init() {
	index = make(map[string]FieldDef, len(KnownFields))
	for _, f := range KnownFields {
		index[f.Canonical] = f
	}
}

// IsKnown reports whether a canonical key is registered (case-insensitive). A key
// that is not known is "non-canonical" — the only kind a provider hint may govern
// and the only kind F39 auto-registration surfaces (ADR-056).
func IsKnown(canonical string) bool {
	_, ok := index[strings.ToLower(strings.TrimSpace(canonical))]
	return ok
}

// Lookup returns the FieldDef for a canonical key (case-insensitive). If the key
// is not registered, a synthesized FieldDef is returned with a title-cased Label
// and empty Display — unknown fields still render, just without registered metadata.
func Lookup(canonical string) FieldDef {
	k := strings.ToLower(strings.TrimSpace(canonical))
	if f, ok := index[k]; ok {
		return f
	}
	label := k
	if k != "" {
		label = strings.ToUpper(k[:1]) + k[1:]
	}
	return FieldDef{Canonical: k, Label: label}
}
