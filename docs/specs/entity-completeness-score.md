# Spec: Entity Completeness Score (F55)

**Status**: Draft
**Phase**: 4 (Curation tooling)
**Issue**: [HOLODEX-260](https://whoiskevinrich.atlassian.net/browse/HOLODEX-260) (F55, shipped) · [HOLODEX-412](https://whoiskevinrich.atlassian.net/browse/HOLODEX-412) (F65 amendment)
**Amended**: 2026-09-20 — **F65.8** ([HOLODEX-435](https://whoiskevinrich.atlassian.net/browse/HOLODEX-435)):
the ring badge becomes a **button** that fires the single-entity enrichment refresh (sweep semantics,
ADR-103 D7); see F65.8 and RD9. Amended 2026-09-18 — **F65 Completeness score v2** rewrites § Scoring model, § Facet tables, the sort, the storage direction, and adds the card ring badge. Requirements added by F65 are numbered `F65.n`; F55 rows whose behavior changed carry a **v2** note. The v1 formula is recorded in [ADR-081](../architecture/ADR-081-entity-completeness-score.md) D3 and is not repeated here.
**Depends on**: per-field source-of-truth decisions and the baseline-source contract ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), [ADR-052](../architecture/ADR-052-baseline-source-contract.md)), metadata source plugins / the provider-agnostic enrichment model ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md), F22), the access-control gating seam ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)), derived/computed fields precedent ([ADR-063](../architecture/ADR-063-derived-computed-fields.md), F45), studio image roles ([ADR-079](../architecture/ADR-079-studio-image-roles.md), F51), and frontend theming ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
**Realizes**: F55 (new). Builds on the extraction-queue UX precedent ([HOLODEX-199](https://whoiskevinrich.atlassian.net/browse/HOLODEX-199)) — its deliberate deferral of bulk-apply directly informs this feature's queue design (§ Scope).
**Architecture**: [ADR-081](../architecture/ADR-081-entity-completeness-score.md) (facet criticality, not-applicable persistence, and the `imdb_id` → `external_provider_id` rename), [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md) (supersedes ADR-081 D5 only — the rename's value must be namespace-qualified, not a bare id), and [ADR-099](../architecture/ADR-099-completeness-score-required-band.md) (F65 — supersedes ADR-081 D3 + D4: required-band score, separate extras, a materialized store with trigger-fed invalidation, and the owner-only list field the ring badge rides).
**Design handoff**: [entity-completeness-handoff.md](../design/entity-completeness-handoff.md) (F55 — queue, panel, browse sort/filter) and, for F65, [completeness-ring-badge-handoff.md](../design/completeness-ring-badge-handoff.md) (the card ring badge, its overfill, the row placement on `/people` + `/studios`, three skins).

---

## Objective

Give the owner a number — the **completeness score** — that says whether an entity's **required**
canonical facets are filled in, and a separate **extras** number for the nice-to-have ones, so "done"
reads as done and a pile of optional detail can never out-vote a missing poster. Surface it three ways:
**a ring badge on every entity card in owner mode** (see completeness at a glance while browsing),
**sortable/filterable on browse list pages** (find the gaps in order), and a **facet-first remediation
queue** (find *specific* holes — "9 videos missing a poster" — and fix them one at a time). A companion, separate
**actionability** signal tells the owner which of those gaps already have an unapplied enrichment
candidate sitting in cache, so "quick wins" are visually distinct from "needs real research."

> **Why this is needed.** Holodex already resolves every field through a single seam — the unified
> resolver ([ADR-051](../architecture/ADR-051-unified-field-resolution.md)) — that knows, per field,
> whether a value exists, where it came from, and whether the owner curated it. But nothing today rolls
> that up into a single per-entity signal. An owner who wants to know "which of my 40 studios are missing
> branding art" or "which videos have no cast credited" has no way to ask that question except scrolling
> the whole library by eye. The completeness score turns the resolver's existing per-field state into an
> actionable, sortable, filterable curation signal — without duplicating or overriding the resolver's
> truth.

---

## Scope

### In scope

- **A completeness score per entity** (video, person, studio), computed from the entity's resolved
  fields — see § Scoring model for the formula. **v2 (F65):** the score is the *required band* alone;
  nice-to-have facets produce a separate `extras` number that is never blended in.
- **A ring badge on every entity card in owner mode (F65)** — required fills the ring; extras draw as a
  second lap over it only once required is full. Rides an owner-only `completeness` object on every list
  item; no per-card fetch, no new endpoint. Visitors never see it. **F65.8:** the ring is also the
  owner's one-click "refresh this entity" — it fires the same per-entity step the F66 sweep runs.
- **A separate actionability metric** — the % of an entity's *missing* facets that already have a cached,
  unapplied enrichment candidate. Actionability never affects the completeness score; it exists purely to
  triage the remediation queue (see below).
- **Tri-state facet status**: every scored facet on an entity is `resolved`, `missing`, or
  `not-applicable`. A facet marked not-applicable is excluded from both the numerator and denominator of
  that entity's score. The **data model** is generic across all scored facets; the **UI affordance** to
  mark a facet not-applicable ships narrow in v1 — only on the video `external_provider_id` facet (see
  below).
- **Generalizing `imdb_id` to a provider-agnostic `external_provider_id` facet.** Production doesn't use
  IMDb, and isn't limited to a single non-IMDb provider either — the registry treats providers as
  declared-not-compiled-in configuration ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md)),
  so an operator-configured provider outside this repo can also populate this facet. The resolved value
  is namespace-qualified (`"<provider>:<id>"`, e.g. `"tmdb:603"`) so it stays self-describing regardless
  of which provider supplied it ([ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md));
  the completeness facet needs to match that, not assume IMDb specifically. Self-published/home content
  legitimately has no external ID at all, which is exactly what the not-applicable affordance is for.
- **Owner-mode browse-page additions**: a "Completeness" sort order (v2: the composite key
  `(required, extras)`) and a "Missing facet" filter chip
  (reusing `FacetFilter.svelte`, the same component and interaction pattern already used for the Tags
  filter) on the video, person, and studio list pages.
- **A facet-first remediation queue** (new owner-mode route): grouped by missing facet (e.g. "Missing
  poster · 9"), each group sub-split into **candidate-ready** (an unapplied enrichment candidate is
  cached) and **needs-research** (nothing cached; the owner has to search or upload). Row-level actions
  only — apply a candidate, jump to search, or upload — one entity at a time.
- **A per-entity completeness breakdown panel** on the video/person/studio detail pages: every scored
  facet listed with its resolved tier (curated / provider / missing / not-applicable), so an owner looking
  at a low score on one entity can see exactly why without leaving the page.
- **One composite Studio `branding_image` facet** — the icon/logo/poster roles introduced by F51
  (ADR-079) score as a single facet ("has at least one branding image"), not three independent ones.
- **Three-skin theming** (Cinémathèque, Broadcast, Brutalist) for every new surface, tokens-only.

### Out of scope (tracked follow-ups, not gaps)

- **Bulk-apply from the remediation queue.** Individual apply/search/upload actions only. Mirrors the
  HOLODEX-199 extraction-queue precedent: an `auto_apply`/bulk flag was deliberately deferred there to
  avoid the queue becoming an untrusted firehose, before enough real usage data existed to trust it. Same
  reasoning applies here — this is a fresh queue with no track record yet.
- **Owner-configurable facet criticality.** v2 has no weight ratio to tune (the bands are never
  blended); what remains tunable is *which band a facet sits in*, and that stays a registry edit
  (`registry.go`, the `edition` precedent) — a per-deployment `criticality:` override in
  `metadata-mappings.yaml` was considered for F65 and deferred with F55.14/F55.18, because the v2
  formula removes the pressure that made per-deployment tuning attractive: an extra that never fills no
  longer touches the score. Auto-muting facets that are missing library-wide was rejected outright — the
  denominator would move under the owner.
- **Not-applicable UI on any facet other than `external_provider_id`.** The tri-state data model is
  generic; every other facet's UI stays binary (resolved/missing) in v1. Widening the affordance to more
  facets (e.g. `deathdate`, studio `country`) is a P2 follow-up (F55.16) once the pattern is proven.
- **A library-wide completeness dashboard/trend chart.** A simple aggregate stat is P1 (F55.15); a full
  historical trend view is not in this spec.
- **Score or actionability exposed on any public (non-owner) endpoint.** Both are owner-curation signals,
  gated the same way the extraction queue is — see § Access control & security.
- **Re-litigating which specific facets are critical vs. nice-to-have vs. optional.** The tables below
  are the owner's 2026-09-18 ruling (F65 RD4); moving a facet between bands later is a one-line registry
  edit plus a boot-time recompute, not a spec change.
- **A score for films.** Films are not a scored entity type; the film facet on a *video* is an extra. If
  films ever get a ring, that is one more `entity_type` in ADR-099's tables, not a formula change.

---

## Personas

- **Owner / admin** — the only persona this feature serves. Sorts/filters browse pages by completeness,
  works the remediation queue, reads per-entity breakdown panels, and toggles the not-applicable flag.
  Reuses the existing owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)).
