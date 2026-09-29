---
paths:
  - "web/**/*.svelte"
  - "web/**/*.css"
  - "web/**/*.ts"
---

# Frontend theming (component discipline)

The UI is built on semantic design tokens with **one look, Cinémathèque** (see
[ADR-021](../../docs/architecture/ADR-021-frontend-theming-and-skins.md) as amended by
[ADR-115](../../docs/architecture/ADR-115-cinematheque-only-skin.md), and
[`docs/design/theming.md`](../../docs/design/theming.md)). Broadcast, Brutalist, the custom
palette and the Appearance tab are **retired**. Don't QA, mock up, propose or extend another skin,
palette or theme switch, and don't present a theme choice as a design option. Tokens stay
mandatory anyway: one source per colour keeps a look change to a one-file edit. Two rules are
load-bearing:

- **Tokens only — never hardcode styling.** Components must use the semantic Tailwind
  utilities backed by CSS variables (`bg-bg`, `bg-surface`, `text-ink`, `text-muted`,
  `border-rule`, `bg-accent`/`text-accent`, `text-accent-ink`, `font-display`/`font-ui`,
  `rounded-theme`, `text-warn`/`border-warn`, `max-w-stage`). **Never** a literal palette or value in a
  component: no `zinc-*`, `sky-*`, hex colors, named font families, or fixed `rounded-lg`/`px`
  radii. A hardcoded value is a theming bug — it forks the colour from its token. Use `--warn`
  (`text-warn`/`border-warn`) for error/attention states — deliberately distinct from
  `--accent`, which doubles as the active/primary color. Flourishes belong in
  `app.css`, attached to the shared hook classes
  (`.app-atmosphere`, `.video-frame`, `.video-grid`, `.skin-title`) — not as per-component
  markup. Layout-mode rules attach to `.video-grid[data-layout='...']` (operator-set
  via `holodex.yaml: card_layout`; not a theme — never gate anything on `[data-theme]`).
  Quick check over components: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` should be empty (raw hex values live only in `app.css` token blocks; `rounded-full` pills are an intentional shape).
  Page width is a token too: a page-level cap is `max-w-stage`, never `max-w-[2600px]`; the two-zone detail shell is `stage-grid` and label/value lists are `field-grid` (both in `app.css`), never a repeated `grid-cols-[…]` string. `rg 'max-w-\[' web/src --glob '*.svelte'` should surface only per-element limits — truncation caps and the Films/People `max-w-[50%]` split — never a page wrapper.
- **Reuse the shared button treatments; never dim a `text-muted` label.** Non-primary
  actions use `.btn-accent` (outlined accent — the affirmative action), `.btn-ghost`
  (bordered neutral — an immediate resolve), or `.btn-quiet` (borderless neutral — a UI-only
  toggle) from `app.css`; solid `bg-accent` stays reserved for a page's one primary action.
  Don't fork a per-file variant. **`disabled:opacity-60` on `text-muted` is a contrast bug** —
  it lands at ~2.9:1 against `--surface`. Withdraw the affordance
  instead (drop the border, demote accent to neutral) and leave the label at full contrast;
  the `.btn-*` classes already do this. Quick check:
  `rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'` should be empty.
- **An owner "add / set" affordance for a missing value is the text CTA:**
  `btn-quiet px-3 py-1.5 text-sm`, labelled `+ Add <noun>` (or `+ Set <noun>` for a single
  scalar), and never accent. This holds whatever the value will look like once filled (prose, a pill,
  a chip, a card), because copying the nearest idiom is how drift starts. HOLODEX-471 copied
  "+ Set part"'s dashed pill for "+ Add overview", and HOLODEX-490 unified them. **The only
  exception is the ghost slot**: an empty *image* slot drawn at the image's own footprint
  (`border-dashed border-rule`, muted, accent on hover). Examples are the poster tile among real
  poster tiles in the Films and People grids, the Person banner band and `EntityImageSlot`. An image
  has a fixed footprint worth reserving, but text doesn't. An empty poster *grid* drops back to the
  text CTA. Don't invent a dashed pill, dashed chip or accent link for a missing value. (Dashed
  *status* encodings, such as pending chips or "not in your library" cast chips, aren't add
  affordances and aren't covered.) Terms: *text CTA* and *ghost slot* in
  `docs/reference/ui-vocabulary.md`. Quick check: every hit of
  `rg '\+ (Add|Set) ' web/src --glob '*.svelte'` is a `btn-quiet` text CTA or an image ghost slot.
  One known holdout remains: the film page's `+ Set edition` scene-row pill. It's a follow-up, not a
  precedent.
- **QA Cinémathèque — and only Cinémathèque.** When verifying any UI change, render and eyeball
  **Cinémathèque**. Check that the accent reads on its background, that decorative elements don't
  collide, that fonts load offline, and that the loading/empty/error/grid states are all themed.
  There is nothing to switch: the tokens sit in `:root` and nothing sets `data-theme`.