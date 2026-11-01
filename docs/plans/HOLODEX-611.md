---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-611
status: in-progress
profile: full                # new query grammar (architecture fork), visitor-supplied trees (security) + UI
depends-on: []
release_note:
approved:
  design:
    on: 2026-11-01
    at: e4657bfe
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
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy` — §38 plan written; tests pending
- [/] security `security-review` — design-phase sign-off 2026-11-01 (Jira comment); re-review the compiler, parse endpoint and MCP argument at build

## Up next — ordered (position = priority)

1. [ ] [backend] `internal/query` parser, caps, printer, canonicalisation, owner-only check — `internal/query/`
2. [ ] [backend] compiler over `VideoFilter.build()` fragments + `expr` key in `videoFilterFromQuery` / `mediaFilterFor` — `internal/repo/`, `internal/api/`
3. [ ] [backend] `GET /api/v1/query/parse`, `canonicalPlaylistQuery` + `rewriteSmartPlaylistRefs`, MCP `search_videos` `expr` — `internal/api/`
4. [ ] [frontend] query mode in nav search, negated `FilterChip`, `QueryChipGroup`, `AdvancedQueryRow`, empty/refused states — `web/src/`
5. [ ] [testing] Q1–Q8, A1–A11, W1–W6, nav-harness `query-back` — `docs/testing-strategy.md` §38
6. [ ] [security] re-run `/security-review` on compiler, parse endpoint, MCP argument
7. [ ] [—] Jira: link HOLODEX-611 to HOLODEX-181 and HOLODEX-15 (Relates)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-11-01 · worklog filled, posture full
- skills: write-spec, architecture, design-handoff, testing-strategy, security-review, implement
- handoff: Crossed into build — design signed off at e4657bfe; draft PR open. Start at the `internal/query` parser.

## Dropped — newest first (the reason is the point)
