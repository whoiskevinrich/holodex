# ADR-025: Tailwind CSS v4 — CSS-First Config via `@theme inline`

**Status**: Accepted
**Date**: 2026-06-12
**Deciders**: Project owner
**Relates to**: [ADR-021](ADR-021-frontend-theming-and-skins.md) (theming & skins — **supersedes §2**, the JS-config utility mapping), [ADR-020](ADR-020-frontend-embed-and-build.md) (frontend build), [ADR-002](ADR-002-frontend-framework.md) (SvelteKit SPA)

---

## Context

The frontend shipped on **Tailwind CSS v3** with a JavaScript config (`tailwind.config.ts`)
and a PostCSS pipeline (`postcss.config.js` running `tailwindcss` + `autoprefixer`).
Tailwind v4 reworked the toolchain: configuration is **CSS-first** (a `@theme` block in the
stylesheet replaces the JS config), a dedicated Vite plugin replaces the PostCSS plugin, and
vendor prefixing / nesting are handled internally by Lightning CSS — so `autoprefixer` and
`postcss` are no longer needed.

Holodex's theming (ADR-021) maps every semantic utility (`bg-bg`, `text-ink`,
`text-accent`, `font-display`, `rounded-theme`, …) to a CSS variable whose value is swapped
at runtime by a `[data-theme]` attribute on `<html>`. This runtime-swap requirement is the
crux of how the migration had to be done — see the decision below.

## Decision

Upgrade to **Tailwind v4** (`tailwindcss` + `@tailwindcss/vite`, both `^4.3.0`) with a
CSS-first config, and retire the JS/PostCSS surface.

### 1. Vite plugin replaces the PostCSS pipeline

`@tailwindcss/vite` is added to `web/vite.config.ts` (before `sveltekit()`).
`web/postcss.config.js`, `autoprefixer`, and `postcss` are **removed** — prefixing is now
done by Lightning CSS inside the plugin.

### 2. `tailwind.config.ts` → `@theme inline` in `app.css`

The token→utility map moves out of `tailwind.config.ts` (deleted) into a `@theme` block at
the top of `web/src/app.css`:

```css
@import 'tailwindcss';

@theme inline {
	--color-bg: var(--bg);
	--color-surface: var(--surface);
	/* … */
	--font-display: var(--font-display);
	--font-ui: var(--font-ui);
	--radius-theme: var(--radius);
}
```

**`inline` is load-bearing, not cosmetic.** Without it, Tailwind resolves each theme
variable **once** (to whatever `var(--bg)` evaluates to in `:root`, which is empty — the
real values live under `[data-theme]`) and emits utilities that point at that frozen
snapshot. With `@theme inline`, the generated utilities emit the `var(--bg)` reference
**directly** (`.bg-bg { background-color: var(--bg) }`), so they pick up the live per-skin
value whenever the `[data-theme]` attribute changes. This is exactly the multi-skin
runtime-swap case `@theme inline` exists for. Verified: a `rounded-theme` element computes
`2px` under Cinémathèque and `0px` under the two mono skins; `--accent` resolves to the
distinct per-skin value in each.

The `@layer base` / `@layer components` blocks (body defaults, decorative skin flourishes)
are unchanged — `@layer` is standard CSS that v4 still honors.

### 3. Preserve v4 Preflight / scale changes

Two v4 defaults are explicitly compensated so the UI is visually unchanged:

- **Button cursor.** v4 Preflight sets `button { cursor: default }`. A base rule restores
  `cursor: pointer` for `button:not(:disabled), [role="button"]:not(:disabled)`.
- **Shadow scale rename.** v4 shifted the shadow scale (`shadow-sm` now equals v3's
  `shadow`). The one `shadow-sm` in the codebase (the resolution badge) became `shadow-xs`
  to keep its original weight.

No other v4 change required a code edit: the codebase already specifies an explicit color on
every `border`/`ring`/`divide`, uses no renamed `rounded-*`/`shadow` scale tokens beyond the
above, and has no Svelte `<style>` blocks needing PostCSS. One v4 change is benign here but
worth recording: `space-y-*`/`space-x-*` swapped their internal selector (from
`> :not([hidden]) ~ :not([hidden])` to `> :not(:last-child)`). The ~8 `space-y-*` call sites
are simple vertical stacks where the two selectors are equivalent, so no markup changed —
confirmed during three-skin QA.

## Consequences

- **Smaller config surface:** two config files deleted (`tailwind.config.ts`,
  `postcss.config.js`) and two dev-deps removed (`autoprefixer`, `postcss`); theming config
  now lives next to the tokens it maps, in `app.css`.
- **Faster builds** via v4's Oxide engine + Lightning CSS (no separate PostCSS pass).
- **The theming contract is unchanged.** The semantic token vocabulary, the `[data-theme]`
  swap, the component discipline ("tokens only"), and the CI guard grep (banning
  `zinc-*`/`rounded-lg`/… in `*.svelte`) all carry over verbatim. ADR-021 stands; only its
  §2 *mechanism* ("`tailwind.config.ts` extends colors…") is superseded by this ADR.
- **Verification stays the same:** `npm run check`, `npm run test`, `npm run build`, and
  three-skin visual/computed-style QA. The Docker build is unaffected (`npm run build`
  auto-detects the CSS-first config).
- **`@theme inline` is now a tripwire:** dropping the `inline` keyword would silently break
  every skin but the one resolvable in `:root`. Captured here and in an `app.css` comment.
