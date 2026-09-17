---
key: HOLODEX-390
status: in-progress
profile: full                # spec + ADR + design + backend + frontend + testing + security — every row applies
depends-on: []
release_note:
---

# HOLODEX-390 · Provider link badge coverage (F63)

Every person, studio, film, and video page shows a clickable provider pill when enrichment knows
the provider's page — fed by the sidecar's `link_templates`, with a provider-returned
`_source_url` as the per-pill fallback — and the media page stops linking TMDB twice.

**Design package:** [spec](../specs/provider-link-badge-coverage.md) · [ADR-098](../architecture/ADR-098-provider-source-url-fallback.md) (amends ADR-083 D2) · [handoff](../design/provider-link-badge-handoff.md) (to be extended with film +
media placement) · testing-strategy § (pending)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/provider-link-badge-coverage.md`
- [x] architecture `architecture` → `docs/architecture/ADR-098-provider-source-url-fallback.md` — `_source_url` storage + per-pill precedence, amends ADR-083 D2
- [x] design `design-handoff` → `docs/design/provider-link-badge-handoff.md` §5 — film DD4 (year line via `trailing`, ruled 2026-09-17) + media DD5 (RD7, after the year); mockup `provider-link-badge-film-media-mockup.svg`
- [ ] backend — `_source_url` ingest + `BuildProviderLink` fallback (392 ✓); `getFilm` (393 ✓) / `getVideo` (394) project `external_links`
- [ ] frontend — mount `ProviderLinkBadge` on film (393 ✓, three skins QA'd) + media (394) headers
- [ ] testing `testing-strategy`
- [ ] security `security-review` — stored provider URL is an outbound href; http(s)-only at ingest

## Up next — ordered (position = priority)

1. [x] [—] ask Kevin: does the production provider return a per-item URL on `/enrich` — **yes, it should, per entity on every `/enrich` (RD10, 2026-09-17)**; its sidecar implements contract §4.12
2. [x] [architecture] ADR-098 for `_source_url` storage + precedence — `docs/architecture/ADR-098-provider-source-url-fallback.md`
3. [x] [backend] HOLODEX-391: `providers/tmdb` declares `link_templates` + drops the `homepage` override — `providers/tmdb/tmdb.go` (~L55 manifest, ~L566 homepage)
4. [x] [spec] contract §2.2 `link_templates` row + §4.11 subsection + §8 example (P0-1) — `docs/specs/metadata-provider-contract.md`
4b. [x] [spec] contract §4.12 `_source_url` subsection + §8 example (P0-4, shape fixed by ADR-098 D1/D2) — `docs/specs/metadata-provider-contract.md`
5. [x] [design] extend the handoff with film + media header placement, SVG mockup committed — `docs/design/provider-link-badge-handoff.md` §5
6. [x] [backend] HOLODEX-392: `_source_url` ingest + `Service.SourceURLs`/`ProviderLink` per-pill fallback — `internal/enrich/service.go`, `internal/api/external_links.go`
7. [x] [backend] HOLODEX-393: `getFilm` projects `external_links` — `internal/api/films.go`
7b. [ ] [backend] HOLODEX-394: `getVideo` projects `external_links` from the resolver's winning `external_provider_id` via `ProviderLink` (ADR-098 D4) — `internal/api/handlers.go`
8. [x] [frontend] HOLODEX-393: badge on the film year line — `web/src/routes/films/[id]/+page.svelte`
8b. [ ] [frontend] HOLODEX-394: badge after the year on the media meta row (handoff DD5) — `web/src/routes/media/[id]/+page.svelte` (~L1319 meta line)
9. [ ] [—] sweep children 391–394 by hand with the epic (epic-keyed branch → CI fires nothing): In Review on ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-17 · HOLODEX-393 film badge (design gate + code)
- skills: design-handoff (§5 by hand: DD4 film / DD5 media, SVG committed), code-review
- handoff: Kevin ruled film placement A — pills on the **year line** (`1999 · IMDb TMDB`), over a dedicated pill line; mounted through `NameEditControl`'s existing `trailing` slot (no component change; pencil docks after the pills like the person title's flags; pills hide while the year is in edit). Design gate closed for both film and media (DD5 = RD7 restated with the per-segment separator rule, so 394 needs no further design). `getFilm` projects `external_links` via `externalLinksForEntity` (`TestExternalLinks_Film`: linked tmdb + label-only imdb, no-ids → null like person/studio); `FilmDetailResponse.external_links` typed; F63 P0-6 ticked, its open design item resolved, F59 P1-2 annotated, testing-strategy §4/§5 rows extended. Live QA'd on backend-films (Aladdin film 1 with tmdb/imdb/other ids): one line in all three skins, pill text = the year's muted color (AA 4.9/5.7/6.3), degraded pill is a `<span>`, owner pencil after the pills keeps "Change the year for this film". 393 In Progress in Jira (swept by hand with the epic). Next: HOLODEX-394 (#7b/#8b), then testing + security gates, then the child sweep.

### 2026-09-17 · HOLODEX-392 `_source_url` ingest + per-pill fallback
- skills: code-review (one finding, fixed: a failed `SourceURLs` read now degrades to template-only instead of dropping every pill)
- handoff: HOLODEX-392 shipped on the epic branch (392 In Progress in Jira; swept by hand with the epic). ADR-098 D1–D3 implemented: `model.SourceURLField`; `runEnrich` reshapes `_source_url` next to `_studio_external_ids` (first http(s) value, garbage clears, absence leaves alone); D3 split into `Service.SourceURLs` (one `EnrichmentForEntity` read per request, keyed by provider) + `Service.ProviderLink(ns, kind, id, stored)` — not the ADR's single-function sketch, so its "once per request" note holds; `externalLinksForEntity` consumes it. `Fake.LinkTemplates` added for in-process precedence tests. Tests: `internal/enrich/source_url_test.go` (ingest ×2, precedence table) + two end-to-end cases in `external_links_test.go` (env fake gained `/enrich`). Stub: `alpha` declares `link_templates`, `bravo` returns `_source_url` (stub.test.mjs asserts never both). Testing-strategy §4 row + invariant written by hand; the testing gate stays open for 393/394 + frontend. **D4 (video) is HOLODEX-394's** — `getVideo` must call the same `ProviderLink` with the winning source's namespace. Next: #5 design extension (film + media header placement, SVG committed), then 393/394.

### 2026-09-17 · website unwind folded into 391 → ADR-098
- skills: code-review, architecture (evaluate pass on the draft — every codebase claim verified)
- handoff: ADR-098 written + indexed (README row; ADR-083 index status annotated "D2 fallback added by ADR-098"). Decisions: `_source_url` rides the `_` sidecar field channel (`fields._source_url`, stored as an `entity_enrichment` row — no table, no migration), ingest mirrors HOLODEX-258 (garbage overwrites, absence leaves alone), precedence `template ?? (ns == provider ? stored : ∅) ?? degraded` with the row looked up by `provider = namespace` (that equality is RD3), TMDB templates-only. Earlier this session: person `website` + studio fallback unwound into HOLODEX-391 (`f164337`). Kevin then ruled the open question: the production video provider **should return `_source_url` per entity on every `/enrich`** (RD10) — contract §4.12 written as its implementation target, §8 example + §4.2 sidecar list updated, spec open questions closed (RD10/RD11). Next: HOLODEX-392 implements ADR-098 D1–D4 (`Service.Enrich` reshaping next to `_studio_external_ids`, `ProviderLink` on the Service, both projections); testing-strategy row rides the testing gate.

### 2026-09-16 · brainstorm → epic + 4 stories → spec
- skills: product-brainstorming, write-spec, code-review
- handoff: F63 spec landed with RD1–RD9 locked (header-pill placement for media was mockup-ruled this session); nothing coded. Next is the ADR (`_source_url` storage) — but HOLODEX-391 (sidecar templates) is independent of it and is the whole visible fix for person/studio, so it can go first if Kevin wants a quick win.

### 2026-09-16 · HOLODEX-391 sidecar templates + homepage unwind
- skills: code-review
- handoff: HOLODEX-391 shipped on the epic branch (391 In Progress in Jira; swept by hand with the epic). Sidecar `/describe` now emits `link_templates` for `tmdb` (person/studio/film/video) + `imdb` (person/film/video), `homepage` = TMDB's own `homepage` or omitted; contract §2.2 row + §4.11 + §8 example + tmdb-provider.md mapping updated; P0-1/P0-2/P0-3 boxes ticked in the F63 spec. Kevin then ruled the person `website` (TMDB person page) and studio `website` fallback (TMDB company page) the same double-link — folded into 391: both now `homepage`-or-omitted; tmdb-provider.md + qa-tmdb-provider.md 8a.4 follow. Not migrated: stored `homepage`/`website` values (RD5). Next is the ADR (`_source_url`), then 392. Pre-existing gofmt hit in `providers/tmdb/main.go` left alone (not this change).
