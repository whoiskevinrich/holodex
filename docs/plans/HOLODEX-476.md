---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-476
status: in-progress
profile: full
depends-on: []
release_note:
---

# HOLODEX-476 · Retire Broadcast and Brutalist — Cinémathèque is the only skin

Done means Holodex ships one skin, Cinémathèque, and the owner can still recolour it with the
custom palette. Broadcast and Brutalist are gone from the CSS, fonts, Appearance tab, the Go
allowlist and placeholders, the geometry harness and the landing page. An instance that was set to
either one comes up in Cinémathèque, and every UI change is QA'd once, not three times.

**Design package:** [ADR-115](../architecture/ADR-115-cinematheque-only-skin.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F67 `instance-skin.md` amended: RD11–RD12, story 9, R17–R23, OQ4 (the Appearance tab with one skin); superseded lines marked *(amended, ADR-115)*
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-115 (supersedes in part ADR-021 and ADR-102)
- [ ] design `design-handoff` → `docs/design/**` — the Appearance tab without skin cards (ADR-115 D5), mockup as SVG
- [ ] backend → `{cmd,internal,providers}/**` — `shippedThemes`, `skinPalettes` / `?skin=` collapse, `defaultSkin` merge, tests
- [ ] frontend → `web/src/**` — app.css blocks and flourishes, `reel` counter, fonts and deps, `theme.svelte.ts` / `types.ts`, Appearance page, `?skin=` call sites, component CLAUDE.md lines
- [ ] testing `testing-strategy` — rewrite §12's "three skins × widths" geometry matrix; drop `web/geometry` `SKINS` (closes HOLODEX-460)
- [ ] security `security-review` — expected n/a (no auth or perimeter change; `PUT /admin/theme` only narrows)

## Up next — ordered (position = priority)

1. [ ] [—] Design: answer F67 OQ4 — the Appearance tab with one skin (plus Custom when configured); SVG mockup in `docs/design/`, Cinémathèque only
2. [ ] [—] `/implement`, then backend (R17/R18/R21), frontend (R19–R21), geometry harness (R22), and docs (R23)
3. [ ] [—] On merge: close HOLODEX-460 as obsolete; the release note says retired-skin instances become Cinémathèque

## Session log — newest first

### 2026-09-27 · session (spec)
- skills: write-spec
- Amended F67 in place instead of starting a new spec. Added an amendment banner, RD11–RD12, story 9, and R17–R23: the allowlist, the silent fallback, removing the skins from the bundle, the Appearance tab, the `?skin=` collapse, the geometry harness and operator docs. Added OQ4 (what the tab shows with one skin). Superseded lines are marked, not deleted. `themePayload` already falls back to Cinémathèque without logging, so R18 needs no backend work.
- handoff: The spec gate is settled; next is the design handoff for F67 OQ4 (the Appearance tab with one skin), mocked in Cinémathèque only.

### 2026-09-27 · session
- skills: product-brainstorming, architecture
- Kevin asked why Claude kept presenting other themes. No single-theme decision existed: ADR-102 (#365) only made the skin instance identity, and ADR-021's "3-skin system" plus "QA all three skins" were still in force. Kevin chose to retire Broadcast and Brutalist entirely, keep the custom palette, and keep the token layer.
- Filed the epic, claimed ADR-115, and wrote it. Corrected the forward-looking instructions (`.claude/CLAUDE.md`, `.claude/rules/frontend-theming.md`, `docs/design/theming.md`, `docs/reference/workflow-idea-to-merge.md`, the F41 spec's QA criteria) so sessions stop QA'ing and proposing the other skins now. History docs are left as records.
- handoff: ADR-115 is written and instructions now say Cinémathèque only; next is the F67 spec amendment, then the Appearance-tab design, then /implement.

## Dropped — newest first (the reason is the point)
