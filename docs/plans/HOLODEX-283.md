---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-283                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Search now returns films alongside people/videos/tags/studios directly, so results appear without extra loading.
---

# HOLODEX-283 · Films: real backend search integration

Fold films into the backend `Search()`/FTS index so `GET /search?q=` returns films natively
(gated on `films_enabled`), then remove the frontend-only merge shim (`api.searchAll`) that
fired a parallel `GET /films?q=` and spliced results client-side. Sub-task of epic HOLODEX-279
(F56 — Films entity).

**Design package:** [docs/specs/films-entity.md](../specs/films-entity.md) (pre-existing, already
specified this exact behavior) · [ADR-017](../architecture/ADR-017-search-architecture.md)
(pre-existing mixed-entity FTS pattern, extended not superseded) · no design-handoff (no new UX
surface) · [docs/testing-strategy.md](../testing-strategy.md) §Search / FTS5

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/films-entity.md` already specified this surface (§"Global
      search and browse"); no new spec needed
- [x] architecture `architecture` → `docs/architecture/ADR-017-search-architecture.md` already
      documents the mixed-entity FTS pattern this extends; no new ADR needed
- [~] design `design-handoff` — not applicable, no new UX surface (backend integration + shim
      removal only; existing `SearchResultsPanel` already renders a generic `films` group)
- [x] backend — `internal/repo/repo.go` `Search()` now queries `SearchFilms` and returns a
      `Films` group when `filmsEnabled`; single `filmsEnabled` param (collapsed from an initial
      two-param draft during `/simplify`)
- [x] frontend — `api.searchAll` shim removed; `navSearch.svelte.ts` and `/search` page call
      `api.search` directly; `SearchResponse.films` is always-present, never optional
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §Search/FTS5 row updated; new
      repo/API tests cover films-enabled/disabled and non-nil-empty-slice behavior
- [x] security `security-review` — not applicable, no new auth/access/infra surface: films data
      was already public via the existing `/films?q=` endpoint under the same `films_enabled` gate

## Up next — ordered (position = priority)

1. [ ] [—] none — this sub-task is complete; remaining F56 work tracked under epic HOLODEX-279

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-27 · session
- skills: simplify
- handoff: HOLODEX-283 implemented and verified end-to-end (backend `Search()` + frontend shim
  removal); `/simplify` caught and fixed a redundant `filmsEnabled`/`hideFullFilmVideos` param
  pair, collapsing back to the original one-arg `Search()` signature; all gates green, ready to
  push and mark the PR ready for review.
