---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-482
status: in-review
profile: ui
depends-on: []
release_note: The studio image halo switch now reads just "Halo" — it no longer names a palette mode, since Cinémathèque is the only look.
---

# HOLODEX-482 · Collapse the studio halo's dark/light modes into one switch

Done means the owner's halo switch on `/studios/{id}` no longer says "· dark" or "on dark
palettes", because Cinémathèque is the only look (ADR-115). The per-mode storage from ADR-109 is
left as it is.

**Context:** spun out of HOLODEX-476 · [ADR-109](../architecture/archive/ADR-109-per-studio-image-halo.md) ·
[ADR-115](../architecture/archive/ADR-115-cinematheque-only-skin.md) · [spec](../specs/studio-images.md) H3/H5

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — skipped: a copy-only change with the same layout. Kevin picked option A (copy only) over B (collapse the model to one boolean with a new ADR and migration) from an inline before/after mockup on 2026-09-28 — until: the switch's layout or behavior changes
- [x] frontend → `web/src/**` — `EntityImageSlot.svelte`: the label is "Halo", the tooltip "Glow behind the {role}", and a per-role `aria-label` so the logo/icon/poster switches have different names. QA'd live on Cinémathèque (`/studios/8`, a temporary test logo, since removed): the switch toggles and `.halo-dark` still glows. `npm run check` 0 errors
- [~] testing `testing-strategy` — skipped: no behavior changed. `halo.test.ts` (PALETTE_MODE is dark) is green, and `web/` has no component-test harness to pin the label — until: the halo model changes (option B)

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-482 to Done — `.github/workflows/jira-sync.yml`

## Session log — newest first

### 2026-09-28 · session
- skills: code-review, handoff
- decision (owner, inline mockup): option A, copy only. B (a new ADR superseding ADR-109's per-mode storage, a migration dropping `light` rows, and no `mode` in the API body) was declined because the pixels are the same and A keeps the per-mode storage in case a light look ever comes back
- build: dropped the mode from the switch label and tooltip; `/code-review high --fix` added a per-role `aria-label` and marked spec H3/H4's light branch as dormant under ADR-115
- branch `HOLODEX-482-halo-one-switch` cut fresh from `origin/main` (the session opened on the merged 476 branch); In Progress fired by hand via MCP (no local JIRA creds)
- handoff: Every gate is settled and the PR is ready. Next: merge, then confirm CI moved 482 to Done.

## Dropped — newest first (the reason is the point)

- Option B, collapsing the halo to one boolean per studio/role: the switch looks the same and the per-mode storage costs nothing to keep (Kevin, 2026-09-28)
