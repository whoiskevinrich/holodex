# ADR-021: Frontend Theming — Semantic Design Tokens & Multi-Skin System

**Status**: Accepted
**Date**: 2026-06-10
**Deciders**: Project owner
**Relates to**: [ADR-002](ADR-002-frontend-framework.md) (SvelteKit SPA), [ADR-007](ADR-007-docker-structure.md) / [ADR-020](ADR-020-frontend-embed-and-build.md) (offline single-image delivery), F8 (dark mode)

---

## Context

The Phase 1 UI shipped functional but visually generic (default system font, monochrome
`zinc` palette, no atmosphere). We want a distinctive, intentional aesthetic — and the
project owner chose to ship **three switchable "skins"** rather than one look:

- **Cinémathèque** (default) — refined film-archive editorial; Fraunces + Archivo; warm
  near-black + film grain; ember accent; letterboxed cards.
- **Broadcast** — retro-futurist CRT; VT323 + Share Tech Mono; scanlines; cyan/amber.
- **Brutalist** — raw catalog; Spline Sans Mono; hairline grid; zero radii; acid-lime.

Two constraints shaped the design:
1. **Offline-only** — the app is a self-hosted Docker image (ADR-007/020); it cannot pull
   web fonts at runtime.
2. The skins must not fork the component tree — three parallel sets of pages would be
   unmaintainable.

## Decision

### 1. Semantic design tokens as CSS custom properties

All color, font, and shape values are CSS variables defined **once per skin** in
`web/src/app.css` under `[data-theme="cinematheque|broadcast|brutalist"]`. The token
vocabulary is intentionally small and semantic (not palette-named):

`--bg`, `--surface`, `--surface-2`, `--ink`, `--muted`, `--rule`, `--accent`,
`--accent-ink`, `--font-display`, `--font-ui`, `--radius`.

### 2. Tailwind maps utilities → tokens

> **Superseded in part by [ADR-025](ADR-025-tailwind-v4-css-first.md).** The *mechanism*
> below moved from `tailwind.config.ts` to a CSS-first `@theme inline` block in `app.css`
> when the project upgraded to Tailwind v4. The token vocabulary, the semantic utilities,
> and the "never name a palette" discipline are unchanged.

`tailwind.config.ts` extends `colors`/`fontFamily`/`borderRadius` to point at the
variables, so components use semantic utilities (`bg-bg`, `text-ink`, `text-accent`,
`font-display`, `rounded-theme`) and the skin swap is free. **Components never name a
palette** (`zinc-*`, hex, etc.) — doing so is a theming bug (it won't react to the skin).

### 3. All visual difference lives in CSS — components are skin-agnostic

Decorative flourishes are pure CSS gated by `[data-theme]`, not per-component markup:
- film grain / scanlines / vignette → `.app-atmosphere::after`
- letterbox bars, scanline wash, brutalist index numbers → `.video-frame::before/::after`
  (the index uses a CSS `counter`, not rendered markup)
- display-face casing and the broadcast caret → `.skin-title`
- staggered grid load → `.video-grid > *` `animation-delay` (respects
  `prefers-reduced-motion`)

A component opts into a flourish by carrying a stable hook class (`.video-frame`,
`.skin-title`, `.video-grid`); the skin owns what that hook looks like.

### 4. Fonts bundled offline via `@fontsource`

Font families are npm dependencies (`@fontsource-variable/*`, `@fontsource/*`) imported in
`app.css` and compiled into the bundle by Vite — no runtime CDN, consistent with the
offline single-image model.

### 5. Skin selection persists; supersedes the dark/light toggle

`web/src/lib/theme.svelte.ts` holds the active skin, applies it as `data-theme` on
`<html>`, and persists to the existing `holodex-theme` localStorage key. The header skin
picker replaces the former dark/light toggle. This generalizes F8.2 (persisted preference)
from two modes to named skins; each skin is its own dark palette.

## Consequences

- Adding a skin = one `[data-theme]` token block (+ any bespoke decorative CSS). No
  component changes.
- **Discipline is load-bearing:** a hardcoded color/font/radius in a component silently
  breaks theming. Enforced as a working agreement (see `.claude/CLAUDE.md`).
- **QA must verify all three skins**, not just the default — a regression often shows in
  only one skin (e.g. a badge/counter collision). Also a working agreement.
- Per-skin **light variants** are out of scope for now (each skin is dark-only); a future
  ADR can add a light axis if wanted.
- Bundle carries five font families (~a few hundred KB of woff2). Acceptable for a
  self-hosted app; could be lazy-loaded per skin later if it matters.
- First paint uses the default skin (`data-theme="cinematheque"` in `app.html`); the saved
  skin is applied on mount. No FOUC for default users.
