# ADR-115: Cinémathèque is the only skin — retire Broadcast and Brutalist

**Status:** Proposed
**Date:** 2026-09-27
**Deciders:** Project owner

**Supersedes in part:** [ADR-021](ADR-021-frontend-theming-and-skins.md) — the three-skin roster
(Context), the per-skin selector list in §1, the Broadcast/Brutalist flourishes in §3, and the
Consequences "adding a skin", "QA must verify all three skins" and "five font families".
§1–§4's token layer, Tailwind mapping, skin-agnostic components and offline fonts **stand**.
[ADR-102](ADR-102-instance-skin-and-settings-store.md) — the "shipped set" of D1/D3 (three ids →
one), D4's closing sentences ("Broadcast/Brutalist become eligible bases…"), and D5's "three cards".
The `settings` store (D2), the `/capabilities` channel (D3), the custom palette (D4–D6) **stand**.
**Relates to:** HOLODEX-476 (epic), HOLODEX-460 (made obsolete), spec F67 (`docs/specs/instance-skin.md`).

---

## Context

ADR-021 shipped three skins because the owner wanted a choice of looks. ADR-102 then made the
skin *instance identity*: one owner-set value that every viewer gets. After that, only one skin
is ever visible on an instance. The other two still cost something on every UI change:

- **QA triples.** The working agreement (ADR-021 Consequences, `.claude/rules/frontend-theming.md`)
  requires eyeballing every change in all three skins. Agent sessions follow it literally. They
  mock up, verify and propose designs in Broadcast and Brutalist, which the owner never uses.
- **Mirrors multiply.** The skin tokens are hand-kept in three places: the `app.css` token blocks,
  `internal/personimage/placeholder.go` `skinPalettes`, and the `site/` landing page. Flourishes
  leak into unconditional CSS: the Brutalist `reel` counter is not skin-gated, and `RelatedShelf`
  borrows `.video-grid` only to reset it. The Appearance cards need a `.skin-card` selector fence
  so a card can show a skin other than the page's.
- **Tests rot.** The geometry harness's broadcast/brutalist cells have errored ever since ADR-102
  (HOLODEX-460), and nobody noticed, because nobody uses those skins.

Variety in *colour* is still wanted. ADR-102 D4's custom palette already covers that: it recolours
a Cinémathèque base with five primaries, and every other token is derived from them. It's already
restricted to `base: cinematheque` (`internal/theme.allowedBases`), so it doesn't depend on the
skins being retired.

## Decision

### D1 — Cinémathèque is the only shipped skin

Broadcast and Brutalist are removed, not frozen. The following all go:
- their `[data-theme]` token blocks and every flourish gated on them in `web/src/app.css`: scanlines
  and CRT vignette, the scanline wash on `.video-frame` and `.portrait-frame`, `.skin-title` uppercase
  and the `▮` caret
- the Brutalist `reel` counter, including its unconditional `counter-reset`/`counter-increment`
- the fonts only they load (VT323, Share Tech Mono, Spline Sans Mono) and their `@fontsource` deps
- their ids in `THEMES` / `THEME_LABELS` / `ShippedTheme`
- their `placeholder.go` palettes
- their geometry-harness cells
- their landing-page and README screenshots

*Why remove rather than freeze:* a frozen skin still breaks whenever a token or hook class changes,
and someone still has to decide whether that matters. Only deleting them ends that cost.

### D2 — The token layer and the hook classes stay

"One skin" means one set of token values, **not** hardcoded colours. Components still use only the
semantic utilities (ADR-021 §2–§3), because the custom palette recolours through those tokens.
`data-theme="cinematheque"` stays on `<html>` as the base the custom palette sits on. The hook
classes (`.app-atmosphere`, `.video-frame`, `.video-grid`, `.skin-title`) stay and carry only
Cinémathèque's look. `@keyframes reel-rise` is the shared grid animation, not the Brutalist
counter, and it stays.

### D3 — The instance-skin domain becomes `{cinematheque, custom}`; stored retired ids degrade, no migration

- `shippedThemes` shrinks to `cinematheque`. `PUT /admin/theme` refuses `broadcast` and `brutalist`
  with 400, like any unknown id.
