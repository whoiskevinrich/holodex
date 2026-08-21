package resolver

import (
	"holodex/internal/mapping"
	"holodex/internal/model"
)

// filmBaseline is the film implementation of BaselineSource (F56, ADR-085 §3):
// the record layer, mirroring personBaseline/studioBaseline. A film's only
// intrinsic field is its name — description/release_date/poster have an empty
// baseline, so under the file-first default an undecided enrichment-only field
// keeps resolving to the provider value (the RD6 additivity property). The
// baseline still claims every key of the baseline namespace (ok=true with no
// value), so resolution never falls through to a provider for a baseline source —
// the ADR-052 ownership rule that makes a standing "record" blank-pin suppress a
// provider value.
//
// Cast and tags are deliberately NOT resolved through filmBaseline: they are a
// read-only set union over the film's attached videos' own video_people/video_tags
// links, assembled by the API layer as a separate read (ADR-085 §3) — forcing a
// union query through the single-value Baseline(src) interface would be a fiction.
//
// The namespace token stays "file": it is the resolver-internal name of the
// baseline layer, shared with the fieldsource grammar and the decision store. The
// film API layer presents it as "record" at the payload edge, exactly as the
// studio/person layers do.
type filmBaseline struct {
	name string
}

// NewFilmBaseline builds the record-layer BaselineSource for a film from its row.
// f may be nil (an empty baseline).
func NewFilmBaseline(f *model.Film) BaselineSource {
	b := filmBaseline{}
	if f != nil {
		b.name = f.Name
	}
	return b
}

// Baseline resolves a baseline-namespace source against the film record: the
// name key → films.name; every other key is claimed with no value.
func (b filmBaseline) Baseline(src mapping.Source) ([]string, bool) {
	if src.Namespace != "file" {
		return nil, false
	}
	if normKey(src.Key) == "name" && b.name != "" {
		return []string{b.name}, true
	}
	return nil, true
}
