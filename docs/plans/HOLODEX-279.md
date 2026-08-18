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
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [design] `/design-handoff` — films list/detail layout, two attach pickers (video→film, film→video bulk), films row on person/studio/tag pages, suspended-film-source visual state (ADR-085 §5 action item), 3-skin QA
2. [ ] [backend] Schema migrations (`films`, `film_videos`, `film_people_roles`, `film_images`, `films_fts` — ADR-085 §1) + asserted-link non-participation guarantee/regression test — `internal/db/migrations/`
3. [ ] [backend] Film resolver source (`resolveDecided`/`gather` `film:` branch) + `films_enabled` config flag + film API — `internal/resolver/`, `internal/config/`, `internal/api/`
4. [ ] [frontend] `/films` list + `/films/[id]` detail + both attach pickers + video-list hiding + films rows — `web/src/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-17 · Brainstormed the Films entity end-to-end, opened epic, wrote spec
- skills: product-brainstorming, write-spec
- handoff: spec is done and epic gates/labels are current (needs-spec cleared); next session should run `/architecture` for ADR-085 — the spec's Open Questions Q1 (multi-film resolver-source candidate naming) and Q2 (field_source_decisions suspend mechanism) are the two decisions that ADR needs to lock before backend work starts.

### 2026-08-18 · Wrote ADR-085, resolving spec Q1/Q2
- skills: architecture
- handoff: ADR-085 is written and Proposed — films compete as a `provider:film:<id>` resolver source injected as synthetic enrichment at the resolver call site (one new narrow branch in `resolveDecided`/`gather` for `film:`-prefixed namespaces), and `films_enabled=false` suspends resolution by simply not injecting those candidates (reuses the existing "decided source currently unmatched → empty" path, no new schema/state). Migration 0042 DDL for `films`/`film_videos`/`film_people_roles`/`film_images`/`films_fts` is specified. Epic gates/labels updated (needs-adr cleared). Next session should run `/design-handoff` — in particular the suspended-film-source visual state (ADR-085 §5 flags this as a required, not-yet-designed action item) plus the films list/detail layout and both attach pickers.
