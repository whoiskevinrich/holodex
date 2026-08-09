# Spec: Entity Completeness Score (F55)

**Status**: Draft
**Phase**: 4 (Curation tooling)
**Issue**: [HOLODEX-260](https://whoiskevinrich.atlassian.net/browse/HOLODEX-260)
**Depends on**: per-field source-of-truth decisions and the baseline-source contract ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), [ADR-052](../architecture/ADR-052-baseline-source-contract.md)), metadata source plugins / the provider-agnostic enrichment model ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md), F22), the access-control gating seam ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)), derived/computed fields precedent ([ADR-063](../architecture/ADR-063-derived-computed-fields.md), F45), studio image roles ([ADR-079](../architecture/ADR-079-studio-image-roles.md), F51), and frontend theming ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
**Realizes**: F55 (new). Builds on the extraction-queue UX precedent ([HOLODEX-199](https://whoiskevinrich.atlassian.net/browse/HOLODEX-199)) — its deliberate deferral of bulk-apply directly informs this feature's queue design (§ Scope).
**Architecture**: [ADR-081](../architecture/ADR-081-entity-completeness-score.md) (facet criticality, not-applicable persistence, score computation, list consumption, and the `imdb_id` → `external_provider_id` rename) and [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md) (supersedes ADR-081 D5 only — the rename's value must be namespace-qualified, not a bare id).
**Design handoff (to be produced)**: `docs/design/entity-completeness-handoff.md` — the remediation queue layout, breakdown panel, and browse-page filter/sort affordances across all three skins. Tracked as `needs-design`.

---

## Objective

Give the owner a single number — the **completeness score** — that says how filled-in an entity's
canonical facets are, weighted by how much each facet matters. Surface it two ways: **sortable/filterable
on browse list pages** (find the gaps at a glance) and a **facet-first remediation queue** (find
*specific* holes — "9 videos missing a poster" — and fix them one at a time). A companion, separate
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
  fields — see § Scoring model for the formula.
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
- **Owner-mode browse-page additions**: a "Completeness" sort order and a "Missing facet" filter chip
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
- **Owner-configurable facet weights.** v1 ships fixed weights (critical = 3, nice-to-have = 1, see §
  Scoring model); a settings surface to retune the ratio is future work (F55.14).
- **Not-applicable UI on any facet other than `external_provider_id`.** The tri-state data model is
  generic; every other facet's UI stays binary (resolved/missing) in v1. Widening the affordance to more
  facets (e.g. `deathdate`, studio `country`) is a P2 follow-up (F55.16) once the pattern is proven.
- **A library-wide completeness dashboard/trend chart.** A simple aggregate stat is P1 (F55.15); a full
  historical trend view is not in this spec.
- **Score or actionability exposed on any public (non-owner) endpoint.** Both are owner-curation signals,
  gated the same way the extraction queue is — see § Access control & security.
- **Re-litigating which specific facets are critical vs. nice-to-have.** The weight table below is a
  first cut the owner can sanity-check before implementation; retuning it later is a config change, not a
  spec change (F55.14).

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
7. **As the owner, I want the score to reflect *curated* data more than *provider-resolved* data**, and
   provider-resolved data more than nothing, so the score rewards real curation instead of being satisfied
   by a low-confidence auto-match.
8. **As the owner, I want critical facets (title, studio, cast, poster) to weigh more than nice-to-have
   ones (release date, genres)** so the score reflects what actually matters for an entity to be usable in
   the library, not just a raw field count.

---

## Scoring model

### Facet weight and source tier

Every scored facet has a **criticality weight** and, per entity, a **source tier**:

| Weight class | Value | Meaning |
|---|---|---|
| Critical | 3 | The entity is meaningfully incomplete without it. |
| Nice-to-have | 1 | Improves the entity but isn't load-bearing. |
| *(excluded)* | — | Not scored at all — see "Excluded fields" below. |

| Tier | Value | Meaning |
|---|---|---|
| Missing | 0.0 | No resolved value for this facet on this entity. |
| Provider-resolved | 0.7 | Resolved, but the winning source is a metadata provider (not owner-curated). |
| Curated / manual | 1.0 | Resolved with a manual/owner-set value or standing decision (ADR-051/052). |

**Formula** (per entity, over its non-not-applicable scored facets):

```
completeness_score = round(100 × Σ(weight × tier) / Σ(weight))
```

**Actionability** (separate, queue-only, does not affect the score above):

```
actionability = (# missing facets with a cached unapplied enrichment candidate) / (# missing facets)
```

### Worked example (video)

| Facet | Weight | Tier | Weight × tier |
|---|---|---|---|
| title *(critical)* | 3 | curated (1.0) | 3.0 |
| studio *(critical)* | 3 | curated (1.0) | 3.0 |
| actors *(critical)* | 3 | provider (0.7) | 2.1 |
| poster_url *(critical)* | 3 | missing (0.0) | 0.0 |
| overview *(nice-to-have)* | 1 | provider (0.7) | 0.7 |
| release_date *(nice-to-have)* | 1 | curated (1.0) | 1.0 |
| genres *(nice-to-have)* | 1 | missing (0.0) | 0.0 |
| external_provider_id | — | *marked not-applicable* — excluded from both sums | — |

Σ(weight × tier) = 9.8, Σ(weight) = 15 → **completeness_score = round(100 × 9.8 / 15) = 65**.

Of this video's 2 missing facets (`poster_url`, `genres`), say a provider candidate is cached for
`poster_url` but not `genres` → **actionability = 1/2 = 50%**. The queue would surface this video's
poster gap in the candidate-ready group and its genre gap in needs-research.

### Facet tables per entity type

Source: `internal/registry/registry.go` `KnownFields`. Fields not listed (e.g. file-technical metadata
like codec/framerate) are not part of this feature — completeness scores canonical *content* metadata,
not technical file properties.

**Video**

| Facet | Weight | Notes |
|---|---|---|
| `title` | Critical | |
| `studio` | Critical | |
| `actors` (cast) | Critical | |
| `poster_url` | Critical | Visual — matches the same aesthetic priority as person `photo`. |
| `overview` | Nice-to-have | |
| `release_date` | Nice-to-have | |
| `genres` | Nice-to-have | |
| `runtime` | Nice-to-have | |
| `status` | Nice-to-have | |
| `original_language` | Nice-to-have | |
| `tagline` | Nice-to-have | |
| `original_title` | Nice-to-have | |
| `homepage` | Nice-to-have | |
| `collection` | Nice-to-have | |
| `external_provider_id` (generalized from `imdb_id`) | Nice-to-have | Tri-state — the only v1 not-applicable UI target. |
| `director` | Nice-to-have | |
| `commentary` | **Excluded** | F52's zero-source, manual-first field — structurally doesn't fit the source-tier model (see below). |

**Person**

| Facet | Weight | Notes |
|---|---|---|
| `photo` | Critical | Visual aesthetic parity with video `poster_url`. |
| `bio` | Nice-to-have | |
| `birthdate` | Nice-to-have | |
| `nationality` | Nice-to-have | |
| `aliases` | Nice-to-have | |
| `website` | **Excluded** | Low signal for "is this person's profile complete." |
| `deathdate` | **Excluded** | Legitimately absent for most (living) people — not a meaningful completeness gap; not the v1 not-applicable target either (see § Out of scope). |
| `age`, `age_at_death` | **Excluded** | Computed/derived from `birthdate` (ADR-063, F45) — scoring them would double-count `birthdate`. |

**Studio**

| Facet | Weight | Notes |
|---|---|---|
| `description` | Nice-to-have | Per the owner's explicit call: "everything is nice to have, nothing critical" for studios. |
| `country` | Nice-to-have | |
| `branding_image` (composite of `icon`/`logo`/`poster`, F51/ADR-079) | Nice-to-have | Resolved if **any** of the three roles is set — scored as one facet, not three. |

### Excluded fields

A field is excluded from scoring (not `missing`, not counted at all) when it structurally can't carry a
meaningful "gap" signal:

- **Zero-source, manual-only fields** (`commentary`, F52) — there's no provider tier to distinguish from
  curated, and "no commentary" is a completely normal, non-deficient state for most videos.
- **Computed/derived fields** (`age`, `age_at_death`, ADR-063) — always in lockstep with their source
  field; scoring them is redundant.
- **Low-signal or usually-absent-by-default fields** (person `website`, `deathdate`) — see the tables
  above for the specific rationale on each.

---

## Functional requirements

### Must-have (P0)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F55.1 | The field registry carries a **criticality** (`critical` \| `nice_to_have` \| unset-for-excluded) per scored facet, matching the tables above. | `Lookup()` for `title` returns `critical`; `Lookup()` for `commentary` returns no criticality (excluded). |
| F55.2 | Every scored facet on an entity resolves to a **tri-state status**: `resolved` (with a source tier), `missing`, or `not-applicable`. | A video with a curated title reads `resolved`/`curated`; an unset genres reads `missing`; a video with `external_provider_id` marked not-applicable reads `not-applicable`. |
| F55.3 | **Completeness score** is computed per the § Scoring model formula, at read/resolve time (not stored), mirroring the ADR-063 computed-field precedent. | For the § Worked example inputs, the API returns `65` for that video. Changing a facet's resolved tier (e.g. curating the poster) changes the score on the next read with no migration or backfill. |
| F55.4 | **Actionability** is computed separately from the score, per the formula above, and never influences `completeness_score`. | A video with 2 missing facets, 1 with a cached candidate, reports `actionability: 50` alongside an unchanged `completeness_score`. |
| F55.5 | Owner-mode video/person/studio browse pages get a **"Completeness" sort order**. | With Owner mode active, the sort dropdown offers Completeness (ascending/descending); selecting it reorders the list by each entity's score. |
| F55.6 | Owner-mode browse pages get a **"Missing facet" filter chip**, built on the existing `FacetFilter.svelte` component (same pattern as the Tags filter), listing scored facets for that entity type. | Selecting "Missing poster" on the video list shows only videos whose `poster_url` facet is `missing`; the browse-page filter and the remediation queue (F55.7) share one backend predicate so their counts never disagree. |
| F55.7 | A new owner-mode **remediation queue** route lists entities grouped by missing facet, each group split into **candidate-ready** and **needs-research**. | Visiting the queue shows a "Missing poster · 9" group with a candidate-ready sub-list and a needs-research sub-list whose combined count is 9. |
| F55.8 | Queue rows support **individual** actions: apply a cached candidate (candidate-ready rows), or jump to search/upload (needs-research rows). No bulk action exists in v1. | Clicking Apply on a candidate-ready row applies that one candidate and removes the row from the queue; there is no "select all" or "apply all" control anywhere on the page. |
| F55.9 | A **per-entity completeness breakdown panel** on the video/person/studio detail page lists every scored facet with its resolved tier. | Opening a video with a 65% score shows a panel listing all 15 scored facets with their tier (curated/provider/missing/not-applicable), matching the § Worked example. |
| F55.10 | The owner can **mark `external_provider_id` not-applicable** for a video via an owner-gated mutation; the flag persists and the facet is excluded from that video's score and from the queue. | An owner PATCH marking the facet not-applicable removes that video from any "missing external ID" queue group and excludes the facet from its score on the next read; a non-owner request is rejected by the gate. |
| F55.11 | `imdb_id` is generalized to a provider-agnostic **`external_provider_id`** concept in the registry and API, without breaking existing resolver/decision plumbing for videos that already have an IMDb value stored. Resolved values are namespace-qualified (`"<provider>:<id>"`) so they stay unambiguous when more than one provider can populate the facet ([ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md)). | Existing videos with a stored `imdb_id` value continue to resolve correctly under the renamed/generalized facet, with their value namespace-qualified (`"imdb:tt..."`) by the migration; no data loss on migration. |
| F55.12 | All new surfaces (browse filter chip, sort option, remediation queue, breakdown panel, not-applicable control) render correctly in **all three skins** using semantic tokens only. | QA in Cinémathèque, Broadcast, and Brutalist: `rg 'zinc-\|sky-\|emerald-\|amber-\|rounded-(lg\|md\|sm\|xl)'` over new components is empty; every state (loading/empty/populated) reads correctly in each skin. |

### Nice-to-have (P1)

| ID | Requirement | Acceptance criteria |
|----|-------------|---------------------|
| F55.13 | Studio `branding_image` composite facet resolves `resolved` if **any** of icon/logo/poster (F51/ADR-079) is set. | A studio with only a logo set (no icon, no poster) reads `branding_image: resolved`. |
| F55.14 | An owner-facing setting to retune the critical/nice-to-have weight ratio (default 3:1). | Changing the ratio in config changes computed scores on next read, no migration required. |
| F55.15 | A simple **library-wide average completeness** stat (per entity type), surfaced on an existing admin/activity surface. | The activity/admin page shows "Videos: 71% avg completeness" (or similar), refreshed on read. |

### Future considerations (P2)

| ID | Requirement | Notes |
|----|-------------|-------|
| F55.16 | Extend the not-applicable UI affordance to more facets beyond `external_provider_id` (e.g. person `deathdate`, studio `country`). | v1 deliberately scopes the UI narrow while keeping the underlying tri-state model generic; this widens the UI once the pattern is proven. |
| F55.17 | Bulk-apply from the remediation queue. | Deferred pending real usage data on the individual-apply-only queue, per the HOLODEX-199 precedent. |
| F55.18 | Per-field configurable weights beyond the two-tier critical/nice-to-have split. | A full per-facet weight table, instead of one shared ratio (F55.14). |

---

## Data, storage & serving (direction — finalized in the ADR)

- **Facet criticality** is new metadata on `registry.FieldDef` (alongside the existing `Canonical`,
  `Label`, `Display`) — a static, code-level table, not a DB-backed setting in v1 (F55.14's config surface
  is P1).
- **The completeness score and actionability are computed, not stored** — read off the same
  `ResolvedField`/`Candidates` data the resolver already produces (ADR-051/052), the same way `age` and
  `age_at_death` are computed on read rather than persisted (ADR-063). No backfill, no migration for the
  score itself.
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
- **New breakdown-panel component** for the detail-page facet list.
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
  curation signal, not library metadata a viewer needs.
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

## Open questions

- **[product] Weight-ratio tuning.** Is critical:nice-to-have = 3:1 the right default, or should it be
  wider (e.g. 5:1) so a single missing critical facet drags the score down harder? Ships with 3:1 in v1;
  F55.14 makes it configurable later.
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
