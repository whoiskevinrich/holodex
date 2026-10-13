---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-572
status: in-review
profile: ui                  # frontend bug fix; the grid snapshot after a Trash restore, no UX change
depends-on: []
release_note: Restoring something from Trash now shows it in the Media grid when you go back, instead of the grid as it was before.
---

# HOLODEX-572 · Restored item missing from the Media grid after returning from Trash

A successful Restore on the Trash page (media or film) clears the pending `browseCache` snapshot, so
the next return to the grid reloads from the server. A film restore changes the grid too, because it
re-hides its full-film file. Only the server knows where a restored item sorts, so the snapshot can't
be patched the way HOLODEX-478's `forgetVideo` patches a delete. The cost is that this one return
starts at the top instead of the saved scroll.

**Design package:** none (no UX change)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no UX change
- [x] frontend → `web/src/**`
- [~] testing `testing-strategy` — a one-line `browseCache.clear()` in a route component with no harness (`clear` is already the snapshot's tested primitive); browser-verified on `backend-amv`: trash a video → grid → Trash → Restore → Back shows it again, count back to 201 — until: a route-component test harness exists

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-572 to Done (Bug on its own keyed branch — jira-sync handles it)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · clear the grid snapshot on a Trash restore
- skills: —
- handoff: Fixed and browser-verified (`browseCache.clear()` in `owner/trash/+page.svelte`'s `restore`); the PR is open and ready — next: merge, then confirm CI moves the issue to Done.

## Dropped — newest first (the reason is the point)
