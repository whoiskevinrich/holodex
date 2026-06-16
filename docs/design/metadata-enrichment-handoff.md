# Design Handoff: Metadata Enrichment UI for People (F22)

**Spec**: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) · **ADR**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

Three surfaces, all on the **Person detail page** (`/people/[id]`):

1. **Enrich action** — an owner-only "Enrich from TMDB" control that opens the picker.
2. **Disambiguation picker** — a modal listbox of provider candidates (label · disambiguation · confidence); the owner picks one to confirm identity, then core fetches and stores fields.
3. **Provenance badges** — every resolved field on the page is labeled with where its value came from ("from TMDB" / "from file"), so machine-added data is never mistaken for file-sourced truth.

All three are **owner-gated** (reuse the ADR-030 capability flag exactly as `/status` does) and **on-demand only** — nothing fetches without the owner clicking.

### Where it sits in the page

The current `/people/[id]` delegates wholly to `EntityVideos` (back-link, title, count, grid). Enrichment adds a **People-detail panel** between the title block and the video grid. This means `EntityVideos` either gains an optional slot/snippet (`{#snippet detail()}`) or the page composes the panel itself above `EntityVideos`. **Recommended:** add an optional `detail` snippet prop to `EntityVideos` so the tag page stays unchanged and the person page fills the slot — keeps the shared component shared.

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.isOwner` / `activity.needToken` (`activity.svelte.ts`) | Same predicates `/status` uses. Controls render only when `isOwner`. |
| Token-locked state | the `/status` unlock `<form>` (lines 108–120) | If `needToken`, show the same "requires admin token" unlock affordance before any control. |
| Confirm-before-act + toast | `/status` `confirmingRescan` + `showToast` pattern | Picker confirmation and post-enrich result reuse this exact flow. |
| Combobox/listbox a11y | the search-history dropdown (PR #21 on `main`) | `role=listbox` + `aria-activedescendant`, ↑/↓/Enter/Esc — the picker mirrors it. |
| Field display | the media "Details" `<dl>` (`media/[id]` lines 143–155) | Person enrichment fields render in the same `<dl>` shape; badges attach per row. |
| Async wrapper | `AsyncState.svelte` | Picker search results and field load use it. |

---

## Layout

| Region | Layout |
|--------|--------|
| Person panel | Full-width `section`, `space-y-4`, sits above the grid inside the `max-w` body. Header row: field `<dl>` on the left, the "Enrich from TMDB" button top-right (`flex items-start justify-between`). On mobile (<640px) the button wraps below the heading (`flex-wrap`). |
| Picker | Centered modal dialog, `role="dialog" aria-modal="true"`, max-width `max-w-lg`, `w-full`, capped height `max-h-[80vh]` with the candidate list scrolling internally. Backdrop overlay dims the page. |
| Provenance badge | Inline, trailing each field value (`<dd>`), `ml-2`, baseline-aligned. |

No new breakpoints — use the existing `sm:` (640) grid the Details block already uses.

---

## Design Tokens Used

Tokens only — every value below is a semantic Tailwind utility backed by a CSS variable (`app.css`). **No `zinc-*`/`sky-*`/hex/named-font/fixed-radius in markup.**

| Token | Usage |
|-------|-------|
| `bg-bg` | Picker backdrop sits over the page; backdrop itself uses `bg-bg/70` (token color + opacity) |
| `bg-surface` | Picker panel, field `<dl>` card, search input |
| `bg-surface-2` | **File-sourced** provenance chip; candidate row hover/selected |
| `text-ink` | Primary text — names, values, candidate labels |
| `text-muted` | Secondary — disambiguation line, confidence, field labels, empty/help text |
| `border-rule` | Picker border, input border, candidate dividers, dl card border |
| `bg-accent` / `text-accent-ink` | Primary CTA: "Enrich", "Confirm" buttons (solid) |
| `text-accent` / `border-accent` | **Provider-sourced** provenance chip (outlined accent); input focus ring (`focus:border-accent`); selected-candidate accent |
| `text-warn` / `border-warn` | Enrich **error** state only (provider unreachable, fetch failed) — never for normal provenance |
| `font-display` via `.skin-title` | Panel/dialog heading |
| `rounded-theme` | All cards, buttons, inputs, candidate rows |
| `rounded-full` | Provenance chips + alias chips (intentional pill shape, allowed) |

### Provenance badge styling (decisive)

Two visually distinct, token-only treatments — chosen so neither reads as an error and the "added by machine" one is noticeable but not alarming:

- **From file** (the baseline/truth): `rounded-full bg-surface-2 px-2 py-0.5 text-xs text-muted` → quiet, recedes.
- **From TMDB** (enriched): `rounded-full border border-accent px-2 py-0.5 text-xs text-accent` → outlined accent, distinct from the *solid* accent used for active CTAs and from the muted file chip.

> Rationale: `--warn` is reserved for error/attention (CLAUDE.md), so provenance must not use it. Accent doubles as the active/primary color, but here it's an **outline** (not a filled active state), which reads as "noteworthy source" without colliding. **QA note:** verify accent-on-surface legibility in all three skins (Brutalist lime `#d6ff3f` and Broadcast cyan `#36e0d0` are bright — the outlined treatment keeps text on `bg-surface`, which is fine, but eyeball it).

