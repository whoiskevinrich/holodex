---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-476
status: in-progress
profile: full
depends-on: []
release_note: Holodex now has a single look, Cinémathèque — the Broadcast and Brutalist skins, the custom palette (theme.custom) and the Owner › Appearance tab are gone, and instances that used them now render in Cinémathèque.
---

# HOLODEX-476 · Cinémathèque is the only look — retire the extra skins, the custom palette and the Appearance tab

Done means Holodex has one look, Cinémathèque, and nothing to choose. Broadcast, Brutalist, the
custom palette (`theme.custom`), `PUT /admin/theme`, `/capabilities.theme`, the paint cache and
`/owner/appearance` are gone from the code, fonts, geometry harness and docs. An instance that was
set to any of them comes up in Cinémathèque with nothing to fix, and every UI change is QA'd once.

**Design package:** [ADR-115](../architecture/ADR-115-cinematheque-only-skin.md) ·
[F67 amendment](../specs/instance-skin.md) (R17–R23)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F67 `instance-skin.md` amended to retire itself: RD11–RD12, story 9, R17–R23; OQ4 withdrawn; superseded lines marked *(amended, ADR-115)*
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-115 (supersedes in part ADR-021 and ADR-102 D1/D3–D6)
- [~] design `design-handoff` → `docs/design/**` — skipped: the epic deletes `/owner/appearance` and adds no UI, so there's nothing to mock up. R20's acceptance covers the tab bar with one tab fewer. Kevin chose this scope on 2026-09-27 — until: any removal leaves a visible gap that needs a new layout
- [x] backend → `{cmd,internal,providers}/**` — R17/R18/R21: `internal/theme` and `internal/api/theme.go` deleted (route, `/capabilities.theme`, config block gone); one Cinémathèque placeholder palette, `?skin=` ignored, `defaultSkin` dropped; tests for the removed route, a stale `?skin=` URL and a leftover `theme.custom` block
- [x] frontend → `web/src/**` — R19–R21 (`d5f0c12d`): tokens in one `:root` block, retired skins/palette/`.skin-card` fence and three fonts gone; `theme.svelte.ts`, `/owner/appearance`, `api.setTheme` and `?skin=` removed; halo reads a fixed dark `PALETTE_MODE`. Browse-grid computed styles match the pre-change baseline token for token; `npm run check` 0 errors, 499 unit tests green
- [x] testing `testing-strategy` — R22 (`ceecf89d`): the geometry harness matrix is one cell per width (no `SKINS`, `--skin` or `goto` skin param; an unknown flag exits 2); testing-strategy §12 describes the four-width matrix and §13 records F67's retirement and the tests that now guard leftover state; 47 harness tests green. Not run live — this worktree has no stress fixture or `backend-stress` profile
- [ ] security `security-review` — expected n/a (an owner-gated write is removed; nothing is added)

## Up next — ordered (position = priority)

1. [ ] [—] R23 docs: `holodex.yaml.example`, `configuration.md` §Appearance, `README.md`, `site/`, screenshots
2. [ ] [—] On merge: close HOLODEX-460 as obsolete. The release note says every instance renders in Cinémathèque, and the Appearance tab and `theme.custom` are gone.

## Session log — newest first

### 2026-09-28 · session
- skills: handoff, implement, code-review
- Merged `origin/main` (#403, list toolbar) in at `5706c31b`. The only conflict was the ADR index, where both rows were kept in order (114, 115). Settled the design gate as `[~]`, because the epic deletes a tab and adds no UI. Rewrote Up next as the build queue and authored the release note.
- Crossed into build. The design phase is settled, and there was no sign-off to ask for because design is `[~]`. The branch was already current with `origin/main`. Opened Draft PR #404, removed `fp:ready-to-build`, and moved the epic to In Progress by hand, since jira-sync skips epics. Before merge, decide whether the squash subject should be `feat(theming)!:` (breaking: `theme.custom` is removed) or plain `feat`.
- Backend landed (R17/R18/R21): `internal/theme`, `internal/api/theme.go`, the `/admin/theme` route, `/capabilities.theme` and the `theme:` config block are deleted. The placeholder uses one Cinémathèque palette and ignores `?skin=`. New tests cover the removed route, the stale `?skin=` URL and a leftover `theme.custom` block. `go test ./...` is green. `/code-review high --fix` folded `authServerH` away. Until the frontend commit, the Appearance tab's PUT gets a 404; the SPA bootstrap is safe because it reads `caps?.theme`.
- Frontend landed (R19–R21, `d5f0c12d`): `app.css` is one `:root` Cinémathèque token block with no skin, palette or `data-theme` selectors. The three retired fonts and their deps are gone. `theme.svelte.ts`, the theme types, `api.setTheme`, `/owner/appearance` and `?skin=` are removed. Found that the halo's `mode` came from the theme store, so `halo.ts` now exports a fixed dark `PALETTE_MODE`, and behaviour and copy are unchanged. Verified live on backend-amv: computed styles on the browse grid match the pre-change baseline token for token (grain, vignette, 9px letterbox bars, Fraunces/Archivo, 70 cards). `/owner/appearance` is a 404, the tab bar wraps at 375px without scrolling, `PUT /admin/theme` gives 404 and `/capabilities` has no `theme`. `/code-review high --fix` caught that the geometry harness waited on `data-theme` (fixed). Two things were filed rather than changed: HOLODEX-482 (halo modes, copy/ADR) and HOLODEX-483 (chip char cap).
- Testing landed (R22, `ceecf89d`): the geometry harness has no skin axis. `SKINS`, `--skin` and `goto`'s skin param are gone, cells are keyed by width, and the report fixtures were renamed. `/code-review high --fix` made an unknown flag exit 2 rather than throw an exit 1, so a stale `--skin` now reads as a usage error, not a regression. testing-strategy §12 describes the four-width matrix. §13 is marked retired, listing the deleted F67 tests and the ones that now guard leftover state, plus the computed-style baseline-diff method used for R19. Dated run records elsewhere stay as history (ADR-115). The harness was not run live: this worktree has no stress fixture and no `backend-stress` profile.
- handoff: Backend, frontend and testing are done; start R23 docs — `holodex.yaml.example`, `docs/reference/configuration.md` §Appearance, `README.md`, `site/` and the retired screenshots — then the security gate (expected n/a).

### 2026-09-27 · session (palette too)
- skills: product-brainstorming, architecture, write-spec
- Kevin asked whether removing the custom palette would be simpler than answering OQ4. It is: once there's one skin, the palette is the Appearance tab's only purpose. I also found I'd been wrong that the tab hosts the studio-halo toggle; that's per studio image (HOLODEX-463). Kevin chose to remove it. I revised ADR-115 in place (Proposed, unmerged): the palette, `/admin/theme`, `/capabilities.theme`, the paint cache and the tab all go, the tokens move to `:root`, and the `settings` table stays for ADR-112. F67's amendment now retires the feature, and OQ4 is withdrawn. The loader isn't strict, so a leftover `theme.custom` is dropped silently. I corrected the instructions and memory that said "plus the custom palette".
- handoff: The ADR and spec are settled for the full retirement; record the design gate as a skip, then /implement.

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

- **Keeping the custom palette** (the first draft of ADR-115) — once there's one skin, it leaves a tab, an endpoint, a setting, a paint cache and a CSS↔Go mirror serving one unused choice.
- **F67 OQ4, what the Appearance tab shows with one skin** — moot, because the tab is removed.
