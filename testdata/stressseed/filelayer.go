package main

import (
	"fmt"
	"os"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
)

// video_people and video_studios are DERIVED tables, not authored ones (ADR-072,
// ADR-053). Their source is the video's resolved person- and studio-typed fields,
// which the resolver reads off the file layer — and cmd/holodex re-derives every
// video's links from that source on startup (backfillPersonLinks,
// backfillStudioLinks), as does every relink trigger (RelinkVideoEntity).
//
// So a fixture that writes those tables directly is writing a cache. The first
// version of this seeder did exactly that for people, and the rows survived until
// the server booted: the startup backfill resolved each video to zero actors,
// wiped all 50 links and orphan-stamped every person. The fixture destroyed
// itself the moment it was served, which is the worst possible failure — wrong
// data at a stable address is precisely what the addressing scheme exists to
// prevent.
//
// Studios were left writing the link table directly by HOLODEX-344 and survived
// only by luck: backfillStudioLinks skips outright when StudioLinkCount > 0, so
// the seeder's own rows suppressed the pass that would have deleted them. The
// links were still a lie — the resolved `studio` field on those pages was empty
// while video_studios claimed a studio — and the first enrich, decision or rescan
// on any of those videos would have pruned them with no orphan grace.
//
// The fix for both is the same: seed the baseline truth instead. Writing the file
// tags the mapping resolves `actors` and `studio` from makes the derivation
// produce the links itself, so the startup backfill agrees with the fixture
// rather than erasing it.

// fileField is one canonical field the fixture populates through the file layer,
// resolved against the *configured* mapping.
//
// The file key cannot be hardcoded: which tag carries actors or studios is a
// per-instance decision in metadata-mappings.yaml (Cast, Artist, Publisher, …).
// Reading it from the same file the server reads is what makes the seeded value
// and the derived value the same value.
type fileField struct {
	canonical string // "actors", "studio"
	role      string // video_people.role for a person-typed field; "" otherwise
	fileKey   string // the file tag the mapping resolves it from
	multi     bool   // whether the mapping unions every value or takes only the first
}

// fixtureFields are the file-layer fields the ladder needs in order to express
// its cardinality and text rungs.
type fixtureFields struct {
	person fileField
	studio fileField

	// overview is the second free-text field on a video and the only one that is
	// not a column: there is no `videos.overview`, so it exists only as a resolved
	// field (registry `overview`, display long_text). The media page renders it
	// through ExpandableText, which is a different container from the title with a
	// different clamp — so the text palette has to reach it, and can only do so
	// through whichever file tag the mapping points at (HOLODEX-346).
	overview fileField

	// enrich is the shadow-store half rather than a file-layer field: which
	// provider namespaces exist, and which canonical fields the mapping lets them
	// disagree about (HOLODEX-348). It rides here because it is derived from the
	// same mapping, and because loading it here is what makes the seeder refuse a
	// ladder the configuration cannot express *before* it touches disk.
	enrich enrichPlan
}

