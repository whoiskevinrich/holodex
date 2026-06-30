# ADR-052: `BaselineSource` contract — the resolver's entity-agnostic baseline seam

**Status:** Accepted
**Date:** 2026-06-30
**Deciders:** Project owner

**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source-of-truth — **realizes its §9 fast-follow ①, the entity-agnostic resolver**, and pins the seam its decision short-circuit will attach to) · [ADR-033](ADR-033-metadata-source-plugins.md) (unified resolution layer F27 — enrichment/curation are already pre-loaded, entity-agnostic maps; this gives the *baseline* layer the same shape) · [ADR-013](ADR-013-metadata-field-mapping.md) (field mapping + the `file:`/`provider:` namespace grammar) · [ADR-004](ADR-004-metadata-extraction.md) (the file layer this baseline reads). **Spec:** [Per-field source-of-truth / F36](../specs/field-source-of-truth.md).

---

## Context

[ADR-051](ADR-051-per-field-source-of-truth-decisions.md) reframes resolution around a **file baseline**: the
file layer is the default source of truth, enrichment is a candidate, and a standing per-item/per-field
decision selects the winner. §9 of that ADR commits the model to being **entity-generic** — `video` ships
first, `person`/`studio` reuse the same `(entity_type, entity_id, canonical_field)` keying — and names **two
dependencies deliberately not built there**, the first being:

> **Entity-agnostic resolver.** `Resolve(v *model.Video, …)` hard-codes the video baseline; People fields
> don't flow through the unified resolver today. Abstracting "baseline source" behind an interface (as
> enrichment/curation already are) is the prerequisite for any non-video entity.

That prerequisite is the **critical-path head** for the whole F36 slice. The resolver's only video-specific
coupling is the `gather` closure in `resolveField`, which reaches the file layer directly:

```go
case src.IsFileTitle():                 // videos.title
case src.Namespace == "file":           // video_metadata tag columns
default:                                 // provider enrichment
```

If the F36 decision short-circuit (ADR-051 §3) and the eventual People/Studio work both land on top of this
video-shaped `gather`/`Resolve(v *model.Video, …)` signature, they will **fight over the same function**:
one wants to add a decision pre-step, the other wants to swap the baseline entity. Pinning the baseline
contract **now**, as a behavior-preserving refactor ahead of any decision code, lets both proceed without
reopening `resolveField`/`Resolve` a second time. This ADR records that contract; the F36 decision store
(migration 0016) and the People/Studio entities remain separate, later work.

### Constraints / forces

