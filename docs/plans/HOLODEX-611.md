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
- [/] backend → `{cmd,internal,providers}/**` — `internal/query`, compiler, `expr` on every entry point, `/query/parse`, playlists, MCP (377c2537)
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy` — Q1–Q8, A1–A11 green; W1–W4 (`query.test.ts`) green; W5, W6 and `query-back` pending
- [/] security `security-review` — design-phase sign-off 2026-11-01 (Jira comment); re-review the compiler, parse endpoint and MCP argument at build

## Up next — ordered (position = priority)

1. [ ] [frontend] `expr` in `filters.ts`/`mediaSchema`, `api.parseQuery`, browse chips (negated `FilterChip`, `QueryChipGroup`, `AdvancedQueryRow`), empty and refused states — `web/src/routes/+page.svelte`
2. [ ] [frontend] nav search query mode: `query` tag, suggestions, error echo, Enter → browse — `web/src/routes/+layout.svelte`
3. [ ] [testing] W5, W6, nav-harness `query-back` — `web/geometry/nav.mjs`
4. [ ] [security] re-run `/security-review` on compiler, parse endpoint, MCP argument
5. [ ] [—] QA the handoff's Cinémathèque checklist, then mark the PR ready

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-11-01 · backend built
- skills: code-review (high --fix)
- handoff: Backend landed in 377c2537 (parser, compiler, `/query/parse`, playlists, MCP; Go suite green); start the frontend at `expr` in `filters.ts` and the browse chips.

### 2026-11-01 · worklog filled, posture full
- skills: write-spec, architecture, design-handoff, testing-strategy, security-review, implement, code-review
- handoff: Crossed into build — design signed off at e4657bfe; draft PR open. Start at the `internal/query` parser.

## Dropped — newest first (the reason is the point)