// loadFields resolves every field the ladder writes through the file layer, and
// fails rather than falling back.
//
// A silent fallback to writing the link tables directly would produce ladders
// that look right in the database and empty themselves on the next boot, and a
// fixture that lies is worse than one that refuses to build. The `multi` check is
// the same guarantee one level up: a replace field resolves through
// firstNonEmpty, so a rung above 1 would land in the manifest as a cardinality
// the page cannot render.
func loadFields(mappingsPath, personasFile string, want ladderDemands) (fixtureFields, error) {
	// mapping.Load treats a missing file as an empty mapping rather than an error,
	// which would send someone off to add an actors field to a file that is not
	// there. Check first and say which of the two problems it actually is.
	if _, err := os.Stat(mappingsPath); err != nil {
		return fixtureFields{}, fmt.Errorf(
			"cannot read metadata mappings at %s: %w.\n"+
				"Point -mappings at the file the `backend-stress` profile passes the server\n"+
				"as METADATA_MAPPINGS_PATH — the fixture and the server have to agree on it",
			mappingsPath, err)
	}

	// Load, not NewStore: the seeder reads the mapping once and never watches it.
	m, err := mapping.Load(mappingsPath)
	if err != nil {
		return fixtureFields{}, fmt.Errorf("load metadata mappings from %s: %w", mappingsPath, err)
	}

	person, err := loadPersonField(m, mappingsPath)
	if err != nil {
		return fixtureFields{}, err
	}
	studio, err := loadFileField(m, "studio", mappingsPath, fieldHint{
		tag:   "Publisher",
		multi: true,
		why: "The fixture builds its links through the file layer so the server's startup\n" +
			"relink derives the same links instead of wiping them",
	})
	if err != nil {
		return fixtureFields{}, err
	}
	overview, err := loadFileField(m, "overview", mappingsPath, fieldHint{
		tag:   "Comment",
		multi: false,
		why: "The text ladder tortures the overview as well as the title, and there is no\n" +
			"videos.overview column — it resolves from the file layer or not at all",
	})
	if err != nil {
		return fixtureFields{}, err
	}

	// The opposite demand to the one below: overview must NOT be multi. A replace
	// field is passed through whole (mapping.go takes TrimSpace(vals[0])), while a
	// multi field is run through SplitMulti — which splits on `, ; /` and newline.
	// The lorem rung is 1500 characters of comma-spliced Latin, so a multi overview
	// would resolve as a list of two dozen fragments and the page would render a
	// value list where the fixture claims a paragraph. That is not a smaller test,
	// it is a different one, and it would look plausible enough to go unnoticed.
	if overview.multi {
		return fixtureFields{}, fmt.Errorf(
			"%s declares `overview` as a MULTI field, but the text ladder writes prose "+
				"through it.\nThe resolver runs a multi field through SplitMulti, which splits "+
				"on %q — so the\nlorem rung would arrive as a list of fragments rather than the "+
				"paragraph the manifest\nclaims. Drop `multi: true` from that field.",
			mappingsPath, multiValueSeparators)
	}

	for _, req := range []struct {
		f    fileField
		want int
	}{{person, want.people}, {studio, want.studios}} {
		if req.want > 1 && !req.f.multi {
			return fixtureFields{}, fmt.Errorf(
				"the ladder has a %s rung at %d, but %s declares `%s` as a REPLACE field.\n"+
					"The resolver takes firstNonEmpty for a replace field, so every rung above 1\n"+
					"would resolve back down to 1 and the manifest would claim a cardinality the\n"+
					"page never renders. Add `multi: true` to that field, or lower the rung.",
				req.f.canonical, req.want, mappingsPath, req.f.canonical)
		}
	}
	plan, err := loadEnrichPlan(m, personasFile, mappingsPath, want.namespaces)
	if err != nil {
		return fixtureFields{}, err
	}

	return fixtureFields{person: person, studio: studio, overview: overview, enrich: plan}, nil
}

// loadPersonField finds the first person-typed field whose configured mapping has
// a file-layer source — the field the fixture can build a cast through. It scans
// the registry rather than naming `actors` outright because which person-typed
// fields exist is the registry's business, and `director` is an equally valid
// carrier on a mapping that omits `actors`.
func loadPersonField(m *mapping.Mappings, mappingsPath string) (fileField, error) {
	for _, def := range registry.PersonTypedFields() {
		f, ok := m.ByCanonical(def.Canonical)
		if !ok {
			continue
		}
		if src, ok := fileSource(f); ok {
			return fileField{canonical: def.Canonical, role: def.Role, fileKey: src.Key, multi: f.Multi}, nil
		}
	}
	return fileField{}, missingFieldErr(mappingsPath, "any person-typed", "actors", fieldHint{
		tag:   "Cast",
		multi: true,
		why: "The fixture builds its cast through the file layer so the server's startup\n" +
			"relink derives the same links instead of wiping them",
	})
}

