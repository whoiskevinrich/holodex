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

- [ ] spec `write-spec` → `docs/specs/**` — amend F67 (`instance-skin.md`) to the `{cinematheque, custom}` domain
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-115 (supersedes in part ADR-021 and ADR-102)
- [ ] design `design-handoff` → `docs/design/**` — the Appearance tab without skin cards (ADR-115 D5), mockup as SVG
- [ ] backend → `{cmd,internal,providers}/**` — `shippedThemes`, `skinPalettes` / `?skin=` collapse, `defaultSkin` merge, tests
- [ ] frontend → `web/src/**` — app.css blocks and flourishes, `reel` counter, fonts and deps, `theme.svelte.ts` / `types.ts`, Appearance page, `?skin=` call sites, component CLAUDE.md lines
- [ ] testing `testing-strategy` — rewrite §12's "three skins × widths" geometry matrix; drop `web/geometry` `SKINS` (closes HOLODEX-460)
- [ ] security `security-review` — expected n/a (no auth or perimeter change; `PUT /admin/theme` only narrows)

## Up next — ordered (position = priority)

1. [ ] [—] Spec: amend F67 `instance-skin.md` (domain, R2/R13, API example `{"theme":"broadcast"}`)
2. [ ] [—] Design: Appearance tab with Cinémathèque + Custom only; SVG mockup in `docs/design/`
3. [ ] [—] `/implement`, then backend, frontend, geometry harness, and `site/` + README + `configuration.md` §Appearance
4. [ ] [—] On merge: close HOLODEX-460 as obsolete; the release note says retired-skin instances become Cinémathèque

## Session log — newest first

### 2026-09-27 · session
- skills: product-brainstorming, architecture
- Kevin asked why Claude kept presenting other themes. No single-theme decision existed: ADR-102 (#365) only made the skin instance identity, and ADR-021's "3-skin system" plus "QA all three skins" were still in force. Kevin chose to retire Broadcast and Brutalist entirely, keep the custom palette, and keep the token layer.
- Filed the epic, claimed ADR-115, and wrote it. Corrected the forward-looking instructions (`.claude/CLAUDE.md`, `.claude/rules/frontend-theming.md`, `docs/design/theming.md`, `docs/reference/workflow-idea-to-merge.md`, the F41 spec's QA criteria) so sessions stop QA'ing and proposing the other skins now. History docs are left as records.
- handoff: ADR-115 is written and instructions now say Cinémathèque only; next is the F67 spec amendment, then the Appearance-tab design, then /implement.

## Dropped — newest first (the reason is the point)
