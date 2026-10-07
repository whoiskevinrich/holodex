# ADR-115: Cinémathèque is the only look — retire Broadcast, Brutalist, the custom palette and the skin setting

> **Archived — not current truth.** This numbered ADR was retired by HOLODEX-528. Its architecture now lives in [stack.md](../stack.md). Its product and UI rules now live in the owning spec and design docs (`docs/specs/`, `docs/design/`). Read those; this file is kept only as history.


**Status:** Proposed
**Date:** 2026-09-27
**Deciders:** Project owner

**Supersedes in part:** [ADR-021](ADR-021-frontend-theming-and-skins.md) — the three-skin roster
(Context), the per-skin selectors in §1, the `[data-theme]` gating and the Broadcast/Brutalist
flourishes in §3, and the Consequences "adding a skin", "QA must verify all three skins" and "five
font families". The token layer (§1), the Tailwind mapping (§2), skin-agnostic components (§3) and
offline fonts (§4) **stand**.
[ADR-102](ADR-102-instance-skin-and-settings-store.md) — **D1, D3, D4, D5 and D6**: there's no
instance skin to choose, no `/capabilities.theme`, no `PUT /admin/theme`, no custom palette, and
no paint cache. **D2 stands.** The generic `settings` table and its ownership boundary still hold
library-owned values, and [ADR-112](ADR-112-completeness-boot-fingerprint.md) uses it.
**Relates to:** HOLODEX-476 (epic), HOLODEX-460 (made obsolete), spec F67 (`docs/specs/instance-skin.md`,
retired by its amendment).

---

## Context

ADR-021 shipped three skins so the owner could pick a look. ADR-102 then made that pick *instance
identity*, and added a custom palette: five hex primaries in `holodex.yaml` over a Cinémathèque
base, with every other colour derived from them. In practice the owner uses only Cinémathèque,
and everything else costs something on every change:

- **QA triples.** The working agreement (ADR-021 Consequences, `.claude/rules/frontend-theming.md`)
  requires three skins plus the palette. Agent sessions follow it literally. They mock up, verify
  and propose designs in skins the owner never uses.
- **Mirrors multiply.** The skin tokens are hand-kept in three places: the `app.css` blocks,
  `internal/personimage/placeholder.go` `skinPalettes`, and `site/`. The palette derivation is kept
  twice, in the `app.css` `[data-palette='custom']` block and in `internal/theme` (about 366 lines),
  with a ΔE test that notices when they drift apart. The Appearance cards need a `.skin-card`
  selector fence. The Brutalist `reel` counter sits in unconditional CSS.
- **Tests rot.** The geometry harness's broadcast/brutalist cells have errored ever since ADR-102
  (HOLODEX-460), and nobody noticed.
- **A tab for one choice.** Once the retired skins are gone, the Appearance tab's only job would be
  choosing between Cinémathèque and the palette. The studio-halo toggle (HOLODEX-463) is **not** on
  that tab; it's set per studio image.

## Decision

### D1 — Cinémathèque is the only look, and nothing chooses it

These are removed:
- the Broadcast and Brutalist skins: their token blocks, flourishes, the `reel` counter and its
  unconditional `counter-reset`/`counter-increment`, the fonts only they load (VT323, Share Tech
  Mono, Spline Sans Mono) and their `@fontsource` deps, their placeholder palettes, their geometry
  cells, and their screenshots
- the custom palette: `theme.custom` in `holodex.yaml`, `internal/theme`, the `[data-palette]`
  derivation block, the boot contrast WARN, and the ΔE gate test
- the instance-skin selection: `PUT /admin/theme`, `/capabilities.theme`, the `theme.active`
  setting, `theme.svelte.ts` with its paint cache, and the `/owner/appearance` tab

*Why remove rather than freeze:* a frozen option still breaks whenever a token or hook class
changes, and someone still has to decide whether that matters. Only deleting it ends that cost.
The owner doesn't use the palette, so keeping it would only be speculation.

### D2 — The token layer and hook classes stay, ungated

"One look" still means components use the semantic utilities only (ADR-021 §2–§3). There's one
source for every colour, and a later look change stays a one-file edit. Cinémathèque's token block
moves from `[data-theme='cinematheque']` to `:root`, and `data-theme` goes from `app.html`. Its
flourishes attach to the hook classes (`.app-atmosphere`, `.video-frame`, `.video-grid`,
`.skin-title`) without a skin selector. `@keyframes reel-rise` is the shared grid animation, not the
Brutalist counter, and it stays.