---

## Components

| Component | Variant / Props | Notes |
|-----------|-----------------|-------|
| `EnrichButton` (or inline) | `disabled` while a fetch is in flight | Solid accent CTA. Label: "Enrich from TMDB" (provider name from `/describe`). Hidden unless `activity.isOwner`. |
| `EnrichPicker.svelte` (new) | props: `personName`, `provider`, `open`; events: `confirm(externalId)`, `close` | Modal listbox. Owns the search input, debounced `/resolve` call, candidate list, keyboard nav. |
| `CandidateRow` | states: default / hover / selected (`aria-selected`) | `label` (ink, `.skin-title`-optional), `disambiguation` (muted, single line, truncated), `confidence` chip (see below). |
| `ProvenanceBadge` (new) | `variant: 'file' | 'provider'`, `label` | The two chip treatments above. `aria-label="source: from {label}"`. |
| Person field `<dl>` | reuse media Details shape | Each `<div>` row: `<dt>` label (muted) + `<dd>` value + trailing `ProvenanceBadge`. |
| Alias chips | `rounded-full border border-rule` | Provider-supplied aliases show a small provider badge; manual aliases don't. |

### Confidence display

Don't show raw `0.98`. Derive a humane label from the numeric confidence returned by `/resolve`:

| Confidence | Chip |
|---|---|
| ≥ 0.85 | `text-accent` "Strong match" |
| 0.5–0.85 | `text-muted` "Possible match" |
| < 0.5 | `text-muted` "Weak match" |

Optionally append the percentage in `text-muted text-xs tabular-nums`. The point is the owner is **always confirming** (no silent auto-apply in v1), so confidence is advisory, not gating.

---

## States and Interactions

| Element | State | Behavior |
|---------|-------|----------|
| Enrich button | default | Solid accent; opens picker. |
| Enrich button | not owner | Not rendered at all (no disabled tease). |
| Enrich button | needs token | Replaced by the `/status`-style unlock form; on success, button appears. |
| Enrich button | fetching | `disabled`, `opacity-60`, label → "Enriching…" (no spinner needed; matches `/status` busy style). |
| Picker | opening | Fade + slight scale-in (see Motion); focus moves to the search input. |
| Picker input | typing | Debounce ~300 ms, then `POST /resolve` with `query`. Below ~2 chars, show help text, don't call. |
| Picker list | loading | `AsyncState` loading → "Searching {provider}…" (muted, centered). |
| Picker list | results | `role="listbox"`; rows navigable by ↑/↓; `aria-activedescendant` tracks the active row; hover and keyboard selection share the `bg-surface-2` highlight. |
| Candidate row | activate (Enter / click) | Becomes `aria-selected`; reveals a "Confirm" CTA (or Enter confirms directly). On confirm → close picker, call `/enrich`, show toast. |
| Picker | confirm success | Toast "Enriched from {provider}." (4 s auto-clear, like `/status`). Panel fields re-render with provenance. |
| Picker | Esc / backdrop click / close btn | Close, return focus to the Enrich button. |
| Provenance badge | — | Static. `title`/`aria-label` give the long form ("source: from TMDB"). |
| Clear-provider control | owner only | A small "Clear TMDB data" text button (muted, `hover:text-ink`) near the panel heading; confirm-before-act like rescan; removes the provider's contribution, fields fall back to next source. |

---

## Responsive Behavior

| Breakpoint | Changes |
|------------|---------|
| Desktop (≥1024) | Field `<dl>` two-column (`sm:grid-cols-2`), Enrich button top-right of the panel. |
| Tablet (640–1024) | Same as desktop; picker stays `max-w-lg` centered. |
| Mobile (<640) | `<dl>` one column; Enrich button wraps under the heading (`flex-wrap`); picker is near-full-width (`w-full` with page gutter), list scrolls within `max-h-[80vh]`. |

---

## Edge Cases

