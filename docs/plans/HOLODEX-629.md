---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-629
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: ui                  # wording fix on an existing surface; behaviour (spec P0-3) unchanged
depends-on: []
release_note: When a file name won't read back, the File names plan now names the value that caused it, such as a title holding ` (`, rather than a cast name that only read back wrong because of it.
---

# HOLODEX-629 · Won't-read-back line blames the wrong token (F86 bug)

Done means a won't-read-back row names the token whose value holds the text that ends it in the
template, ahead of tokens that only read back wrong as a knock-on. The refusal itself (spec P0-3)
is unchanged.

## Gates — definition of done

- [~] design `design-handoff` — no change: the handoff's plan-row explanation line keeps its wording; only which token it names changes.
- [x] frontend → `web/src/**`. `readBack` in `web/src/lib/rename.ts` returns the first token holding its breaker, else the first that differs, and finds a token written optional (`{tok?}`).
- [x] testing `testing-strategy`. `rename.test.ts` "blames the token whose value holds its breaker" covers the owner's case (title with ` (` before `({people})`) and an optional `{title?}`; the rename vitest suite passes.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-629 moved to Done (CI moves the branch's own key).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-07 · read-back blame
- skills: code-review high --fix
- handoff: Fix and test landed and every gate is settled, so the PR opens ready for review; next is merging it.

## Dropped — newest first (the reason is the point)

- Balanced-bracket parsing so such titles rename at all: a change to the shared F48 filename grammar, needing a spec and architecture edit. Not asked for; the owner chose the message fix.