### D3 — Leftover state degrades to nothing; no migration

- A `settings['theme.active']` row is ignored because nothing reads it. It can stay, since the
  table has no CHECK constraint and an orphaned row is harmless.
- A `theme.custom` block left in `holodex.yaml` becomes an unknown key. The config loader doesn't
  use strict decoding, so the key is silently ignored and never causes a boot failure.
- A stale `holodex-theme-cache` in a browser is never read again.
- `?skin=` on person-image URLs goes away with a single placeholder palette. The server ignores
  unknown query params, so cached URLs keep working. The duplicate `defaultSkin` constant in
  `cmd/holodex/main.go` goes too.

### D4 — QA is Cinémathèque

This replaces ADR-021's "QA must verify all three skins". Every UI change is verified in
Cinémathèque, which is the only look. Adding a skin, a palette or a theme switch again requires a
new ADR that supersedes this one. Agent instructions must not offer a theme choice as a design
option.

## Options Considered

| Option | Verdict |
|---|---|
| **Freeze the two skins:** keep them selectable, unsupported, never QA'd | Rejected. They'd rot visibly, and they keep the mirrors and fences alive. |
| **Instructions only:** stop QA'ing, keep ADR-021 as is | Rejected. It contradicts an Accepted ADR, so the next design session would bring the skins back. That's how this drift happened. |
| **Retire the skins, keep the custom palette** (this ADR's first draft) | Rejected on review. It leaves a tab, an endpoint, a setting, a paint cache and a CSS↔Go mirror serving one unused choice, and it needs a design pass to decide what a one-option tab shows. |
| **Retire everything but Cinémathèque and its tokens** (chosen) | One look, no choice to maintain, no mirrors, and no design gate beyond deleting a tab. |

## Consequences

- Every UI change needs one QA pass. The bundle drops three font families. About 366 lines of Go
  and their mirror in CSS go.
- HOLODEX-460 is resolved by deletion. `web/geometry` loses its skin axis.
- An instance set to Broadcast, Brutalist or a custom palette becomes Cinémathèque on upgrade. The
  release note says so. That's acceptable for a single-owner, self-hosted app.
- `/owner` loses its Appearance tab. The owner nav has one tab fewer; there's nothing to design.
- Recolouring now means editing the `:root` token block, which is one file. ADR-102 D4–D6 and git
  history keep the palette design if it's ever wanted back.
- **History isn't rewritten.** Older ADRs, specs, plans, handoffs and mockups still mention skins
  and palettes, and they stay as records. Forward-looking instructions have been corrected:
  CLAUDE.md, `.claude/rules/frontend-theming.md`, `docs/design/theming.md`,
  `docs/reference/workflow-idea-to-merge.md`, and the F41 spec's QA criteria. `graphify-out/` gets
  regenerated, not edited.

## Action Items (HOLODEX-476)

1. [x] Correct the agent instructions and forward-looking docs.
2. [x] Spec: the F67 amendment retires the instance skin and the palette.
3. [ ] Backend:
   - delete `internal/theme`, `internal/api/theme.go` and the route, `theme` in `/capabilities`,
     and `ThemeCustom` in `internal/config`
   - collapse `skinPalettes` and `?skin=`, and drop `defaultSkin`
   - update the tests: `theme_test.go` goes, and `personimage_test.go` and the `settings_test.go`
     fixture values change
4. [ ] Frontend:
   - `app.css`: the retired blocks and flourishes, `reel`, the `[data-palette]` layer and the
     `.skin-card` fence go, and the Cinémathèque tokens move to `:root`
   - drop the fonts and their deps
   - delete `theme.svelte.ts` and the theme types, `/owner/appearance` and its tab entry, and the
     `?skin=` call sites
   - update `api.test.ts`, and the component CLAUDE.md lines that describe removed code
5. [ ] Geometry harness: drop `SKINS`, `--skin` and the obsolete `holodex-theme` write, fix the
   fixtures, and close HOLODEX-460.
6. [ ] Docs: `holodex.yaml.example`, `docs/reference/configuration.md` §Appearance, `site/`,
   `README.md` and the screenshots.
7. [ ] Testing strategy: rewrite §12's geometry matrix and retire the F67 rows (R11 ΔE gate, R12
   contrast WARN).
