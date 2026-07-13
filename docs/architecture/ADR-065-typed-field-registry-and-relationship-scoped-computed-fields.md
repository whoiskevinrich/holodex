# ADR-065: Typed field registry + relationship-scoped computed fields (queryable-fields substrate, Phase 1)

**Status:** Superseded
**Date:** 2026-07-12
**Deciders:** Project owner

**Extends:** [ADR-063](ADR-063-derived-computed-fields.md) (the compute-on-read, non-adoptable derived-field genre — this ADR is its relationship-scoped twin) · [ADR-052](ADR-052-baseline-source-contract.md) (entity-agnostic `ResolveFields` core; `registry.FieldDef` the canonical vocabulary both draw from) · [ADR-013](ADR-013-metadata-field-mapping.md) (`FieldDef`/`mapping.Field` registry this adds a type dimension to). **Realizes the deferred item in:** [ADR-062](ADR-062-in-app-field-promotion.md) §D-filterable ("promote to facet… needs the browse-facet value-validation review") — this ADR builds the *type/operator taxonomy* that item was blocked on, without itself wiring a facet (see Non-Goals). **Spec:** [Queryable fields substrate — Phase 1](../specs/queryable-fields-substrate.md) (F46 Phase 1). **Issue:** [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) (epic [HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178)). **Security:** `/security-review` not expected — read-only computed values, no new auth/access/infra surface (confirm at implementation time, per spec).

**Superseded (2026-07-12):** [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) was closed Won't Do the same day this ADR was written — the generic typed-field-registry substrate proposed here had zero shipped consumers left to justify it (per the ticket's closing comment: building it now would be speculative infrastructure against this repo's simplicity-first working agreement). Its pilot case, Age in Media, shipped instead as a narrow, bespoke API-layer join with no registry involvement — [HOLODEX-173](https://whoiskevinrich.atlassian.net/browse/HOLODEX-173), spec [`age-in-media.md`](../specs/age-in-media.md). The design recorded below is not wrong, just unbuilt — epic [HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178) stays open, and this ADR should be revisited (confirmed, revised, or replaced) if a future phase — most likely [HOLODEX-180](https://whoiskevinrich.atlassian.net/browse/HOLODEX-180) — actually needs the generic substrate.

---

## Context

The spec resolved three product-level questions with the owner (which video-side date feeds Age in Media, how much of the typed model Phase 1 builds, whether a value index is persisted) but explicitly left two **architecture-level** questions open:

1. **Which existing canonicals get a `FieldType` in this pass** — all of them, or just enough to prove each of the four types?
2. **The exact shape/injection boundary of the relationship-scoped compute pass** — a `DeriveRelationship`-shaped function vs. folding into the video/person handler directly vs. something else — and, not asked explicitly but forced by the code: **where does a relationship-scoped value live once computed**, since it belongs to neither entity's own resolved field list?

This ADR answers both, plus the mechanical question the spec's FR1–FR3 raise but don't settle: the *type* of `registry.FieldDef.Type` and the operator-taxonomy structure (FR1/FR2), and — new relative to ADR-063 — how a **two-entity** input (person × video) reaches a pure derive pass without breaking the one-entity shape `Derive(resolved, now)` already commits to.

### Current state (survey, 2026-07-12)

| Seam | Today | File |
|---|---|---|
| `registry.FieldDef` | `{Canonical, Label, Display, Description, Computed, DependsOn}` — no type/operator concept (ADR-063) | `internal/registry/registry.go:19` |
| `resolver.Derive(resolved, now)` | Pure post-pass over **one** entity's `[]ResolvedField`; a formula reads `map[string]string` built from that one slice (`firstValues`) | `internal/resolver/derive.go:26` |
| Formula registry | `derivations map[string]formula` keyed by canonical, `func(in map[string]string, now) (value string, ok bool)` — single-entity input only | `internal/resolver/derive.go:66` |
| Video detail read | `getMedia` resolves the video's own fields into one `resolved []resolver.ResolvedField`; separately, `v.People []model.Person` is a **name+id-only** batch join (`attachAssociations`) — no birthdate, no resolve pipeline | `internal/api/handlers.go:433`, `internal/repo/repo.go:577` |
| Person detail read | `personResolved(r, id, p)` runs the full pipeline (enrichment + curation + decisions + `Derive`) for **one** person, given an `*http.Request` | `internal/api/person_fields.go:110` |
| Decision-endpoint guard | `personReplaceField` rejects any `registry.Lookup(canonical).Computed` canonical with 400 — **generic**, keyed off the registry, not person-specific code | `internal/api/person_decisions.go:108` |
| `model.Person` / `model.Video` | Both live in `internal/model`, which `internal/resolver` **imports** — `model` cannot import `resolver` back (cycle) | `internal/model/model.go:24,54` |