- **Viewer** — unaffected. No score, filter, sort, or queue surface is visible outside Owner mode.
- **Metadata provider (system actor)** — unaffected directly; its resolved output is what the score reads.
  An applied enrichment candidate moves a facet from `missing` to `resolved` the same way it always has.

---

## User stories

Ordered by priority.

1. **As the owner browsing my video library, I want to sort by completeness score** so I can see my
   least-filled-in entities first without scrolling the whole library by eye.
2. **As the owner, I want to filter the browse list by a specific missing facet** (e.g. "Missing poster")
   so I can target one kind of gap at a time — this is the more useful surface once I know what I'm
   looking for, vs. sorting by the aggregate score.
3. **As the owner, I want a queue grouped by missing facet, split into "ready to apply" vs. "needs
   research"** so I can knock out the quick wins first and know which entities actually need me to go find
   something.
4. **As the owner, I want to apply a cached enrichment candidate to fill a gap directly from the queue**
   so fixing a quick win doesn't require navigating to the entity page first.
5. **As the owner looking at one entity's low score, I want a breakdown panel showing exactly which
   facets are missing, resolved, or not-applicable** so I understand the score without cross-referencing
   the queue.
6. **As the owner, when a video genuinely has no external provider ID** (self-published/home content), **I
   want to mark that facet not-applicable** so it stops permanently dragging down that video's score and
   cluttering the queue.