- **No provider configured / provider disabled:** Enrich button absent; if the owner expects it, a muted hint "No metadata source configured" (links to docs). Don't error.
- **Provider unreachable (`/healthz` down or `/resolve` fails):** picker shows a `border-warn` inline message "TMDB is unavailable right now." — single failure, not a page break. Mirrors `/status` provider-health.
- **Empty search results:** "No matches for '{query}'." (muted) + keep the input focused to retype.
- **No embedded ID + ambiguous name:** the normal path — that's exactly what the picker is for. (For People, this is the *dominant* path; embedded-ID auto-match is rare until Series/Video generalization.)
- **Long bio / disambiguation text:** bio wraps; disambiguation line single-line `truncate` with `title` full text; candidate label `truncate` at the row width.
- **International text (CJK aliases, e.g. 宮崎駿):** must render in all skins — Broadcast/Brutalist use mono display faces; verify the field `<dl>` (body uses `font-ui`) shows CJK acceptably (mono UI fonts fall back for CJK — check it doesn't tofu).
- **Photo asset:** if `assets.photo` present, show it; while downloading, reuse the `thumb-shimmer` hook; on failure, fall back to the existing no-photo treatment (don't block field display).
- **Re-enrich:** identity already confirmed → skip the picker, go straight to `/enrich` with the stored `external_id`; toast on completion.
- **Slow connection:** all provider calls are explicit and show loading; nothing auto-polls (unlike activity).

---

## Animation / Motion

All motion gated behind `@media (prefers-reduced-motion: no-preference)`, consistent with the grid/shimmer/activity-dot conventions in `app.css`.

| Element | Trigger | Animation | Duration | Easing |
|---------|---------|-----------|----------|--------|
| Picker panel | open | fade + scale 0.98→1 | 150 ms | `cubic-bezier(0.2,0.7,0.2,1)` (matches `reel-rise`) |
| Backdrop | open | fade 0→opacity | 150 ms | ease |
| Candidate highlight | hover/active | background only (no transform) | instant/100 ms | — |
| Enriching button | busy | none (opacity state only) | — | — |

Skin flourishes (if any) belong in `app.css` gated by `[data-theme]`, attached to a shared hook class — **not** per-component markup. The picker likely needs no skin-specific flourish; keep it clean.

---

## Accessibility Notes

- **Picker is a modal dialog:** `role="dialog" aria-modal="true"`, labelled by its heading (`aria-labelledby`). Trap focus within; Esc closes; focus returns to the Enrich button on close.
- **Search + results = combobox/listbox:** input owns `role="combobox" aria-expanded aria-controls={listId} aria-activedescendant={activeRowId}`; list is `role="listbox"`; rows `role="option" aria-selected`. Mirror the search-history dropdown's exact pattern (it already nailed `aria-activedescendant`).
- **Keyboard:** ↑/↓ move active option (wrap or clamp — match search history), Enter confirms active, Esc closes, Tab moves to Confirm/Cancel. The whole flow must be operable without a mouse.
- **Provenance badges:** decorative chip text is fine, but add `aria-label="source: from TMDB"` so screen readers get the full phrase, not just "TMDB".
- **Confidence chips:** advisory; include the word ("Strong match") not just color — never rely on accent color alone to convey match strength (color-blind safe).
- **Announce results:** the results count region should be polite-live (`aria-live="polite"`) so "3 matches" is announced after a search.
- **Owner controls:** when hidden (non-owner), they're absent from the DOM — not visually hidden — so there's nothing misleading in the a11y tree.

---

## Three-skin QA checklist (required before merge — CLAUDE.md)

Render `/people/[id]` and exercise the picker in **Cinémathèque, Broadcast, Brutalist**:

- [ ] **Provenance chips legible** in each skin — outlined-accent "from TMDB" reads on `bg-surface`; muted "from file" recedes; neither collides with the resolution badge / active accent.
- [ ] **Picker panel + backdrop** correct per skin radius (`--radius`: 2px / 0 / 0) — no stray rounded corners on Broadcast/Brutalist.
- [ ] **Confidence chips** — "Strong match" accent text legible on each accent (lime/cyan/gold).
- [ ] **Heading** uses `.skin-title` (uppercase + caret on Broadcast, uppercase on Brutalist).
- [ ] **CJK aliases** render (no tofu) in mono-faced skins.
- [ ] **Focus ring** (`focus:border-accent`) visible on the input + buttons in each skin.
- [ ] **Reduced-motion**: picker open is instant when `prefers-reduced-motion: reduce`.
- [ ] **Loading / empty / error / populated** states all themed (no raw white/black, no hardcoded color).
- [ ] **Owner vs locked**: with `ADMIN_TOKEN` set and no token entered, controls are hidden and the unlock form shows; after unlock, the Enrich flow works.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` returning empty for the new components.

---

## Open questions for the build (non-blocking)

1. **Photo crop UI** — Phase 3 F14.3 wants standard aspect-ratio derivatives. Does v1 just store + display the provider photo as-is, deferring crop to the person-photos story? (Leaning: store as-is, no crop UI in F22.)
2. **Panel placement** — `detail` snippet on `EntityVideos` vs. page-composed panel. (Recommended: snippet, above.)
3. **Where else provenance shows** — v1 is the person page; the media Details `<dl>` gets the same badge when Video enrichment generalizes (not now).
