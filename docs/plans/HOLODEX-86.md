---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-86
status: in-progress
profile: ui                  # feedback line on existing enrich actions; no product rule changes
depends-on: []
approved:
  design:
    on: 2026-10-14
    at: 9438ab05
release_note: After you enrich a video, person, studio or film from a metadata provider, the page now says which provider it came from and how many fields it filled in, for a few seconds.
---

# HOLODEX-86 · Success toast after enrich

Child of HOLODEX-168 (Multi-provider enrichment UX). From checklist 3.10: the enrichment handoff
promised an "Enriched from {provider}" toast, but the app has no toast system and the picker
closing was the only feedback. The owner chose an inline status line over a new toast primitive
(option A of three, 2026-10-14), on all three enrich paths: the picker's confirm and auto-apply,
the per-provider Refresh, and Refresh all.

**Design package:** [metadata-enrichment-handoff.md](../design/metadata-enrichment-handoff.md)
(Interaction States, "Picker | confirm success"; Edge Cases, "Re-enrich") ·
[enrich-success-line-mockup.svg](../design/enrich-success-line-mockup.svg)

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**` — handoff edited in place (toast → inline line) + `enrich-success-line-mockup.svg`; owner signed off at 9438ab05 and confirmed the build matches
- [x] frontend → `web/src/**` — `StatusLine` + `enrichedLine`/`refreshAllLine`; wired on media/person/studio/film; live-verified on `/people/9050`
- [x] testing `testing-strategy` — `statusLine.test.ts`, new `api.test.ts` cases; HOLODEX-86 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-86 to Done (the branch carries its own key)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · session
- skills: code-review (high --fix), handoff, implement
- handoff: Crossed into build — design signed off at 9438ab05, build confirmed against the mockup, all gates settled; the PR is ready for review. Next: merge and confirm CI moves HOLODEX-86 to Done.

## Dropped — newest first (the reason is the point)

- **A shared toast primitive (option B)** — 2026-10-14. Three specs already chose inline aria-live outcome lines over a toast; a global toast is a cross-app design change for one line of feedback.
- **Enrich queue rows** — 2026-10-14. A queue row already flips to its applied state, which is the feedback; it has no page status slot.
