# Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)

**Spec**: [configurable-provider-search-patterns.md](../specs/configurable-provider-search-patterns.md) ·
**ADR**: [ADR-080](../architecture/ADR-080-configurable-provider-search-patterns.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins** (applies to the optional P1 caption
below; the P0 scope introduces no new markup).
**Prior art**: [`EnrichPicker.svelte`](../../web/src/lib/components/enrichment/EnrichPicker.svelte)
(F22.5b) — the component this change deliberately does **not** modify.
**Surfaces**: `web/src/routes/media/[id]/+page.svelte` (one prop value changes); `person/[id]` and
`studios/[id]` (explicitly **unchanged** — see Non-goals). Backend: `getMedia` response gains
`enrich_queries` (out of scope for this document; see the spec/ADR).

---

## Overview

This is the rare handoff whose headline finding is **zero component changes**. ADR-080 D5 already
concluded `EnrichPicker.svelte` needs no code: it takes a generic `entityName` string prop and seeds
its search box from it (`EnrichPicker.svelte:37`), auto-searches on open, and lets the owner retype —
none of that changes. The only thing that changes is **which string** the video-detail page passes
into that prop. This document exists to specify that string precisely (content, not layout), plus one
optional P1 affordance, so an implementer isn't guessing at edge cases the spec left as prose.

### Design-system fit (the `/design-system` check)

**Zero new components. Zero new tokens. Zero new CSS.** One call-site's prop *value* changes:

- `media/[id]/+page.svelte` currently passes `entityName={video.title}` into `EnrichPicker`
  (per the ADR-080/spec research, `+page.svelte:1053`). It changes to
  `entityName={enrich_queries?.[provider.name] ?? sanitizedFallback}` — a data change, not a markup
  change.
- `person/[id]/+page.svelte` and `studios/[id]/+page.svelte` keep `entityName={person.name}` /
  `entityName={studio?.name}` **exactly as today** — F53 is video-only (spec Non-goals). Call this
  out explicitly so an implementer doesn't "helpfully" extend the change to those pages: there is no
  studio/year/performer data on those entities to build a query from.
- The optional P1 caption (below) reuses the picker's own existing status-line idiom
  (`text-xs text-muted`, same row family as the "Type at least two characters…" line at
  `EnrichPicker.svelte:241-255`) — no new token, no new component.

---

## Content specification (the string the owner actually sees)

This is the load-bearing part of this handoff — the spec's FR3/FR4 describe the *algorithm*; this
section pins down what a human looks at.

| Scenario | Resolved title (raw) | Rendered `entityName` seed |
|---|---|---|
| Full pattern renders (`{studio?} {title?} {performers?} {year?}`, all resolve) | *(irrelevant — pattern wins)* | `Wicked Pictures Selena Sky 2023` |
| Pattern configured but a required token is missing → falls through | — | falls to the next-lower tier per ADR-080 D2/D3, ultimately the sanitized floor below |
| No pattern configured anywhere for this provider | `[MyStudio] My Title (Some Actor, Other Actor) 720p` | `MyStudio My Title Some Actor Other Actor` |
| Clean, already-tagged title, no pattern configured | `The Matrix` | `The Matrix` (sanitizer is a no-op on clean text) |
| Title with a non-resolution number | `Agent 007` | `Agent 007` (unchanged — `\d{3,4}p`/`[48]k` are word-bounded, don't match `007`) |
| **Degenerate: sanitization would yield an empty string** | `[720p]` | `[720p]` (raw, **unsanitized** — see Edge Cases) |

### Edge case: empty sanitization result

The spec (FR4) doesn't say what happens if stripping brackets/commas/resolution tokens leaves
**nothing** (a filename that is *only* bracket/resolution noise, e.g. `[720p]` or `(1080p)` alone —
rare, but real for a badly-named scrape). Resolution for implementation: **if the sanitized result is
empty or whitespace-only, use the raw, unsanitized title instead.** The search box must never be
seeded blank — an owner staring at an empty box with no idea why is a worse experience than seeing the
one cluttered string sanitization couldn't improve. This is a one-line addition to `sanitizeTitle`'s
contract: return the input unchanged when the stripped result is empty.

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| Search input (`EnrichPicker.svelte:229`) | On open | Seeded with the value above; unchanged focus/select-all behavior (`onMount`, `input?.focus(); input?.select();`) |
| Search input | Owner types | Unchanged — debounced search exactly as today, no new logic |
| Auto-search on open (F22.5b) | Seed ≥ 2 chars | Unchanged trigger condition; fires against the new seeded value instead of the raw title. A sanitized/pattern-built query is *more* likely to land the single-strong-match auto-apply path (RD1), which is the intended effect, not a new code path |
| P1 caption *(optional, see below)* | Seed differs from raw resolved title | A muted one-line hint appears under the input |

---

## Optional P1: seeded-value transparency caption

Not required for v1 (spec P1-a) — flagged here so it's fully specified if picked up, rather than
half-designed later.

**Trigger**: render only when the seeded `entityName` differs from the video's raw resolved `title`
(i.e., a pattern rendered, or the sanitizer changed something) — never shown when the seed equals the
raw title verbatim (nothing to explain).

**Copy**: `Pre-filled from {source}.` where `{source}` is:
- `"search pattern"` when a D2 pattern tier rendered (operator/provider/default).
- `"filename cleanup"` when only the D4 sanitizer changed the floor tier.

**Placement**: directly below the search input, same row family as the existing status line
(`EnrichPicker.svelte:241`, `text-xs text-muted`) — but as an **additional** line above it, not a
replacement, since the status line's job (search progress/result count) is unrelated and must keep
updating independently.

```
┌ Enrich from tmdb ──────────────────────────────────────┐
│ [ Wicked Pictures Selena Sky 2023                    ] │
│ Pre-filled from search pattern.                         │
│ 2 matches — Tab or ↑/↓ to choose, then click or press…  │
│ ...                                                      │
└──────────────────────────────────────────────────────────┘
```

No icon, no dismiss control — it's informational only, reusing the exact muted-text idiom the picker
already uses one line below it. Disappears automatically the moment the owner edits the box (the
caption describes the *seed*, not the current value — once the owner has typed, showing a stale
"pre-filled from…" claim would be misleading, so it hides on the first `oninput`).

---

## Non-goals (explicitly out of this change)

- **Person/Studio picker seeding.** Stays `entityName={person.name}` / `entityName={studio?.name}`,
  unchanged. No studio/year/performer fields exist on those entities to build a pattern from (spec
  Non-goals).
- **Any change to `EnrichPicker.svelte`'s own file** beyond what the P1 caption would add, if built.
  The debounce, roving-tabindex listbox, auto-apply, and "None of these match" flows are all
  untouched.
- **A settings UI to preview/edit the pattern.** Config stays YAML-only (spec Non-goals); this handoff
  covers only what the owner sees in the existing picker.

---

## Accessibility Notes

- No new interactive elements in the P0 scope — the input's existing `role="combobox"`,
  `aria-expanded`, `aria-controls` are unaffected by a different initial value.
- `input?.select()` on mount (existing behavior) means a screen-reader/keyboard user who wants to
  keep the pre-filled seed just needs to *not* type; this is unchanged from today's title-seeded
  behavior, just seeded with a different (better) string.
- If the P1 caption is built: mark it plain text within the existing `aria-live="polite"` region is
  **not** required — it renders once at mount alongside the input, not as a dynamic status update, so
  a separate live region would over-announce. Static text is sufficient; a screen reader encounters it
  in normal reading order same as any other static label.

---

## Measured contrast

No new color combination — the P1 caption (if built) reuses `text-muted` on `bg-surface`, already
measured in `writeback-selection-handoff.md`'s contrast table (4.67–16.76:1 across all three skins).
Nothing to re-measure.
