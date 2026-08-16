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
	// EntityKind marks a video field whose resolved value(s) name a linkable entity
	// (F40, ADR-072): "" (not entity-typed), EntityKindPerson, or EntityKindStudio.
	// RelinkVideoEntity, the extractor's person-key source list, and the link
	// picker's target set all read this marker — no field name is hardcoded outside
	// the registry.
	EntityKind string
	// Role is the video_people.role a resolved name from this field links as; only
	// meaningful when EntityKind == EntityKindPerson. Empty means "no role" (the
	// unset sentinel stored as '', never SQL NULL) — a person-typed field can be
	// entity-linkable without every credit having a meaningful role.
	Role string
	// Criticality is the facet's weight class for the entity completeness score
	// (F55, ADR-081 D1): CriticalityCritical, CriticalityNiceToHave, or "" (the
	// zero value — excluded from scoring; the default for most fields, since only
	// P0-scored facets per entity type get an explicit tag). A field can never be
	// both Computed and criticality-tagged — the scorer treats Computed as an
	// automatic exclusion, reusing the invariant ADR-063 already established for
	// age/age_at_death rather than re-deriving it here.
	Criticality string
}

// EntityKind values (F40, ADR-072) — see FieldDef.EntityKind.
const (
	EntityKindPerson = "person"
	EntityKindStudio = "studio"
)

// Criticality values (F55, ADR-081 D1) — see FieldDef.Criticality.
const (
	CriticalityCritical   = "critical"
	CriticalityNiceToHave = "nice_to_have"
)

// KnownFields is the full canonical field registry. Order is documentation order;
// lookup is by Canonical key via Lookup().
var KnownFields = []FieldDef{
	// ---- Video / film fields ----
	{
		Canonical:   "title",
		Label:       "Title",
		Display:     "",
		Description: "Display title of the video. Prefer a provider value over the filename-derived title.",
		Criticality: CriticalityCritical,
	},
	{
		Canonical:   "original_title",
		Label:       "Original Title",
		Display:     "",
		Description: "Title in the original language when it differs from the primary title.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "overview",
		Label:       "Overview",
		Display:     "long_text",
		Description: "Plot summary or description. Trimmed to ≤4000 chars at a sentence boundary.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "tagline",
		Label:       "Tagline",
		Display:     "",
		Description: "Short marketing tagline.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "release_date",
		Label:       "Released",
		Display:     "",
		Description: "Release date in YYYY-MM-DD format.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "runtime",
		Label:       "Runtime (min)",
		Display:     "",
		Description: "Runtime in minutes (integer string).",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "genres",
		Label:       "Genres",
		Display:     "",
		Description: "Genre list. Multi-valued; each value is one genre name.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "status",
		Label:       "Status",
		Display:     "",
		Description: "Release status (e.g. Released, Post Production, In Production).",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "original_language",
		Label:       "Language",
		Display:     "",
		Description: "ISO 639-1 language code of the original language.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "homepage",
		Label:       "Website",
		Display:     "url",
		Description: "Official website URL for the film. Rendered as a link (opens in a new tab).",
		Criticality: CriticalityNiceToHave,
	},
	{
		// F55 (ADR-081 D5, value shape per ADR-082): generalized from the old
		// imdb_id name. The value is namespace-qualified ("<provider>:<id>",
		// e.g. "tmdb:603", "imdb:tt1234567") so it stays unambiguous when more
		// than one provider can populate this facet.
		Canonical:   "external_provider_id",
		Label:       "External ID",
		Display:     "",
		Description: "External metadata-provider identifier, namespace-qualified (\"<provider>:<id>\").",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "poster_url",
		Label:       "Poster",
		Display:     "image_url",
		Description: "Poster image URL. Must be on an operator-allowlisted CDN host (ADR-039). Rendered as <img>.",
		Criticality: CriticalityCritical,
	},

	// ---- Person fields ----
	{
		Canonical:   "bio",
		Label:       "Bio",
		Display:     "long_text",
		Description: "Biography or description. Trimmed to ≤4000 chars at a sentence boundary.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "birthdate",
		Label:       "Born",
		Display:     "",
		Description: "Birth date in YYYY-MM-DD format.",
		Criticality: CriticalityNiceToHave,
	},
	{
		// Excluded from completeness scoring (F55, ADR-081): legitimately absent
		// for most (living) people — not a meaningful completeness gap.
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
		Criticality: CriticalityNiceToHave,
	},
	{
		// Excluded from completeness scoring (F55, ADR-081): low signal for "is
		// this person's profile complete."
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
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "photo",
		Label:       "Photo",
		Display:     "image_url",
		Description: "Portrait image. Delivered as an asset (not a field value) by providers that support it.",
		Criticality: CriticalityCritical,
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
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "country",
		Label:       "Country",
		Display:     "",
		Description: "Origin country of the studio (ISO 3166-1 code as provided by the source).",
		Criticality: CriticalityNiceToHave,
	},
	// "logo" was a plain image_url field through F38 (ADR-057); retired in F51
	// (ADR-079) — the studio logo (plus icon/poster) is now a downloaded asset in
	// studio_images, delivered like a person photo, not a resolved field value.
	{
		// Composite completeness facet (F55, ADR-081 D1): resolved when the
		// studio has at least one image in any of the icon/logo/poster roles
		// (F51/ADR-079), scored as one facet, not three. Synthetic — never
		// produced by a provider, file, or the resolver; studio_images is
		// queried directly for its status. It exists here only so the
		// completeness scorer has a single, code-reviewed place to carry the
		// facet's criticality weight, the same as every other scored facet.
		Canonical:   "branding_image",
		Label:       "Branding image",
		Display:     "",
		Description: "Whether the studio has at least one icon/logo/poster image set (F51, ADR-079).",
		Criticality: CriticalityNiceToHave,
	},

	// ---- File-metadata fields (examples; operators add more via metadata-mappings.yaml) ----
	{
		Canonical:   "actors",
		Label:       "Actors",
		Display:     "",
		Description: "Cast members. Multi-valued; each value is one performer's name. Written as a comma-delimited Artist tag.",
		EntityKind:  EntityKindPerson,
		Role:        "actor",
		Criticality: CriticalityCritical,
	},
	{
		Canonical:   "studio",
		Label:       "Studio",
		Display:     "",
		Description: "Production company, publisher, or label. Typically sourced from Publisher/Label/Studio file tags.",
		EntityKind:  EntityKindStudio,
		Criticality: CriticalityCritical,
	},
	{
		Canonical:   "collection",
		Label:       "Collection",
		Display:     "",
		Description: "Album or collection name. Typically sourced from the Album file tag.",
		Criticality: CriticalityNiceToHave,
	},
	{
		Canonical:   "director",
		Label:       "Director",
		Display:     "",
		Description: "Director(s). Multi-valued.",
		EntityKind:  EntityKindPerson,
		Role:        "director",
		Criticality: CriticalityNiceToHave,
	},
}

// PersonTypedFields returns the registered fields whose resolved values name a
// linkable person (F40, ADR-072 P0-5) — the derivation/extractor/link-picker
// target set, read from the registry instead of hardcoding field names.
func PersonTypedFields() []FieldDef {
	var out []FieldDef
	for _, f := range KnownFields {
		if f.EntityKind == EntityKindPerson {
			out = append(out, f)
		}
	}
	return out
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
