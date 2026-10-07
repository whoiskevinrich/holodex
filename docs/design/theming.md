# Design Handoff — Holodex Theming & Skins

**Status**: Implemented (Phase 1)
**Date**: 2026-06-10
**Architecture**: [ADR-021](../architecture/archive/ADR-021-frontend-theming-and-skins.md), amended by
[ADR-115](../architecture/archive/ADR-115-cinematheque-only-skin.md)

Holodex has **one look: Cinémathèque** (dark). There is nothing to choose.
[ADR-115](../architecture/archive/ADR-115-cinematheque-only-skin.md) (HOLODEX-476) retired Broadcast,
Brutalist, the custom palette (`theme.custom`), the instance-skin setting and the **Owner ›
Appearance** tab. The tokens live in one `:root` block in `web/src/app.css`, and nothing sets
`data-theme`. The one mirror left is `internal/personimage/placeholder.go`, which copies four
tokens for the standalone placeholder SVG — change them there too.

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
| `--radius` | `rounded-theme` | corner radius |
| `--container-stage` | `max-w-stage` | reading-width ceiling for detail and owner pages (2600px). The one **constant** here — identical under any palette, so it lives in a plain `@theme` block rather than the `@theme inline` one. Never write `max-w-[2600px]`. Browse grids are deliberately exempt: a grid can always add another column, so it stays edge-to-edge (HOLODEX-331). |

## The skin

| Skin | Display / UI font | Background | Accent | Signature |
|---|---|---|---|---|
| **Cinémathèque** | Fraunces / Archivo | `#0c0a09` warm-black + film grain + vignette | `#e8a33d` ember | letterbox bars on cards |

Broadcast (VT323 / Share Tech Mono, cyan, scanlines) and Brutalist (Spline Sans Mono,
acid-lime, zero radii) were **retired** by ADR-115 and removed in HOLODEX-476, along with their
fonts. They're named here only so older docs that mention them make sense.

## Shared hook classes (the skin owns the look)

- `.app-atmosphere` (on `<body>`) — grain / vignette overlay.
- `.video-frame` — the 16:9 thumbnail well; the letterbox bars attach here.
- `.video-grid` — grid wrapper; the staggered load animation (disabled under
  `prefers-reduced-motion`).
- `.skin-title` — display-face headings.

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
falls to 2.9:1 against `--surface` in Cinémathèque.
Instead the *affordance* is withdrawn — the border drops, or the accent demotes to neutral —
so the label stays at full token contrast (4.7:1 or better).


## Adding a skin

Don't. ADR-115 makes Cinémathèque the only look, with no palette and no switch. To change the
look, edit its token block. A second skin, a palette or a theme switch needs a new ADR that
supersedes ADR-115.

## QA checklist (every UI change)

Render and eyeball **Cinémathèque** — the only look. Confirm: fonts load (offline), the accent
reads on `--accent` fills, no decorative-element collisions, and the grid/empty/loading/error
states are all themed.
