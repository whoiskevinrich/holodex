---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-279                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-279 · F56 — Films entity (scenes, asserted video links, resolver-source writeback)

Films become a first-class, flag-gated entity: browsable like Person/Studio, attached to videos
("scenes") via an explicit, durable owner assertion (not a derived link), with resolver-source
Album/Title writeback, inherited cast/tags, enrichment, posters, and a two-region detail page.
Done means all seven gates below are checked and the feature merges to main behind
`films_enabled` (default false).

**Design package:** [films-entity.md](../specs/films-entity.md) · [ADR-085](../architecture/ADR-085-films-entity.md) · design handoff (not yet written) · testing-strategy (not yet written)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/films-entity.md`
- [x] architecture `architecture` → [ADR-085](../architecture/ADR-085-films-entity.md) (asserted-link model, film resolver source, films_enabled suspend semantics)
- [ ] design `design-handoff` → `docs/design/**` (films list/detail, two attach pickers, films row on person/studio/tag pages)
- [/] backend — CRUD read/create + attach/detach/bulk-attach done; poster pipeline (`film_images.go`) and `film_people_roles` CRUD still open, see Up next
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [design] `/design-handoff` — films list/detail layout, two attach pickers (video→film, film→video bulk), films row on person/studio/tag pages, suspended-film-source visual state (ADR-085 §5 action item), 3-skin QA
2. [ ] [backend] → [HOLODEX-280](https://whoiskevinrich.atlassian.net/browse/HOLODEX-280) `film_images.go` — poster/thumb asset pipeline for films; needs extending `internal/imagesink` + a new `filmimage` package mirroring `studioimage` + config/env/main.go wiring. Its own vertical slice, deliberately deferred out of the session that shipped CRUD/attach.
3. [ ] [backend] → [HOLODEX-281](https://whoiskevinrich.atlassian.net/browse/HOLODEX-281) `film_people_roles` CRUD — film-level additive billing/role data (director, billing order). Only read-only inherited cast (`FilmCast`, set union over attached videos) exists so far; the additive roles table has no API surface yet.
4. [ ] [frontend] `/films` list + `/films/[id]` detail + both attach pickers + video-list hiding + films rows — `web/src/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-21 · session (cont. 4)
- skills: (none — direct implementation, plus a `code-simplifier` sub-agent pass before commit)
- handoff: Built the film API handler layer — `internal/api/films.go` (`mountFilms`/`listFilms`/`getFilm`/`createFilm`, get-or-create 200-vs-201 semantics on a name+year collision), `internal/api/film_fields.go` (`resolveFilm` — a trimmed `resolveStudio`, no promotion/claims/completeness machinery), and `internal/api/film_videos.go` (attach/detach/bulk-attach, the `repo.ErrSceneNumberTaken`/`FilmSceneCollision` → 409-naming-the-occupant translator, all-or-nothing bulk semantics). Backing repo layer (`internal/repo/films.go`): `CreateFilm`/`ListFilms`/`GetFilm`/`SearchFilms`/`AttachFilmVideo`/`DetachFilmVideo`/`BulkAttachFilmVideos`/`FilmCast`/`FilmTags`/`FilmStudios`, all owner-assertion writes (never touched by `RelinkVideoEntity`). Wired `/films` routes into `handlers.go`'s `Mount()`, gated on `h.filmsEnabled` at the route-registration level (unregistered entirely when off, confirmed by `TestFilmsDisabled_RoutesUnregistered`'s 404-not-403 assertion) — both the public `GET /films`/`GET /films/{id}` and the owner-gated `mountFilms(r)` call. Added `internal/repo/films_test.go` (4 new tests: create dedup/year-legality, list/search incl. zero-count films, attach collision/already-attached/detach-idempotency, bulk-attach sequential-numbering/all-or-nothing rollback) and `internal/api/films_test.go` (6 tests: routes-unregistered-when-disabled, create get-or-create, attach/detach incl. non-idempotent re-attach/re-detach, scene collision on both single and bulk attach paths). Full `go build`/`go vet`/`go test ./...` clean (all packages, ~90s). Ran the `code-simplifier` agent against every new/changed file before committing, per the pre-commit checklist. **Deliberately deferred this session** (own follow-up work, see renumbered Up next): `film_images.go` (poster pipeline, filed as [HOLODEX-280](https://whoiskevinrich.atlassian.net/browse/HOLODEX-280)) and `film_people_roles` CRUD (film-level billing/role data, filed as [HOLODEX-281](https://whoiskevinrich.atlassian.net/browse/HOLODEX-281)) — both would have doubled this session's scope; the Atlassian connector wasn't authorized non-interactively in the session that did the work, so both issues were filed from a follow-up session once it was. Next session: either of the two filed backend follow-ups above, or jump to `/design-handoff` (still the oldest open gate) now that there's a real `/films` API surface to design against.

### 2026-08-21 · session (cont. 3)
- skills: (none — direct implementation)
- handoff: `FilmsForVideo` repo method (`internal/repo/films.go`) + the `getMedia` call-site injection (`internal/api/handlers.go`) that finally makes `films_enabled` observable — the flag was wired last session but consumed nowhere. `injectFilmSources` builds synthetic `"film:<id>"` enrichment candidates (canonical `collection` for every attachment, plus `title` when `is_full_film`) and is called right after `enr := enrichmentFromRows(enrichRows)`, gated on `h.filmsEnabled`; when off, the call is skipped entirely (ADR-085 §5's read-suppression, not a resolver-state change). Added `TestFilmsForVideo` (repo-level: ordering, per-video isolation) and `TestFilmSourceInjection_SceneVsFullFilm` (`internal/api/film_injection_test.go`, full HTTP round-trip against a real server: scene vs. full-film candidate shape, and — the important negative case — flipping `films_enabled` off on a video with a standing film decision resolves to *no value*, never a silent fallback to the file baseline). Full `go build`/`go vet`/`go test ./...` clean. Next session: the film API handler layer (`films.go`/`film_fields.go`/`film_images.go`/`film_videos.go`, mirroring `studios.go`) — attach/detach/bulk-attach with scene-number-collision 409s is the last backend gap before `/films` routes exist to hit at all. The design-handoff artifact (`docs/design/films-entity-handoff.md`) is still unwritten and blocks the design gate.

### 2026-08-21 · session (cont. 2)
- skills: (none — direct implementation)
- handoff: Wired `films_enabled` end-to-end following the `card_layout` precedent exactly: `internal/config.FilmsEnabled` (`FILMS_ENABLED` env, default false) → `Handlers.SetFilmsEnabled` → `main.go` wiring → surfaced on `GET /capabilities` (`films_enabled` field) so the SPA can gate films routes/nav. Documented in `holodex.yaml.example` (paired with the existing `card_layout: poster` guidance, per ADR-085) and a new "Films" section in `docs/reference/configuration.md`. Added `TestFilmsEnabledConfig` (default-off + env-override, mirroring `TestDefaultSourceConfig`'s shape — no existing capabilities-endpoint test covers individual fields like `card_layout`, so none was added there either, for consistency). Full `go build`/`go vet`/`go test ./...` clean. The flag is wired but not yet consumed anywhere (no route gating, no resolver-source injection point yet) — next session: `FilmsForVideo` repo method + the `getMedia` call-site injection that actually reads this flag, then the film API handler layer.

### 2026-08-21 · session (cont.)
- skills: (none — direct implementation, continuing the system-design build sequence)
- handoff: Wrote the zero-relink-participation regression test first, per ADR-085 action item 6 (`internal/api/film_links_test.go`, `TestFilmVideosSurviveFullRelinkCycle`) — seeds a `film_videos` row directly via SQL (no attach endpoint exists yet) and asserts it survives byte-for-byte across a full scan (`RelinkVideoEntity`) → enrich → decision (`SetDecision`) → curation (`SetCuration`) cycle, since there is and must never be a `RelinkFilmVideos` function. Added `model.Film` (`internal/model/model.go`, minimal: ID/Name/Year — mirrors Studio/Person in keeping resolver-only fields off the struct) and `filmBaseline` (`internal/resolver/film_baseline.go` + `film_baseline_test.go`, mirrors `studioBaseline` exactly: name is baseline-backed, everything else empty-but-claimed for RD6 additivity). Implemented the one genuine resolver-core diff ADR-085 §4 calls out: `resolveDecided`'s and `gather`'s `film:`-prefixed branches (`internal/resolver/resolver.go:491-524`, `:464-475`) reading `enrichment[name][f.Canonical]` directly instead of scanning `ParsedSources`, covered by `internal/resolver/film_source_test.go` (decided-wins, undecided-never-auto-wins, suspended-drops-not-fallback, multi-film-disambiguates-by-namespace). Full `go build`/`go vet`/`go test ./...` clean. Next session: `films_enabled` config flag + `FilmsForVideo` repo method + `getMedia` injection, then the film API handler layer (see renumbered Up next above) — the design-handoff artifact (`docs/design/films-entity-handoff.md`) still needs to be written before that gate can flip.

### 2026-08-21 · session
- skills: design-handoff, system-design
- handoff: Iterated the films_entity_design_handoff mockup (visualize widget, not yet written to docs/design/) against real SourceBadge/CurationChip/ProvenanceBadge/app.css control shapes per feedback — cast-inherited-from-scenes + poster enrich/upload affordance on Film Detail, per-row optional scene-number input on the film→video picker, and relabeled the resolved "Album" field to "Film" (registry.go + metadata-mappings.yaml.example — canonical id `collection` unchanged, only the Label/description and the example mapping's opinionated Album-first default moved; committed separately). Ran `/engineering:system-design` to turn ADR-085 into a concrete implementation blueprint grounded in the Studio-entity precedent (studioBaseline, RelinkVideoEntity call sites, studios.go handler layout, the getMedia injection point, resolveDecided/gather line numbers — all confirmed unshifted against disk). Merged origin/main, which claimed migration number 0042 for `writeback_cascade_delete` — renumbered the films migration to 0043 and fixed all four doc references (ADR-085, spec, architecture README). Wrote and verified migration 0043 (films/film_videos/film_people_roles/film_images/films_fts + triggers, ADR-085 §1) — both up and down paths round-trip clean. Next session should write the zero-relink-participation regression test first (ADR-085 action item 6), then `filmBaseline` + the `resolveDecided`/`gather` "film:" prefix branch, per the system-design build sequence. The written `docs/design/films-entity-handoff.md` artifact still needs to be produced before the design gate can flip.

### 2026-08-17 · Brainstormed the Films entity end-to-end, opened epic, wrote spec
- skills: product-brainstorming, write-spec
- handoff: spec is done and epic gates/labels are current (needs-spec cleared); next session should run `/architecture` for ADR-085 — the spec's Open Questions Q1 (multi-film resolver-source candidate naming) and Q2 (field_source_decisions suspend mechanism) are the two decisions that ADR needs to lock before backend work starts.

### 2026-08-18 · Wrote ADR-085, resolving spec Q1/Q2
- skills: architecture
- handoff: ADR-085 is written and Proposed — films compete as a `provider:film:<id>` resolver source injected as synthetic enrichment at the resolver call site (one new narrow branch in `resolveDecided`/`gather` for `film:`-prefixed namespaces), and `films_enabled=false` suspends resolution by simply not injecting those candidates (reuses the existing "decided source currently unmatched → empty" path, no new schema/state). Migration 0042 DDL for `films`/`film_videos`/`film_people_roles`/`film_images`/`films_fts` is specified. Epic gates/labels updated (needs-adr cleared). Next session should run `/design-handoff` — in particular the suspended-film-source visual state (ADR-085 §5 flags this as a required, not-yet-designed action item) plus the films list/detail layout and both attach pickers.
