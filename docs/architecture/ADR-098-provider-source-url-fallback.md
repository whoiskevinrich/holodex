# ADR-098: Provider source URL — `_source_url` as the per-pill fallback behind link templates

**Status:** Proposed
**Date:** 2026-09-17
**Deciders:** Project owner (brainstorm + spec 2026-09-16)

**Extends:** [ADR-083](ADR-083-provider-link-badge-person-studio.md) (revisits **D2 only** — "a namespace
with no matching template entry renders with no href"; D1 and D3 stand, and D2's template path stays
primary) · [ADR-054](ADR-054-studio-external-id-dedup.md) / [HOLODEX-258](https://whoiskevinrich.atlassian.net/browse/HOLODEX-258)
(the `_`-prefixed provider→core sidecar field channel and its "garbage overwrites, absence leaves alone"
ingest rule this reuses verbatim) · [ADR-033](ADR-033-metadata-source-plugins.md) (additive shadow store;
the provider is untrusted input).
**Relates to:** [ADR-055](ADR-055-enrichment-unique-key-invariant.md) D2 (a namespace is a shared identity space, which
is why a stored URL may only ever sit behind its *own* provider's pill) · [ADR-082](ADR-082-external-provider-id-namespace-qualified-value.md)
(video's single resolved `external_provider_id` — the pill ADR-083 D3 gives video) ·
[ADR-096](ADR-096-entity-identity-card.md) D2 (`entity_external_ids`, the row set person/studio/film pills
are projected from).
**Contract:** [metadata provider contract](../specs/metadata-provider-contract.md) §4.11 (`link_templates`,
landed with HOLODEX-391) / §4.12 (new: `_source_url` on `/enrich`, to be written from this ADR) / §8 example.
**Spec:** [Provider link badge coverage (F63)](../specs/provider-link-badge-coverage.md) — RD1 (both
mechanisms), RD2 (namespace-keyed pills), RD3 (fallback only when `ns == provider`), RD9 (TMDB uses
templates only), P0-4 / P0-5.
**Issue:** [HOLODEX-390](https://whoiskevinrich.atlassian.net/browse/HOLODEX-390) (epic) ·
[HOLODEX-392](https://whoiskevinrich.atlassian.net/browse/HOLODEX-392) (this ADR's implementation) ·
[HOLODEX-391](https://whoiskevinrich.atlassian.net/browse/HOLODEX-391) (templates, landed first).

---

## Context

ADR-083 D2 made the provider-link badge a pure function of `/describe.link_templates`: the provider
declares `namespace → entity kind → URL template`, core substitutes the id, and a namespace with no
template renders as a degraded, href-less pill. That is the right primary mechanism — one declaration
covers every entity the provider ever touches, core builds the URL, nothing per-entity is stored.

Two things F63 surfaced make it insufficient on its own:

1. **Not every provider has a template-shaped page.** The partner video provider that motivated ADR-095
   keys its pages on the *item* it matched, and its per-item URL is not a function of
   `(namespace, kind, id)` alone — it is whatever the provider returned for that item. A template cannot
   express it. Until now the only way for such a provider's pill to link was to abuse a canonical field
   (`homepage`/`website`), which is exactly the double-link HOLODEX-391 just unwound.
2. **The badge is spreading.** F63 puts the pill on film and media pages too (HOLODEX-393/394). Every
   page that gains one inherits the "degraded unless templated" rule, so the gap in (1) would now show on
   four surfaces instead of two.

The obvious fix is the one `candidates[].profile_url` already uses on `/resolve` (F47): let the provider
hand core an absolute URL for its own page, validate the scheme, and use it. The questions this ADR
settles are **where that URL rides on `/enrich`**, **where core keeps it**, and **how it composes with
templates** without letting one provider's page hide behind another namespace's pill.

### Forces

- **Template first.** A declared template is provider-*policy* (one line, every entity); a returned URL is
  provider-*data* (one row per enrich). When both exist the policy wins — it is what the provider committed
  to in its manifest, and it is what core can build without a stored row. RD1.
- **Namespace-keyed pills stay namespace-keyed.** ADR-083 D3 gives one pill per stored id, keyed by
  namespace; ADR-055 D2 makes a namespace a shared identity space (TMDB emits `imdb:` ids). A stored TMDB
  page must therefore never sit behind the IMDb pill — the reader clicks "IMDb" and lands on TMDB. RD2/RD3.
- **Additive shadow store.** ADR-033: provider output lands in `entity_enrichment` as an additive shadow,
  cleared per provider, never flattened. A per-entity provider URL is provider output and has that
  lifecycle: written on enrich, gone on clear.
- **Never a field.** The URL must not enter the resolver, the field list, curation, claims/promotions,
  completeness scoring, or writeback. The codebase already has a channel with exactly those properties.
- **Untrusted input.** It becomes an `<a href>` in a visitor's browser. Same posture as
  `sanitizeProfileURL`: http(s) or nothing, and a bad value never fails the enrich.
- **Scale.** Personal library; one extra row per `(entity, provider)` and one map lookup per pill is
  free. No new table is worth its migration unless the existing row shape cannot hold the data.
- **No data migration.** RD5 for `homepage`/`website` applies here too: nothing is backfilled; a URL
  appears on the next enrich.

## Decision

### D1 — Transport: `fields._source_url` on `/enrich`, riding the `_` sidecar channel

The provider sends its page URL as one more **`_`-prefixed field** in the `/enrich` response —
`"fields": { "_source_url": ["https://provider.example/item/123"] }` — not a new top-level key.
`model.InternalFieldPrefix` already marks a provider→core sidecar field (`_studio_external_ids`,
HOLODEX-258; `_person_external_ids`, synthesized by core): stored as an ordinary `entity_enrichment`
row, hidden from every field listing (`Service.FieldsFromRows`), refused by claims and promotions
(`field_claims.go`, `field_promotions.go`), skipped by the resolver's non-canonical pass
(`resolver/auto_register.go`), and never a registry key, so canonical resolution cannot see it. A new
top-level key would need every one of those exclusions re-implemented for a second shape; the sidecar
channel has them already, tested. **Single-valued:** the first surviving value is the URL; extra
values are dropped at ingest (D2).

Rejected: a top-level `source_url` (needs new plumbing, and the underscore is the contract's own
"internal, never a field" marker — the spec asks for it by name); a `website`/`homepage` field value (the
double-link HOLODEX-391 removed; those keys mean the entity's *own* site).

### D2 — Ingest: validate like `profile_url`; garbage overwrites, absence leaves alone

In `Service.Enrich`, after `sanitizeFields`, the `_source_url` sidecar is reshaped exactly the way
`_studio_external_ids` is (`service.go` ~L515): **only when the provider sent the key** (`raw, ok`),
`fields["_source_url"]` becomes `[]string{u}` for the first value `u` that passes `validHTTPURL`
(absolute, `http`/`https`, host present — the same predicate `sanitizeProfileURL` and
`ValidateLinkTemplate` share), or `[]string{}` when none does. The empty slice matters: `UpsertEnrichment`
upserts per key and never deletes a key merely absent from the map, so "provider sent garbage" must
*overwrite* to clear a stale row, while "provider omitted it" leaves any earlier row alone (ADR-033's
additive store — the provider may have dropped a key it once sent; the last good URL is still its page).
A malformed value never fails the enrich; the rest of the response lands. `Service.Clear` →
`DeleteEnrichmentByProvider` removes the row with the provider's other rows — no new cleanup path.

No new table, no migration: the row is `(entity_type, entity_id, provider, "_source_url", url)` under
the existing `UNIQUE (entity_type, entity_id, provider, field_key)`.

### D3 — Per-pill precedence: `template(ns, kind) ?? (ns == provider ? stored : ∅) ?? degraded`

`Service.BuildProviderLink(ctx, namespace, entityKind, id)` grows a sibling that takes the entity:

```
ProviderLink(ctx, entityType, entityID, namespace, id) (url string, ok bool)
  1. template  := LinkTemplates()[namespace][entityType]     → BuildLink(template, id)   if present
  2. stored    := row (entityType, entityID, provider=namespace, "_source_url")          if present
  3. degraded  → ("", false)
```

Step 2 looks the row up **by `provider = namespace`**. That single equality *is* RD3: a stored URL can
only ever back the pill whose namespace is the provider that stored it. There is no "which provider
enriched this entity" bookkeeping to keep — a TMDB row is keyed `tmdb`, so it can back the `tmdb`
pill and nothing else; the `imdb` pill on the same entity falls to a template or to degraded. Foreign
namespaces are template-only by construction.

`externalLinksForEntity` (person / studio / film projection, ADR-083 D1) and the media page's single
resolved pill (ADR-083 D3 / P0-7) both call it; step 2 reads the entity's enrichment rows once per
request, not once per pill.

### D4 — Video: the winning source's provider, same rule

Video has no identity rows; its pill is the resolver's winning `external_provider_id` (ADR-082). The
projection takes the winning value's namespace as `ns` and applies D3 unchanged: `provider = ns`. A
value that won from the file layer (`file:` source) has no provider row and no provider-declared
template unless some provider templates that namespace — it renders degraded, which is the owner's
ruling in P0-7 (the identity signal always renders). A value TMDB wrote as `imdb:tt…` gets the `imdb`
template or degraded, never TMDB's stored page — RD3 again.

### D5 — TMDB declares templates only

The reference sidecar does **not** also emit `_source_url` (RD9). Its pages are template-shaped, the
templates landed in HOLODEX-391, and a redundant per-entity row would only exercise the fallback path
in production for no reader-visible gain. The fallback is proven by `internal/enrich` and
`internal/api` unit tests and by the enrich stub, not by TMDB.

## Options Considered

### D1 / D2 — where the URL rides and where it lives

| Option | Verdict |
|---|---|
| **`fields._source_url` → `entity_enrichment` row (chosen)** | Reuses the `_` sidecar channel end-to-end: ingest reshaping, per-provider clear, field-list/claims/promotions/resolver exclusion, all pre-existing and tested. Zero schema change |
| Top-level `/enrich` key + new `entity_source_urls(entity_type, entity_id, provider, url)` table | Cleanest on paper, but a migration plus a second storage/clear/read path for one string per row — and every exclusion the `_` prefix gives for free would need re-deriving. Rejected on "no new table unless the row shape can't hold it" |
| Column on `job_runs` (the enrich-run activity entry) | Wrong lifecycle — activity is append-only history; the pill wants the *current* provider page, cleared with the provider |
| Put it in `entity_external_ids` next to the id | Ids are identity rows (ADR-096 D2), provider-independent by ADR-055 D2; a URL is per-provider data. Mixing them re-creates the ns/provider confusion D3 exists to prevent |

### D3 — how a stored URL composes with templates

| Option | Verdict |
|---|---|
| **Template wins; stored only for `provider == namespace` (chosen)** | Policy over data; foreign namespaces structurally template-only; no enriching-provider bookkeeping |
| Stored URL wins over template | Lets a provider's per-item quirk override its own declared shape — but also lets a stale row outlive a corrected template until the next enrich. Rejected: the manifest is re-read on every `/describe`, the row is not |
| Stored URL may back any pill on the entity the provider enriched | The "IMDb pill lands on TMDB" failure. Rejected (RD3) |

## Trade-off Analysis

- **Reuse vs. explicitness.** A `_source_url` row is less discoverable than a dedicated table — someone
  reading the schema will not see it. Mitigated by the contract §4.12 subsection, this ADR, and the
  `model.InternalFieldPrefix` doc-comment naming it alongside `_studio_external_ids`.
- **Additive store vs. staleness.** "Absence leaves alone" (D2) means a provider that *stops* returning
  the URL keeps its last one until the owner clears the provider. Consistent with every other enrichment
  row; and the alternative (delete on absence) would drop rows on a partial provider response, the very
  case ADR-033's degradation order protects.
- **Template-first vs. provider intent.** If a provider both templates a namespace and returns a
  per-item URL that differs, the template wins and the per-item URL is dead weight. That is the
  contract's guidance ("declare a template if your pages are template-shaped; return `_source_url`
  only when they are not"), so the conflict is a provider authoring error, surfaced by the doc rather
  than by precedence gymnastics.

## Consequences

- `entity_enrichment` may now hold a `_source_url` row per `(entity, provider)`. Everything that
  already skips `InternalFieldPrefix` rows skips it; a new consumer of enrichment rows must keep that
  filter (it already had to, for `_studio_external_ids`).
- `ExternalLink.URL` on person/studio/film/video projections becomes non-empty for a templated
  namespace **or** for the provider's own pill when it stored a URL. The frontend contract is unchanged
  (`{namespace, label, url}`); the client still re-validates the scheme before rendering an href.
- The contract gains §4.12 and an `_source_url` line in the §8 `/enrich` example; the enrich stub
  (`testdata/enrich-stub/`) returns one so the fallback path is exercised locally without a real
  provider.
- Security posture: one more provider-attested URL becomes a visitor-facing href, validated by the
  same `validHTTPURL` predicate as `profile_url` and link templates. It is never dialed server-side.
  The `/security-review` gate on the epic covers it.
- No data migration, no protocol bump. A provider that never sends `_source_url` and a Holodex that
  never reads it both behave exactly as before.

## Action Items

- [ ] **HOLODEX-392** — D1/D2 ingest reshaping in `Service.Enrich` next to `_studio_external_ids`;
  D3 `ProviderLink` on the Service; `externalLinksForEntity` + the video projection consume it. Unit
  tests: three precedence branches × both projection paths (`external_links_test.go`); malformed
  `_source_url` drops without failing the enrich; omitted key leaves a prior row; `Clear` removes it.
- [x] Contract §4.12 + §8 example. · [ ] `docs/testing-strategy.md` row (F63 P0-4, testing gate).
- [ ] `testdata/enrich-stub/` returns `_source_url` for one persona; `link_templates` for another, so
  both branches are visible in a local QA run.
- [ ] `model.InternalFieldPrefix` doc-comment lists `_source_url` with the other sidecar keys.
- [x] ~~Confirm with the partner video provider that its `/enrich` can return a per-item URL~~ —
  **owner ruling 2026-09-17 (F63 RD10):** it should, per entity, on every `/enrich`; contract §4.12
  is the implementation target for that sidecar (downstream repo, refreshed post-merge).
