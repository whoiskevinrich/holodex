# Spec: Age-in-media — person's age at the time of a video (cross-entity derived field)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-12
**Feature block**: split out of **F45** — the **cross-entity** (relational) member of the derived-field
genre. A property of the *person-in-this-video*, not of the person or the video alone: computed on-the-fly
at the API layer by joining a person's resolved `birthdate` with a video's resolved `release_date`. Reuses
F45's age arithmetic; does **not** go through F45's single-entity `Derive()` post-pass or the
`registry.FieldDef.Computed`/`DependsOn` mechanism (see FR2 — confirmed no cross-entity registry path exists
today).

**Issue**: [HOLODEX-173](https://whoiskevinrich.atlassian.net/browse/HOLODEX-173) *(parent epic
[HOLODEX-18](https://whoiskevinrich.atlassian.net/browse/HOLODEX-18) — Enrichment fields; split from
[HOLODEX-73](https://whoiskevinrich.atlassian.net/browse/HOLODEX-73))*
**ADR**: N/A per the ticket — reuses [ADR-063](../architecture/ADR-063-derived-computed-fields.md)'s age
arithmetic; a bespoke join at the API layer introduces no new architecture. Revisit only if the resolver
package's exported surface needs formalizing (see Open Items).
**Design**: Not yet landed — `needs-design` (exact placement/style of the age annotation on the
`PersonPoster` cast card is a `/design-handoff` decision, out of scope for this spec).
**Testing**: Not yet landed — `needs-testing`; see Test Notes below for `/testing-strategy` input.

**Depends on** (all shipped):
- [HOLODEX-73](https://whoiskevinrich.atlassian.net/browse/HOLODEX-73) / F45 — **Released**. Supplies the
  shared age arithmetic (`wholeYearsBetween`, `parseDate`, `internal/resolver/derive.go`) and the
  "compute-on-read, no storage, no placeholder-when-uncomputable" conventions this spec reuses.
- Per-field source-of-truth ([F36](field-source-of-truth.md) / F37 people /
  [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) — `personResolved`
  (`internal/api/person_fields.go:110-137`) is what supplies a person's **resolved** (not raw baseline)
  `birthdate`, honoring any owner source decision.
- Video-level TMDB enrichment reaching `release_date` — was blocked, now unblocked (config-wiring fix,
  verified 2026-07-12; see HOLODEX-173 comments). Per-library coverage still varies with enrichment adoption,
  same as any other TMDB field — that is normal data state, not a spec blocker.

**Touches** read-only computed values only — **no auth / access / infrastructure change, no migration, no
stored column**. Per the ticket, **`/security-review` is N/A** (recorded on the gate).

**Related**: [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) ("F46 Phase 1 — Typed
queryable-field substrate, piloted via Age in Media video flair") targets overlapping end-user payoff (the
same cast-grid age annotation) as part of a larger queryable-field-substrate epic. This spec deliberately
scopes to HOLODEX-173's original, narrower ask — render-only, no query/search/facet substrate. The project
owner should revisit HOLODEX-176's scope once this ships (narrow it to exclude Age-in-Media, or fold this
work in as its proof case) — not decided by this spec.

---

## Problem Statement

A video's cast grid shows a person's name and headshot, but not the fact that actually contextualizes the
performance: **how old were they when this was made?** A viewer scanning "Dune (1984)" has no way to tell,
at a glance, that a given actor was 25 rather than 55 in that role — they'd have to open the person's page,
read a birthdate, and do the arithmetic against the film's release year themselves. Provider snapshots are
stateless and video-agnostic, so no provider can supply this — it is **relational** (a joint property of a
specific person *and* a specific video), which is exactly what F45's single-entity `Derive()` engine cannot
produce.

## Resolved Decisions

Settled in the 2026-07-08 brainstorm (recorded on the ticket) and carried forward from F45's established
conventions:

| # | Question | Decision |
|---|---|---|
| D1 | What are the inputs? | Person's **resolved** `birthdate` × the video's **resolved** `release_date` only. |
| D2 | Where does it render? | On the video's cast grid — the `PersonPoster` card block (`web/src/routes/media/[id]/+page.svelte`). **Not** on the person page (the person page already has F45's own intrinsic Age). |
| D3 | Any schema change? | **No.** `video_people` stays a bare `(video_id, person_id)` pair; the age is computed on-the-fly at the API layer, not persisted. Persisting a credit-context row is only worth it if other per-credit data (character, billing order — see HOLODEX-180) is ever added; out of scope here. |
| D4 | What date gates the computation? | **`release_date` only — no fallback to `recorded_at`.** Empirically (2026-07-08 probe on the films testbed), `recorded_at` is the file's *download* date (8/8 coverage) and would silently mislabel a freshly-downloaded 1984 film as "made in 2026." `release_date` coverage was 0/8 at probe time due to a config-wiring bug (since fixed and verified — see header). The field must render only when a trustworthy `release_date` exists, otherwise sit absent. |
| D5 | Missing-input display | **Absent, for owner and visitor alike** — same convention as F45's D3. No placeholder, no "—", no enrichment nudge. |

**Genre invariants inherited from F45** (ADR-063), still true here even though this is cross-entity:

1. Computed, source-less, read-only — never written to `field_source_decisions`, never adoptable/curatable.
2. Compute-on-read, always — no migration, no stored column.
3. Reuses the closed, code-reviewed age arithmetic (`wholeYearsBetween`) — no formula DSL, no divergent math.

## Goals

1. **Answer "how old were they, here?" directly on the cast grid** — zero extra clicks, zero storage,
   zero staleness (a video's release date never changes, so once computed for a given person+video pair the
   value is stable — no time-varying recompute concern like F45's person-page Age).
2. **Reuse, don't duplicate, F45's age math.** Same `wholeYearsBetween` convention (leap-day handling
   included) — one source of truth for "how do we count years between two dates" across the whole app.
3. **Coverage scales with enrichment, on both sides of the join.** The annotation appears exactly where a
   video has `release_date` *and* a cast member has `birthdate` — visible proof that enrichment adoption
   pays off, same pattern as F45.
4. **No new curation surface, no new storage.** Purely additive to the video detail API response and its
   cast card rendering.

## Non-Goals

- **The F46 typed queryable-field substrate** (registry, operators, value index, search/filter/facet UI for
  this or any field) — *(Why: that is [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176)'s
  scope, a separate and larger epic. This spec is render-only, per HOLODEX-173's original ask. See "Related"
  above.)*
- **A `registry.FieldDef.Computed`/`DependsOn` entry, or extending `resolver.Derive()` to be cross-entity** —
  *(Why: confirmed during exploration that `Derive()` and the registry's computed-field mechanism operate on
  one entity's own resolved fields only; there is no cross-entity registry path today, and building one is
  more machinery than this single relational field needs. A bespoke join at the API layer is sufficient and
  matches the ticket's own framing.)*
- **Persisting a per-credit context row** (character name, billing order) — *(Why: D3 — only worth adding if
  other per-credit data is ever needed; tracked separately under
  [HOLODEX-180](https://whoiskevinrich.atlassian.net/browse/HOLODEX-180).)*
- **`recorded_at` fallback when `release_date` is absent** — *(Why: D4, the hard constraint — `recorded_at`
  is a download date, not a production/release date, and would produce a confidently wrong number.)*
- **Batch-optimizing the per-cast-member resolve** — *(Why: v1 calls the existing `personResolved` once per
  cast member at page-load time (see FR1). Holodex is a personal-library server, not IMDb-scale — cast lists
  are small. Revisit only if this becomes a measured performance issue, not preemptively.)*
- **Gating on the person's `deathdate`** — *(Why: unlike F45's person-page Age/Age-at-death branch,
  age-in-media answers "how old were they at the time," which is well-defined even for a since-deceased
  person, including posthumous releases. `deathdate` is not consulted — see FR2.)*

## Users & Value

- **Visitor** — sees each cast member's age at the time of the video, right on the cast grid, with zero
  extra navigation. Sees nothing extra when it can't be computed (no confusing placeholder).
- **Owner** — gets this for free the moment both the video and the relevant person are enriched; nothing to
  curate, configure, or keep fresh.
- **Operator** — no new config surface, no migration, no YAML.

## Functional Requirements

### FR1 — Per-cast-member resolved birthdate

`getMedia` (`internal/api/handlers.go:433`) currently returns `v.People []model.Person` populated by
`attachAssociations` (`internal/repo/repo.go:577-627`), whose cast query
(`repo.go:590-593`) selects only `p.id, p.name` — no birthdate. Two changes:

1. Extend that query to also select `p.birthdate, p.deathdate` (existing `people` table columns) so the
   `model.Person` values passed into `NewPersonBaseline` carry real baseline data, not just id+name.
2. In `getMedia`, for each cast member, call the existing `h.personResolved(r, person.ID, &person)`
   (`internal/api/person_fields.go:110-137`) and extract the `birthdate` canonical's resolved value (if
   present) from the returned `[]resolver.ResolvedField`. This is the **resolved** value — it already
   honors any owner field-source decision or provider adoption, not just the raw file/baseline value.

### FR2 — Age-in-media arithmetic (reuse, not extend, F45's engine)

Confirmed during exploration: `resolver.Derive` (`internal/resolver/derive.go:26`) and the registry's
`Computed`/`DependsOn` mechanism (`internal/registry/registry.go:36,41`) operate on a **single entity's own**
resolved fields — there is no cross-entity registry path. Age-in-media is therefore a **bespoke join at the
API layer**, not a new registry entry:

- Export (or add a thin exported wrapper around) `wholeYearsBetween(start, end time.Time) int`
  (`internal/resolver/derive.go:123`) and `parseDate(s string) (time.Time, bool)` (`derive.go:111`) so
  `internal/api` can call them directly, reusing the exact leap-day convention F45 already established
  (ADR-063 §D4) instead of writing divergent date math.
- New helper, e.g. `ageInMedia(birthdate, releaseDate string) (years int, ok bool)`: parse both with
  `parseDate`; if either fails to parse, or `birthdate` is after `releaseDate` (a data inconsistency — would
  yield a negative age), return `ok=false`. Otherwise return `wholeYearsBetween(birthdate, releaseDate)`.
- **No `deathdate` gating** (see Non-Goals) — the helper takes only `birthdate` and `releaseDate`.

### FR3 — Gate strictly on the video's resolved `release_date` (D4)

In `getMedia`, the video's own `resolved` fields are already computed via `resolver.Resolve`
(`internal/api/handlers.go:478`, `internal/resolver/resolver.go:259-268`). Look up the entry where
`Canonical == "release_date"` and take its first value:

- If absent (no resolved `release_date` for this video), **no cast member gets an age** — the field is
  omitted entirely for every person on that video's cast grid, regardless of their own birthdate coverage.
- **No fallback to `recorded_at`** under any circumstance — enforced by never reading that canonical in this
  code path at all (not just "preferring" release_date).

### FR4 — Response shape: video-scoped, not the generic `Person` type

Add an `age_in_media` field (nullable integer, whole years) to the video detail response's per-credit
entries — **not** to the generic `model.Person` (`internal/model/model.go:54-67`) /
`types.ts Person` (`web/src/lib/types.ts:34-43`) type, since age-in-media is meaningless outside a specific
video's context (a person's search-result or index-page entry has no video to be "in"). Introduce a
video-detail-scoped shape carrying the existing `Person` fields plus `age_in_media: number | null`, used only
in `MediaDetailResponse`'s `video.people[]` (`types.ts:263-273`).

- Omit / `null` when uncomputable (missing video `release_date`, missing person `birthdate`, or the
  birthdate-after-release_date guard in FR2) — no placeholder text, matching D5.
- Each cast member is independent: one person missing a birthdate does not suppress ages for the rest of the
  cast on the same video.

### FR5 — Frontend render on the `PersonPoster` cast card

`web/src/routes/media/[id]/+page.svelte:379-400` (the cast grid) and
`web/src/lib/components/PersonPoster.svelte:7-17` (currently `{personId, name, version?, eager?}` props, no
age) need to accept and render `age_in_media` when present. Exact placement/style (inline with the name
caption vs. a small badge on the poster, etc.) is a `/design-handoff` decision — not fixed by this spec, per
the ticket's own `needs-design` gate.

- No visible element renders when `age_in_media` is `null`/absent (D5) — no "can't compute" placeholder.
- Visible to owner and visitor alike — no admin-only gating (matches D5 / F45's D3 precedent).
- Token-only styling; QA across all three skins (`cinematheque`/`broadcast`/`brutalist`) per this repo's
  frontend theming rule.

## Acceptance Criteria

1. **Age renders when computable.** Given a video with resolved `release_date = 1984-12-14` and a cast
   member with resolved `birthdate = 1959-02-11`, the cast card shows `age_in_media = 25`.
2. **No video release_date → no ages anywhere on that video's page.** Given a video with no resolved
   `release_date` (e.g. unenriched), no cast member shows an age — not even a placeholder — regardless of
   their own birthdate coverage.
3. **No `recorded_at` fallback.** Given a video with `recorded_at` populated but no resolved `release_date`,
   age-in-media is **not** computed from `recorded_at` for any cast member (guard test: the code path never
   reads that canonical here).
4. **Per-member independence.** Given a video with `release_date` set and a cast of 3, where one person has
   no `birthdate`, that one person's `age_in_media` is `null`/absent while the other two still show theirs.
5. **Invalid-combination guard.** Given `birthdate` after `release_date` (a data inconsistency), the helper
   returns not-computable rather than a negative or nonsensical number.
6. **No `deathdate` gating.** Given a cast member who has since died, their `age_in_media` computes normally
   from `birthdate`/`release_date`, unaffected by their `deathdate` or by what their person-page Age/Age-at-death
   currently shows.
7. **Resolved, not raw, birthdate.** Given a person whose file-baseline `birthdate` differs from an owner-adopted
   provider `birthdate` (a standing field-source decision), age-in-media uses the **resolved** (winning)
   value — matching whatever the person's own detail page shows.
8. **Shared arithmetic.** `ageInMedia` delegates to the same `wholeYearsBetween` used by F45's `deriveAge` —
   verified by a leap-day boundary case matching F45's existing test convention, not a re-derived formula.
9. **No new curation surface.** `age_in_media` is never written to `field_source_decisions`, carries no
   promote/source-select affordance in the SPA — it is inline per-credit JSON, not a resolved-field row.
10. **Renders cleanly across skins.** The age annotation uses only design tokens; QA'd in all three skins,
    no skin-specific hardcoding.

## Test Notes (for `/testing-strategy`)

- **`ageInMedia` unit** — happy path; missing/unparseable birthdate; missing/unparseable release_date;
  `birthdate` after `release_date` guard; leap-day boundary (mirrors F45's existing `deriveAge` coverage,
  confirming the shared helper is actually shared, not reimplemented); a case where `deathdate` is set but
  irrelevant to the result.
- **API integration (`getMedia`)** — response includes `age_in_media` per cast member when both inputs
  resolve; entirely `null`/absent when the video has no `release_date`; independent per-member when only
  some cast members have birthdates; uses the **resolved** birthdate (post field-source-decision), not the
  raw file baseline, for a person with a standing override.
- **No-`recorded_at`-fallback guard test** — video with `recorded_at` set but no `release_date` → zero ages
  rendered for any cast member.
- **Frontend** — cast card renders the age annotation when present; renders nothing extra when absent (no
  placeholder); visible identically to owner and visitor; skin QA (cinematheque/broadcast/brutalist).

## Open Items

- **Design handoff** — exact placement/style of the age annotation on the `PersonPoster` cast card
  (`needs-design`, per the ticket's own gate) — not decided by this spec.
- **HOLODEX-176 scope overlap** — flagged to the project owner (see "Related" above); this ticket ships the
  standalone render-only feature described here, and HOLODEX-176's F46-substrate scope should be revisited
  (narrowed to exclude Age-in-Media, or reframed to build on top of what ships here) once this lands. Not
  decided by this spec.
- **Exporting `wholeYearsBetween` / `parseDate`** from `internal/resolver` (or adding thin exported
  wrappers) is an implementation detail for whoever picks up the ticket — flagged here so it isn't missed
  during implementation.
- **N+1 `personResolved` calls per cast member at page-load** — accepted for v1 at personal-library scale
  (see Non-Goals); revisit only if a measured performance issue surfaces on cast-heavy videos.

## Rollout

- **No migration, no stored column, no flag** — additive to the video detail API response and its cast
  card rendering only. A video with a resolved `release_date` and cast members with resolved `birthdate`s
  simply starts showing ages on deploy.
- **Docs** — cross-link this spec from `derived-person-fields.md`'s existing forward-reference (its
  Non-Goals section already names "Age-in-media... split to its own linked story").
- **Coverage note** — the annotation appears wherever both a video has a trustworthy `release_date` (TMDB
  video enrichment) and a cast member has a resolved `birthdate` (TMDB person enrichment) — it scales with
  enrichment adoption on both sides of the join, not separately.
