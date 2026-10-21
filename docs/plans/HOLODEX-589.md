---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-589
status: in-progress
profile: feature             # paged-list follow-ups: an API payload change, a11y focus fix, shared control
depends-on: []
release_note: Every paged list now shares one Load more control, the tags page loads lighter (no aliases in the list, categories counted instead of read up front), and the last Load more in Manage tags moves focus to the first new pill.
---

# HOLODEX-589 · Paged-list follow-ups: shared Load more everywhere, Tags manage-mode focus, lighter Tags pages

Spun out of HOLODEX-354 (F80 entity-list paging), whose PR left these five items out on purpose.
All five are settled: four changed code, and the display-names read was measured and left alone.

**Design package:** none. The Load more control's look is unchanged; the only visible change is
that browse and the owner queues now format the count with a thousands separator, like F80's lists.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `entity-identity.md` API table: aliases on `/tags/{id}`, not the `/tags` list
- [~] design `design-handoff` → `docs/design/**` — no UX change; the existing `LoadMoreButton` is reused as is
- [x] backend → `{cmd,internal,providers}/**` — `ListTagsFiltered` drops the alias batch; `TestEntityAliases` flipped (list omits, `GetTag` keeps)
- [x] frontend → `web/src/**` — `LoadMoreButton` on 8 more surfaces (`noun`/`disabled`/`class` knobs); `PagedList.count()`; `/tags` `data-pill` focus target and counted categories
- [x] testing `testing-strategy` — `pagedList.test.ts` covers `count()`; HOLODEX-589 live QA on the `-big` fixture recorded in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-589 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-20 · session
- skills: code-review (high --fix), handoff
- handoff: All five items done and live-verified on the `-big` fixture: /tags pages are 19.6KB (from 31KB), categories are read with `limit=1` until the tags run out, Manage-mode focus lands on the first new pill, and display names were measured and left alone. Next: merge the PR.

## Dropped — newest first (the reason is the point)

- **Narrowing `DisplayNames` to the page's ids** — measured, not needed: it reads only entities with a name decision through a covering index (0 in the `-big` fixture's 2,064 people), and `/people` answers in 3–15ms.
