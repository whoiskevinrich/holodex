# Design Handoff — Holodex Theming & Skins

**Status**: Implemented (Phase 1)
**Date**: 2026-06-10
**Architecture**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)

Holodex ships **three switchable skins**. The default is **Cinémathèque**. All three are
dark. The skin is **instance identity** ([ADR-102](../architecture/ADR-102-instance-skin-and-settings-store.md)):
the owner picks it on **Owner › Appearance**, it persists server-side and arrives with
`/capabilities.theme`, and the SPA applies it as `data-theme` on `<html>` for every viewer —
there is no per-browser preference (only a paint cache, `holodex-theme-cache`, that the server
value always overwrites). A custom palette (`theme.custom` in `holodex.yaml`) rides a base
skin's `data-theme` plus five inline custom properties on `<html>`, and `data-palette="custom"`
switches on the derivation block in `app.css` that computes every pair-partner token
(`surface`, `surface-2`, `rule`, `accent-ink`, `warn-ink`, `logo-plate`, `logo-plate-ink`) from
them with oklab `color-mix()`. `internal/theme` mirrors that block in Go for the boot-time
contrast WARN, and `TestDeriveMatchesCinematheque` gates both: Cinémathèque re-expressed as its
five primaries must land within ΔE\*ab ≤ 2 of the hand-tuned block. **Change a percentage in one
place, change it in the other** — the test is what notices when you forget.

## Design tokens (the contract)

Every surface is built from these semantic tokens — components reference them only via the
mapped Tailwind utilities, never a literal palette.

| Token | Tailwind utility | Meaning |
|---|---|---|
| `--bg` | `bg-bg` | page background |
| `--surface` | `bg-surface` | panels, inputs, cards |
| `--surface-2` | `bg-surface-2` | insets, chips, thumbnail wells |
| `--ink` | `text-ink` | primary text |
| `--muted` | `text-muted` | secondary text, labels |
| `--rule` | `border-rule` | borders, dividers |
| `--accent` | `bg-accent` / `text-accent` | accent fills & emphasis |
| `--accent-ink` | `text-accent-ink` | text on an accent fill |
| `--warn` | `text-warn` / `border-warn` | error / attention states (deliberately distinct from `--accent`, which doubles as the active/primary color) |
| `--logo-plate` | `bg-logo-plate` | light neutral backing for arbitrary brand logos (e.g. studio logos, F38) — most are drawn for a light background, so a dark skin surface would hide black/white-on-transparent marks |
| `--font-display` | `font-display` | titles / wordmark (`.skin-title`) |
| `--font-ui` | `font-ui` | body & UI (default on `<body>`) |
| `--radius` | `rounded-theme` | corner radius (0 for the mono skins) |
| `--container-stage` | `max-w-stage` | reading-width ceiling for detail and owner pages (2600px). The one **constant** here — identical on every skin, so it lives in a plain `@theme` block rather than the `@theme inline` one. Never write `max-w-[2600px]`. Browse grids are deliberately exempt: a grid can always add another column, so it stays edge-to-edge (HOLODEX-331). |

## Skins

| Skin | Display / UI font | Background | Accent | Signature |
|---|---|---|---|---|
| **Cinémathèque** (default) | Fraunces / Archivo | `#0c0a09` warm-black + film grain + vignette | `#e8a33d` ember | letterbox bars on cards |
| **Broadcast** | VT323 / Share Tech Mono | `#060814` + scanlines | `#36e0d0` cyan (`#ffb23e` amber) | scanline wash, uppercase, `▮` caret |
| **Brutalist** | Spline Sans Mono (both) | `#0a0a0a` | `#d6ff3f` acid-lime | hairline grid, zero radii, `01/02` index counters |

## Shared hook classes (skins own the look)

- `.app-atmosphere` (on `<body>`) — grain / scanline / vignette overlay.
- `.video-frame` — the 16:9 thumbnail well; letterbox bars (Cinémathèque), scanline wash
  (Broadcast), and the CSS-`counter` index number (Brutalist) attach here.
- `.video-grid` — grid wrapper; provides the `counter-reset` and the staggered load
  animation (disabled under `prefers-reduced-motion`).
- `.skin-title` — display-face headings; applies per-skin casing and the Broadcast caret.

## Shared button treatments

Three roles for **non-primary** actions, so a row of controls reads as a hierarchy rather
than a wall of identical links. Solid `bg-accent` is deliberately not among them — it stays
reserved for a page's one primary action.

- `.btn-accent` — outlined accent; the affirmative action in a row (Stage, Review, Merge).
- `.btn-ghost` — bordered neutral; an immediate, row-clearing resolve (Dismiss, Revert).
- `.btn-quiet` — borderless neutral; a UI-only toggle with no side effect (Cancel, Undo).

Each owns colour, border, radius and disabled semantics only; call sites keep their own
sizing utilities, which still win (Tailwind orders `utilities` after `components`).

**Disabled never dims the label with `opacity`.** On `text-muted` a blanket `opacity-60`
falls to 2.4:1 (Broadcast) / 2.7:1 (Brutalist) / 2.9:1 (Cinémathèque) against `--surface`.
Instead the *affordance* is withdrawn — the border drops, or the accent demotes to neutral —
so the label stays at full token contrast (4.7:1 or better in every skin).

Do not add a `transition` on `color`/`border-color` to these: an Appearance-tab pick swaps the
underlying tokens at runtime, which makes the swap animate and can leave the control stuck
on the previous skin's colour.

## Adding a skin

1. Add one `[data-theme="newskin"]` block in `web/src/app.css` setting every token.
2. (Optional) add bespoke decorative CSS gated by that selector on the shared hook classes.
3. Add the id + label to `THEMES` / `THEME_LABELS` in `web/src/lib/theme.svelte.ts`.
4. No component changes. **QA the new skin and re-QA the existing three.**

## QA checklist (every UI change)

Render and eyeball **all three skins** (switch on **Owner › Appearance**), plus the custom
palette when one is configured, not just the default — regressions frequently appear in only
one skin. The Appearance cards themselves render each skin's flourishes inside a `.skin-card`
fence (`app.css`, the two-branch `.video-frame` selectors) — a new `.video-frame` flourish must
keep both branches or it will bleed across cards. Confirm: fonts load (offline),
the accent reads on `--accent` fills, no decorative-element collisions (badges vs.
counters), and the grid/empty/loading/error states all themed.
