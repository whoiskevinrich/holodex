---
key: HOLODEX-539
status: in-progress
profile: ui                  # UX change, behavior unchanged
depends-on: []
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

1. [ ] [—] `/implement` → opens the PR (design gate is settled; the guard refuses `gh pr create` before it)
2. [ ] [human] Watch the open/close animation in a visible window (handoff QA 5; the agent pane was hidden)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · designed, built, verified
- skills: code-review
- handoff: Fold built per the approved mockup (label toggle, collapsed for visitors too). Deep-link landing verified with `elementFromPoint` as owner and visitor. Two review findings were left out: expanding the SourceBadge on landing, and a fold component shared with the media page.

## Dropped — newest first (the reason is the point)
