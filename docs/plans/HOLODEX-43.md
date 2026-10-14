---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-43
status: in-progress
profile: feature             # new owner-visible indicator + an additive API field
depends-on: []
release_note:
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
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [backend] Add `locked` to each core role in the person image set — `internal/repo/person_images.go`
2. [ ] [frontend] Lock icon + accessible name in the core-slot Edit control — `web/src/routes/people/[id]/+page.svelte`
3. [ ] [testing] Go + vitest coverage, testing-strategy entry, live preview check — `docs/testing-strategy.md`
4. [ ] [—] On merge: sweep HOLODEX-43 to Done by hand if CI doesn't (child of epic HOLODEX-13)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · spec + design, sign-off, crossed into build
- skills: implement
- handoff: Crossed into build — design signed off at 07017d71. Start at the backend `locked` field.

## Dropped — newest first (the reason is the point)
