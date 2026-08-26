---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-280                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Films now support owner-uploaded poster and thumbnail images, served at the standard portrait aspect ratio.
---

# HOLODEX-280 · Film poster/thumbnail asset pipeline

Owner can upload, replace, and remove a film's poster and thumbnail images via the API; both
are self-hosted, normalized, and served the same way Studio's icon/logo/poster images already
are (F51/ADR-079). Done means: `go build`/`go vet`/`go test ./...` clean, new backend tests
mirroring `studio_images_test.go`'s coverage shape, frontend type-check + unit tests clean, the
upload/replace/remove/visitor flow verified live in a browser across all three skins.

**Design package:** spec §P1-2 ([films-entity.md](../specs/films-entity.md)) · architecture
[ADR-085](../architecture/ADR-085-films-entity.md) (film_images schema, reuses
[ADR-079](../architecture/ADR-079-studio-image-roles.md) verbatim) · design: none needed — the
Images section is mechanical reuse of Studio's existing (undocumented-as-a-handoff) pattern,
see session log · testing-strategy: [§ "Film poster/thumb self-hosted images"](../../docs/testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → already covered by `docs/specs/films-entity.md` §P1-2; no new spec needed
- [x] architecture `architecture` → already covered by ADR-085 (film_images schema, reuses ADR-079 verbatim); no new ADR needed
- [x] design `design-handoff` → not needed; Studio's Images section has no dedicated handoff doc either — this ticket mirrors its live implementation exactly (see session log)
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` → new `internal/filmimage`, `internal/repo/film_images_test.go`-shaped coverage (via `film_images_test.go` in `internal/api`), testing-strategy.md updated
- [x] security `security-review` → clean; owner-gated mutations, ids-only disk paths, shared personimage.Normalize hardening, no findings

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] film image disk/repo/imagesink/API layers — `internal/filmimage/`, `internal/repo/film_images.go`, `internal/imagesink/sink.go`, `internal/api/film_images.go`
2. [x] [frontend] `FilmImageSlot.svelte` + Images section on the film detail page — `web/src/lib/components/film/FilmImageSlot.svelte`, `web/src/routes/films/[id]/+page.svelte`
3. [ ] [—] provider-sourced film image writeback (enrichment wiring into `imagesink.Sink`'s dispatch) → HOLODEX-284

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-25 · full implementation + live verification
- skills: simplify, security-review
- handoff: HOLODEX-280 is implementation-complete and verified live (upload/replace/remove/visitor
  view/three skins) — ready to push and open a PR. The one deliberately deferred piece is
  provider-sourced writeback (wiring `imagesink.Sink`'s entityType dispatch + `LockedFilmImageRoles`
  for a real enrichment provider), tracked separately as HOLODEX-284 since no enrichment caller
  exists yet — don't build it speculatively here.
