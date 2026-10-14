---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-43
status: in-progress
profile: feature             # new owner-visible indicator + an additive API field
depends-on: []
release_note: On a person's page, the owner now sees a small lock in the Edit control of any headshot, banner or poster they uploaded or promoted, so it's clear that re-enriching won't replace it.
approved:
  design:
    on: 2026-10-14
    at: 07017d71
---

# HOLODEX-43 · Owner-view provenance badge on core images

F33 (people-images.md, F25.31) locks a core slot the owner set by hand (upload or promoted) so
enrichment never overwrites it. This story makes the lock legible to the owner: a lock icon folded
into each locked slot's existing Edit control (Option B, owner-approved from mockups). Story under
epic HOLODEX-13 (People).

**Design package:** [people-images-handoff.md § Core-slot lock](../design/people-images-handoff.md#core-slot-lock-owner-view-holodex-43) · [core-image-lock-mockup.svg](../design/core-image-lock-mockup.svg) · spec [F25.35](../specs/people-images.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F25.35 added; F33's deferred indicator marked done, per-slot pin still deferred
- [x] design `design-handoff` → `docs/design/**` — Core-slot lock section + committed SVG mockup
- [x] backend → `{cmd,internal,providers}/**` — `PersonImageSlot.locked` (additive), derived server-side by LockedCoreRoles' upload/promoted rule
- [x] frontend → `web/src/**` — `person/CoreImageEditButton.svelte` replaces the page's `editBtn` markup; owner compared the build against the mockup and confirmed the match 2026-10-14
- [x] testing `testing-strategy` — `TestPersonImageSetLocked`, the wire check in `TestPersonDetailImageSet`, SSR `coreImageEditButton.test.ts`; live-verified on the stress fixture; HOLODEX-43 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] On merge: sweep HOLODEX-43 to Done by hand if CI doesn't (child of epic HOLODEX-13)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · spec + design, sign-off, build, verify
- skills: implement, code-review (high --fix), handoff
- handoff: All gates settled; the owner confirmed the build matches the mockup and PR #516 is marked ready. Next: merge, then confirm HOLODEX-43 reaches Done.

## Dropped — newest first (the reason is the point)
