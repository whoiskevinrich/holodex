---
key: HOLODEX-508
status: in-progress
profile: backend
depends-on: []
release_note: "Search now finds a studio by any of its aliases, such as WB for Warner Bros., and lists it once under its own name."
---

# HOLODEX-508 · Global search matches studio aliases

The other unmet half of F43 P0-9 (spun out of HOLODEX-507). Global search reads `studios_fts` only, so a
studio is not found by an alias like "WB". Done = a `searchStudiosByAlias` over `entity_aliases_fts`
(`entity_type='studio'`), merged into `res.Studios` deduped by id, a repo test, and the spec's P0-9 note
closed for studios.

**Design package:** spec [entity-identity.md](../specs/entity-identity.md) (P0-9 already requires it; the
inline HOLODEX-507 note is extended for studios) · no ADR (completes ADR-061) · no UI.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — P0-9 note + studio acceptance case in entity-identity.md
- [x] backend → `{cmd,internal,providers}/**`
- [x] testing `testing-strategy` — `TestSearchMatchesStudioAlias`, failed before the fix

## Up next — ordered (position = priority)

1. [ ] [—] Review and merge PR #430; CI moves HOLODEX-508 to Done
2. [ ] [—] Studio/tag matches don't pull their media into Videos (P0-9 "+ its media") → HOLODEX-511

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-02 · fix built and tested in one session
- skills: code-review (high --fix), handoff, implement
- note: security — one parameterized read-only FTS query, no auth/access/infra change, so no `/security-review`.
- handoff: Crossed into build (no approve gates in this posture); PR #430 open and marked ready. Start at: review and merge it.

## Dropped — newest first (the reason is the point)
