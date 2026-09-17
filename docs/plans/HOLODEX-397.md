---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-397
status: in-progress
release_note: The Film and Media detail pages now show a studio's logo (when it has one) instead of a monogram — the plate follows the logo's shape, so wordmarks read at full width.
---

# HOLODEX-397 · Studio logo on the Film and Media detail pages

Both pages share `StudioLinkCard`, which was specced (HOLODEX-290) to draw `icon_url` only. TMDB
enrichment fills only the `logo` role, and the studio hero doesn't draw it either, so enriched
studios show a monogram everywhere. Change the card to draw `logo_url` → `icon_url` → monogram in
a fixed-height, aspect-following plate (`h-12 min-w-12 max-w-48`); name and count stay.

## Gates — definition of done

- [~] spec `write-spec` — n/a: presentation-only, no new capability or data
- [~] architecture `architecture` — n/a: ADR-079 image roles / serving untouched
- [x] design `design-handoff` — options B + logo-first approved 2026-09-16:
  `docs/design/studio-logo-link-card-handoff.md` + `studio-logo-link-card-mockup.svg`;
  HOLODEX-290 handoff carries a supersession note
- [ ] frontend — `StudioLinkCard.svelte` per handoff §2 (one file; call sites unchanged)
- [x] testing `testing-strategy` — row added (design gate); QA per handoff §11 once implemented
- [~] security `security-review` — n/a: no auth/access/infra change
- [~] backend — n/a: `logo_url` already populated at both call sites

## Up next — ordered (position = priority)

1. [ ] [—] Implement handoff §2 in `web/src/lib/components/entity/StudioLinkCard.svelte`;
   update the `StudioLinkCard` row in `web/src/lib/components/entity/CLAUDE.md`
2. [ ] [—] Run handoff §11 (11.1 smoke, 11.2–11.10 agent via `javascript_tool`, 11.11–11.13 human)
3. [ ] [—] Mark the Draft PR ready (fires In Review); on merge, HOLODEX-397 → Done via CI
4. [ ] [—] HOLODEX-399 (studio hero draws its own logo) is filed, not started — separate branch

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · design gate
- skills: design-handoff
- handoff: Jira story filed, branch keyed, design approved from inline mockups (B + logo-first),
  handoff + SVG committed, Draft PR open. Next session: implement §2 — a ~6-line change to one
  component — then QA §11.
