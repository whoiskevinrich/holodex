---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-86
status: in-progress
profile: ui                  # feedback line on existing enrich actions; no product rule changes
depends-on: []
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

- [/] design `design-handoff` → `docs/design/**` — handoff edited in place (toast → inline line) + SVG; owner chose option A in chat, sign-off on the committed artifact pending
- [x] frontend → `web/src/**` — `StatusLine` + `enrichedLine`/`refreshAllLine`; wired on media/person/studio/film; live-verified on `/people/9050`
- [x] testing `testing-strategy` — `statusLine.test.ts`, new `api.test.ts` cases; HOLODEX-86 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] Owner signs off the committed handoff + SVG; `/implement` records it and opens the PR
2. [ ] [—] Live-check the collapsed Refresh-all wording (pane stopped drawing before it could be seen)
3. [ ] [—] Merge; sweep HOLODEX-86 to Done by hand (a child of HOLODEX-168 — CI moves only the branch's own key, which this is, but confirm)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · session
- skills: code-review (high --fix)
- handoff: Inline enrich success line built on all four detail pages and live-verified for picker, auto-apply and Refresh; next the owner signs off the committed handoff and SVG, then `/implement` opens the PR.

## Dropped — newest first (the reason is the point)

- **A shared toast primitive (option B)** — 2026-10-14. Three specs already chose inline aria-live outcome lines over a toast; a global toast is a cross-app design change for one line of feedback.
- **Enrich queue rows** — 2026-10-14. A queue row already flips to its applied state, which is the feedback; it has no page status slot.