- A `settings['theme.active']` row that still holds a retired id is reported as `cinematheque`. That
  is `themePayload`'s existing unknown-id fallback. The row is left alone, and the next owner
  selection overwrites it. **No migration**, because `settings` has no CHECK constraint and
  validation is Go-only (ADR-102 D2).
- A stale `holodex-theme-cache` paint cache value degrades the same way (`baseOf` falls back to
  the default).

### D4 — The per-skin image parameter collapses

`?skin=` on person-image URLs only existed to pick a per-skin placeholder palette. With one skin
the parameter and `skinPalettes` collapse to a single Cinémathèque palette, and the duplicate
`defaultSkin` constant (`cmd/holodex/main.go`) merges into `api.ThemeDefault`. The server ignores
unknown query parameters, so cached URLs that still carry `?skin=` keep working. Whether a custom
palette should tint placeholders is **out of scope**, just as it is today.

### D5 — The Appearance tab stays, and shrinks

The Appearance tab still hosts the custom-palette selection and the studio-halo toggle
(HOLODEX-463). Its radio group becomes Cinémathèque, plus Custom when a palette is configured.
The `.skin-card` fence goes, because no card ever renders a skin other than the page's. The layout
is the **design gate's** decision, not this ADR's.

### D6 — The QA obligation is Cinémathèque plus the custom palette

This replaces ADR-021's "QA must verify all three skins". Every UI change is verified in
Cinémathèque, and also under the custom palette when one is configured. Adding a skin again
requires a new ADR that supersedes this one. Agent instructions must not offer a skin choice as a
design option.

## Options Considered

| Option | Verdict |
|---|---|
| **Freeze:** keep them selectable, unsupported, never QA'd | Rejected. They'd rot visibly for any instance that picked them, and they keep the mirrors and fences alive. |
| **Instructions only:** stop QA'ing, keep ADR-021 as is | Rejected. It contradicts an Accepted ADR, so the next design or architecture session would bring them back. That's how this drift happened. |
| **Retire the custom palette too** | Rejected by the owner. It's the only colour-variety surface left, and it already has a Cinémathèque base. |
| **Retire, keep tokens + palette** (chosen) | One look, less QA, one mirror fewer, and owner recolouring survives. |

## Consequences

- Every UI change needs one QA pass instead of three. The bundle drops three font families.
- HOLODEX-460 is resolved by deletion. `web/geometry` loses its skin axis, so the matrix becomes
  one skin × viewport widths.
- An instance currently set to Broadcast or Brutalist silently turns into Cinémathèque on upgrade.
  That's acceptable for a single-owner, self-hosted app, and the release note should say so.
- **History isn't rewritten.** Older ADRs, specs, plans, handoffs and their mockups still mention
  three skins, and they stay as records. Forward-looking instructions have been corrected:
  CLAUDE.md, `.claude/rules/frontend-theming.md`, `docs/design/theming.md`,
  `docs/reference/workflow-idea-to-merge.md`, and the F41 spec's QA criteria.
  `graphify-out/` gets regenerated, not edited.

## Action Items (HOLODEX-476)

1. [x] Correct the agent instructions and forward-looking docs (this ADR's push).
2. [ ] Spec: amend F67 (`docs/specs/instance-skin.md`) to the `{cinematheque, custom}` domain.
3. [ ] Design: shrink the Appearance tab (D5).
4. [ ] Backend: `shippedThemes`, `skinPalettes`/`?skin=` collapse, `defaultSkin` merge, tests
   (`theme_test.go`, `personimage_test.go`, `settings_test.go` fixture values).
5. [ ] Frontend: `app.css` blocks and flourishes, the `reel` counter and `RelatedShelf` rationale,
   fonts and deps, `theme.svelte.ts` / `types.ts`, the Appearance page, `?skin=` call sites,
   `theme.test.ts` / `api.test.ts`, and the component CLAUDE.md lines that describe removed code.
6. [ ] Geometry harness: drop `SKINS` and `--skin`, fix the fixtures, close HOLODEX-460.
7. [ ] `site/`, `README.md`, `docs/reference/configuration.md` §Appearance and the screenshots: drop the skin switcher and the retired-skin images.
8. [ ] Testing strategy: rewrite §12's "three skins × widths" matrix.
