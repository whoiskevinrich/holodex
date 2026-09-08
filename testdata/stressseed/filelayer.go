package main

import (
	"fmt"
	"os"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
)

// video_people is a DERIVED table, not an authored one (ADR-072). Its source is
// the video's resolved person-typed fields, which the resolver reads off the file
// layer — and cmd/holodex re-derives every video's links from that source on
// startup (backfillPersonLinks).
//
// So a fixture that writes video_people directly is writing a cache. The first
// version of this seeder did exactly that, and the rows survived until the server
// booted: the startup backfill resolved each video to zero actors, wiped all 50
// links and orphan-stamped every person. The fixture destroyed itself the moment
// it was served, which is the worst possible failure — wrong data at a stable
// address is precisely what the addressing scheme exists to prevent.
//
// The fix is to seed the baseline truth instead. Writing a file tag the mapping
// maps to `actors` makes the derivation produce the links itself, so the startup
// backfill agrees with the fixture rather than erasing it.

// personField is the canonical person-typed field the fixture builds its cast
// through, plus the file-tag key the *configured* mapping reads it from.
//
// The key cannot be hardcoded: which tag carries actors is a per-instance
// decision in metadata-mappings.yaml (Cast, Artist, Performer, …). Reading it
// from the same file the server reads is what makes the seeded value and the
// derived value the same value.
type personField struct {
	canonical string // "actors"
	role      string // the video_people.role the registry gives that field
	fileKey   string // the file tag the mapping resolves it from
}

// loadPersonField finds the first person-typed field whose configured mapping
// has a file-layer source, which is the field the fixture can populate from the
// baseline layer.
//
// It fails rather than falling back. A silent fallback to writing video_people
// directly would produce a people ladder that looks right in the database and
// empties itself on the next boot, and a fixture that lies is worse than one
// that refuses to build.
func loadPersonField(mappingsPath string) (personField, error) {
	// mapping.Load treats a missing file as an empty mapping rather than an error,
	// which would send someone off to add an actors field to a file that is not
	// there. The default path (./metadata-mappings.yaml) does not exist in a
	// worktree, so this is the common case, not the exotic one — check first and
	// say which of the two problems it actually is.
	if _, err := os.Stat(mappingsPath); err != nil {
		return personField{}, fmt.Errorf(
			"cannot read metadata mappings at %s: %w.\n"+
				"Point -mappings at the file the `backend-stress` profile passes the server\n"+
				"as METADATA_MAPPINGS_PATH — the fixture and the server have to agree on it",
			mappingsPath, err)
	}

	// Load, not NewStore: the seeder reads the mapping once and never watches it.
	m, err := mapping.Load(mappingsPath)
	if err != nil {
		return personField{}, fmt.Errorf("load metadata mappings from %s: %w", mappingsPath, err)
	}

	for _, def := range registry.PersonTypedFields() {
		f, ok := m.ByCanonical(def.Canonical)
		if !ok {
			continue
		}
		for _, src := range f.ParsedSources {
			// file:title aliases the videos.title column rather than a file tag,
			// so it cannot carry a cast list.
			if src.Namespace == "file" && !src.IsFileTitle() {
				return personField{canonical: def.Canonical, role: def.Role, fileKey: src.Key}, nil
			}
		}
	}

	return personField{}, fmt.Errorf(
		"no person-typed field in %s maps to a file tag.\n"+
			"The fixture builds its cast through the file layer so the server's startup\n"+
			"relink derives the same links instead of wiping them, which needs a mapping\n"+
			"like:\n\n"+
			"  - canonical: actors\n    label: Actors\n    multi: true\n    sources:\n      - Cast\n\n"+
			"Point -mappings at the file the `backend-stress` profile uses, or add the field.",
		mappingsPath)
}

// castTags renders a cast as file-layer rows.
//
// One row per name rather than one delimited row: the resolver splits multi
// values on any of , ; / and newline (mapping.SplitMulti), so a joined string
// would silently fracture any name containing one of those. Separate rows carry
// the same meaning — video_metadata has no unique key on (video_id, source_key)
// and the resolver unions the rows — with nothing to escape.
func (pf personField) castTags(names []string) ([]model.ExtraMetadata, error) {
	out := make([]model.ExtraMetadata, 0, len(names))
	for _, name := range names {
		// Separate rows avoid the delimiter problem for the tag itself, but not
		// for the name inside it: the resolver splits every value it reads. A
		// name carrying a separator would be derived as two people while the
		// seeder's own reconcile counted it as one, so the fixture and the served
		// page would disagree about a number the manifest states as fact.
		if strings.ContainsAny(name, multiValueSeparators) {
			return nil, fmt.Errorf(
				"fixture name %q contains one of %q, which the resolver splits on "+
					"(mapping.SplitMulti) — the server would derive more people than the "+
					"manifest claims", name, multiValueSeparators)
		}
		out = append(out, model.ExtraMetadata{SourceKey: pf.fileKey, Value: name})
	}
	return out, nil
}

// multiValueSeparators mirrors mapping.SplitMulti's separator set. Duplicated
// rather than imported because SplitMulti exposes the behaviour, not the set;
// the guard above exists to fail loudly if the two ever drift apart.
const multiValueSeparators = ",;/\n"