### Forces

- **The one-entity `Derive` shape must not be bent to fit a two-entity input.** `Derive(resolved, now)` reads one entity's own resolved rows; forcing `age_in_media` into that loop would look for `release_date` on a *person's* resolved rows (never present) and `birthdate` on a *video's* (never present) — harmless (never computable) but a wrong abstraction, and it would silently normalize as "just another computed field" when it fundamentally isn't one.
- **A relationship value has no home in either entity's own field list** (new relative to ADR-063). Age is a fact about a *person*; Age in Media is a fact about a *person having appeared in a specific video* — it cannot be appended to the person's `resolved[]` (no video context there) or the video's `resolved[]` (one row per canonical, not per cast member).
- **`internal/model` cannot import `internal/resolver`** (cycle: resolver already imports model). Whatever holds the relationship-computed value cannot be a new field on `model.Person`/`model.Video` typed as `resolver.ResolvedField`.
- **Resolver purity is non-negotiable** (ADR-051, carried by ADR-063). The relationship pass gains no clock; `now` stays a parameter, injected at the API handler boundary exactly as `Derive` already is.
- **Reuse, don't re-derive, per-field correctness.** A cast member's birthdate may carry a standing per-field source decision (ADR-051) or curation (F30) that a raw shadow-table read would miss. The existing `personResolved` already applies all of that; re-deriving a parallel "just read birthdate" path would risk drifting out of sync with it.
- **Prove the taxonomy without over-building it** (spec D2/D3). Phase 1 needs the full 4-type/3-operator *model* as a declared contract, but zero consumers wire an operator to a UI or a query this phase — so the operator map is data, not behavior.
- **Scale is bounded.** This is a personal, single-owner media server (not a multi-tenant product); a video's cast list is a handful of people. A design that costs a few extra queries *per video-detail page view* (not per list-page row) is proportionate; a batched, index-backed version is a deferred optimization, not a Phase 1 requirement.

---

## Decision

Add the **typed field registry** (`FieldType` + operator taxonomy) to `registry.FieldDef` as declarative, consumer-free data (FR1/FR2). Add a **second, relationship-scoped derive pass**, `resolver.DeriveRelationship`, that is `Derive`'s two-entity twin rather than a bent version of it — same pure, non-adoptable, clock-injected shape, but taking **two** already-resolved slices and returning a **standalone** slice of rows that belongs to neither entity's own field list. The API handler (`getMedia`) is the injection boundary: it already resolves the video's own fields, and for Phase 1's bounded cast-list scale, it additionally calls the existing `personResolved` per cast member (reuse, not a new query path) to get a correctness-preserving birthdate, then calls `DeriveRelationship` and attaches the result as a **new, parallel top-level payload key** — never mutating `model.Person`/`model.Video` or their existing field lists — resolving the `model`↔`resolver` import-cycle constraint for free.

### D1 — `FieldType` + operator taxonomy live on `registry.FieldDef`, as inert declarative data

```go
// FieldType classifies a field's comparison semantics (F46 Phase 1) — text,
// categorical, numeric, or date. It is a declarative contract for a future
// operator/query surface; Phase 1 registers it but wires no consumer.
type FieldType string

const (
    TypeText        FieldType = "text"        // free-text; default when unset
    TypeCategorical FieldType = "categorical"  // bounded value set
    TypeNumeric     FieldType = "numeric"
    TypeDate        FieldType = "date"
)

type FieldDef struct {
    // ... existing Canonical/Label/Display/Description/Computed/DependsOn ...
    Type FieldType // "" behaves as TypeText (mirrors Display's "" == inline-text default)
}

// Operator is one comparison a FieldType supports.
type Operator string

const (
    OpEquals   Operator = "equals"
    OpContains Operator = "contains"
    OpRange    Operator = "range"
)

// OperatorsByType is the FieldType → valid-operator-set contract (FR2). Pure data —
// no caller consults it this phase; Phase 2 (Nationality, categorical/equals) and a
// future facet UI are the first consumers.
var OperatorsByType = map[FieldType][]Operator{
    TypeText:        {OpContains},
    TypeCategorical: {OpEquals},
    TypeNumeric:     {OpEquals, OpRange},
    TypeDate:        {OpEquals, OpRange},
}
```

