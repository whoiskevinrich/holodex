package api

import (
	"context"
	"strings"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Film billed cast (F59/ADR-089 D1/D2, HOLODEX-310).
//
// A film's Cast is the read-only union of its attached scenes' people (ADR-085 RD3).
// A provider additionally supplies the cast billed on the release. Those are two
// different claims and both are true, so the page shows the union in full and then
// only its COMPLEMENT — "billed on the release, in no scene you own". At realistic
// scale (a 10-person union against TMDB's 20-name window) rendering both lists in full
// would be roughly half duplicates; the difference is the only rendering where the
// second group carries information the first does not, and it is exactly the
// scene-coverage signal ADR-085 deferred as P1-3.
//
// **Storage: none.** ADR-089 D1 originally had this landing in `film_people_roles`,
// which is keyed by person_id and would therefore require creating a Person row for
// every billed performer — including ones the library holds no footage of, who would
// then appear in /people with empty pages. The billed list is already persisted in the
// additive enrichment shadow (ADR-033) as the provider's `actors` field, so the
// difference is computed at read time from data that is already there. That keeps
// everything D1 actually decided (film-level, no video mutated, fully reversible, the
// two claims kept distinct) and drops the collateral entity creation. Clearing the
// provider removes the shadow rows and the group disappears on its own.
// `film_people_roles` stays a purely owner-asserted table with no provider writer.

// billedCastField is the provider's flat cast key (contract §4.2a `actors`). The
// structured people[] channel is video-only, so a film's billed cast arrives here.
const billedCastField = "actors"

// FilmBilledCredit is one provider-billed cast member absent from every scene the
// owner holds. PersonID is set only when the name already resolves to a person in the
// library (someone present elsewhere but not in THIS film's scenes) — that person gets
// a link; a name that resolves to nobody is inert text, never a created entity.
type FilmBilledCredit struct {
	Name     string `json:"name"`
	PersonID int64  `json:"person_id,omitempty"`
}

// filmBilledCast returns the billed cast absent from the scene union, and how many
// distinct names the provider billed in total. Both are zero-valued when no provider
// has supplied a cast, so a film with no enrichment renders exactly as it did before.
//
// Matching is by resolved identity, not display string: each billed name goes through
// repo.LookupEntityIDByName, which consults the canonical nameKey and then the alias
// table. So "Timothee Chalamet" against a canonical "Timothée Chalamet", or a
// merged-away name reaching its survivor, both count as COVERED rather than being
// reported as a phantom absence. The false-positive direction matters more than the
// false-negative one here: wrongly telling an owner their complete rip is incomplete is
// worse than quietly omitting one genuinely missing performer.
func (h *Handlers) filmBilledCast(ctx context.Context, rows []repo.EnrichmentRow, union []model.Person) ([]FilmBilledCredit, int) {
	// Keyed by id AND by name: the id set answers "did the alias-aware lookup land on a
	// union member", while the name map answers the same question without a query AND
	// still yields the id, which the dedupe below needs. Storing only a bool here was a
	// bug — the no-query path could not register the person, so two spellings of one
	// union member counted as two billed credits.
	inUnion := make(map[int64]bool, len(union))
	unionIDByName := make(map[string]int64, len(union))
	for _, p := range union {
		inUnion[p.ID] = true
		unionIDByName[normalizedName(p.Name)] = p.ID
	}

	absent := []FilmBilledCredit{}
	seen := map[string]bool{}
	seenID := map[int64]bool{}
	total := 0

	for _, row := range rows {
		if row.FieldKey != billedCastField {
			continue
		}
		for _, raw := range row.Values {
			name := strings.TrimSpace(raw)
			key := normalizedName(name)
			if name == "" || seen[key] {
				continue
			}
			seen[key] = true

			// Fast path, and the common one: a billed name matching a union member's name
			// outright is covered, so skip the lookup entirely. Without this every billed
			// name costs a query (up to two — canonical then alias) on a public, uncached
			// endpoint, and a fully-covered film pays ~20 of them per page load to learn
			// what this map already knows. It must still register the id, or a second
			// spelling of the same person would be counted again. A name that misses falls
			// through to the alias-aware lookup, so the result is unchanged either way.
			if uid, ok := unionIDByName[key]; ok {
				if seenID[uid] {
					continue
				}
				seenID[uid] = true
				total++
				continue
			}

			id, found, err := h.repo.LookupEntityIDByName(ctx, model.EnrichEntityPerson, name)
			if err != nil {
				// A lookup failure must not cost the page its cast section; treat the
				// name as unresolvable and let the string comparison below decide.
				h.log.Warn("film billed cast: person lookup", "name", name, "err", err)
				found = false
			}
			// Two providers can bill the same person under different spellings; dedupe
			// on the resolved id so they collapse to one entry rather than two.
			if found {
				if seenID[id] {
					continue
				}
				seenID[id] = true
			}
			total++

			// Covered when the billed name resolves, through the alias spine, to someone
			// already in the union — a name matching a union member's name outright was
			// handled by the fast path above and never reaches here.
			if found && inUnion[id] {
				continue
			}
			absent = append(absent, FilmBilledCredit{Name: name, PersonID: idOrZero(found, id)})
		}
	}
	return absent, total
}

// normalizedName mirrors the identity spine's person nameKey (lower(trim(...))) for the
// in-memory comparisons above, so the string fallback folds the same way the SQL does.
func normalizedName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func idOrZero(found bool, id int64) int64 {
	if found {
		return id
	}
	return 0
}
