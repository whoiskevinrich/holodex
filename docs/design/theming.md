# Design Handoff — Holodex Theming & Skins

**Status**: Implemented (Phase 1)
**Date**: 2026-06-10
**Architecture**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)

Holodex ships **three switchable skins**. The default is **Cinémathèque**. All three are
dark; skin selection persists (localStorage `holodex-theme`) and is applied as
`data-theme` on `<html>`.

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

## Adding a skin

1. Add one `[data-theme="newskin"]` block in `web/src/app.css` setting every token.
2. (Optional) add bespoke decorative CSS gated by that selector on the shared hook classes.
3. Add the id + label to `THEMES` / `THEME_LABELS` in `web/src/lib/theme.svelte.ts`.
4. No component changes. **QA the new skin and re-QA the existing three.**

## QA checklist (every UI change)

Render and eyeball **all three skins** (switch via the header picker), not just the
default — regressions frequently appear in only one skin. Confirm: fonts load (offline),
the accent reads on `--accent` fills, no decorative-element collisions (badges vs.
counters), and the grid/empty/loading/error states all themed.