**Registration scope (settles Open Item 1): the minimum set that proves the taxonomy, not a backfill of every canonical.** Per FR1's own acceptance criterion, only: `birthdate`/`release_date` → `TypeDate`, the new `age_in_media` → `TypeNumeric`, plus **one existing representative each** for `TypeText` and `TypeCategorical` — `title` (free text) and `status` (release status is a genuinely bounded set: Released/Post Production/In Production/…). No other `KnownFields` entry gains a `Type` this phase. Rationale: FieldType has zero consumers in Phase 1 (D2/D3 of the spec); annotating the full registry now would be speculative work with nothing to verify it against. Phase 2/3 annotate their own fields as they're built — a registration, not a rewrite (spec Goal 2/Success Metric 3).

### D2 — `DeriveRelationship`: a second pass, not a bent `Derive`

`Derive(resolved, now)` stays exactly as ADR-063 defined it — untouched, still iterating only entity-intrinsic `Computed` fields. A **new, parallel** function is added rather than extending `Derive`'s loop to sometimes expect two inputs:

```go
// DeriveRelationship appends one computed ResolvedField per registered
// relationship-scoped canonical (F46 Phase 1, ADR-063's two-entity twin) whose
// person-side and video-side inputs are both present and parseable. Pure: reads
// only already-resolved values from the two input slices; `now` is the sole clock
// source — internal/resolver gains no clock (ADR-051, carried from ADR-063).
//
// Unlike Derive, the returned rows are NOT inserted into either input slice — a
// relationship value (e.g. this person's age *in this video*) has no home in
// either entity's own resolved[] (ADR-052/063 contract for both stays exactly as
// today). The caller (the API handler) attaches the result to the pairing it
// describes.
func DeriveRelationship(person []ResolvedField, video []ResolvedField, now time.Time) []ResolvedField
```

