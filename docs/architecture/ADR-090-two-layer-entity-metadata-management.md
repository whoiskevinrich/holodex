# ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field

**Status:** Proposed
**Date:** 2026-09-04
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source-of-truth
decisions — this ADR names that mechanism *layer 2* and fixes what may and may not be asked of it) ·
[ADR-052](ADR-052-baseline-source-contract.md) (`BaselineSource`, the entity-agnostic baseline seam
that makes layer 2 generic in the first place) · [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md)
(F47's entity-generic review queue + durable dismissal — the canonical *layer 1*) ·
[ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md) (filename extraction's
confidence-gated routing — a second layer 1, video-scoped).
**Relates to:** [ADR-033](ADR-033-metadata-source-plugins.md) (`entity_enrichment`, the shadow store
every layer-1 adoption writes into) · [ADR-030](ADR-030-access-control-gating-seam.md) (both layers
are owner-gated) · [ADR-085](ADR-085-films-entity.md) / [ADR-086](ADR-086-film-provider-enrichment.md)
(film's baseline and its enrich-queue widening) · [ADR-072](ADR-072-person-link-resolved-derivation.md)
(entity-link fields — see §Scope) · [ADR-081](ADR-081-entity-completeness-score.md) (the completeness
queue, layer-1-*shaped* but not an adoption verdict) · [ADR-075](ADR-075-tag-governance-and-video-enrichment.md)
(tag governance — the entity family this model does not cover).
**First instance:** [HOLODEX-194](https://whoiskevinrich.atlassian.net/browse/HOLODEX-194) /
[media-page-extraction-handoff.md](../design/media-page-extraction-handoff.md) — the video page's
"Extract from filename" panel. This ADR generalizes the model that design arrived at; it does not
depend on that ticket landing.

---

## Context

Holodex has four owner review queues — Duplicates (F43), Enrichment (F47), Extraction (F48),
Completeness (F55) — and a per-field source-of-truth control that renders on the entity's own detail
page (F36/ADR-051). These grew separately, and nothing wrote down how they relate.

The immediate trigger was a near-miss while designing HOLODEX-194. The first mockup showed filename
extraction dead-ending in its own review panel, with nothing connecting it to the resolved-field list
where provenance is already displayed. The obvious-looking fix — put the provider's competing value
into the extraction row so the owner could choose between filename, tag and TMDB in one place — would
have built a second, differently-shaped conflict UI on top of a shipped one.

That near-miss is the reason for this ADR. The two questions *look* like one and are not:

| | The question | Answered against | Lifetime |
|---|---|---|---|
| **Adoption** | Should this candidate enter the store at all? | The entity's own baseline (`file` tag, `record`) | Transient — verdict recorded, row disappears |
| **Precedence** | Of the values now stored, which wins for this field? | Every stored namespace (`file`/`record`, `filename`, `tmdb`, …) | Standing — a durable per-field decision |

Adoption is a gate on the shadow store. Precedence is a decision *over* it. Conflating them produces
either a review queue that must understand provider trust order, or a source chip row that must
understand confidence thresholds and dismissals.

### The pattern is already half-built, inconsistently

An earlier draft of this ADR asserted that every review queue is reachable only from `/owner`. That
is wrong, and the correction is what makes this ADR worth writing: **two of the three adoption queues
already have inline entity-page entry points, and extraction is the outlier.**

| Layer-1 queue | Owner tab | Inline on the entity page? |
|---|---|---|
| Enrichment (F47) | `/owner/enrichment` | **Yes** — `EnrichProviderChips` → `EnrichPicker` on person (`people/[id]/+page.svelte:627`), studio (`studios/[id]/+page.svelte:425`), video (`media/[id]/+page.svelte:1115`) |
| Duplicates (F43) | `/owner/duplicates` (person/studio/tag only) | **Yes** — `MergeOfferCard` on person (`:525`), studio (`:316`), tag (`tags/[id]/+page.svelte:394`); video uses `CollisionOfferCard` via `NameEditControl` (`media/[id]/+page.svelte:325`) |
| Extraction (F48) | `/owner/extraction` | **No** — and `ExtractionQueueRow` carries no link even *back* to the video |

So D2 below is not a new direction. It is the completion of one already applied twice and skipped
once, which is also why the extraction gap read as a bug to the owner rather than as a missing
feature.

### What is generic, and what is not

Layer 2's core is genuinely entity-agnostic. `BaselineSource` is a one-method interface
(`internal/resolver/resolver.go:249`); `ResolveFields` (`:306`) is the entity-agnostic core and
`Resolve` (`:289`) is merely the video wrapper. Four baselines implement it — video → `file`
(`resolver.go:261`), person → `record` (`person_baseline.go:26`), studio → `record`
(`studio_baseline.go:27`), film → `record` (`film_baseline.go:33`). `resolvePrecedence` (`:612`)
returns the winning value tagged with its namespace, so a new source is a new namespace and nothing
else.

Layer 1 is generic **only where the question is**. `enrichment_dismissals`
(`migrations/0024_enrichment_dismissals.up.sql:14`) is keyed `(entity_type, entity_id, provider)`
because "is this the right provider record for this entity?" is a question every entity has.
`metadata_extraction_review` (`0025_…:15`) is keyed on `video_id` alone because only videos have
filenames. That asymmetry is correct and this ADR does not disturb it.

Coverage as of 2026-09-04 (a snapshot, not maintained by any test):

| Entity | Layer 2 on the detail page | Layer 1 inline |
|---|---|---|
| Video `media/[id]` | Full — `SourceBadge` (`:1171`, `:1195`, default `file` baseline), `CurationFieldRow` (`:1197`), `ProvenanceBadge` (`:1164`) | Enrichment + collision; **no extraction** |
| Person `people/[id]` | Full — `SourceBadge` (`:671`, `:728`, `baselineKey="record"`), `SourceEditModal` (`:777`), `CurationFieldRow` (`:706`) | Enrichment + merge |
| Studio `studios/[id]` | Full — `SourceBadge` (`:459`, `:498`), `CurationFieldRow` (`:515`) | Enrichment + merge |
| Film `films/[id]` | **Partial** — `SourceBadge` at one site (`:240`), two scalar fields; no `CurationFieldRow`, no `ProvenanceBadge` | **None** — no enrichment wiring (HOLODEX-281 deferred) |
| Tag `tags/[id]` | **None** — no `ResolvedField` model at all | Merge only |
| Category `categories/[id]` | **None** | None |

Two live-component notes for implementers: `SourceSelect.svelte` has **zero render sites** — it is
imported at `media/[id]/+page.svelte:32` and never instantiated. The live pair is `SourceBadge` +
`SourceEditModal`. And the enrichment queue's frontend type union is `'person' | 'studio' | 'video'`
(`web/src/lib/types.ts:639`), so film rows the backend can already emit (`enrich_queue.go:45`) have
no rendering path.

## Decision

**Entity metadata management is two layers with fixed responsibilities, and the entity's own detail
page is a first-class surface for both.**

### D1 — The two layers, and the boundary between them

Layer 1 answers adoption; layer 2 answers precedence. A feature belongs to exactly one.

A layer-1 surface presents a candidate against **the entity's own baseline only** — the file's tag,
the existing DB record. It never presents a competing *provider* value, because that comparison is
layer 2's and already has a UI. Verdicts are: adopt (write to the shadow store under that source's
namespace), keep the baseline, or dismiss (durably, until re-triggered).

A layer-2 surface presents every stored namespace for one field as peers and records a standing
decision. It never acquires confidence scores, thresholds, or dismissal semantics.

**Layer 2's single-winner model applies to scalar *replace* fields only.** Merge fields
(`mapping.Field.Merge`, `internal/mapping/mapping.go:64`; `multi: true` implies merge) resolve as a
deduplicated cross-source union and are curated with a drop-any chip model in `CurationFieldRow` —
there is no winner to decide. D1 governs where a *candidate* is adopted for both kinds; only
precedence is replace-only.

### D2 — Layer 1 is reachable from the entity's own detail page

A review surface that exists only in `/owner` forces the owner to leave the entity to act on it. The
entity page is where the problem is noticed, so it is where the verdict should be available.
Enrichment and Duplicates already work this way; extraction is the gap to close.

The `/owner` hub tab is **not** removed and does not become secondary. It remains the cross-library
roll-up, the only place to work a queue in bulk, and the only way to find entities you were not
already looking at. The two surfaces are one queue at two scopes and **must share one resolve path**,
so a verdict recorded in either place is immediately gone from both.

This is a direction for new and touched surfaces, not a mandate to retrofit everything at once.

### D3 — Adoption must visibly land in layer 2

When a layer-1 verdict adopts a value, the entity page must refetch its resolved fields so the value
appears carrying its source's `ProvenanceBadge`, alongside its `tmdb`- and baseline-sourced
neighbours.

This is the load-bearing half. Without it the panel merely empties and the owner infers that
something happened somewhere; the feature reads as a side quest rather than as a source. With it, the
fact that `filename` is a peer of `tmdb` becomes visible rather than merely true in the resolver.

### D4 — A new source is a namespace, not a subsystem

Anything producing field values for an entity — a provider, filename extraction, a future sidecar or
import — enters through `entity_enrichment` under its own namespace and is thereby a layer-2 chip
with no new UI. If a proposed source appears to need its own conflict-resolution surface, that is the
signal it has been modelled wrong.

### D5 — Layer 1 storage is generic only where the question is

Use the entity-generic `(entity_type, entity_id, …)` shape when the adoption question applies to
every entity, as `enrichment_dismissals` does. Keep a scoped table when the question is intrinsic to
one entity type, as `metadata_extraction_review` is to videos. Do not generalize a table to satisfy
this ADR's symmetry — a filename column on a person's review row is not a generalization, it is a
column that never fills. Generalize the placement and layering, which is what this ADR is about.

## Scope — where this model does not apply

Stated plainly, because the model is narrower than "all entities":

- **Tags and categories have no layer 2 at all.** No `ResolvedField`, no `BaselineSource`
  implementation, no source control. Their curation is name/parent/category/alias operations plus the
  duplicate-merge queue — layer 1 with no layer 2. Nothing here proposes giving them a chip row.
- **Merge / multi-valued fields have no winner** (see D1). The union-plus-drop model is a different
  selection model in the same shell, and "which source wins" is undefined for them.
- **Entity-link fields are not scalars.** Video↔person/studio links (F40/ADR-072) are edited through
  pickers, and film cast/tags/studios are a deliberately read-only derived union over the film's
  videos (`films/[id]/+page.svelte:31-33`). D1's boundary still applies — an extraction row proposing
  a cast is an adoption question — but D3 has no provenance-badge equivalent on a link chip today.
  Left open deliberately.
- **Identity fields are an exception inside layer 2.** Selecting a non-baseline chip for a person or
  studio `name` fires `onadopt` and opens a rename/collision confirm flow instead of writing a
  decision. Identity is layer-1-shaped even though it lives inside a layer-2 control; that is
  intentional and predates this ADR.
- **The Completeness queue is layer-1-shaped but is not an adoption verdict.** It ranks entities by
  missing data rather than holding candidates awaiting a verdict, and already satisfies D2 via
  `CompletenessPanel`. Named here only so nobody forces it into D1's adopt/keep/dismiss vocabulary.
- **Film is a half-instance, not a confirming case.** Layer 2 covers two scalar fields with no
  provenance badge; layer 1 exists in the backend enrich queue but has no frontend path. Film is the
  entity furthest from this model.

Honest summary: **the two-layer model is established for scalar replace fields on video, person and
studio, partially on film, and not at all on tags or categories.**

## Consequences

**Good**

- The near-miss that produced this ADR becomes unrepresentable: "put the provider value in the review
  row" is a documented violation of D1 rather than a plausible-sounding improvement.
- New sources cost no new UI. D4 means a future extraction-like source inherits the chip row for
  free, exactly as `filename` does.
- The inconsistency becomes visible and namable. "Extraction is the only queue with no inline entry
  point" is a concrete gap; before this ADR it was invisible.

**Costs and risks**

- **Two surfaces per queue is more frontend to keep honest.** The mitigation is a hard requirement,
  not a convention: one resolve endpoint, one shared staging helper. A second resolve path would let
  the two drift, and is the failure this ADR is most exposed to.
- **D3 costs a refetch** on every adoption. Accepted; the alternative is an invisible outcome.
- **Retrofitting is unscheduled and deliberately un-ticketed.** A broad "apply the pattern
  everywhere" backlog item rots, whereas an ADR is consulted when someone touches the seam. The cost
  is that adoption is opportunistic and the coverage tables above will go stale.
- **The coverage tables are snapshots** verified 2026-09-04, guarded by no test. Orientation, not
  current truth.

## Alternatives considered

**One unified surface per entity — merge adoption and precedence into a single control.** Rejected:
it forces one widget to carry confidence scores, dismissal state, provider trust order and standing
decisions at once. The two questions have different lifetimes and different comparands.

**Keep all review in `/owner`, deep-linking from the entity page with the entity pre-filtered.** The
cheapest option and genuinely viable — and it is roughly what the enrichment queue already does in
the *outbound* direction (its rows link out to the entity). Rejected because it preserves the context
switch that motivated the change: the owner still leaves the video to fix the video. Retained as the
fallback if D2's maintenance cost proves worse than expected.

**Make every layer-1 table entity-generic for symmetry.** Rejected as false symmetry; D5 records the
narrower true rule.

**An always-on panel showing pending candidates on every entity page, with no trigger.** Deferred
rather than rejected — strictly additive over D2, and it changes what the hub is for. Revisit once at
least two entity types have a layer-1 extraction-style surface and there is evidence about how often
pending rows accumulate.
