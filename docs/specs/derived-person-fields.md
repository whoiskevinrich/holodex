# Spec: Derived / calculated person fields (F45)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-08
**Feature block**: **F45** — an **intrinsic derived-field engine**: a new field-genre of **computed,
source-less, read-only** fields that the resolver appends after resolution, seeded with **Person's Age**
(`now − birthdate`) and **Age at death** (`deathdate − birthdate`). Computed on read, never stored, never
curatable.

**Issue**: [HOLODEX-73](https://whoiskevinrich.atlassian.net/browse/HOLODEX-73) *(parent epic
[HOLODEX-18](https://whoiskevinrich.atlassian.net/browse/HOLODEX-18) — Enrichment fields)*
**ADR**: [ADR-063](../architecture/ADR-063-derived-computed-fields.md) — derived-field genre
(`FieldDef.Computed`/`DependsOn`) + pure `Derive(resolved, now)` post-pass + non-adoptable `computed:`
provenance token + handler-injected clock; extends [ADR-052](../architecture/ADR-052-baseline-source-contract.md) /
[ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), sibling of
[ADR-056](../architecture/ADR-056-provider-field-render-hints.md)
**Design**: [derived-person-fields-handoff.md](../design/derived-person-fields-handoff.md) +
[QA checklist](../design/derived-person-fields-qa-checklist.md) — bare-number Age/Age-at-death row directly
under Birthdate; provenance is a **hover tooltip on the value** — "calculated from Born" on `title` +
`aria-label`, **no icon/badge** (D5 revised 2026-07-10, superseding the earlier icon-only badge)
**Testing**: [testing-strategy.md](../testing-strategy.md) — F45 block landed (§4 component rows + cardinal
invariants, §5 computed-row/provenance frontend row, §6 E2E flow 20, §9 phasing narrative, §10 concrete cases);
the `deriveAge` / `deriveAgeAtDeath` unit tests (missing-input branch, deathdate branch, leap-day boundary) land
with the implementation

**Depends on** (all shipped):
- the entity-agnostic resolver + canonical registry ([ADR-052](../architecture/ADR-052-baseline-source-contract.md),
  `ResolveFields`, `registry.FieldDef`, `internal/registry`) — the seam the `Derive` post-pass plugs into
- per-field source-of-truth ([F36](field-source-of-truth.md) /
  [ADR-051](../architecture/ADR-051-field-source-of-truth.md), `ResolvedField`, `WinningSource`,
  `fieldsource` grammar) — the resolver is already **clock-free / pure**; "now" is injected by the caller
- presence-driven auto-registration ([F39](provider-render-hints.md) /
  [ADR-056](../architecture/ADR-056-provider-field-render-hints.md), `ResolvedField.AutoRegistered`,
  `AutoRegisterFields`, `AutoFieldRows.svelte`, `ProvenanceBadge.svelte`) — derived rows reuse the same
  read-only render path and the "no Decision → not adoptable" convention
- person enrichment baseline ([F37](people-source-of-truth.md), `NewPersonBaseline`, `personResolved`) —
  supplies `birthdate` / `deathdate` from person-enrichment

**Touches** read-only computed values only — **no auth / access / infrastructure change, no migration, no
stored column**. Per the ticket, **`/security-review` is N/A** (recorded on the gate).

---

## Problem Statement

A person page shows the facts a provider hands us — birthdate, deathdate — but not the facts a human reads
_off_ those facts. "Born 1990-03-14" makes a reader do the arithmetic to answer the only question they
actually asked: **how old are they?** Every provider snapshot is stateless and frozen at fetch time, so a
provider **cannot** supply age — it is **now-relative** and would be stale the moment it is stored. Today that
value simply isn't there; the reader computes it in their head, or it's wrong.

## Resolved Decisions

The mechanism was settled in the 2026-07-08 brainstorm (recorded on the ticket); three product/UX questions
were resolved with the project owner on 2026-07-08:

| # | Question | Decision |
|---|---|---|
| D1 | How does a computed Age render? | **Bare number** — `34`. The field label carries the noun ("Age"); the value is just the integer. Age-at-death renders the same way under an "Age at death" label. Whole years only (floor); no months/days. |
| D2 | Where does the Age row sit? | **Directly under Birthdate**, in the primary bio group — read age where you'd expect it, adjacent to the input it's derived from. Not in the "Additional details" auto-field block. |
| D3 | What shows when Age can't compute (no birthdate)? | **Nothing, for everyone.** The row is simply **absent** when a required input is missing — for owner and visitor alike. No placeholder, no "—", **no enrichment nudge.** *(This supersedes the brainstorm's item-5 "missing-input → enrichment nudge" idea — the "can't compute" state exists in the engine but produces no UI here.)* |

**Genre invariants** carried from the brainstorm (for the ADR to formalize):

1. Derived fields are **computed, source-less, read-only** — a distinct genre from baseline / enrichment /
   auto-registered.
2. **Compute-on-read, always.** No migration, no stored column. Time-varying values (Age) *must* be
   computed live; static ones *shouldn't* be stored either, to avoid staleness.
3. **Closed Go registry of formulas**, keyed by canonical — **no formula DSL**. Each formula declares its
   required inputs and returns `(value, computable)`.
4. The resolver stays **pure / clock-free** (ADR-051). `now` is passed **into** the derive pass as a
   parameter; nothing inside the resolver reads the clock.
5. Provenance is **transitive and read-only**: a derived value's provenance is "calculated from Birthdate",
   never a provider or file source, and it carries **no `Decision`** → it is **not adoptable / not
   curatable** (same convention as auto-registered rows).

## Goals

1. **Answer "how old?" directly.** A person with a birthdate shows an **Age** that is correct at read time,
   with zero storage and zero staleness.
2. **A reusable derived-field genre, not a one-off.** Ship the engine (registry flag + pure `Derive`
   post-pass + `computed` provenance) so the next now-relative or intrinsic-computed field is a formula
   registration, not new plumbing.
3. **Coverage scales 1:1 with enrichment.** Because Age rides on `birthdate` (100% of enriched people on the
   films testbed carry a clean ISO birthdate), Age appears exactly when person-enrichment has run — it is not
   decoration.
4. **Keep the resolver pure.** The clock is injected; the resolver package gains no `time.Now`, no I/O.
5. **No new curation surface.** Derived rows are display-only; they add nothing to
   `field_source_decisions` / `metadata_curation` / writeback.

## Non-Goals

- **Age-in-media** (a person's age at the time of a specific video) — *(Why: cross-entity (person × video),
  needs the video's date as a second input; split to its own linked story. The derive engine here owns only
  **person-intrinsic** computations.)*
- **Provider "computed" fields** (zodiac sign, proprietary scores, etc.) — *(Why: these are **already just
  enrichment** — a provider maps a value to a canonical key and it flows through resolve today. They do not
  touch this engine, which owns only what a stateless snapshot **can't** produce.)*
- **A formula DSL / user-authored formulas** — *(Why: the formula set is a closed, code-reviewed Go registry;
  arbitrary user formulas are a security and complexity surface with no demand.)*
- **Storing / writing back derived values** to files or the DB — *(Why: they are compute-on-read by
  definition; writing them back would create the staleness the design exists to avoid.)*
- **An enrichment nudge on the missing-input state** — *(Why: D3 — the row is simply absent when it can't
  compute; a "add a birthdate to calculate age" prompt is deferred, not built here.)*
- **Sub-year precision** (age in months/days) and **video / studio derived fields** — *(Why: not asked for;
  whole-year Age on people is the seed. The engine is entity-generic, so these are later formula
  registrations.)*

## Users & Value

- **Visitor** — reads a person's Age at a glance, correct today, with a "calculated from Birthdate"
  provenance so it's clearly derived, not a stored fact. Sees an Age-at-death for deceased people instead of
  a running age.
- **Owner** — gets Age for free the moment enrichment supplies a birthdate; nothing to curate, configure, or
  keep fresh.
- **Operator** — no new config surface, no migration, no YAML.

## Functional Requirements

### FR1 — Derived-field genre in the registry

`registry.FieldDef` (`internal/registry/registry.go`) gains two fields:

- `Computed bool` — marks a canonical as derived (source-less, read-only).
- `DependsOn []string` — the canonical inputs the formula requires (e.g. `["birthdate"]`,
  `["birthdate","deathdate"]`).

Two new canonical entries are registered: **`age`** (`DependsOn: ["birthdate"]`) and **`age_at_death`**
(`DependsOn: ["birthdate","deathdate"]`), both `Computed: true`. Labels: "Age" and "Age at death".

### FR2 — Pure `Derive` post-pass

A pure function appends computed rows after resolution, modeled on the existing `AutoRegisterFields` append:

```
Derive(resolved []ResolvedField, now time.Time) []ResolvedField
```

- Reads only the already-resolved input values (by canonical) out of `resolved`; **no I/O, no package-level
  clock** — `now` is the sole time source and is passed by the caller.
- For each registered `Computed` field whose `DependsOn` inputs are **all present and parseable**, appends one
  `ResolvedField` with the computed value.
- If any required input is **missing or unparseable**, the field is **omitted** (D3) — `computable=false`
  yields no row.
- Emitted rows are stamped: `Computed: true` (new flag on `ResolvedField`, mirroring `AutoRegistered` /
  `Promoted`), `WinningSource` in the `computed:` namespace (FR4), `Decision`/`Candidates`/`InSync` **nil**
  (not adoptable), and provenance carrying the input canonical(s) for the transitive "calculated from …"
  badge.

### FR3 — Formula registry (closed, Go)

A closed Go registry keyed by canonical, one entry per computed field. Each formula:

- Declares required inputs (mirrors `DependsOn`) and returns `(value string, computable bool)`.
- **`deriveAge`** — `floor(now − birthdate)` in whole years. Returns `computable=false` if `birthdate` absent
  / unparseable. If `deathdate` is also present, `deriveAge` yields **no row** (age-at-death takes over — one
  person shows exactly one of the two).
- **`deriveAgeAtDeath`** — `floor(deathdate − birthdate)` in whole years; requires **both** inputs.

The two age formulas are the **one conceptual function branching on `deathdate`**: a living person shows
`age`; a deceased person shows `age_at_death` **instead of** a running age. No person shows both.

### FR4 — `computed` provenance token

`internal/fieldsource/fieldsource.go` gains a **non-adoptable** `computed` provenance token: a `Computed`
constant + a `ForComputed(canonical)` formatter + an `IsComputed` recognizer (mirroring the `provider:`
helpers). A derived row's `WinningSource` is `computed:<canonical>` (e.g. `computed:age`).

Per [ADR-063](../architecture/ADR-063-derived-computed-fields.md) D3, the token is **deliberately kept out
of `Valid()`/`ForNamespace()`** — those encode *adoptable decision* sources, and a computed field can never be
pinned. Non-adoptability is enforced structurally (the row **carries no `Decision`/`Candidates`**, like the
auto-registered convention, and is never written to `field_source_decisions`) **plus** an API guard that
rejects a decision naming a `computed:` source / `Computed` canonical. It is display metadata only.

### FR5 — Placement: under Birthdate (D2)

The person detail payload orders `age` / `age_at_death` **immediately after `birthdate`** in the primary bio
group — not in the "Additional details" auto-field block. `personResolved`
(`internal/api/person_fields.go`) runs `Derive(resolved, now)` after `ResolveFields` (and after
`appendAutoRegistered`), then positions the computed rows adjacent to their dependency. `now` is injected at
the handler boundary (the API layer owns the clock; the resolver does not).

### FR6 — Render + provenance (D1)

- Derived rows render through the existing read-only path (`AutoFieldRows.svelte` treatment or the primary
  `<dl>`, per the design handoff), value as a **bare integer** (`34`).
- `ProvenanceBadge.svelte` gains a **computed / derived** treatment (no provider brand icon; a small muted
  "calculated" pill analogous to the existing "file" pill), with the transitive label **"calculated from
  Birthdate"** (from the row's dependency provenance).
- No owner affordances on the row — no promote pill, no source-select, no curation (it is not adoptable).

### FR7 — MCP / API surface

Derived rows appear in the same `resolved[]` array the person detail endpoint already returns, so they flow
to the SPA and to any MCP `resolved`-field consumer for free, tagged `computed: true` and with the
`computed:` winning source. No new endpoint.

## Acceptance Criteria

1. **Age computes on read.** *(D1)* Given a person with `birthdate = 1990-03-14` and no `deathdate`, when the
   person page is viewed on 2026-07-08, then an **Age** row shows **`36`** (bare integer), positioned
   directly under Birthdate *(D2)*.
2. **Age is time-varying, never stored.** Given the same person, when `now` advances past their next birthday
   (injected as a later clock in a test), then the rendered Age increments — with **no** DB write and **no**
   migration/column touched.
3. **Age-at-death replaces running age.** Given a person with both `birthdate` and `deathdate`, when viewed,
   then an **Age at death** row shows `floor(deathdate − birthdate)` and **no** running "Age" row is present
   (exactly one of the two).
4. **Missing input → no row.** *(D3)* Given a person with **no** `birthdate`, when viewed as owner **or** as
   visitor, then **neither** an Age nor an Age-at-death row appears — no placeholder, no "—", no nudge.
5. **Unparseable input → no row.** Given `birthdate = "unknown"` (non-ISO), then `deriveAge` returns
   `computable=false` and no Age row renders (no error, no partial value).
6. **Not adoptable / not curatable.** Given a rendered Age row, when inspected in the payload, then it carries
   `computed: true`, `winning_source = "computed:age"`, and **nil** `decision` / `candidates` / `in_sync`;
   and the SPA shows **no** promote pill, source-select, or curation control on it.
7. **Transitive provenance.** Given a rendered Age row, then hovering the value shows a tooltip reading
   **"calculated from Born"** (the dependency's registry label), with the same phrase on `aria-label` — and the
   row shows **no** icon, badge, provider brand icon, or "file" pill *(D5 revised)*.
8. **Resolver stays pure.** `grep` over `internal/resolver/` finds **no** `time.Now` and no package-level
   clock; `Derive` takes `now` as a parameter. A resolver unit test passes a fixed `now` and asserts a
   deterministic Age.
9. **Leap-day boundary.** Given `birthdate = 2000-02-29`, computing age on `2026-02-28` vs `2026-03-01`
   crosses the birthday exactly once (documented convention asserted in a unit test).
10. **Renders cleanly.** The Age row renders as a plain `text-ink` vital under Born with its hover tooltip and
    no icon/badge; nothing skin-dependent (token discipline still holds).

## Test Notes (for /testing-strategy)

- **`deriveAge` unit** — present/absent/unparseable birthdate; deathdate-present ⇒ no running age; fixed
  `now`; leap-day boundary (AC-9).
- **`deriveAgeAtDeath` unit** — both inputs present; one missing ⇒ no row; `floor` correctness.
- **`Derive` pass** — purity (fixed `now` in, deterministic out; no I/O); ordering (row adjacent to
  Birthdate); stamping (`Computed`, `computed:` source, nil Decision).
- **API integration** — `personResolved` emits the derived rows in `resolved[]` for a birthdate-bearing
  person and omits them otherwise; owner and visitor see the same (D3).
- **SPA** — read-only render, no owner affordances; provenance is the value's hover tooltip (no badge/icon).

## Open Items

- ~~**ADR** — formalize the derived-field genre~~ — **landed**:
  [ADR-063](../architecture/ADR-063-derived-computed-fields.md) (registry `Computed`/`DependsOn`, pure
  `Derive(resolved, now)` post-pass, non-adoptable `computed:` provenance token kept out of the decision
  grammar, handler-injected clock). ADR D3 refines this spec's FR4.
- ~~**Design handoff** — final call on whether Age renders in the primary `<dl>` vs. an `AutoFieldRows`-style
  row, the exact computed-provenance pill (icon/label/tone), and copy for "calculated from Birthdate".~~ —
  **landed**: [derived-person-fields-handoff.md](../design/derived-person-fields-handoff.md). Renders in the
  **primary bio `<dl>` directly under Birthdate** (D2, not an auto-field row); provenance is a **hover tooltip
  on the value** (D5 revised 2026-07-10) — "calculated from Born" on `title` + `aria-label`, **no icon/badge**
  (superseding the earlier icon-only glyph). Handoff §3 also pins the `providerFromWinningSource("computed:…")`
  fix so the row doesn't render a phantom provider bubble.
- **Age formatting locale** — bare integer whole years is decided (D1); if a future field needs unit/locale
  formatting that's a per-formula concern, not an engine change.
- ~~Missing-input enrichment nudge~~ — **cut** (D3); revisit only if enrichment-completion prompts become a
  cross-field theme.

## Rollout

- **No migration, no stored column, no flag** — the engine is compute-on-read and additive; a person with a
  birthdate simply starts showing Age on deploy.
- **Docs** — add the derived-field genre to `docs/reference/configuration.md` (or the field-model reference)
  and cross-link the new ADR from `docs/architecture/README.md`.
- **Coverage note** — Age appears wherever person-enrichment has supplied a clean ISO birthdate (3/3 enriched
  people on the films testbed at spec time); it scales with enrichment adoption, not separately.
