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

- [x] spec `write-spec` → `docs/specs/**` — RD13 + P0-12 in entity-identity.md
- [~] architecture — no technology fork (an alias is an ordinary alias; the backfill rides the existing identity-backfill pattern)
- [ ] backend → `{cmd,internal,providers}/**` — add the alias on create + rename, plus the one-time backfill
- [ ] testing `testing-strategy` — P0-12 acceptance cases (creation paths, collision skip, rename, removal not re-added, backfill idempotent)

## Up next — ordered (position = priority)

1. [ ] Backend: add the dashed alias wherever a tag is created or renamed, skipping on collision
2. [ ] Backfill existing multi-word tags once, visible in System Activity
3. [ ] Tests for every P0-12 acceptance case; update `docs/testing-strategy.md`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-10 · spec the dashed tag alias
- skills: write-spec, implement
- handoff: Crossed into build — backend posture has no approve gate, spec settled at 91c1a62; main already current; draft PR open. Start at the backend: dashed alias on tag create + rename.