Registry marker: a `Computed` field is **relationship-scoped** iff it declares person-side and video-side inputs separately, via two new `FieldDef` slices (rather than overloading the existing `DependsOn`, which stays entity-intrinsic per ADR-063 and is read by `Derive`'s existing loop):

```go
type FieldDef struct {
    // ... existing fields, plus D1's Type ...
    PersonDependsOn []string // relationship-scoped only: canonicals read from the person side
    VideoDependsOn  []string // relationship-scoped only: canonicals read from the video side
}
```

`age_in_media`: `Computed: true`, `Type: TypeNumeric`, `PersonDependsOn: ["birthdate"]`, `VideoDependsOn: ["release_date"]`. A field is relationship-scoped iff either list is non-empty — no separate bool; the two lists **are** the marker, so there is nothing to drift out of sync. `Derive`'s existing filter (`computedFields`, entity-intrinsic) and a new `relationshipFields` filter partition `KnownFields` by this predicate — mutually exclusive, so a field is iterated by exactly one pass.

Formula shape mirrors `Derive`'s exactly, widened to two input maps:

```go
type relationshipFormula func(person, video map[string]string, now time.Time) (value string, computable bool)

var relationshipDerivations = map[string]relationshipFormula{
    "age_in_media": deriveAgeInMedia,
}

func deriveAgeInMedia(person, video map[string]string, now time.Time) (string, bool) {
    bd, ok := parseDate(person["birthdate"])
    if !ok {
        return "", false
    }
    rd, ok := parseDate(video["release_date"])
    if !ok {
        return "", false
    }
    age := wholeYearsBetween(bd, rd)
    if age < 0 {
        return "", false // release before birth — nonsensical, omit (mirrors deriveAgeAtDeath)
    }
    return strconv.Itoa(age), true
}
```

Each emitted row is stamped exactly like `Derive`'s: `Computed: true`, `WinningSource: fieldsource.ForComputed("age_in_media")`, `DerivedFrom` naming both sides' labels (`["Born", "Released"]`), nil `Decision`/`Candidates`/`InSync` — **the same non-adoptable guarantee, inherited automatically**: `registry.Lookup("age_in_media").Computed == true` already trips the existing generic decision-endpoint guard (`internal/api/person_decisions.go:108`) with zero new code, because that guard keys off the registry, not a person-specific predicate.

There is no "insert after dependency" positional step (unlike `Derive`'s `insertComputed`) — a relationship row has no single host list to insert into; `DeriveRelationship` returns a small standalone slice (0 or 1 row in Phase 1) for the caller to place.

### D3 — Injection boundary: `getMedia` reuses `personResolved` per cast member; output is a new top-level payload key, never `model.Person`/`model.Video`

**Where the two inputs come from.** The video's own `resolved` is already computed once in `getMedia` (`internal/api/handlers.go:478`) — reused as-is for the `video` side. For the `person` side, `getMedia` calls the **existing** `h.personResolved(r, p.ID, &p)` once per linked person in `v.People`. This is a deliberate reuse, not a new read path: it is the one function that already applies enrichment + curation + standing per-field decisions correctly for a person, so a cast member's Age-in-Media birthdate is exactly as correct as their own person-page Age. Building a leaner "just fetch birthdate" batched query was considered and rejected for Phase 1 (Options, below) — the birthdate must go through the full correctness pipeline, and a video's cast list is small enough that N calls to an existing function is proportionate at this project's scale.

**Where the output goes.** `model.Person`/`model.Video` gain **no new field** — `internal/model` cannot import `internal/resolver` (cycle: `resolver` already imports `model`), so a `resolver.ResolvedField`-shaped value cannot live on either struct. Instead, `getMedia`'s response gains a new, parallel top-level key, keyed by person ID, alongside the existing `"resolved"` key that already carries the video's own fields:

```go
writeJSON(w, http.StatusOK, map[string]any{
    "video":                  v,
    "metadata":               extra,
    "fields":                 fields,
    "resolved":               resolved,               // the video's own fields (unchanged)
    "enriched":                enriched,
    "studios":                 studios,
    "related_person_fields":  castAges,               // NEW — map[int64][]resolver.ResolvedField, keyed by person.ID
})
```

`castAges` holds, per person ID, the (0 or 1, in Phase 1) relationship-scoped rows `DeriveRelationship` returned for that pairing. The key is named for the *relationship*, not the one Phase-1 field (`age_in_media`) — Phase 3's role-scoped joins (HOLODEX-180/181) are also person×video and can add rows to the same map without a payload reshape. This satisfies FR8 (structured field data via the existing payload, no dedicated endpoint) the same way F45's `resolved[]` did, without conflating a per-canonical (video-scoped) list with a per-pairing (relationship-scoped) one.

The frontend (`web/src/routes/media/[id]/+page.svelte`, `PersonPoster.svelte`) reads `related_person_fields[person.id]` to find the `age_in_media` row for that cast member — exact rendering/placement is a `/design-handoff` call (spec Open Question).

### What is explicitly *not* in this ADR

- **A persisted, queryable value index** — spec D3; no consumer this phase.
- **Any operator wired to a UI or a query path** — spec D2/Non-Goal; `OperatorsByType` is data only.
- **A generic N-entity relationship engine** — every relationship-scoped field in epic HOLODEX-178 (Age in Media, Phase 3's role-scoped joins) is person×video. Building a type-parametrized "any two entities" engine now would be speculative; `DeriveRelationship`'s two named parameters (`person`, `video`) are deliberately concrete, not generic.
- **A batched, single-query birthdate fetch for cast lists** — considered (Options B) and deferred; today's per-person reuse is correct and proportionate at this project's scale.
- **Role-filtering** ("lead actor's age") — blocked on `video_people.role` (ADR-059, Proposed) and TMDB billing-order persistence (HOLODEX-180); out of scope per spec.

---

## Options Considered

### D2 — how the two-entity input reaches a pure derive pass

#### A — a new `DeriveRelationship(person, video, now)` pass, parallel to `Derive` (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — mirrors `Derive`'s exact shape (formula registry, pure, clock-injected); one new predicate to partition entity-intrinsic vs. relationship-scoped `Computed` fields |
| Cost | O(relationship-fields) per (person, video) pairing on read; zero storage |
| Scalability | Person×video is fixed for this epic; a third relationship shape would be a new pass, not a generalization of this one — acceptable, matches "closed formula set, no DSL" (ADR-063 D4) |
| Team familiarity | High — line-for-line structural twin of a pass already shipped and understood |

**Pros:** keeps `Derive`'s contract (one entity, one slice) exactly as ADR-063 left it — no caller of `Derive` needs to know a two-entity variant exists; the non-adoptable/clock-injection guarantees are inherited, not re-implemented. **Cons:** two passes to keep mentally distinct (mitigated: the `PersonDependsOn`/`VideoDependsOn` marker makes the partition structural, not conventional — a field can't be ambiguously in both).

#### B — bend `Derive` to accept an optional second input slice

**Pros:** one function. **Cons:** `Derive`'s single-slice contract is used and tested (ADR-063 AC-8/AC-9) as a one-entity pass; adding an optional second slice makes every existing call site's meaning conditional on which fields happen to be registered, and the "insert after dependency" positional logic (`insertComputed`) has no sensible meaning for a row with no host list. Rejected — the resulting function would do two different things badly rather than one thing each, cleanly.

#### C — fold the relationship computation directly into the `getMedia` handler (no resolver-package pass)

**Pros:** no new resolver-package surface. **Cons:** violates ADR-051/063's load-bearing purity boundary — the formula (birthdate/release_date arithmetic, the "release before birth" guard) would live in the API layer instead of the pure, unit-testable `internal/resolver` package; Phase 3's role-scoped joins would have nowhere principled to extend. Rejected.

### D3 — where a relationship-scoped value lives in the payload

#### A — a new top-level `related_person_fields` key, keyed by person ID (chosen)

**Pros:** no `model` package change (avoids the `model`↔`resolver` import cycle entirely); reuses the existing "structured field data in the detail payload" convention (FR8) rather than a new endpoint; extensible to Phase 3's additional relationship fields without a reshape. **Cons:** a second place (beside `resolved[]`) a frontend consumer must know to look — mitigated: the two keys answer different questions (per-canonical vs. per-cast-member) and the naming makes that explicit.

#### B — add a `resolver.ResolvedField`-typed field to `model.Person`

**Cons:** **not viable** — `internal/resolver` already imports `internal/model`; `model` importing `resolver` back is a compile-time cycle. Rejected on that basis alone, independent of design taste.

#### C — a bare value (e.g. `age_in_media: 34`) instead of a `ResolvedField`-shaped row

**Pros:** simpler payload. **Cons:** violates FR5 — the spec requires `age_in_media` to carry the same `computed: true` / nil-`decision` shape as every other computed field, so a bare int would be a visibly different provenance contract for the one relationship-scoped field versus every entity-intrinsic one. Rejected.

### D3 (birthdate source) — how `getMedia` gets each cast member's resolved birthdate

#### A — reuse the existing `personResolved(r, id, p)` per cast member (chosen)

**Pros:** zero new repo code; guaranteed to match the person's own page (same decisions/curation applied); correctness by construction. **Cons:** N extra calls (3 queries each: enrichment/curation/decisions) per video-detail view, where N = cast size — bounded and infrequent (a single-item page view, not a list-page row), acceptable at this project's scale (single-owner, personal server).

#### B — a new batched repo query that resolves just `birthdate` across a set of person IDs in one round trip

**Pros:** fewer queries; avoids building each person's full field schema (bio, aliases, etc.) just to read one value. **Cons:** duplicates the decision/curation precedence logic `personResolved` already implements correctly — a second implementation of the same rule is exactly the kind of drift risk ADR-051's per-field decision model warns about; **speculative** ahead of any evidence that cast-list-sized N+1 is a real cost. Deferred: if cast lists or traffic grow enough to matter, a batched variant is a targeted follow-up, sized to the actual bottleneck — not built pre-emptively here (Consequences, below).

---

## Trade-off Analysis

**A second pass vs. a bent single pass.** `Derive`'s clean one-entity contract is worth protecting: ADR-063 built it to be a template Phase 2/3 register against without re-litigating the taxonomy, and `DeriveRelationship` extends that template rather than compromising it. The cost — two passes instead of one — is small and the split is structural (marked by which `DependsOn`-family a field populates), not a judgment call a future contributor has to make each time.

**Correctness (full `personResolved` reuse) vs. read efficiency.** The alternative to N `personResolved` calls is a leaner batched birthdate-only query, but that query would need to *reimplement* the decision/curation precedence that already lives correctly in `personResolved` — paying a duplication-and-drift risk to save a few queries on a page that is viewed one video at a time, not iterated over a list. At this project's single-owner scale, the queries are the cheaper risk. This is explicitly called out as revisitable if evidence says otherwise (Consequences).

**A named `related_person_fields` payload key vs. touching `model`.** The import-cycle constraint makes this not really a choice — `model` cannot import `resolver` — but even absent that constraint, keeping relationship-scoped output out of `model.Person`/`model.Video` is the right call: those structs are shared across list pages, related shelves, and search, where a relationship-to-*this-video* value has no meaning. Scoping it to the one handler that has both entities in view (`getMedia`) keeps every other `model.Person`/`model.Video` consumer untouched.

---

## Consequences

**What becomes easier**
- Phase 2 (Nationality) registers a `FieldType`/operators for its own canonical without touching `DeriveRelationship`, `Derive`, or the taxonomy — a pure registration (spec Success Metric 3).
- Phase 3 (role-scoped joins, once `video_people.role` and TMDB billing-order persistence land) extends `DeriveRelationship`'s formula registry and `related_person_fields`'s map value with a second row — the pass and the payload shape are already relationship-generic within the person×video pairing.
- A future non-Age relationship fact (still person×video) is a formula + two `DependsOn` lists, not new plumbing — mirrors ADR-063's own "next computed field is a registration" promise, one level up.

**What becomes harder**
- Two derive passes to keep straight (`Derive` vs. `DeriveRelationship`) — mitigated by the structural `PersonDependsOn`/`VideoDependsOn` marker and by `DeriveRelationship` being a line-for-line structural twin, not a divergent design.
- `getMedia` now makes up to N extra `personResolved` calls for a video's cast list — a cost to watch, not yet a problem; **if** cast-list size or traffic ever make this measurable, Option B (a batched birthdate-only query) is the sized follow-up, not a pre-emptive build.
- A second top-level payload key (`related_person_fields`) alongside `resolved` — contributors must know which one to consult for a per-canonical vs. per-cast-member fact; the naming and this ADR are the record of that split.

**What we'll need to revisit**
- **Design handoff** — cast-poster placement for the age number (spec Open Question); no subtitle/caption slot exists on `PersonPoster.svelte` today.
- **A batched birthdate fetch** — only if Option B's deferred cost materializes (Consequences, above).
- **`OperatorsByType` consumers** — Phase 2's facet UI (Nationality) is the first thing that reads this map for anything other than documentation.
- **Generalizing beyond person×video** — not anticipated by this epic, but if it arises, `DeriveRelationship`'s concrete two-parameter shape would need revisiting (deliberately not generalized now — Non-Goals).

---

## Action Items

1. [ ] `registry.FieldDef` gains `Type FieldType` (D1); `FieldType`/`Operator` types + `OperatorsByType` map added to `internal/registry`; `title`/`status`/`birthdate`/`release_date`/`age_in_media` annotated per D1's minimum scope — no other `KnownFields` entry touched.
2. [ ] `registry.FieldDef` gains `PersonDependsOn`/`VideoDependsOn []string`; `age_in_media` registered `Computed: true` with both populated (D2).
3. [ ] `resolver.DeriveRelationship(person, video []ResolvedField, now time.Time) []ResolvedField` (`internal/resolver/derive_relationship.go` or appended to `derive.go`) + closed `relationshipDerivations` formula registry + `deriveAgeInMedia` (D2); reuses `parseDate`/`wholeYearsBetween` from `derive.go` unchanged.
4. [ ] `getMedia` (`internal/api/handlers.go`): for each `p := range v.People`, call the existing `h.personResolved(r, p.ID, &p)`, extract `birthdate`'s resolved row, call `resolver.DeriveRelationship` against the video's own `resolved`, and assemble `related_person_fields map[int64][]resolver.ResolvedField` into the response (D3) — no change to `model.Person`/`model.Video`.
5. [ ] Assert resolver purity: extend `TestResolverPackageIsClockFree`'s grep guard (already scoped to `internal/resolver/`) to cover the new file; unit tests for `deriveAgeInMedia` (missing/unparseable birthdate or release_date, release-before-birth guard) with a fixed `now`; confirm `registry.Lookup("age_in_media").Computed` trips the existing generic decision-endpoint guard with no new guard code (mirrors ADR-063 AC).
6. [ ] API integration test over `getMedia`: a video with a resolved `release_date` and a linked person with a resolved `birthdate` produces a `related_person_fields[person.id]` row carrying `computed: true` and nil `decision`/`candidates`/`in_sync`; a video with no `release_date` produces no rows for any cast member, regardless of their birthdate coverage (spec FR4 ACs).
7. [ ] Cross-link this ADR from `docs/architecture/README.md` and from ADR-063 (§"What is explicitly not in this ADR" — Age-in-media, now realized here) and ADR-062 (§D-filterable, the taxonomy it was blocked on).
8. [ ] `/design-handoff` for cast-poster age placement; `/testing-strategy` update for the F46 Phase 1 block; `/security-review` — confirm N/A at implementation time per spec framing (read-only, no new auth/access/infra surface).
