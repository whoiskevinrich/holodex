# ADR-063: Derived / computed fields — a compute-on-read field genre

**Status:** Proposed
**Date:** 2026-07-08
**Deciders:** Project owner

**Extends:** [ADR-052](ADR-052-baseline-source-contract.md) (entity-agnostic `ResolveFields` core — the derive pass appends to its `[]ResolvedField` output for any entity) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source-of-truth — the resolver is **pure / clock-free**; the derive pass keeps it that way by taking `now` as a parameter) · [ADR-013](ADR-013-metadata-field-mapping.md) (canonical field mapping + the `FieldDef` registry this extends). **Relates to:** [ADR-056](ADR-056-provider-field-render-hints.md) (presence-driven auto-registration — the **closest sibling**: derived rows reuse its *display-only, no-Decision, appended-after-canonical* convention, and are deliberately outside the F36/F30 decision & curation model) · [ADR-033](ADR-033-metadata-source-plugins.md) (the enrichment shadow store that supplies `birthdate`/`deathdate`). **Spec:** [Derived / calculated person fields (F45)](../specs/derived-person-fields.md). **Issue:** [HOLODEX-73](https://whoiskevinrich.atlassian.net/browse/HOLODEX-73) (epic [HOLODEX-18](https://whoiskevinrich.atlassian.net/browse/HOLODEX-18)).

---

## Context

A person page shows the facts a provider hands us — `birthdate`, `deathdate` — but not the fact a human reads *off* those facts: **how old are they?** Every provider snapshot is stateless and frozen at fetch time, so a provider **cannot** supply age — it is **now-relative** and would be stale the moment it is stored. Today the value simply isn't there; the reader does the arithmetic in their head.

The unified resolution stack already has the shape this needs. [ADR-052](ADR-052-baseline-source-contract.md) made the merge core (`ResolveFields`) entity-agnostic and **pure**; [ADR-056](ADR-056-provider-field-render-hints.md) established the precedent of **appending display-only rows** after the canonical resolve (`AutoRegisterFields`) that carry a `WinningSource` for provenance but **no `Decision`**, so they render but are not curatable. A computed field is the same *kind* of row — read-only, provenance-bearing, non-adoptable — with one new twist: its value is **calculated from other resolved fields at read time** rather than read from a store.

What is genuinely new, and what this ADR pins:

1. A **field genre** the registry has no concept of today — *computed, source-less, read-only* — distinct from baseline / enrichment / auto-registered.
2. A **clock** enters the read path for the first time. The resolver is clock-free by contract (ADR-051), and must stay that way; a now-relative value forces a decision about **where `now` is injected**.
3. A **provenance kind** that is not a provider, a file, or a decision — "calculated from Birthdate" — that must not be mistaken for an adoptable source.

### Current state (survey, 2026-07-08)

| Seam | Today | File |
|---|---|---|
| `registry.FieldDef` | `{Canonical, Label, Display, Description}`; registered as a package-level `KnownFields []FieldDef` slice, indexed at `init()` | `internal/registry/registry.go:19` |
| `resolver.ResolvedField` | carries `WinningSource`, `AutoRegistered`, `Promoted`, and `Decision`/`Candidates`/`InSync` (nil ⇒ non-adoptable) | `internal/resolver/resolver.go:48` |
| `AutoRegisterFields` | pure pass; stamps `WinningSource`, `AutoRegistered:true`, **nil** `Decision`; sorted after canonical fields | `internal/resolver/auto_register.go:64` |
| `fieldsource` grammar | encodes the three **adoptable decision** sources only — `file`, `manual`, `provider:<name>`; `Valid()` = "well-formed decision source" | `internal/fieldsource/fieldsource.go` |
| `personResolved` | `ResolveFields` → `markPromoted` → `appendAutoRegistered`, returns `[]ResolvedField` | `internal/api/person_fields.go:110` |
| Resolver clock | **none** — `internal/resolver/` has no `time` import, no `time.Now` (verified) | — |
| Handler clock | **none** — `personResolved`/`appendAutoRegistered` take no clock; `Handlers` has no `now` seam | — |

### Forces

- **Compute-on-read, never store.** Time-varying values (Age) *must* be computed live; static ones (age-at-death) *shouldn't* be stored either — storing derived values re-creates the staleness the genre exists to avoid. So: no migration, no column, no shadow row.
- **The resolver stays pure / clock-free** ([ADR-051](ADR-051-per-field-source-of-truth-decisions.md)). `now` is external input, injected at the edge; nothing in `internal/resolver/` may read the clock. This is an asserted invariant (AC-8: a grep guard).
- **Reuse the auto-registered convention, don't re-invent it.** Derived rows are display-only and non-adoptable — the same posture ADR-056 already ships. The diff should be *a pass + a flag + a provenance token*, not a new editing model.
- **Non-adoptable must be structurally true.** A computed row can never be pinned as a source-of-truth. That has to hold at the type level (no `Decision`) **and** at the API decision endpoint, not by convention.
- **Entity-generic.** The mechanism rides the one `ResolveFields` core (ADR-052); Age is the person seed, but nothing about the engine is person-specific — a future video/studio computed field is a formula registration, not new plumbing.
- **Closed formula set.** Formulas are code-reviewed Go, keyed by canonical — **no DSL, no user-authored formulas** (a security/complexity surface with no demand; spec non-goal).

---

## Decision

Add a **derived-field genre** as a **pure post-resolution pass** over `[]ResolvedField`, driven by a **closed Go formula registry**, stamped with a **non-adoptable `computed:` provenance token**, with the **clock injected at the API handler boundary** so the resolver stays pure. Seeded with `age` and `age_at_death` on Person.

### D1 — Genre marker on the registry: `Computed` + `DependsOn`

`registry.FieldDef` gains two fields:

```go
type FieldDef struct {
    Canonical   string
    Label       string
    Display     string
    Description string
    Computed    bool     // derived: source-less, read-only, computed-on-read
    DependsOn   []string // canonical inputs the formula reads (e.g. ["birthdate"])
}
```

Two entries join `KnownFields`, both `Computed: true`:

- **`age`** — `DependsOn: ["birthdate"]`, `Label: "Age"`.
- **`age_at_death`** — `DependsOn: ["birthdate", "deathdate"]`, `Label: "Age at death"`.

Keeping the genre in the **one** field vocabulary (rather than a separate registry) means every existing consumer — `Lookup`, label/display resolution, the SPA field switch — already knows these canonicals; `Computed` is just a new predicate on a known shape. `DependsOn` is the declarative contract that lets the derive pass gather inputs generically and lets the provenance badge name them.

### D2 — A pure `Derive(resolved, now)` post-pass, modeled on `AutoRegisterFields`

```go
// Derive appends one computed ResolvedField per registered Computed canonical whose
// DependsOn inputs are all present and parseable in `resolved`. Pure: no I/O, no
// package clock — `now` is the sole time source, supplied by the caller.
func Derive(resolved []ResolvedField, now time.Time) []ResolvedField
```

- Reads only already-resolved input **values by canonical** out of `resolved`; touches no store and no clock beyond the `now` parameter.
- For each `Computed` field whose `DependsOn` inputs are all present and parseable, runs its formula (D4) and appends one `ResolvedField`.
- If any required input is **missing or unparseable**, `computable=false` ⇒ **no row** (spec D3 — absent, never a placeholder).
- Each emitted row is stamped exactly like an auto-registered row **plus** the new marker: `Computed: true`; `WinningSource = fieldsource.ForComputed(canonical)` (D3); `Label`/`Display` from the registry; and `Decision`/`Candidates`/`InSync` left **nil** — structurally non-adoptable.

This is the auto-registration pass's twin: same append-after-canonical shape, same non-adoptable stamping, differing only in that the value is *computed from `resolved`* rather than read from the shadow store. It lives in `internal/resolver/` and stays pure — `now` in, deterministic rows out.

### D3 — `computed:` is a provenance token, **not** an adoptable decision source

`internal/fieldsource/fieldsource.go` gains a single definition of the token and a recognizer, mirroring the `provider:` helpers:

```go
const Computed = "computed"                       // provenance namespace (NOT a decision source)
func ForComputed(canonical string) string { ... } // "computed:<canonical>"
func IsComputed(s string) bool            { ... } // recognizes a computed WinningSource
```

**`computed` is deliberately excluded from `Valid()`.** `Valid()` answers *"is this a well-formed **adoptable decision** source?"* — and a computed field can never be pinned (there is no underlying store to select). Adding it to `Valid()` would let the decision endpoint accept `POST {source: "computed:age"}`, directly contradicting non-adoptability. `ForNamespace()` is likewise untouched — it maps a value's namespace back to a *decision* source for reporting an undecided replace-field's implicit selection, and a computed row is neither.

Non-adoptability is therefore enforced **two ways**: structurally (the row carries nil `Decision`/`Candidates`, so no SPA affordance and nothing to write) **and** by an explicit guard at the decision API — a `POST` naming a `Computed` canonical (or any `computed:` source) is rejected `400`, never silently written to `field_source_decisions`.

> **Refines spec FR4.** FR4 says the token gains "`Valid`/`ForNamespace` support." That is corrected here: the token gets a **constant + formatter + recognizer** for single-source-of-truth and badge detection, but is **kept out of** `Valid()`/`ForNamespace()` precisely because it is non-adoptable. The spec's FR4 wording is updated to match. This aligns with the auto-registered precedent, which carries its `WinningSource` as a plain provenance string and never enters the decision grammar.

### D4 — Closed Go formula registry; one function branching on `deathdate`

A closed registry keyed by canonical, one entry per computed field. Each formula declares its inputs (mirroring `DependsOn`) and returns `(value string, computable bool)`:

- **`deriveAge`** — `floor(now − birthdate)` whole years. `computable=false` if `birthdate` is absent/unparseable. **If `deathdate` is also present, `deriveAge` yields no row** — age-at-death takes over.
- **`deriveAgeAtDeath`** — `floor(deathdate − birthdate)` whole years; requires **both** inputs.

The two are the **one conceptual function branching on `deathdate`**: a living person shows a running `age`; a deceased person shows `age_at_death` **instead**. No person ever shows both. Whole years only (floor); leap-day convention (a `Feb-29` birthdate ticks over exactly once between `Feb-28` and `Mar-01`) is asserted in a unit test (AC-9). Sub-year precision and locale formatting are explicit non-goals (per-formula concerns, not engine changes).

### D5 — The clock is injected at the API handler boundary

There is no clock at the read-path handler today. Rather than reach for `time.Now()` inline, add a **`now func() time.Time` seam on `Handlers`** (defaulting to `time.Now` at construction, overridable in tests). `personResolved` chains `Derive(resolved, h.now())` after `appendAutoRegistered`:

```go
// internal/api/person_fields.go, personResolved:
resolved = h.appendAutoRegistered(r.Context(), rows, personizeResolved(resolved))
resolved = resolver.Derive(resolved, h.now())   // clock enters here, at the edge
return resolved
```

The API layer **owns the clock**; the resolver package gains nothing. This is what keeps AC-8 true (a grep over `internal/resolver/` still finds no `time.Now`) while making Age's time-varying behavior directly testable: AC-2 (Age increments as `now` advances) and AC-9 (leap-day) are exercised at the API integration layer by setting `h.now`, with no wall-clock flake. Placement (spec D2 — computed rows positioned **immediately after `birthdate`** in the primary bio group, not the auto-field block) is a payload-ordering step applied after `Derive` appends, and is a design-handoff detail rather than an architectural seam.

### What is explicitly *not* in this ADR

- **Age-in-media** (a person's age at a specific video's date) — cross-entity (person × video), needs a second input; its own linked story. This engine owns only **person-intrinsic** computations.
- **Provider "computed" fields** (zodiac, proprietary scores) — those are **already just enrichment**: a provider maps a value to a canonical key and it flows through resolve today. They never touch this engine, which owns only what a stateless snapshot **can't** produce.
- **Writeback / storage** of derived values, a **formula DSL**, and **sub-year precision** — spec non-goals.

---

## Options Considered

### A — Registry flag + pure `Derive(resolved, now)` post-pass + `computed:` provenance (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one pass, two `FieldDef` fields, a formula registry, a provenance token; mirrors `AutoRegisterFields` |
| Cost | Zero storage, zero migration; O(computed-fields) over an already-resolved slice on read |
| Scalability | Entity-generic — rides the ADR-052 core; the next computed field is a formula registration |
| Team familiarity | High — the auto-registered convention (ADR-056) is the exact template |

**Pros:** reuses the display-only/non-adoptable machinery already shipped; keeps the resolver pure (clock at the edge); genre is a first-class registry concept, so labels/render/MCP flow for free. **Cons:** introduces the first clock into the read path (mitigated: injected seam, not ambient) and a provenance kind that must be kept out of the decision grammar (D3).

### B — Compute at enrich time and persist as a shadow enrichment value

**Pros:** no read-path clock; renders through the existing enrichment path unchanged. **Cons:** **staleness** — a now-relative Age is wrong the instant after it's written and needs a re-enrich to correct; re-creates exactly the problem the genre exists to avoid. Violates the compute-on-read invariant. Rejected.

### C — Compute in the SPA from `birthdate`

**Pros:** no backend change at all. **Cons:** the calculation (floor, deathdate branch, leap-day rule) lives only in the browser — no MCP/API consumer sees Age, provenance can't be expressed as a resolved row, and the rule scatters across client code. Contradicts the "coverage scales 1:1 with enrichment, flows to every `resolved[]` consumer" goal. Rejected.

### D — A formula DSL / user-authored formulas

**Pros:** maximal extensibility. **Cons:** an untrusted-input execution surface and real complexity for a set that is, today, two functions; no demand. Explicit spec non-goal. Rejected — the registry is a closed, code-reviewed Go set.

### D3 sub-decision — where the `computed` token lives

**Chosen:** a constant + formatter + recognizer in `fieldsource`, **excluded from `Valid()`/`ForNamespace()`**. **Alternatives:** (a) add it to `Valid()` per FR4's literal wording — rejected: makes it an accepted decision source, contradicting non-adoptability; (b) a bare `"computed:"+canonical` string with no shared constant, exactly like auto-registered — rejected: the SPA badge and the API guard both need to *recognize* the kind, and three string literals drift. The chosen path is the middle that keeps the token single-sourced without letting it enter the decision grammar.

---

## Trade-off Analysis

**Compute-on-read vs. store-and-serve.** The decisive force is staleness: a now-relative value is only ever correct if computed at read time, so persistence (Option B) is not a cheaper version of the same thing — it is a *wrong* version for the flagship field. Paying an O(few) computation per person read is trivially cheaper than a correctness bug, and it drops the whole migration/column/backfill surface.

**A read-path clock vs. resolver purity.** Age forces a clock into the read path — but *where* it enters is the real decision. Injecting it as a `Handlers.now` seam (D5) keeps the resolver clock-free (its purity is a load-bearing property for ADR-051's determinism and its test suite) and makes the time-varying behavior a controllable test input rather than wall-clock flake. The cost is one small seam on `Handlers`; the alternative (ambient `time.Now()` in the handler) saves nothing and forfeits the deterministic tests AC-2/AC-9 require.

**Reuse vs. a new provenance kind.** Derived rows are 90% the auto-registered row (ADR-056): display-only, appended after canonical, no `Decision`. Reusing that convention is what makes the diff small. The 10% that is new — a provenance that is neither provider, file, nor decision — is contained by keeping `computed:` a *display token* outside the decision grammar (D3), so the "non-adoptable" invariant is structural, not conventional, and the decision endpoint stays exactly as strict as before.

---

## Consequences

**What becomes easier**
- A person with an enriched `birthdate` shows a correct-today **Age** with zero storage, zero curation, zero config — and an **Age at death** for the deceased.
- The next now-relative or intrinsic-computed field (any entity) is a **formula registration + a `Computed` registry entry**, not new plumbing — the pass, the token, and the render path already exist.
- Derived rows flow to the SPA **and** every MCP/`resolved[]` consumer for free, tagged `computed: true` with a `computed:` winning source.
- The read-path clock is now a **testable seam**, which future time-relative features (freshness badges, "enriched N days ago") can reuse.

**What becomes harder**
- The read path holds a clock. Contributors must inject it through `Handlers.now` and pass it *into* `Derive` — never add `time.Now` to `internal/resolver/` (guarded by the AC-8 grep test).
- One more invariant to respect: a `computed:` source must never enter `Valid()`/the decision store; the API guard and the nil-`Decision` stamping are both load-bearing.
- `age`/`age_at_death` are mutually exclusive by construction (D4) — a change to one formula must preserve the "exactly one row" property.

**What we'll need to revisit**
- **Design handoff** — the exact computed-provenance pill (icon/tone/copy "calculated from Birthdate") and whether Age renders in the primary `<dl>` vs. an `AutoFieldRows`-style row (`needs-design`).
- **Entity generalization** — video/studio computed fields when a use case arrives; the engine is ready, only formulas + placement are per-entity.
- **Age-in-media** — the cross-entity computation split to its own story; it will need a two-input (person × video-date) variant of the derive pass, likely a different injection boundary.

---

## Action Items

1. [ ] `registry.FieldDef` gains `Computed bool` + `DependsOn []string`; register `age` / `age_at_death` in `KnownFields` (D1).
2. [ ] `fieldsource`: add `Computed` const + `ForComputed` + `IsComputed`; **do not** touch `Valid()`/`ForNamespace()`; add the decision-endpoint guard rejecting a `computed:`/`Computed`-canonical decision (D3).
3. [ ] `resolver.Derive(resolved, now)` pure pass + `deriveAge` / `deriveAgeAtDeath` closed formula registry; add `Computed bool` to `ResolvedField` (D2/D4).
4. [ ] `Handlers.now func() time.Time` seam (default `time.Now`); `personResolved` chains `Derive(resolved, h.now())` after `appendAutoRegistered`; position computed rows under `birthdate` (D5).
5. [ ] Assert resolver purity: grep guard for no `time.Now` in `internal/resolver/`; unit tests for `deriveAge`/`deriveAgeAtDeath` (missing/unparseable input, deathdate branch, leap-day) with a fixed `now` (`/testing-strategy`).
6. [ ] Update spec FR4 wording to match D3; cross-link this ADR from the spec and `docs/architecture/README.md`; add the derived-field genre to the canonical-fields / configuration reference.
7. [ ] `/design-handoff` for the computed row + provenance pill (`needs-design`); `/security-review` is **N/A** (read-only computed values, no auth/access/infra change — recorded on the gate).
