---
paths:
  - "web/**/*.svelte"
  - "web/**/*.css"
  - "web/**/*.ts"
---

# Frontend theming (component discipline)

The UI is built on semantic design tokens with three switchable skins (see
[ADR-021](../../docs/architecture/ADR-021-frontend-theming-and-skins.md) and
[`docs/design/theming.md`](../../docs/design/theming.md)). Two rules are load-bearing:

- **Tokens only — never hardcode styling.** Components must use the semantic Tailwind
  utilities backed by CSS variables (`bg-bg`, `bg-surface`, `text-ink`, `text-muted`,
  `border-rule`, `bg-accent`/`text-accent`, `text-accent-ink`, `font-display`/`font-ui`,
  `rounded-theme`, `text-warn`/`border-warn`). **Never** a literal palette or value in a
  component: no `zinc-*`, `sky-*`, hex colors, named font families, or fixed `rounded-lg`/`px`
  radii. A hardcoded value is a theming bug — it won't react to the skin. Use `--warn`
  (`text-warn`/`border-warn`) for error/attention states — deliberately distinct from
  `--accent`, which doubles as the active/primary color. Skin-specific flourishes belong in
  `app.css` gated by `[data-theme]`, attached to the shared hook classes
  (`.app-atmosphere`, `.video-frame`, `.video-grid`, `.skin-title`) — not as per-component
  markup. Layout-mode rules attach to `.video-grid[data-layout='...']` (operator-set
  via `holodex.yaml: card_layout`; not a skin — do not gate with `[data-theme]`).
  Quick check over components: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` should be empty (raw hex values live only in `app.css` token blocks; `rounded-full` pills are an intentional shape).
- **Reuse the shared button treatments; never dim a `text-muted` label.** Non-primary
  actions use `.btn-accent` (outlined accent — the affirmative action), `.btn-ghost`
  (bordered neutral — an immediate resolve), or `.btn-quiet` (borderless neutral — a UI-only
  toggle) from `app.css`; solid `bg-accent` stays reserved for a page's one primary action.
  Don't fork a per-file variant. **`disabled:opacity-60` on `text-muted` is a contrast bug** —
  it lands at 2.4–2.9:1 against `--surface` depending on skin. Withdraw the affordance
  instead (drop the border, demote accent to neutral) and leave the label at full contrast;
  the `.btn-*` classes already do this. Quick check:
  `rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'` should be empty.
- **QA all three skins.** When verifying any UI change, render and eyeball **Cinémathèque,
  Broadcast, and Brutalist** (switch via the header picker), not just the default —
  regressions routinely appear in only one skin (e.g. a badge/counter collision, an accent
  that doesn't read on its background). Confirm fonts load offline and the
  loading/empty/error/grid states are all themed.