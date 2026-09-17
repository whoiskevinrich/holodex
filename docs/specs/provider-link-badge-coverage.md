# Spec: Provider link badge coverage — person, studio, film, and media (F63)

**Status**: Draft
**Phase**: New epic (Jira [HOLODEX-390](https://whoiskevinrich.atlassian.net/browse/HOLODEX-390))
**Owner**: Project owner
**Date**: 2026-09-16
**Feature block**: **F63** — when enrichment knows a provider's page for an entity, the entity's
detail page carries a clickable badge that opens it. The badge itself shipped with F55/ADR-082
(video, designed) and HOLODEX-266/ADR-083 (person + studio, built) — this block **feeds** it
(the TMDB sidecar has never declared a single `link_templates` entry, so every pill in production
renders in the degraded "known to" state), adds a provider-returned URL as a per-pill fallback for
providers whose ids do not template, and finishes the two entities the badge never reached: film
and media.

**Depends on** (all shipped):
- [ADR-083](../architecture/ADR-083-provider-link-badge-person-studio.md) (HOLODEX-266) —
  `ProviderLinkBadge`, the `external_links` projection for person/studio, `link_templates` in the
  `/describe` manifest, `enrich.ValidateLinkTemplate` / `BuildLink`, and the D2 degraded state.
  This spec **keeps** D1 (read-only projection), D2 (template resolved server-side), and D3 (one
  badge per stored id, namespace-keyed) unchanged; the ADR this block adds only *widens* D2 with a
  fallback.
- [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md) (F55) —
  video's `external_provider_id` registry facet carries a `<namespace>:<id>` scalar with one
  resolved winner; its Display section already prescribes "use the namespace to build a provider
  link". That badge was designed but never built (see
  [HOLODEX-266's worklog](../plans/HOLODEX-266.md), 2026-08-09 entry).
- [ADR-096](../architecture/ADR-096-entity-identity-card.md) D2 (F60) — `entity_external_ids`
  is the one table person / studio / film ids live in; film ids are already written on adoption.
- [ADR-059](../architecture/ADR-059-provider-brand-icon.md) — the brand icon each pill shows.
- [metadata-provider-contract.md](metadata-provider-contract.md) — the sidecar contract this block
  extends (§2.2 gains `link_templates`, §4 gains `_source_url`).

**Related**: [film-provider-enrichment-ux.md](film-provider-enrichment-ux.md) (F59) — this block
resolves its deferred **P1-2** (provider link badge on films) ·
[docs/design/provider-link-badge-handoff.md](../design/provider-link-badge-handoff.md) — the
person/studio badge handoff this block's design gate extends ·
[ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md) — the badge is a
*precedence-layer* read-out of stored identity, never an adoption surface.

---

## Problem Statement

`ProviderLinkBadge` renders on every person and studio page, but the only provider in the wild
(TMDB) never declared the `link_templates` the badge needs — partly because
[the contract spec](metadata-provider-contract.md) never documented the key — so the owner sees a
pill that looks clickable and isn't. Film ids are stored but never projected to the page; media
has no badge at all, and its TMDB link only exists because the sidecar overwrote the `homepage`
field with the TMDB page instead of the film's real website. The owner's verification job — "did
enrichment match the right thing?" — currently ends at a dead pill on two entities and has no
starting point on the other two.

The owner's production library also runs a provider that treats the **media file itself** as the
canonical unit (its own page per file), with ids that do not reduce to a `{id}` substitution. A
template-only badge can never link those.

## Goals

1. **Every stored id links when it can.** On a TMDB-enriched person, studio, film, or video, the
   pill is an `<a>` to the provider page — no core change needed beyond the sidecar declaring
   templates.
2. **Providers without template-able ids still link.** A sidecar can hand core the exact page URL
   on `/enrich`; the pill for that provider's own namespace uses it when no template matches.
3. **All four entities share one badge, one placement rule, one payload shape.** Film and media
   consume the same `external_links: [{provider, label, url?}]` projection and mount the same
   component as person/studio.
4. **Nothing is linked twice.** With the badge owning the TMDB link, `homepage` returns to meaning
   the film's actual website.

## Non-Goals

- **Provider-keyed pills ("View on TMDB").** Rejected in the brainstorm: a pill is an identity
  claim keyed by namespace (ADR-083 D3); a provider-keyed CTA would demote ids to chips and needs
  an ADR superseding 083. Not this block.
- **Video joining the identity spine.** `entity_external_ids` exists for scan-time lookup; nothing
  looks a video up by external id. Video's badge reads the resolver's winning
  `external_provider_id` (ADR-082), and `identityEntityType` keeps excluding video.
- **A wrong-match recovery affordance next to the badge.** Clicking through and finding the
  wrong entity is a real failure mode; the correction path in v1 is the existing re-enrich flow
  (`EnrichPicker`). Recorded as a gap, not built here.
- **Linking the `External ID` chip in the media Metadata grid** (ADR-082's display rule).
  Owner ruled the header pill is the placement; the chip stays a curatable value. ADR-082 action
  item 6 remains open, separately.
- **Generalising any `url`-display field into a badge**, link previews, and badges on cards /
  browse tiles. Detail pages only.
- **A Holodex-owned default template table for well-known namespaces** (e.g. built-in `imdb`).
  Templates stay provider-declared (ADR-083 D2); an id whose namespace no configured provider
  templates renders degraded, which is the designed state.

## User Stories

Owner (the only role that enriches; visitors see the same badge when a value exists — the
visitor/owner rule for entity data points):

- As the owner, I want the provider pill on a person, studio, film, or video to open that entity's
  page on the provider, so I can confirm enrichment matched the right one without re-searching.
- As the owner running a provider whose ids are slugs, I want that provider's pill to still link,
  so the badge is not a TMDB-only feature.
- As the owner, I want a film's pills to include its IMDb id when the provider supplied one, so I
  can cross-check on the site I actually use.
- As the owner, I want the media page's `Homepage` row to be the film's real website again, so I
  don't see the same TMDB link twice with different labels.
- As a visitor, I want the pill to render as plain "known to IMDb" text when no link can be
  built, so I still learn the entity is identified even if I can't click through.

## Requirements

### Must-have (P0)

- **P0-1 · Contract: `link_templates` documented.** `docs/specs/metadata-provider-contract.md`
  §2.2 gains a `link_templates` row and a §4 subsection matching what
  `internal/enrich/enrich.go` already parses: `{ "<namespace>": { "<entity kind>": "<https
  template with exactly one {id}>" } }`, entity kinds `person` / `studio` / `film` / `video`,
  http(s) only, invalid entries dropped per-entry (not per-manifest), namespace-keyed so a
  provider may declare templates for a foreign namespace it emits (`imdb`). Additive; unknown
  key ignored by older Holodex.
  - [x] The row and subsection exist, cross-linked, with a TMDB-shaped example.
  - [x] The `/describe` reference example in §8 includes a `link_templates` block.

- **P0-2 · TMDB sidecar declares templates** (HOLODEX-391). `providers/tmdb` `/describe` emits:
  `tmdb` → person `https://www.themoviedb.org/person/{id}`, studio
  `https://www.themoviedb.org/company/{id}`, film + video `https://www.themoviedb.org/movie/{id}`;
  `imdb` → person `https://www.imdb.com/name/{id}/`, film + video
  `https://www.imdb.com/title/{id}/`.
  - Given a person enriched from TMDB before this change, when the sidecar is upgraded and its
    manifest re-read, then the person page's `TMDB` pill becomes an `<a target="_blank">` with no
    core deploy and no re-enrich.
  - Given a video whose winning `external_provider_id` is `imdb:tt0133093`, then its link is
    `https://www.imdb.com/title/tt0133093/`.
  - [x] Templates pass `enrich.ValidateLinkTemplate`; a sidecar unit test asserts the manifest
    shape.

- **P0-3 · TMDB sidecar stops overriding `homepage`** (HOLODEX-391). The video/film `homepage`
  field carries `det.Homepage` (the film's own website) or is omitted when TMDB has none — never
  the TMDB page. Behaviour change, called out in the sidecar changelog.
  - Given a film with a real homepage, then the media page `Homepage` row shows that site and the
    header pill shows TMDB — one TMDB link on the page.
  - Given a film TMDB has no homepage for, then the `Homepage` row is absent (no empty value, no
    fallback to the TMDB page).
  - [x] Previously-stored `homepage` values are **not** migrated; they refresh on the next enrich.
    Documented in the release note.
  - **Same rule for `website`** (owner ruling 2026-09-17, folded into HOLODEX-391): the person
    `website` field carries the person's own `homepage` or is omitted — never the TMDB person
    page — and the studio `website` field drops its TMDB-company-page fallback. Both were the
    same double-link once the pill links; stored values likewise refresh on the next enrich.

- **P0-4 · Contract: `_source_url` on `/enrich`** (HOLODEX-392). A provider may include a
  `_source_url` key in the `/enrich` response: the absolute http(s) URL of **its own page** for the
  entity it just enriched. Underscore-prefixed = internal contract (never a field, never resolved,
  never curatable). Holodex validates the scheme at ingest and drops anything else silently (same
  posture as `candidates[].profile_url`). Stored per `(entity_type, entity_id, provider)` next to
  the enrichment row; storage shape is the ADR's call.
  - [x] Contract §4.12 subsection + §8 example. · [ ] `docs/testing-strategy.md` row (testing gate).
  - [ ] A non-http(s) or malformed `_source_url` is dropped and the rest of the enrich succeeds.

- **P0-5 · Per-pill link precedence** (HOLODEX-392). For a pill with namespace `ns` on an entity
  enriched by provider `p`:
  `template(ns, kind) ?? (ns == p ? storedSourceURL(entity, p) : nil) ?? degraded`.
  A stored URL never rescues a *foreign* namespace's pill (TMDB's page must not sit behind an
  IMDb pill). Applies identically to the person/studio/film projection and to video's single
  resolved pill.
  - Given a provider `acme` with no templates that returned `_source_url`, then the `acme` pill
    links to that URL.
  - Given the same entity also carries `imdb:…` and no provider templates `imdb`, then the IMDb
    pill renders degraded, not linked to the `acme` URL.
  - Given both a template and a stored URL for `ns == p`, then the template wins (provider
    declared it; core built it).
  - [ ] Unit tests in `internal/api/external_links_test.go` cover all three branches for both
    projection paths.

- **P0-6 · Film page badge** (HOLODEX-393). `getFilm` projects `external_links` through the same
  `externalLinksForEntity` path person and studio use; the film page mounts `ProviderLinkBadge`
  in its header meta line (the `EntityVideoMeta` pattern). Resolves
  [film-provider-enrichment-ux.md](film-provider-enrichment-ux.md) P1-2.
  - Given a film adopted from TMDB with an IMDb id, then the film header shows `IMDb` and `TMDB`
    pills, alphabetical by label (ADR-083 DD3 order).
  - Given a film with no external ids, then no pill and no trailing separator render.
  - [ ] Visible to visitors when ids exist; no owner gate on the badge.

- **P0-7 · Media page badge** (HOLODEX-394). `getVideo` projects `external_links` as a single-
  element array built from the resolver's winning `external_provider_id` (label from the
  namespace, URL via P0-5 with the winning source's provider), or an empty array when the field
  has no value. The media page appends ` · [pill]` to the header meta line after the year, exactly
  as `EntityVideoMeta` does — **owner ruling 2026-09-16**, chosen over linking the Metadata grid's
  `External ID` chip. The chip in the Metadata grid is unchanged.
  - Given a video whose winning `external_provider_id` came from a provider with a `video`
    template, then the header shows one linked pill.
  - Given the winner came from the file layer (`file:` source) with a namespace no provider
    templates, then the pill renders degraded ("known to IMDb") — **owner ruling 2026-09-16**:
    the identity signal always renders, matching person/studio.
  - Given `external_provider_id` has no value, then the meta line is byte-identical to today.
  - Under a provider that treats the media file as the canonical unit, the pill opens that file's
    own page; under TMDB it opens the film's — the provider's `video` template decides.

- **P0-8 · Three-skin QA** on the film and media headers, per `.claude/rules/frontend-theming.md`;
  the pill's tokens are already skin-safe (ADR-083 handoff), so this is placement + wrap only.

### Should-have (P1)

- **P1-1 · Sidecar conformance smoke** (`metadata-provider-contract.md` §9): the reference
  provider's `/describe` includes `link_templates`, so a new sidecar author sees the key in the
  contract *and* the example they copy.
- **P1-2 · Stale-`homepage` hint.** Until the next enrich, a media page can still show the old
  TMDB-page `homepage` next to the new TMDB pill. An "Enrich again" nudge is F47/HOLODEX-86
  territory; nothing here, but note it in the release note.

### Future considerations (P2)

- **P2-1 · ADR-082 display rule for the `External ID` chip** (strip namespace, brand icon) —
  owner declined to link it; the strip-for-label half is still open as ADR-082 action item 6.
- **P2-2 · Wrong-match recovery** adjacent to the badge ("not this one?" → `EnrichPicker` scoped
  to the same provider). Design once the badge exists and the failure is observed, not before.
- **P2-3 · Card-level provider affordance** — no: cards are for browsing, not verification.

## Component map

| Surface | Component | Status |
|---|---|---|
| Pill | `ProviderLinkBadge` | **Reused unchanged** — `link.url` presence already decides `<a>` vs `<span>` |
| Header meta line (film, media) | `EntityVideoMeta` pattern | **Reused** on film; media appends to its own meta line (it has no video count) |
| Brand icon | `ProviderIcon` via `providers.iconUrl(namespace)` | Reused; monogram fallback when the namespace has no icon (IMDb on a TMDB-only install) is expected |
| API payload | `api.ExternalLink` / `ExternalLink` (`types.ts`) | Reused; film and video gain the field |
| Link builder | `BuildProviderLink` (`internal/enrich/service.go`) | **Widened** with the P0-5 fallback |
| Contract | `metadata-provider-contract.md` §2.2, §4, §8 | **Extended** (P0-1, P0-4) |

## Success Metrics

This is an owner-facing single-user surface; the metrics are correctness, not adoption.

- **Leading**: on the production instance after HOLODEX-391 deploys, 100 % of person/studio pills
  on TMDB-enriched entities are `<a>` (audit via the `external_links[].url` field on the API — a
  one-line script, no UI needed). Today: 0 %.
- **Leading**: after HOLODEX-392 + the production provider's `_source_url` emit, every video
  enriched by that provider shows a linked pill.
- **Lagging**: zero pages showing two links to the same TMDB page one enrich cycle after
  HOLODEX-391.

## Resolved decisions

| # | Decision | Ruled |
|---|---|---|
| RD1 | Mechanism is **both** — templates primary, `_source_url` fallback | brainstorm 2026-09-16 |
| RD2 | Pills stay **namespace-keyed** (one per id); provider-keyed rejected | brainstorm 2026-09-16 |
| RD3 | Fallback applies only when `ns == enriching provider` | brainstorm 2026-09-16 |
| RD4 | Video stays on the resolver; no identity row | brainstorm 2026-09-16 |
| RD5 | Unwind the TMDB `homepage` override; no data migration | brainstorm 2026-09-16 |
| RD6 | No wrong-match recovery in v1 | brainstorm 2026-09-16 |
| RD7 | Media badge = **header pill after the year**, not the Metadata chip | spec 2026-09-16 (mockup-backed) |
| RD8 | Degraded pill still renders on film/video (ADR-083 D2 parity) | spec 2026-09-16 |
| RD9 | TMDB sidecar uses **templates only**; it does not also emit `_source_url` | spec 2026-09-16 |
| RD10 | The production video provider **returns `_source_url` per entity on every `/enrich`** (its pages are item-keyed, not template-shaped); contract §4.12 is its implementation target | owner 2026-09-17 |
| RD11 | `_source_url` rides the `_` sidecar field channel and is stored as an `entity_enrichment` row — no new table, no migration | ADR-098 D1, 2026-09-17 |

## Open Questions

- ~~**[owner]** Does the production provider return a per-item URL on `/enrich`?~~ **Resolved
  2026-09-17 → RD10:** it should, per entity, on every `/enrich`; contract §4.12 is what its sidecar
  implements against. Core is unaffected either way.
- ~~**[architecture]** Where the stored `_source_url` lives~~ **Resolved by
  [ADR-098](../architecture/ADR-098-provider-source-url-fallback.md) D1:** a reserved `_`-prefixed
  key in `fields`, stored as an ordinary `entity_enrichment` row — no migration.
- **[design]** Film header: the film page has a banner/poster header (F59) rather than the
  person page's portrait hero — the handoff decides which line the pills join.

## Timeline Considerations

Four stories in dependency order; the first is independently shippable and is the whole visible
fix for person/studio:

1. HOLODEX-391 — sidecar templates + `homepage` unwind (P0-1 doc, P0-2, P0-3). Ships alone.
2. HOLODEX-392 — `_source_url` + precedence + ADR (P0-4, P0-5). Needs the ADR first.
3. HOLODEX-393 — film badge (P0-6). Needs only #1 to be useful; the design handoff extension
   covers film + media together.
4. HOLODEX-394 — media badge (P0-7). Depends on #1 (or the page shows TMDB twice) and on #3's
   handoff.

The epic's one PR accumulates the gates (ADR-069); it stays Draft until spec, ADR, handoff, and
testing-strategy rows are green.
