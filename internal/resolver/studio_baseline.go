package resolver

import (
	"holodex/internal/mapping"
	"holodex/internal/model"
)

// studioBaseline is the studio implementation of BaselineSource: the record layer
// (F38, ADR-053), mirroring personBaseline. A studio's only intrinsic field is its
// name — every other canonical studio field has an empty baseline, so under the
// file-first default an undecided enrichment-only field keeps resolving to the
// provider value (the RD6 additivity property). The baseline still claims every
// key of the baseline namespace (ok=true with no value), so resolution never falls
// through to a provider for a baseline source — the ADR-052 ownership rule that
// makes a standing "record" blank-pin suppress a provider value.
//
// The namespace token stays "file": it is the resolver-internal name of the
// baseline layer, shared with the fieldsource grammar and the decision store. The
// studio API layer presents it as "record" at the payload edge (RD5), exactly as
// the person layer does.
type studioBaseline struct {
	name string
}

// NewStudioBaseline builds the record-layer BaselineSource for a studio from its
// row. s may be nil (an empty baseline).
func NewStudioBaseline(s *model.Studio) BaselineSource {
	b := studioBaseline{}
	if s != nil {
		b.name = s.Name
	}
	return b
}

// Baseline resolves a baseline-namespace source against the studio record: the
// name key → studios.name; every other key is claimed with no value.
func (b studioBaseline) Baseline(src mapping.Source) ([]string, bool) {
	if src.Namespace != "file" {
		return nil, false
	}
	if normKey(src.Key) == "name" && b.name != "" {
		return []string{b.name}, true
	}
	return nil, true
}
