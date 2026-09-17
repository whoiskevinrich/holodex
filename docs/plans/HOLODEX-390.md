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

**Design package:** [spec](../specs/provider-link-badge-coverage.md) · ADR (pending — amends
ADR-083 D2) · [handoff](../design/provider-link-badge-handoff.md) (to be extended with film +
media placement) · testing-strategy § (pending)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/provider-link-badge-coverage.md`
- [ ] architecture `architecture` → `docs/architecture/ADR-*` — `_source_url` storage + per-pill precedence, amends ADR-083 D2
- [ ] design `design-handoff` → `docs/design/provider-link-badge-handoff.md` — film + media header placement (RD7 ruled: header pill after the year)
- [ ] backend — `_source_url` ingest + `BuildProviderLink` fallback; `getFilm`/`getVideo` project `external_links`
- [ ] frontend — mount `ProviderLinkBadge` on film + media headers; three-skin QA
- [ ] testing `testing-strategy`
- [ ] security `security-review` — stored provider URL is an outbound href; http(s)-only at ingest

## Up next — ordered (position = priority)

1. [ ] [—] ask Kevin: does the production provider return a per-item URL on `/enrich` (blocking for HOLODEX-392's consumer, not for core)
2. [ ] [architecture] ADR for `_source_url` storage + precedence — `node scripts/adr-claims.mjs` first — `docs/architecture/ADR-0XX-provider-source-url-fallback.md`
3. [x] [backend] HOLODEX-391: `providers/tmdb` declares `link_templates` + drops the `homepage` override — `providers/tmdb/tmdb.go` (~L55 manifest, ~L566 homepage)
4. [x] [spec] contract §2.2 `link_templates` row + §4.11 subsection + §8 example (P0-1) — `docs/specs/metadata-provider-contract.md`
4b. [ ] [spec] contract `_source_url` subsection after the ADR (P0-4) — `docs/specs/metadata-provider-contract.md`
5. [ ] [design] extend the handoff with film + media header placement, SVG mockup committed — `docs/design/provider-link-badge-handoff.md`
6. [ ] [backend] HOLODEX-392: `_source_url` ingest + `BuildProviderLink` per-pill fallback — `internal/enrich/service.go`, `internal/api/external_links.go`
7. [ ] [backend] HOLODEX-393/394: `getFilm` + `getVideo` project `external_links` — `internal/api/films.go`, `internal/api/handlers.go`
8. [ ] [frontend] mount the badge on film + media headers — `web/src/routes/films/[id]`, `web/src/routes/media/[id]/+page.svelte` (~L1319 meta line)
9. [ ] [—] sweep children 391–394 by hand with the epic (epic-keyed branch → CI fires nothing): In Review on ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm → epic + 4 stories → spec
- skills: product-brainstorming, write-spec, code-review
- handoff: F63 spec landed with RD1–RD9 locked (header-pill placement for media was mockup-ruled this session); nothing coded. Next is the ADR (`_source_url` storage) — but HOLODEX-391 (sidecar templates) is independent of it and is the whole visible fix for person/studio, so it can go first if Kevin wants a quick win.

### 2026-09-16 · HOLODEX-391 sidecar templates + homepage unwind
- skills: code-review
- handoff: HOLODEX-391 shipped on the epic branch (391 In Progress in Jira; swept by hand with the epic). Sidecar `/describe` now emits `link_templates` for `tmdb` (person/studio/film/video) + `imdb` (person/film/video), `homepage` = TMDB's own `homepage` or omitted; contract §2.2 row + §4.11 + §8 example + tmdb-provider.md mapping updated; P0-1/P0-2/P0-3 boxes ticked in the F63 spec. Not migrated: stored `homepage` values (RD5). Next is the ADR (`_source_url`), then 392. Pre-existing gofmt hit in `providers/tmdb/main.go` left alone (not this change).