- **Behavior-preserving.** This is a pure refactor — the video resolution result is byte-identical before and
  after. No new default, no decision logic (that is ADR-051's separate slice). The existing resolver test
  suite is the regression guard.
- **Resolution stays pure** ([ADR-033](ADR-033-metadata-source-plugins.md)). The baseline is supplied
  pre-loaded, exactly like `Enrichment` and `Curation`; the seam adds no I/O and no per-field query.
- **The resolver must not know which namespace is "intrinsic."** For a video the baseline namespace is `file`;
  a person/studio baseline is scan-derived and has no file. So *ownership of the baseline namespace belongs to
  the baseline*, not the resolver — otherwise the `file` special-casing simply moves rather than disappears.
- **Video-first, minimal surface.** Ship the interface + the video implementation + an entity-generic core.
  Do **not** build a `person`/`studio` baseline here (those need the entity-promotion fast-follows ② / ③).

---

## Decision

Introduce a **`BaselineSource`** interface as the resolver's baseline-layer seam, make the merge core
(`ResolveFields`) take it, and reduce `Resolve(v *model.Video, …)` to a thin video wrapper.

### 1 — The `BaselineSource` interface

```go
// BaselineSource supplies an entity's baseline ("intrinsic") field values — the
// layer that is the default source of truth before enrichment and curation.
type BaselineSource interface {
    Baseline(src mapping.Source) (vals []string, ok bool)
}
```

`Baseline` reports:

- **`(vals, true)`** when `src` targets *this* entity's baseline layer — **even when `vals` is empty**, so
  resolution does **not** fall through to a provider for a baseline source. (Precedence still advances to the
  *next configured source* in the field's mapping; it just won't reinterpret a baseline source as a provider
  lookup.)
- **`(nil, false)`** when `src` names a provider/enrichment namespace, telling the resolver to consult the
  pre-loaded `Enrichment` map as before.

Deciding *which namespace is intrinsic* is the baseline's job, which is what removes all `file` special-casing
from the resolver. This mirrors how `Enrichment`/`Curation` are already entity-agnostic pre-loaded inputs.

### 2 — The video implementation (the file layer)

```go
func NewVideoBaseline(v *model.Video, extra []model.ExtraMetadata) BaselineSource
```

`videoBaseline` owns the `file` namespace: `file:title → videos.title`, `file:<tag> → video_metadata`
columns (indexed case-insensitively, the existing `indexExtra` behavior). `v` may be nil (an empty baseline).
This is the only place that knows the video baseline is the file layer.

### 3 — The entity-agnostic core

`ResolveFields(baseline BaselineSource, enrichment, curation, fields)` is the merge core. `Resolve` becomes:

```go
func Resolve(v *model.Video, extra, enrichment, curation, fields) []ResolvedField {
    return ResolveFields(NewVideoBaseline(v, extra), enrichment, curation, fields)
}
```

`resolveField`'s `gather` now consults `baseline.Baseline(src)` first, then enrichment — replacing the
video-specific `switch`. `BrowseTitle` (the list-media helper) likewise builds a `NewVideoBaseline` internally;
its signature is unchanged. All existing call sites and the field-mapping precedence/merge semantics are
untouched.

### 4 — What is explicitly *not* in this ADR

The F36 **decision** layer (the standing per-field source selection, `field_source_decisions` migration 0016,
the file-first default, the decision short-circuit, writeback-of-decided-value) is **ADR-051's** slice and
lands on top of this seam. The **person/studio** `BaselineSource` implementations are the entity-promotion
fast-follows (ADR-051 §9 ② apply-the-People-refactor, ③ promote-Studio). This ADR ships only the contract and
the video implementation, so the decision work and the entity work can build in parallel without colliding.

---

## Options Considered

### A — `BaselineSource` interface owning its namespace + entity-agnostic core (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one interface, one impl, one core extraction; behavior-preserving |
| Cost | Zero runtime cost (pre-loaded like enrichment/curation; no new I/O) |
| Scalability | Good — any future entity plugs in one `BaselineSource` impl, no resolver change |
| Team familiarity | High — mirrors the existing `Enrichment`/`Curation` pre-load shape exactly |

**Pros:** Removes all `file` special-casing from the resolver; the decision short-circuit and the entity work
attach to a stable seam; directly testable with a non-video baseline. **Cons:** One small added public surface
(`BaselineSource`, `NewVideoBaseline`, `ResolveFields`) before its non-video consumers exist.

### B — Keep `Resolve(v *model.Video, …)`; thread a baseline only when People arrives

**Pros:** No surface added now. **Cons:** Guarantees a *second* reopening of `resolveField`/`Resolve` exactly
when the F36 decision code is also touching it — the collision §Context warns about. Rejected: the seam is
cheap now and contentious later.

### C — Pass the baseline as raw data (`title string`, `byFileTag map[...]`) instead of an interface

**Pros:** No interface. **Cons:** Bakes the *video's* baseline shape into the core signature, so a person's
scan-derived baseline can't satisfy it without another signature change — it defeats the entity-agnostic goal.
Rejected.

---

## Trade-off Analysis

The tension is **add a seam now vs. defer until a second entity exists**. Deferring looks cheaper but isn't:
the F36 decision slice (ADR-051) is imminent and lands on the exact function a future People refactor would
also rewrite, so "later" means reworking decision code that has already shipped. Pinning the contract as a
behavior-preserving refactor — guarded byte-for-byte by the existing resolver suite — is the cheap moment to do
it. The only cost is a small public surface (`BaselineSource` / `NewVideoBaseline` / `ResolveFields`) ahead of
its non-video callers, which is justified because it is *the* prerequisite both the decision short-circuit and
every non-video entity build on, and because it makes entity-agnosticism a directly testable property today.

---

## Consequences

**What becomes easier**
- ADR-051's decision short-circuit attaches to `ResolveFields`/`gather` without re-deriving the baseline shape.
- A future `person`/`studio` entity inherits unified resolution by supplying one `BaselineSource` — no change
  to `ResolveFields`, `resolveField`, or precedence/merge logic.
- The entity-agnostic claim is now a unit-testable property (a non-video `BaselineSource` driven through
  `ResolveFields`), not a design aspiration.

**What becomes harder**
- A small public surface exists before its non-video consumers (`BaselineSource`, `NewVideoBaseline`,
  `ResolveFields`); contributors must route file-layer access through the baseline, not re-add a `file` branch.

**What we'll need to revisit**
- **People baseline** (ADR-051 §9 ②) — implement a `personBaseline` over the scan-derived person record; its
  "baseline name" is also an identity key (alias/merge, ADR-036/F23).
- **Studio promotion** (ADR-051 §9 ③) — once `studio` is an `entity_type`, add its `BaselineSource`.
- **Decision short-circuit** (ADR-051 §3) — the decision pre-step consults the same `gather`; confirm the seam
  needs no further change when it lands.

---

## Action Items

1. [x] Add the `BaselineSource` interface + `videoBaseline`/`NewVideoBaseline` to `internal/resolver`.
2. [x] Extract the entity-agnostic `ResolveFields` core; reduce `Resolve` to the video wrapper; route
   `resolveField`/`BrowseTitle` through the baseline. Behavior-preserving (existing suite is the guard).
3. [x] Unit-test the seam with a non-video `BaselineSource` and assert `Resolve == ResolveFields(NewVideoBaseline(…))`.
4. [x] Note the seam in `docs/testing-strategy.md` F36 block; add this ADR to `docs/architecture/README.md`.
5. [ ] Build the person/studio `BaselineSource` impls when their entity-promotion fast-follows are scheduled
   (ADR-051 §9 ② / ③).
