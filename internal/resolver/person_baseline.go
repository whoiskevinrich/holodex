package resolver

import (
	"holodex/internal/mapping"
	"holodex/internal/model"
)

// personBaseline is the person implementation of BaselineSource: the record
// layer (F37, ADR-052). A person's only intrinsic field is its name — every
// other canonical person field has an empty baseline, so under the file-first
// default an undecided enrichment-only field keeps resolving to the provider
// value (the RD6 additivity property). The baseline still claims every key of
// the baseline namespace (ok=true with no value), so resolution never falls
// through to a provider for a baseline source — the ADR-052 ownership rule
// that makes a standing "record" blank-pin suppress a provider value.
//
// The namespace token stays "file": it is the resolver-internal name of the
// baseline layer, shared with the fieldsource grammar and the decision store.
// The person API layer presents it as "record" at the payload edge (RD4).
type personBaseline struct {
	name string
}

// NewPersonBaseline builds the record-layer BaselineSource for a person from
// its row. p may be nil (an empty baseline).
func NewPersonBaseline(p *model.Person) BaselineSource {
	b := personBaseline{}
	if p != nil {
		b.name = p.Name
	}
	return b
}

// Baseline resolves a baseline-namespace source against the person record:
// the name key → people.name; every other key is claimed with no value.
func (b personBaseline) Baseline(src mapping.Source) ([]string, bool) {
	if src.Namespace != "file" {
		return nil, false
	}
	if normKey(src.Key) == "name" && b.name != "" {
		return []string{b.name}, true
	}
	return nil, true
}
