---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-611
status: in-progress
profile: full                # new query grammar (architecture fork), visitor-supplied trees (security) + UI
depends-on: []
release_note: Search videos with AND, OR and NOT across Tags, people, studios, duration and more — type a query like tag:comedy duration:<2m in the search box, see it as chips on Media, and save it as a smart playlist.
approved:
  design:
    on: 2026-11-01
    at: c12730ed
---

# HOLODEX-611 · F87 Query language — AND/OR/NOT video search over tags, people, studio, duration and provider match

One query for videos that can say AND, OR and NOT across Tags, people, studio, duration and "matched
to a provider", typed into the nav search box or built with chips on browse, for owners and
visitors. Smart playlists store it as an `expr` key. Done is the parser, compiler, parse endpoint,
playlist and MCP wiring, and the query mode and chips in the SPA, all shipped behind the caps.

**Design package:** [spec](../specs/query-language.md) ·
[browse-and-list-state.md "Boolean queries"](../architecture/browse-and-list-state.md) ·
[security-perimeter.md "Query expressions"](../architecture/security-perimeter.md) ·
[handoff](../design/query-language-handoff.md) · [testing-strategy §38](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `query-language.md` F87, P0-1..P0-10
- [x] architecture `architecture` → `docs/architecture/**` — `expr` text key parsed once in Go
- [x] design `design-handoff` → `docs/design/**` — `query-language-handoff.md` + mockup SVG
- [x] backend → `{cmd,internal,providers}/**` — `internal/query`, compiler, `expr` on every entry point, `/query/parse`, playlists, MCP (377c2537)
- [x] frontend → `web/src/**` — query mode, chips, Advanced row, empty and refused states (c12730ed); owner confirmed build vs mockup 2026-11-01
- [x] testing `testing-strategy` — Q1–Q8, A1–A11, W1–W6 green (§38.3 rows name the files), `query-back` passes; Go fuzz in CI (`make test-fuzz`)
- [x] security `security-review` — design sign-off and build re-review 2026-11-01 (Jira comments), no findings

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm HOLODEX-611 reaches Done; the epic HOLODEX-178 does not move on its own
2. [ ] [human] QA item 8: read query 1's chips with a screen reader — `docs/design/query-language-handoff.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-11-01 · frontend built, ready for review
- skills: code-review (high --fix), security-review, handoff
- handoff: Every gate settled — backend 377c2537, frontend c12730ed, build re-review clean, owner confirmed the build against the mockup; PR #577 marked ready. Next is review and merge.

### 2026-11-01 · backend built
- skills: code-review (high --fix), security-review, handoff
- handoff: Backend landed in 377c2537 (parser, compiler, `/query/parse`, playlists, MCP; Go suite green); start the frontend at `expr` in `filters.ts` and the browse chips.

### 2026-11-01 · worklog filled, posture full
- skills: write-spec, architecture, design-handoff, testing-strategy, security-review, implement, code-review
- handoff: Crossed into build — design signed off at e4657bfe; draft PR open. Start at the `internal/query` parser.

## Dropped — newest first (the reason is the point)
