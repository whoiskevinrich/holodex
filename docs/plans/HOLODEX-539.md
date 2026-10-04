---
key: HOLODEX-539
status: in-review
profile: ui                  # UX change, behavior unchanged
depends-on: []
approved:
  design:
    on: 2026-10-04
    at: 56e0afed
release_note: "The Details card on a person's page now starts collapsed, so the page opens on what matters; click Details to see every field."
---

# HOLODEX-539 · Person Details card collapsible, collapsed by default

Done means the Person page Details field list is a fold that is closed on every load, for owner and
visitor alike. The label (chevron + `Details · N fields`) is the toggle, the Enrich chips stay in the
header, and a `#field-*` deep link opens the fold before it scrolls.

**Design package:** spec n/a (no new functionality) · architecture n/a (no technology fork) · [handoff](../design/person-details-fold-handoff.md) · [testing-strategy § 5 row](../testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done. -->

- [x] design `design-handoff` → `docs/design/person-details-fold-handoff.md` (option A + visitors collapsed, both approved by the owner 2026-10-04)
- [x] frontend → `web/src/routes/people/[id]/+page.svelte`
- [x] testing `testing-strategy` → `detailsFold.test.ts` + §5 row; live `[agent]` QA recorded in the handoff

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge when CI passes (owner asked, 2026-10-04); CI moves HOLODEX-539 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · designed, built, verified
- skills: code-review, implement
- handoff: Owner approved the build ("looks good"), which covers the [human] animation check; PR #453 marked ready to squash-merge once CI is green. Left out from review: SourceBadge expand on landing, and a fold component shared with media.

## Dropped — newest first (the reason is the point)
