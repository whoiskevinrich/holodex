package resolver_test

import (
	"slices"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// personTestFields mirrors the shape the API synthesizes for a person (F37):
// name is the only baseline-backed field, scalars are provider-only, aliases
// is a merge field. photo is excluded (asset, not a field).
func personTestFields() []mapping.Field {
	return []mapping.Field{
		stubField("name", false, "file:name", "tmdb:name"),
		stubField("bio", false, "tmdb:bio"),
		stubField("birthdate", false, "tmdb:birthdate"),
		stubField("nationality", false, "tmdb:nationality"),
		stubField("website", false, "tmdb:website"),
		mergeField("aliases", "tmdb:aliases"),
	}
}

var (
	testPerson   = &model.Person{ID: 7, Name: "Hayao Miyazaki"}
	personEnrich = resolver.Enrichment{"tmdb": {
		"name":        {"Miyazaki Hayao"},
		"bio":         {"Japanese filmmaker."},
		"birthdate":   {"1941-01-05"},
		"nationality": {"Japan"},
		"website":     {"https://example.org"},
		"aliases":     {"宮崎駿", "Miyazaki-san"},
	}}
)

// resolvedByCanonical pulls one canonical field out of a resolved set, reporting
// whether it survived resolution at all (a decided-empty field drops).
func resolvedByCanonical(fields []resolver.ResolvedField, canonical string) (resolver.ResolvedField, bool) {
	for _, f := range fields {
		if f.Canonical == canonical {
			return f, true
		}
	}
	return resolver.ResolvedField{}, false
}

// --- personBaseline (P0-1 / QA 2.1) ------------------------------------------------

func TestPersonBaseline_NameResolvesFromRecord(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), personEnrich, nil, personTestFields(), resolver.Options{})
	name, ok := resolvedByCanonical(got, "name")
	if !ok {
		t.Fatal("name field missing")
	}
	// The record name wins the file-first default over the provider spelling.
	if name.Values[0] != "Hayao Miyazaki" || name.WinningSource != "file:name" {
		t.Fatalf("record name should win: %+v", name)
	}
	// The provider spelling survives as a selectable candidate; the baseline
	// candidate is anchored first.
	if len(name.Candidates) != 2 || name.Candidates[0].Source != "file" || name.Candidates[0].Value != "Hayao Miyazaki" {
		t.Errorf("baseline candidate should anchor first: %+v", name.Candidates)
	}
	if v := providerCandidate(name, "tmdb"); v != "Miyazaki Hayao" {
		t.Errorf("want tmdb name candidate, got %q", v)
	}
}

func TestPersonBaseline_ClaimsNamespaceWithEmptyValue(t *testing.T) {
	// A hypothetical baseline source on a non-name key is claimed (ok=true) with
	// no value, so file-first ordering does not fall through to a provider *for
	// that source* — the provider source resolves on its own merits.
	fields := []mapping.Field{stubField("bio", false, "file:bio", "tmdb:bio")}
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), personEnrich, nil, fields, resolver.Options{})
	bio, ok := resolvedByCanonical(got, "bio")
	if !ok || bio.Values[0] != "Japanese filmmaker." || bio.WinningSource != "tmdb:bio" {
		t.Fatalf("empty baseline must yield to the provider source: %+v", got)
	}
	// The baseline candidate is still offered — with an empty value (RD3).
	if len(bio.Candidates) == 0 || bio.Candidates[0].Source != "file" || bio.Candidates[0].Value != "" {
		t.Errorf("empty baseline candidate should anchor first: %+v", bio.Candidates)
	}
}

func TestPersonBaseline_NilPersonIsEmptyBaseline(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewPersonBaseline(nil), personEnrich, nil, personTestFields(), resolver.Options{})
	name, ok := resolvedByCanonical(got, "name")
	if !ok || name.Values[0] != "Miyazaki Hayao" || name.WinningSource != "tmdb:name" {
		t.Fatalf("empty record name should fall through to the provider: %+v", got)
	}
}

// --- RD6 additivity (cardinal, QA 2.2) ---------------------------------------------

func TestPersonBaseline_RD6Additivity(t *testing.T) {
	// An enriched person with zero decisions resolves every synthesized field to
	// exactly the raw enrichment values (name aside, which is the record's own) —
	// the refactor changes nothing until the owner decides.
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), personEnrich, nil, personTestFields(), resolver.Options{})
	for canonical, want := range map[string][]string{
		"name":        {testPerson.Name},
		"bio":         personEnrich["tmdb"]["bio"],
		"birthdate":   personEnrich["tmdb"]["birthdate"],
		"nationality": personEnrich["tmdb"]["nationality"],
		"website":     personEnrich["tmdb"]["website"],
		"aliases":     personEnrich["tmdb"]["aliases"],
	} {
		f, ok := resolvedByCanonical(got, canonical)
		if !ok {
			t.Errorf("%s: field missing", canonical)
			continue
		}
		if !slices.Equal(f.Values, want) {
			t.Errorf("%s: undecided resolution must equal raw enrichment: got %v want %v", canonical, f.Values, want)
		}
	}
}

// --- Decision short-circuit for person fields (QA 2.3) -----------------------------

func TestPersonBaseline_RecordBlankPinSuppressesProvider(t *testing.T) {
	// A standing record decision on an enrichment-only field pins it to the empty
	// baseline: the provider value is suppressed, but the field STAYS in the
	// resolved set with no values (F37 RD3) — dropping it would hide the pin and
	// leave no control to change or clear it. A re-enrich can never resurrect the
	// provider value while the pin stands.
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), personEnrich, nil, personTestFields(), decide("bio", "file", ""))
	f, ok := resolvedByCanonical(got, "bio")
	if !ok {
		t.Fatalf("blank-pinned field must stay visible, got %+v", got)
	}
	if len(f.Values) != 0 {
		t.Fatalf("blank-pinned field must carry no values, got %+v", f.Values)
	}
	if f.Decision == nil || !f.Decision.Standing || f.Decision.Source != "file" {
		t.Errorf("want standing file (record) decision, got %+v", f.Decision)
	}
	if len(f.Candidates) == 0 {
		t.Errorf("blank-pinned field must keep its candidates so the pin can be re-decided, got %+v", f)
	}
	// The other fields are untouched by the bio pin.
	if f, ok := resolvedByCanonical(got, "nationality"); !ok || f.Values[0] != "Japan" {
		t.Errorf("unrelated fields must keep resolving: %+v", got)
	}
}

func TestPersonBaseline_ProviderPinFollowsReEnrich(t *testing.T) {
	// The pin is on the *source*: a re-enriched value flows straight through.
	reenriched := resolver.Enrichment{"tmdb": {"bio": {"Refreshed biography."}}}
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), reenriched, nil, personTestFields(), decide("bio", "provider:tmdb", ""))
	f, ok := resolvedByCanonical(got, "bio")
	if !ok || f.Values[0] != "Refreshed biography." {
		t.Fatalf("provider pin must follow the live enrichment value: %+v", got)
	}
	if f.Decision == nil || !f.Decision.Standing || f.Decision.Source != "provider:tmdb" {
		t.Errorf("want standing provider decision, got %+v", f.Decision)
	}
}

func TestPersonBaseline_ManualPinStaysFrozen(t *testing.T) {
	reenriched := resolver.Enrichment{"tmdb": {"bio": {"Provider churn."}}}
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson), reenriched, nil, personTestFields(), decide("bio", "manual", "My own words."))
	f, ok := resolvedByCanonical(got, "bio")
	if !ok || f.Values[0] != "My own words." {
		t.Fatalf("manual pin must stay frozen: %+v", got)
	}
}
