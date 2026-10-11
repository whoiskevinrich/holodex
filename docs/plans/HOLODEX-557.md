---
key: HOLODEX-557
status: in-progress
profile: backend
depends-on: []
release_note: "A multi-word tag now gets an alias spelled with dashes, so `science fiction` is also found as `science-fiction` and a dashed genre in a file no longer creates a second tag. Existing tags get the alias too."
---

# HOLODEX-557 · Tags automatically get a dashed alias for their multi-word name

Relates to HOLODEX-507. Tag identity ignores case and whitespace but not punctuation, so
`science fiction` and `science-fiction` are different names. A multi-word tag now carries its
dashed spelling as an ordinary alias. Owner's picks (2026-10-10): backfill existing tags once; skip
the alias silently when the dashed spelling belongs to another tag (the Duplicates queue takes the
pair, no auto-merge); a rename adds a dashed alias for the new name; spaces to dashes only.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — RD13 + P0-12 in entity-identity.md; RD12 widened to collapse whitespace
- [~] architecture — no technology fork (an alias is an ordinary alias; the backfill rides the existing identity-backfill pattern)
- [x] backend → `{cmd,internal,providers}/**` — `internal/repo/tag_dashed_alias.go` hooked into the create choke point + rename; `tag-dash-backfill` boot job; removal suppressed; orphan sweep ignores the automatic alias
- [x] testing `testing-strategy` — `tag_dashed_alias_test.go` covers every P0-12 case; `docs/testing-strategy.md` invariant added

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge PR #485 on green CI, then confirm CI moved HOLODEX-557 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-10 · spec and build the dashed tag alias
- skills: write-spec, implement, code-review high --fix, handoff
- handoff: Owner widened RD12: tag names and aliases now collapse whitespace runs to one space (tagNorm on every write; migration 0061 for existing names). That closes the doubled-space gap; the only gap left is that a denied dashed spelling is still added. PR #485 stays ready; start at squash-merge on green CI.
