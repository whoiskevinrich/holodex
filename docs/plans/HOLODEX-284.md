---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-284
status: in-review
depends-on: []
release_note: Films can now be enriched from TheMovieDB — description, release date, and poster — the same way people and studios already are.
---

# HOLODEX-284 · Film provider enrichment (ADR-086)

Implement ADR-086 (already decided/merged via #253): films get their own `entity_type: "film"`
enrichment lifecycle, independent of any attached video, with the poster stored as a
`film_images` asset (never a resolved field) — TMDB reused via an entity-type-aware field remap.
Sub-task of epic HOLODEX-279 (F56 — Films entity).

**Design package:** [ADR-086](../architecture/ADR-086-film-provider-enrichment.md) (pre-existing,
merged via #253 — this ticket is the implementation of its 12 Action Items) · no new spec/design-
handoff (backend/API surface only, reuses the existing Person/Studio enrich UI unmodified).

## Gates — definition of done

- [x] spec — `docs/specs/metadata-provider-contract.md` §4.2c / `docs/specs/films-entity.md` Q3
      updated per ADR-086 Action Items 7-8; no new spec doc
- [x] architecture — ADR-086 itself (pre-existing, merged via #253); this ticket implements its
      Action Items 1-6, 10 (item 9, the ADR README row, already landed with #253)
- [~] design — not applicable, no new UX surface (reuses the existing Person/Studio enrich
      decision-chip/curation UI unmodified per ADR-086's "What becomes easier")
- [x] backend — all of ADR-086's Action Items 1-6, 10 implemented: `mountEnrich`/`enrich_review.go`
      gain a `film` case (both `films_enabled`-gated, matching every other film route — a real gap
      found and fixed mid-implementation, since the ADR text didn't call this out explicitly);
      `ImageSink`/`downloadAssets`/`assetRoleFor` widened a third time (Person → Studio → Film);
      `film_images` repo adapter + `LockedFilmImageRoles` (ADR-049 parity); TMDB's `buildMovieEnrichResponse`
      remaps `overview`→`description` and poster→asset for `entity_type: "film"`; `metadata-sources.yaml.example`
      documents `film` as a fourth declarable type; the `"film:"` synthetic-namespace exact-prefix
      regression test (Action Item 10) added in `internal/resolver/film_source_test.go`
- [ ] frontend — not started this session (no new UX surface per the design gate above; the
      existing Person/Studio enrich components already work generically once the backend exposes
      `entity_type: "film"` — verifying that end-to-end in the browser is the one thing this
      worklog leaves open)
- [x] testing `testing-strategy` — new tests: `TestServiceFilmEnrich`/`TestEnrichDownloadsFilmAssets`
      (internal/enrich), `TestFilmEnrichFlow`/`TestFilmEnrichGated`/`TestFilmEnrichUnregisteredWhenFilmsDisabled`
      (internal/api), `TestEnrichQueue_IncludesFilms`/`TestFilmEnrichDismissAndRefreshRoundTrip`
      (internal/api), `TestFilmImage_ProviderSurfacesWhenNoUpload` (internal/repo, added by this
      session's `/code-review high --fix` pass — see below)
- [x] security `security-review` — ran this session (self-contained Agent-tool review, since the
      built-in `/security-review` skill gathers its diff from the wrong cwd in this worktree
      layout): no high-confidence findings

## Up next — ordered (position = priority)

1. [ ] Live-verify in the browser: run a real film TMDB enrich end-to-end (resolve → apply →
   poster visible on the Film page) against a `films_enabled`/TMDB-configured local stack — this
   session verified the backend/API layer only (Go tests + source reading), not a live UI pass.
2. [ ] Decide whether to amend commit `c146cb0`'s subject to drop the embedded `(HOLODEX-284)`
   key (violates `.claude/CLAUDE.md`'s "Branch ↔ Jira linkage" no-key-in-subject rule) — left
   unfixed by the `/code-review high --fix` pass below since fixing it needs a force-push to the
   already-pushed branch, which needs explicit user sign-off first.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-27 · ADR-086 implementation + code-review pass
- skills: code-review (`/code-review high --fix`)
- backend: implemented all of ADR-086's Action Items (see backend gate above); found and fixed a
  real gap mid-implementation where film enrich routes and the F47 review-queue's film entry were
  initially registered unconditionally instead of `films_enabled`-gated like every other film
  route. Committed as `c146cb0`, pushed, opened PR #263.
- also: ran `/code-review high --fix` against the PR #263 diff (8 finder angles). Found and fixed
  a real correctness bug this ADR's own implementation had introduced: the film-image read path
  (`filmImageVersions`/`serveFilmImage`) stayed hardcoded to `source='upload'`, so a provider
  poster from enrichment was stored but never surfaced anywhere (`PosterURL` stayed empty, the
  serve route 404'd) — the exact deliverable this ticket exists to ship. Fixed by adding
  `GetFilmImageDisplayed` (upload-over-provider priority, matching the comment ADR-086's own diff
  had already promised but never implemented) and locking it in with a new repo-level regression
  test. Also applied two small cleanup fixes (a duplicated string-formatting helper replaced with
  the existing `fieldsource.ForProvider`; a repeated anonymous struct type in `enrich.go` named).
  Left three lower-value cleanup findings unapplied (near-duplicate studio/film enrich handlers,
  an inline vs. table-driven TMDB field remap, an unused-but-consistent test fake) as deliberate,
  documented style calls rather than clear improvements. Also found this ticket's own worklog file
  had never been created (this file) and that the shipped commit subject embeds the Jira key
  against repo convention (see `Up next` #2 — left for explicit user sign-off, since fixing it
  needs a force-push).
- handoff: backend implementation + code-review fixes are done and pushed to PR #263. Remaining
  before this ticket can close: a live browser QA pass of the film enrich flow (`Up next` #1), and
  a decision on whether to amend the commit subject (`Up next` #2).
