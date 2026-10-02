---
key: HOLODEX-508
status: in-progress
profile: backend
depends-on: []
release_note: ""
---

# HOLODEX-508 · Global search matches studio aliases

The other unmet half of F43 P0-9 (spun out of HOLODEX-507). Global search reads `studios_fts` only, so a
studio is not found by an alias like "WB". Done = a `searchStudiosByAlias` over `entity_aliases_fts`
(`entity_type='studio'`), merged into `res.Studios` deduped by id, a repo test, and the spec's P0-9 note
closed for studios.

**Design package:** spec [entity-identity.md](../specs/entity-identity.md) (P0-9 already requires it; the
inline HOLODEX-507 note is extended for studios) · no ADR (completes ADR-061) · no UI.

## Gates — definition of done

- [ ] spec `write-spec` → `docs/specs/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [backend] `searchStudiosByAlias` + merge into the Studios block of `Search` — internal/repo/aliases.go
2. [ ] [testing] repo test: studio found by alias, once, by canonical name — internal/repo/identity_ops_test.go
3. [ ] [spec] close P0-9 for studios — docs/specs/entity-identity.md

## Session log — append-only (cap: last 8 sessions; older → archive/)

## Dropped — newest first (the reason is the point)
