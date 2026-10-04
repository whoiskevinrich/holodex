---
key: HOLODEX-529
status: in-progress
profile: ui                  # a state bug in an existing dialog; behaviour restored, no UX change
depends-on: []
release_note: On a media page in a playlist or Play all run, a Move to Trash or Delete permanently dialog that is open when the item ends now closes as playback moves on, instead of switching to name (and offering to delete) the next item.
---

# HOLODEX-529 · Media detail delete dialog names a different video during playlist playback

This is done when a delete confirm opened on one media item can never name, or act on, another.

**Root cause:** the `/media/[id]` route component is reused across `/media/A → /media/B`, and in a
run playback keeps going behind the modal. When the item ended the run advanced, and `confirmMode`
was not part of the per-id reset, so the still-open dialog rendered B's title and Confirm would
have deleted B.

**Design package:** bug fix, no spec or ADR (delete behaviour is unchanged).

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**`. No UX change: the dialog's look and copy are untouched; this restores which video it belongs to.
- [x] frontend → `web/src/**`. The per-id load effect resets `confirmMode`, `deleteBusy`, `deleteError`, `deleteMenuOpen`; `confirmDelete` ignores an in-flight result once `pageGeneration` moves on.
- [x] testing `testing-strategy`. `deleteConfirmReset.test.ts` (structural, like `playerElement.test.ts`); live repro and fix on the stress fixture; row in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] Sweep HOLODEX-529 to Done once its PR merges (CI moves it to In Review on ready and Done on merge; check that it did).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-04 · fixed the delete dialog carrying over a run advance
- skills: code-review, handoff
- handoff: Fixed, reviewed and verified live (repro without the reset, closes with it); every gate settled. Next: review and squash-merge the PR.

## Dropped — newest first (the reason is the point)