7. **As the owner, I want a full ring to mean "every required facet is present"** — regardless of how
   many extras are filled and regardless of whether a provider or I supplied the value — so I can trust
   the badge without reading a legend. *(v2, replaces the v1 story that the score should reward curated
   over provider-resolved values; provenance stays visible in the breakdown panel's `ProvenanceBadge`.)*
8. **As the owner, I want a missing critical facet (title, studio, cast, poster) to always cost more than
   any number of missing nice-to-haves** so the sort surfaces the entity with a real gap above the one
   that merely lacks a tagline. *(v2: enforced structurally — extras are a separate number, not a
   weight.)*
9. **As the owner scanning a grid in owner mode, I want to see each card's completeness at a glance** so I
   can spot what needs work while browsing, without switching to the Completeness sort first. *(F65)*
10. **As the owner, I want facets that will never fill for my library** (theatrical-release metadata on
    non-theatrical content) **to stop counting against anything** so the numbers describe my library,
    not TMDB's schema. *(F65)*

---

## Scoring model

> **v2 (F65, 2026-09-18).** This section replaces the v1 weighted-sum model. v1 blended critical and
> nice-to-have facets into one number with a 3:1 weight and a 0 / 0.7 / 1.0 source tier; with 4 critical
> and 12 nice-to-have video facets the two bands carried *equal* aggregate weight, so every required
> facet curated and nothing else scored 50 while a missing poster with everything else curated scored
> 88. The owner's report — "a media file can have all required facets and still sit at 69%" — was this
> inversion. [ADR-099](../architecture/ADR-099-completeness-score-required-band.md) D1 records the
> decision; the v1 formula is preserved in ADR-081 D3.

### Bands, not weights

Every scored facet sits in exactly one **band**; the bands are never combined into one number.

| Band | Registry value | What it produces |
|---|---|---|
| **Required** | `critical` | `required` — **the score**: the number in the panel, the ring on the card, the primary sort key. |
| **Extras** | `nice_to_have` | `extras` — a separate number: the sort tiebreaker and the ring's overfill. |
| Optional | `optional` | Nothing. Listed in the panel payload (with tier, label and `curatable`, so a deep link can render an empty row — F60 RD6) but never scored, never counted as missing, never queued. |
| *(excluded)* | *(unset)* | Nothing, and not listed — see "Excluded fields". |

A facet is **present** when it has a resolved value (`WinningSource != ""`) and **missing** otherwise.
Presence is **binary**: a provider-resolved value and an owner-curated value count the same. Provenance
is the breakdown panel's job (`ProvenanceBadge`, design handoff DD7), not the score's. A facet in
`facet_not_applicable` (ADR-081 D2) is excluded from its band's numerator and denominator.

**Formula** (per entity, per band, over that band's applicable facets):

```
required = round(100 × present_critical      / applicable_critical)
extras   = round(100 × present_nice_to_have  / applicable_nice_to_have)
```

**Edge rules:**

| Case | Rule |
|---|---|
| No applicable required facets for the entity type (studios today) | `required` is **`null`**, not 100. `extras` *is* the ring and the primary sort key for that type. A studio with nothing filled draws an empty ring, never a full one. |
| No applicable extras for the entity type | `extras` is `null`; the ring never overfills. |
| Every required facet marked not-applicable on one entity | `required` is `null` for that entity (same rule, per entity). |
| Optional facets | Never in either band. |

**Actionability** (unchanged from v1 — queue-only, never affects either number):

```
actionability = (# missing scored facets with a cached unapplied enrichment candidate) / (# missing scored facets)
```

"Scored" here means required + extras; optional facets are neither missing nor actionable.

### Worked examples

| Entity | Required facets | Extras facets | `required` | `extras` | Ring |
|---|---|---|---|---|---|
| Video A | title ✓ studio ✓ actors ✓ poster ✓ | overview ✓ genres ✓ release_date ✗ external_provider_id *n/a* | **100** | round(100 × 2/3) = **67** | full, overfilled ⅔ of a lap |
| Video B | title ✓ studio ✓ actors ✓ **poster ✗** | all four ✓ | **75** | **100** | ¾ ring — no overfill, however many extras are filled |
| Video C | all four ✓ (two of them provider-resolved) | none ✓ | **100** | **0** | full — provider-resolved is present |
| Person | photo ✗ | bio ✓ birthdate ✗ | **0** | **50** | empty ring, no overfill |
| Studio | *(none)* | branding_image ✓ | **null** | **100** | full — extras is the ring for studios |
| Studio, nothing set | *(none)* | branding_image ✗ | **null** | **0** | empty ring |

Under v1 a video shaped like A (required done, extras patchy) scored in the 60s while one shaped like B
(poster missing, every extra filled) scored 88 — B ranked as *more* complete. Under v2, B's ring is
visibly ¾ and it sorts below A.

### Facet tables per entity type

Source: `internal/registry/registry.go` `KnownFields`. Fields not listed (e.g. file-technical metadata
like codec/framerate) are not part of this feature — completeness scores canonical *content* metadata,
not technical file properties. **v2 (F65 RD4)** demotes the facets marked *Optional (F65)* below; each
was a nice-to-have in v1.

**Video**

| Facet | Band | Notes |
|---|---|---|
| `title` | **Required** | |
| `studio` | **Required** | |
| `actors` (cast) | **Required** | |
| `poster_url` | **Required** | Visual — matches the same aesthetic priority as person `photo`. |
| `overview` | Extras | |
| `release_date` | Extras | |
| `genres` | Extras | |
| `external_provider_id` | Extras | Tri-state — the only v1 not-applicable UI target; keeps its toggle. |
| `original_title` | Optional (F65) | Theatrical-release metadata; structurally empty for non-theatrical content. |
| `tagline` | Optional (F65) | Same. |
| `status` | Optional (F65) | Same. |
| `original_language` | Optional (F65) | Same. |
| `homepage` | Optional (F65) | Same. |
| `collection` (film) | Optional (F65) | Same. |
| `director` | Optional (F65) | Rarely applies to the owner's content. |
| `runtime` | Optional (F65) | Always present from the file layer — free points that said nothing. |
| `edition`, `part` | Optional | Unchanged (F60 RD6, HOLODEX-389). |

**Person**

| Facet | Band | Notes |
|---|---|---|
| `photo` | **Required** | The only required person facet, so a person's ring is 0 or 100 — by design: "has a face" is the at-a-glance question. |
| `bio` | Extras | |
| `birthdate` | Extras | |
| `nationality` | Optional (F65) | Low signal for "is this profile complete". |
| `alternate_names` (aliases) | Optional (F65) | Most people legitimately have none. |
| `website` | **Excluded** | Unchanged — low signal. |
| `deathdate` | **Excluded** | Unchanged — legitimately absent for most (living) people. |
| `age`, `age_at_death` | **Excluded** | Unchanged — computed from `birthdate` (ADR-063). |

**Studio**

| Facet | Band | Notes |
|---|---|---|
| *(no required facets)* | — | Per the owner's F55 call, "everything is nice to have, nothing critical." `required` is `null`; `extras` is the ring (§ Edge rules). |
| `branding_image` (composite of `icon`/`logo`/`poster`, F51/ADR-079) | Extras | Resolved if **any** of the three roles is set — one facet, not three. With `description` and `country` demoted this is the *only* studio extra, so a studio's ring is 0 or 100: "has branding art". |
| `description` | Optional (F65) | |
| `country` | Optional (F65) | |

### Excluded fields

A field is excluded from scoring (not `missing`, not counted at all, not listed) when it structurally
can't carry a meaningful "gap" signal:

- **Zero-source, manual-only fields** — there's no provider tier to distinguish from curated, and "no
  value set" is a completely normal, non-deficient state. (F52's `commentary` field was the original
  exemplar of this category; it was retired 2026-08-16, [HOLODEX-115](https://whoiskevinrich.atlassian.net/browse/HOLODEX-115) —
  see [video-owner-mode-editing.md](video-owner-mode-editing.md)'s superseded note. The category itself
  remains valid for any future zero-source field.)
- **Computed/derived fields** (`age`, `age_at_death`, ADR-063) — always in lockstep with their source
  field; scoring them is redundant.
- **Low-signal or usually-absent-by-default fields** (person `website`, `deathdate`) — see the tables
  above for the specific rationale on each.

*Optional* is the softer sibling of excluded: the facet is still **listed** (so the panel and deep links
can show an empty, curatable row) but never scored. Use *excluded* only when listing it would be noise
too.

---

## Functional requirements

### Must-have (P0)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F55.1 | The field registry carries a **criticality** (`critical` \| `nice_to_have` \| `optional` \| unset-for-excluded) per facet, matching the tables above. **v2:** the F65 demotions land here. | `Lookup()` for `title` returns `critical`; for `tagline` returns `optional`; for `deathdate` returns no criticality (excluded). |
| F55.2 | Every scored facet on an entity resolves to a **tri-state status**: `resolved` (with a source tier), `missing`, or `not-applicable`. | A video with a curated title reads `resolved`/`curated`; an unset genres reads `missing`; a video with `external_provider_id` marked not-applicable reads `not-applicable`. |
| F55.3 | **Completeness score** is computed per the § Scoring model formula by the pure `resolver.Complete` post-pass. **v2:** the result is *also* materialized per entity for list surfaces ([ADR-099](../architecture/ADR-099-completeness-score-required-band.md) D3/D4 — see F65.5–7); the detail page still computes live. | For § Worked examples Video A the API returns `required: 100, extras: 67`. Curating the poster on Video B moves it to `required: 100` on the next owner read of any list, with no manual backfill. |
| F55.4 | **Actionability** is computed separately from the score, per the formula above, and never influences `completeness_score`. | A video with 2 missing facets, 1 with a cached candidate, reports `actionability: 50` alongside an unchanged `completeness_score`. |
| F55.5 | Owner-mode video/person/studio browse pages get a **"Completeness" sort order**. **v2:** the key is the composite `(required, extras)` — `required` first (or `extras` alone where `required` is `null` for the type), `extras` breaks ties, then the type's default order. | With Owner mode active, the sort dropdown offers Completeness (ascending/descending); ascending on the video list puts Video B (75/100) before Video A (100/67) and Video A before Video C (100/0). |
| F55.6 | Owner-mode browse pages get a **"Missing facet" filter chip**, built on the existing `FacetFilter.svelte` component (same pattern as the Tags filter), listing scored facets for that entity type. | Selecting "Missing poster" on the video list shows only videos whose `poster_url` facet is `missing`; the browse-page filter and the remediation queue (F55.7) share one backend predicate so their counts never disagree. |
| F55.7 | A new owner-mode **remediation queue** route lists entities grouped by missing facet, each group split into **candidate-ready** and **needs-research**. | Visiting the queue shows a "Missing poster · 9" group with a candidate-ready sub-list and a needs-research sub-list whose combined count is 9. |
| F55.8 | Queue rows support **individual** actions: apply a cached candidate (candidate-ready rows), or jump to search/upload (needs-research rows). No bulk action exists in v1. | Clicking Apply on a candidate-ready row applies that one candidate and removes the row from the queue; there is no "select all" or "apply all" control anywhere on the page. |
| F55.9 | A **per-entity completeness breakdown panel** on the video/person/studio detail page lists every scored facet with its resolved tier. **v2:** the panel headline shows `required` as the score and `extras` beside it; provenance (curated/provider) stays per-row via `ProvenanceBadge` since it no longer moves the number. | Opening Video A shows `100` with `extras 67`, and lists its 8 scored facets with their tier; optional facets are not listed in the panel (F60 RD6). |
| F55.10 | The owner can **mark `external_provider_id` not-applicable** for a video via an owner-gated mutation; the flag persists and the facet is excluded from that video's score and from the queue. | An owner PATCH marking the facet not-applicable removes that video from any "missing external ID" queue group and excludes the facet from its score on the next read; a non-owner request is rejected by the gate. |
| F55.11 | `imdb_id` is generalized to a provider-agnostic **`external_provider_id`** concept in the registry and API, without breaking existing resolver/decision plumbing for videos that already have an IMDb value stored. Resolved values are namespace-qualified (`"<provider>:<id>"`) so they stay unambiguous when more than one provider can populate the facet ([ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md)). | Existing videos with a stored `imdb_id` value continue to resolve correctly under the renamed/generalized facet, with their value namespace-qualified (`"imdb:tt..."`) by the migration; no data loss on migration. |
| F55.12 | All new surfaces (browse filter chip, sort option, remediation queue, breakdown panel, not-applicable control, **v2: ring badge**) render correctly in **all three skins** using semantic tokens only. | QA in Cinémathèque, Broadcast, and Brutalist: `rg 'zinc-\|sky-\|emerald-\|amber-\|rounded-(lg\|md\|sm\|xl)'` over new components is empty; every state (loading/empty/populated) reads correctly in each skin. |
| F65.1 | **Required-band score.** `required` is computed per § Scoring model over `critical` facets only, with binary presence; no provider/curated weighting anywhere in the number. | Video C (all four required present, two provider-resolved) reports `required: 100`. Video B reports `required: 75`. |
| F65.2 | **Separate extras.** `extras` is computed over `nice_to_have` facets only and is never summed, averaged or weighted into `required`; both are integers 0–100 or `null` per § Edge rules. | Video B reports `extras: 100` and `required: 75` — no field in the payload combines them. A studio reports `required: null`. |
| F65.3 | **Optional demotions.** The facets marked *Optional (F65)* in § Facet tables carry `optional` criticality: listed in the panel payload, never scored, never missing, never queued, never in the "Missing facet" chip. | The video "Missing facet" chip offers at most `title, studio, actors, poster_url, overview, release_date, genres, external_provider_id` (a facet no video is missing has no stored missing row and is not offered — there is nothing to filter to); a video with no `tagline` has no `tagline` entry in any queue group. |
| F65.4 | **Ring badge on every entity card in owner mode.** Wherever a video / person / studio card renders (browse grids, entity Videos sections, landing shelves, film scenes), the owner sees a ring in the card's bottom-left slot: `required` fills the ring; once `required` is 100, `extras` draws as a second lap over it (the "overfill"). Where `required` is `null`, `extras` fills the ring and there is no overfill. Nothing renders for a visitor or when the item carries no `completeness`. | Video B shows a ¾ ring and no overfill regardless of its extras; Video A shows a full ring with ⅔ of a second lap; a studio with branding shows a full ring; a visitor's grid shows no rings. |
| F65.5 | **Owner-only `completeness` on list items.** Every video / person / studio list response carries `completeness: { required, extras }` per item when the requester passes the owner gate, on every sort — not only the Completeness sort — and the field is absent for a visitor (same redaction seam as file metadata). Values come from the materialized store ([ADR-099](../architecture/ADR-099-completeness-score-required-band.md) D3), so the default-sort list page pays no per-request library resolve. | `GET /media` as owner: every item has `completeness`; as visitor: no item does. A grid of 50 cards issues no request beyond the list call. |
| F65.6 | **Materialized store with trigger-fed invalidation.** The score is persisted per entity and invalidated by SQL triggers on every table `Complete` reads from, drained on the next owner read; boot, mapping reload, and promotion/claim writes mark every entity dirty. The detail page computes live and rewrites a stale row. A test enumerates the input tables and asserts each write leaves a dirty row. | Curating a poster from the media page, then loading `/` as owner, shows the updated ring without a restart. Adding a scored input table without its trigger fails the enumerating test. |
| F65.7 | **Composite sort and facet counts read the store.** The Completeness sort is a SQL `ORDER BY` with normal `LIMIT`/`OFFSET` paging; `GET /completeness/facets` counts come from the stored missing-facet rows. The remediation queue keeps its live resolve (it needs actionability). | Page 2 of the Completeness sort is a `LIMIT 50 OFFSET 50` query, not a full-library resolve; the chip's "Missing poster · 9" equals the number of stored `poster_url` missing rows. |
| F65.8 | **The ring is a button that fires a single-entity refresh.** `CompletenessRing` renders a `<button type="button">` (never `role="img"`) whenever it is mounted; clicking it calls the existing owner-gated `POST /{people\|studios\|media\|films}/{id}/enrich/refresh-all` — the same `RefreshPair` fan-out over every provider that supports the kind, `Force: true`, that the F66 sweep runs per entity (ADR-103 D7). **Sweep semantics:** fire-and-forget; a `needs_review` result is not surfaced (no picker — the entity page is where review happens); `rate_limited` / errors return the ring to idle with no toast. While the request is in flight the ring is `aria-busy`, disabled, and draws a spinning quarter arc; when it resolves the mount site re-fetches the item (or its list) and the ring redraws to the stored score, which the F65.6 dirty-drain refreshed on that owner read. **Never nested in a link:** at every mount the ring is a sibling of the card/row `<a>` (or checkbox `<label>`), so a click neither navigates nor toggles selection. Props: `required`, `extras`, `size`, plus `entity: { kind, id }` and `onrefreshed()`. Still owner-only by payload. | Owner clicks the ring on Video B (poster missing, a provider has one): the request fires, the ring spins, the grid re-fetches, Video B's ring is now full. Click on a person row in select mode: the checkbox does not toggle. A visitor sees no ring. Tab reaches the ring after the card link. |

### Nice-to-have (P1)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F55.13 | Studio `branding_image` composite facet resolves `resolved` if **any** of icon/logo/poster (F51/ADR-079) is set. | A studio with only a logo set (no icon, no poster) reads `branding_image: resolved`. |
| F55.14 | ~~An owner-facing setting to retune the critical/nice-to-have weight ratio (default 3:1).~~ **Superseded by F65** — v2 has no ratio. What remains is *band assignment*; a per-deployment `criticality:` override in `metadata-mappings.yaml` is the surviving form of this idea, deferred (§ Scope). | — |
| F55.15 | A simple **library-wide average completeness** stat (per entity type), surfaced on an existing admin/activity surface. | The activity/admin page shows "Videos: 71% avg completeness" (or similar), refreshed on read. |

### Future considerations (P2)

| ID | Requirement | Notes |
|----|-------------|-------|
| F55.16 | Extend the not-applicable UI affordance to more facets beyond `external_provider_id` (e.g. person `deathdate`, studio `country`). | v1 deliberately scopes the UI narrow while keeping the underlying tri-state model generic; this widens the UI once the pattern is proven. |
| F55.17 | Bulk-apply from the remediation queue. | Deferred pending real usage data on the individual-apply-only queue, per the HOLODEX-199 precedent. |
| F55.18 | ~~Per-field configurable weights beyond the two-tier split.~~ **Superseded by F65** — bands, not weights. See F55.14. | — |

---

## Data, storage & serving (direction — finalized in the ADR)

- **Facet criticality** is new metadata on `registry.FieldDef` (alongside the existing `Canonical`,
  `Label`, `Display`) — a static, code-level table, not a DB-backed setting in v1 (F55.14's config surface
  is P1).
- **The completeness score is computed by the pure resolver post-pass and, as of v2, materialized for
  list surfaces** ([ADR-099](../architecture/ADR-099-completeness-score-required-band.md) D3/D4):
  `entity_completeness(entity_type, entity_id, required, extras)` backs the badge and the SQL sort;
  `entity_completeness_missing(entity_type, entity_id, canonical, band)` backs the "Missing facet" chip
  and its counts; `completeness_dirty` is filled by `AFTER INSERT/UPDATE/DELETE` triggers on every input
  table (the FTS triggers are the precedent) and drained under `writeMu` on the next owner read. Rows are
  a cache of `Complete`'s output, never a source of truth — the detail page computes live and self-heals
  the row. **Actionability stays computed, not stored** (queue-only; its candidate inputs are not in the
  store).
- **Tri-state not-applicable status needs new persistence.** The resolver's existing `FieldDecision{Source,
  Standing, ManualValue}` (ADR-052) is the natural seam — likely a new `Standing` value (e.g.
  `not_applicable`) rather than a parallel table, so the not-applicable flag rides the same per-field
  standing-decision infrastructure already used for manual overrides. Exact shape (new `Standing` value vs.
  a dedicated column/table) is an ADR decision, not settled here.
- **Actionability reads the existing `Candidates` field** on `ResolvedField` — no new storage. A facet is
  "candidate-ready" when its `Candidates` list is non-empty and the facet's tier is `missing`.
- **The browse-page filter and the remediation queue share one backend predicate** ("entities where facet
  X is missing/not-applicable") so their counts can't drift apart — this is a requirement (F55.6), not
  just an implementation preference.

---

## Frontend / theming requirements

- **Reuse `FacetFilter.svelte`** as-is for the "Missing facet" browse filter — same component, new
  instance, no new filter-chip component needed.
- **Reuse `SortDropdown`** — add "Completeness" as a new sort option alongside the existing ones.
- **New remediation-queue route**, structurally mirroring the existing `/owner/extraction` queue (route
  shape, owner-mode gating, empty/loading states).
- **Visual language, not shared code, from `ExtractionQueueRow.svelte`** — the established
  `tier: 'conflict' | 'weak' | null` badge pattern (`text-warn`/`border-warn` tokens) is the right
  "needs attention" vocabulary for candidate-ready vs. needs-research rows, applied fresh to this queue's
  own row component (the two queues have different interaction models and should not share a component).
- **New breakdown-panel component** for the detail-page facet list. **v2:** headline = `required`, with
  `extras` beside it; per-row `ProvenanceBadge` unchanged.
- **New ring-badge component (F65)** rendered by `VideoCard`, the people grid card, and the studio card in
  the free **bottom-left** slot (top-left = resolution bucket, top-right = scene number, bottom-right =
  duration). SVG ring: track `rule`/`muted`, required arc `accent`, overfill arc `ink`, on the
  `bg-black/70` chip idiom the duration pill already uses — **no new tokens**. Renders only when the item
  carries `completeness` (owner) — no `isOwner` check in the card, the payload is the gate. The second
  lap is drawn only when `required === 100` (or `required === null`: never). Design handoff to fix the
  exact geometry and the three-skin QA list.
- **Tokens only** — no hardcoded palette/radii/fonts on any new component.
- **QA all three skins** for every state: loading, empty, populated, and the not-applicable control.

---

## Access control & security

- **Every new surface is owner-mode only.** Sort/filter options, the remediation queue route, the
  breakdown panel's owner-only controls, and the not-applicable mutation are all gated behind the existing
  owner check (ADR-030) — same choke point as the extraction queue and enrichment.
- **No new external network surface.** The score and actionability are computed from data the resolver
  already holds; nothing new is fetched from providers by this feature.
- **The not-applicable toggle is the only new mutation.** It's a simple owner-gated boolean-ish flag on an
  existing per-field decision record — no new PII, no binary ingest, no new attack surface class beyond
  "another owner-gated write."
- **Score/actionability values are not exposed on public (non-owner) endpoints** — they're an owner
  curation signal, not library metadata a viewer needs. **v2:** the list-item `completeness` object
  (F65.5) is stripped for visitors by the same `redactFileMetadataForVisitors` seam that strips file
  metadata; a visitor list response must not carry the key at all, not carry it as `null`.
- **The trigger migration (F65.6) is a schema change on tables that already exist** — no new external
  surface, but `/security-review` should confirm the triggers cannot be reached by an unauthenticated
  write path that was previously harmless (they only `INSERT OR IGNORE` an id pair; no data leaves).
- A **lightweight `/security-review`** is still warranted before merge (per the project's
  auth/access-touching change-routing rule) given the new mutation, even though the surface is small.

---

## Success metrics

**Leading (days–weeks):**
- **Queue usage** — remediation-queue page views and apply/search/upload actions taken per week, after
  enabling.
- **Sort/filter adoption** — % of owner browse-page sessions that use the Completeness sort or a
  "Missing facet" filter within the first month.

**Lagging (weeks–months):**
- **Library-wide average completeness trend** — the F55.15 aggregate stat, tracked quarter-over-quarter,
  should trend upward once the owner starts using the queue.
- **Critical-facet-missing count** — the number of entities missing at least one *critical* facet
  (title/studio/cast/poster for video, photo for person) should decrease over a quarter.

Measurement: instrument via the existing activity/metrics surfaces (F21, ADR-026/028), consistent with how
other owner-tooling features measure adoption.

---

## Resolved Decisions

- **Multi-provider external-ID shape.** Production enrichment is not limited to a single provider —
  ADR-033's declared-not-compiled-in provider registry lets an operator run providers outside this repo
  against a live instance. Resolved: `external_provider_id` stays a **single scalar** `field_key` (no
  provider column, no schema change) but its **value** is namespace-qualified (`"<provider>:<id>"`),
  reusing the convention `entity_enrichment.match_id`/`_studio_external_ids`/`person_external_ids`
  already established, rather than F49's `field_claims`-style dedicated `provider` column (that
  mechanism disambiguates key *identity*, not a value). See
  [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md), which
  supersedes [ADR-081](../architecture/ADR-081-entity-completeness-score.md) D5 on this point only.

---

### F65 (2026-09-16 brainstorm → 2026-09-18 spec), decided by the owner via cards

- **RD1 — Required band is the score; extras separate.** A 75/25 band split and "keep the formula, just
  prune" were both rejected: each keeps one blended number, so the ring could not say "required is done"
  without a legend. "Required only with extras unscored" was rejected because the sort would quantize to
  ~5 buckets per type. [ADR-099](../architecture/ADR-099-completeness-score-required-band.md) D1/D2.
- **RD2 — Binary presence.** The 0.7 provider tier is dropped from scoring. In a required-only score it
  was the *only* thing between 70 and 100 on the ring, and for the owner's job (fill gaps) a
  provider-resolved poster is a present poster. Provenance stays in the panel.
- **RD3 — Mute lever is the registry.** Demotions are `CriticalityOptional` edits in `registry.go`, the
  `edition` precedent — not a per-deployment yaml override (deferred with F55.14), not auto-mute by
  library prevalence (rejected: moving denominator).
- **RD4 — The demoted list.** Video: `original_title`, `tagline`, `status`, `original_language`,
  `homepage`, `collection`, `director`, `runtime`. Person: `nationality`, `alternate_names`. Studio:
  `description`, `country`. Accepted consequences: a person's ring is 0/100 (photo), a studio's ring is
  0/100 (branding image).
- **RD5 — Ring meter, bottom-left slot.** Chosen over a percent pill (a grid of digits), a gap count
  (only honest once the noise is gone, and still digits), and a warn-tinted frame with no badge (uses the
  `warn` token for something that is not a problem).
- **RD6 — Overfill = second lap (O2).** Extras draw over the ring only once `required` is 100, so a
  poster-missing card can never look done. Chosen over concentric rings (the inner ring is ~8 px at 8
  columns) and ring + `+N` count (digits again).
- **RD7 — Always on in owner mode, on every card.** Not only under the Completeness sort, and not only
  on the three browse pages: `VideoCard` and the entity cards render it wherever the payload carries
  `completeness`, so shelves, entity Videos sections and film scenes show it too. This is what made
  materialization necessary (RD8).
- **RD8 — Materialize.** Compute-on-read was kept for the detail page and the queue; list surfaces read
  a store invalidated by SQL triggers and drained lazily on owner reads. The *mechanism* (triggers +
  dirty set, not Go write-time hooks) is ADR-099 D4's call, made after inventorying ~30 write sites with
  no choke point — the owner chose "materialize", ADR-099 chose how.
- **RD9 (2026-09-20, HOLODEX-435) — The ring acts.** Decided while designing the F68 person hover
  card, where the ring is the owner's only affordance: a static indicator next to a "go to the profile
  and press Refresh" path was one click too many. "Same as the sweep, for one entity" was chosen over a
  page-style refresh (picker on `needs_review`, inline errors) because a card is not a page — the sweep
  already defines what an unattended per-entity refresh does, and `refresh-all` is literally that code.
  Hoisting the ring out of every `<a>` was the cost accepted (the scene-badge precedent in
  `video/CLAUDE.md`); making the ring a button *only* in the hover card was rejected as two contracts
  for one component.

---

## Open questions

- ~~**[product] Weight-ratio tuning.**~~ Resolved by F65 RD1 — there is no ratio in v2.
- **[engineering] `imdb_id` rename timing.** Does F55.11 rename the canonical key outright (with a
  migration/alias for existing data) or introduce `external_provider_id` as a new field that supersedes
  `imdb_id` going forward, leaving the old key intact for already-stored values? Affects the ADR and the
  migration plan.

---

## Timeline & phasing

**Ships as a single release** — no phased rollout, no incremental gates between P0 requirements. All of
F55.1–F55.12 land together before this feature becomes visible in Owner mode. (The owner explicitly chose
this over a phased approach when this spec was scoped.)

Suggested internal build order (informal, non-gating — engineering may resequence freely):
1. Registry criticality metadata + tri-state facet status + score/actionability computation (F55.1–4) —
   everything else reads off this.
2. `external_provider_id` generalization + not-applicable mutation (F55.10–11) — needed before the queue
   can correctly exclude not-applicable facets.
3. Browse sort/filter (F55.5–6) and remediation queue (F55.7–8) — share the backend predicate, reasonable
   to build together.
4. Breakdown panel (F55.9) and three-skin QA pass (F55.12) last, once the data surfaces above are stable.

**F65 amendment — ships as one PR on the epic branch (HOLODEX-412), all gates green before ready:**
1. Registry demotions (F65.3) + `resolver.Complete` → `Required`/`Extras` with binary presence and the
   `null` rules (F65.1–2). Every existing scorer test is rewritten against § Worked examples.
2. Migration: `entity_completeness`, `entity_completeness_missing`, `completeness_dirty`, the trigger set;
   drain + upsert; mark-all on boot / mapping reload / promotions / claims; enumerating trigger test
   (F65.6).
3. List paths read the store — SQL composite sort, owner-only `completeness` on items, facets endpoint
   from the missing rows; detail self-heal (F65.5, F65.7). Queue untouched.
4. Ring badge component + card wiring (F65.4); panel headline; three-skin QA per the design handoff.

---

## Artifacts to produce (project working agreements)

- [x] This spec (`docs/specs/entity-completeness-score.md`).
- [ ] **ADR** — facet-criticality data model, tri-state not-applicable persistence shape, score-computation
      seam. Blocks P0 implementation. (`needs-adr` on HOLODEX-260.)
- [ ] **Design handoff** — remediation queue, breakdown panel, browse filter/sort, all three skins.
      (`needs-design` on HOLODEX-260.)
- [ ] **Testing strategy** — add an F55 block to `docs/testing-strategy.md` (scoring formula, tri-state
      resolution, queue predicate parity with the browse filter, not-applicable mutation, three-skin QA).
      (`needs-testing-strategy` on HOLODEX-260.)
- [ ] **Security review** before merge — new owner-gated mutation surface. (`needs-security-review` on
      HOLODEX-260.)
- [ ] Add this spec to the `docs/architecture/README.md` phase-specs index.

**F65 (HOLODEX-412):**
- [x] This amendment.
- [x] **ADR** — [ADR-099](../architecture/ADR-099-completeness-score-required-band.md) (supersedes
      ADR-081 D3 + D4).
- [x] **Design handoff** — [completeness-ring-badge-handoff.md](../design/completeness-ring-badge-handoff.md)
      + [completeness-ring-badge-mockup.svg](../design/completeness-ring-badge-mockup.svg) (ring geometry,
      overfill, empty/`null` states, card + row placement, three-skin QA). Landed 2026-09-18.
- [ ] **Testing strategy** — F65 block: band formulas and `null` rules against § Worked examples,
      composite sort order, visitor redaction of `completeness`, trigger-coverage enumeration, detail
      self-heal.
- [ ] **Security review** — owner gating of the new list field; the trigger migration.
      (`needs-security-review` on HOLODEX-412.)