// loadFileField resolves one named canonical field to its file-layer source.
func loadFileField(m *mapping.Mappings, canonical, mappingsPath string, hint fieldHint) (fileField, error) {
	f, ok := m.ByCanonical(canonical)
	if !ok {
		return fileField{}, missingFieldErr(mappingsPath, canonical, canonical, hint)
	}
	src, ok := fileSource(f)
	if !ok {
		return fileField{}, fmt.Errorf(
			"field %q in %s names no file-layer source, only provider ones.\n"+
				"The fixture has no provider to enrich from, so it can only seed a field the\n"+
				"file layer can carry. Add a bare file tag to that field's `sources`.",
			canonical, mappingsPath)
	}
	return fileField{canonical: canonical, fileKey: src.Key, multi: f.Multi}, nil
}

// fileSource returns the field's first baseline source. file:title aliases the
// videos.title column rather than a file tag, so it cannot carry a link list.
func fileSource(f mapping.Field) (mapping.Source, bool) {
	for _, src := range f.ParsedSources {
		if src.Namespace == "file" && !src.IsFileTitle() {
			return src, true
		}
	}
	return mapping.Source{}, false
}

// fieldHint is the worked example a missing-field error prints. It is per-field
// rather than one generic template because the interesting half of the example is
// the part that differs: a cast or studio field must be `multi: true` or its rungs
// collapse to one, while an overview must NOT be, or the resolver splits the prose
// on its commas. A single template would have told half the readers the wrong
// thing in the one place they are already lost.
type fieldHint struct {
	tag   string // a plausible file tag to hang the field off
	multi bool   // whether the suggested mapping should carry `multi: true`
	why   string // what the fixture needs this field for
}

func missingFieldErr(mappingsPath, subject, canonical string, hint fieldHint) error {
	multi := ""
	if hint.multi {
		multi = "    multi: true\n"
	}
	return fmt.Errorf(
		"no %s field in %s maps to a file tag.\n%s, which needs a mapping like:\n\n"+
			"  - canonical: %s\n%s    sources:\n      - file:%s\n\n"+
			"Point -mappings at the file the `backend-stress` profile uses, or add the field.",
		subject, mappingsPath, hint.why, canonical, multi, hint.tag)
}

// linkTags renders a list of entity names as file-layer rows for this field.
//
// One row per name rather than one delimited row: the resolver splits multi
// values on any of , ; / and newline (mapping.SplitMulti), so a joined string
// would silently fracture any name containing one of those. Separate rows carry
// the same meaning — video_metadata has no unique key on (video_id, source_key)
// and the resolver unions the rows — with nothing to escape.
func (ff fileField) linkTags(names []string) ([]model.ExtraMetadata, error) {
	out := make([]model.ExtraMetadata, 0, len(names))
	for _, name := range names {
		// Separate rows avoid the delimiter problem for the tag itself, but not
		// for the name inside it: the resolver splits every value it reads. A
		// name carrying a separator would be derived as two entities while the
		// seeder's own reconcile counted it as one, so the fixture and the served
		// page would disagree about a number the manifest states as fact.
		if strings.ContainsAny(name, multiValueSeparators) {
			return nil, fmt.Errorf(
				"fixture name %q contains one of %q, which the resolver splits on "+
					"(mapping.SplitMulti) — the server would derive more entities than the "+
					"manifest claims", name, multiValueSeparators)
		}
		out = append(out, model.ExtraMetadata{SourceKey: ff.fileKey, Value: name})
	}
	return out, nil
}

// multiValueSeparators mirrors mapping.SplitMulti's separator set. Duplicated
// rather than imported because SplitMulti exposes the behaviour, not the set;
// the guard above exists to fail loudly if the two ever drift apart.
const multiValueSeparators = ",;/\n"
