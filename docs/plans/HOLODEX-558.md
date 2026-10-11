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
- [x] backend → `{cmd,internal,providers}/**` — `addDashedTagAliasTx` skips a denied spelling (covers create, rename, backfill)
- [x] testing `testing-strategy` — two tests in `tag_dashed_alias_test.go`, mutation-checked; `docs/testing-strategy.md` line widened

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR on green CI, then confirm CI moved HOLODEX-558 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · spec and build the deny-list skip
- skills: implement, code-review high --fix, handoff
- handoff: Spec (forward-only, owner's pick), the one-line skip and its tests all landed; every gate settled, main (#488) merged in cleanly, and PR #489 marked ready. Start at squash-merge on green CI.
