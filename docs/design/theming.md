# Design Handoff — Holodex Theming & Skins

**Status**: Implemented (Phase 1)
**Date**: 2026-06-10
**Architecture**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md), amended by
[ADR-115](../architecture/ADR-115-cinematheque-only-skin.md)

Holodex has **one skin: Cinémathèque** (dark). [ADR-115](../architecture/ADR-115-cinematheque-only-skin.md)
retires Broadcast and Brutalist (HOLODEX-476). Until that epic removes them from the code they
still render on the Appearance tab, but they are unsupported: don't QA, extend or propose them.
The owner can recolour Cinémathèque with a custom palette. The active choice is **instance
identity** ([ADR-102](../architecture/ADR-102-instance-skin-and-settings-store.md)):
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
| `--radius` | `rounded-theme` | corner radius |
| `--container-stage` | `max-w-stage` | reading-width ceiling for detail and owner pages (2600px). The one **constant** here — identical under any palette, so it lives in a plain `@theme` block rather than the `@theme inline` one. Never write `max-w-[2600px]`. Browse grids are deliberately exempt: a grid can always add another column, so it stays edge-to-edge (HOLODEX-331). |

## The skin

| Skin | Display / UI font | Background | Accent | Signature |
|---|---|---|---|---|
| **Cinémathèque** | Fraunces / Archivo | `#0c0a09` warm-black + film grain + vignette | `#e8a33d` ember | letterbox bars on cards |

Broadcast (VT323 / Share Tech Mono, cyan, scanlines) and Brutalist (Spline Sans Mono,
acid-lime, zero radii) are **retired** by ADR-115. They're recorded here only so you recognise
leftover CSS while HOLODEX-476 removes it.

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

Do not add a `transition` on `color`/`border-color` to these: an Appearance-tab pick swaps the
underlying tokens at runtime, which makes the swap animate and can leave the control stuck
on the previous palette's colour.

## Adding a skin

Don't. ADR-115 makes Cinémathèque the only skin; owner colour variation goes through the custom
palette (ADR-102 D4). A second skin needs a new ADR that supersedes ADR-115.

## QA checklist (every UI change)

Render and eyeball **Cinémathèque**, plus the custom palette when one is configured (switch
on **Owner › Appearance**). Don't QA Broadcast or Brutalist, and don't fix regressions that
show only in them. Until HOLODEX-476 lands, the Appearance cards render flourishes inside a
`.skin-card` fence (`app.css`, the two-branch `.video-frame` selectors), so a new `.video-frame`
flourish must keep both branches. Confirm: fonts load (offline), the accent reads on `--accent`
fills, no decorative-element collisions, and the grid/empty/loading/error states are all themed.
