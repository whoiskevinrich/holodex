---
key: HOLODEX-558
status: in-progress
profile: backend
depends-on: []
release_note: "A tag spelling you've blocked is no longer added as a multi-word tag's automatic dashed alias."
---

# HOLODEX-558 · A blocked dashed spelling is never added as a tag's automatic alias

Spun out of HOLODEX-557. The automatic dashed alias (F43 RD13 / P0-12) skipped the tag deny-list, so
a blocked spelling could still show up in search as a name for the tag. Owner's pick (2026-10-11):
**forward-only**, like blocking a tag name. New automatic aliases skip a blocked spelling, and blocking
later leaves an existing one for the owner to remove.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — RD13 + two P0-12 cases in entity-identity.md
- [~] architecture — no technology fork (one more skip condition on an existing path)
- [ ] backend → `{cmd,internal,providers}/**` — skip a denied spelling in the dashed-alias helper
- [ ] testing `testing-strategy` — create / rename / backfill skip a blocked spelling; a later block leaves the alias

## Up next — ordered (position = priority)

1. [ ] [backend] Skip a denied spelling in `addDashedTagAliasTx` — `internal/repo/tag_dashed_alias.go`
2. [ ] [testing] Cover create, rename, backfill, and the forward-only later block — `internal/repo/tag_dashed_alias_test.go`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · spec the deny-list skip
- skills: —
- handoff: Spec amended (RD13 sentence + two P0-12 cases) with the owner's forward-only pick; branch keyed. Start at /implement.
