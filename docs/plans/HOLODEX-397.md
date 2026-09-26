---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-397
status: in-progress
profile: ui
release_note: The Film and Media detail pages now show a studio's logo (when it has one) instead of a monogram — the plate follows the logo's shape, so wordmarks read at full width.
---

# HOLODEX-397 · Studio logo on the Film and Media detail pages

Both pages share `StudioLinkCard`, which was specced (HOLODEX-290) to draw `icon_url` only. TMDB
enrichment fills only the `logo` role, and the studio hero doesn't draw it either, so enriched
studios show a monogram everywhere. Change the card to draw `logo_url` → `icon_url` → monogram in
a fixed-height, aspect-following plate (`h-12 min-w-12 max-w-48`); name and count stay.

## Gates — definition of done

- [x] design `design-handoff` — options B + logo-first approved 2026-09-16:
  `docs/design/studio-logo-link-card-handoff.md` + `studio-logo-link-card-mockup.svg`;
  HOLODEX-290 handoff carries a supersession note
- [x] frontend — `StudioLinkCard.svelte` per handoff §2 (one file; call sites unchanged); entity
  `CLAUDE.md` row updated
- [x] testing `testing-strategy` — row added; §11 agent QA (11.1–11.10) passed 2026-09-17 against
  synthetic 5:1 / 12:1 / portrait / square / both / icon-only / none studios; human 11.11–11.13 open

## Up next — ordered (position = priority)

1. [ ] [—] Human QA 11.11–11.13 `[human]`: film + media page in all three skins with a real TMDB
   logo; phone-width wrap with two logo studios (this test DB has no video with two studios)
2. [ ] [—] On merge, HOLODEX-397 → Done via CI (branch-keyed; PR marked ready → In Review)
3. [ ] [—] HOLODEX-399 (studio hero draws its own logo) is filed, not started — separate branch

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-17 · implemented, agent-QA'd
- skills: code-review high --fix (1 low finding, fixed: `??` → `||`)
- handoff: `StudioLinkCard` now draws logo → icon → monogram in an `h-12 min-w-12 max-w-48` plate;
  all seven image branches measured in the driven browser across three skins, no overflow at 375px.
  PR marked ready. Only the human skin look (11.11–11.13) remains.

### 2026-09-16 · design gate
- skills: design-handoff
- handoff: Jira story filed, branch keyed, design approved from inline mockups (B + logo-first),
  handoff + SVG committed, Draft PR open. Next session: implement §2 — a ~6-line change to one
  component — then QA §11.
