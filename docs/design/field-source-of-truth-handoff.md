# Design Handoff: Per-field source-of-truth decisions (F36)

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) · **ADR**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)
**Builds on**: [Metadata Curation (F30)](metadata-curation-handoff.md) (chips/provenance), [Refresh Metadata (F31)](metadata-refresh-handoff.md) (header cluster, refetch idiom).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

> **Refinement — HOLODEX-112 (shipped on top of PR #71).** The original control was three stacked
> elements: a read-only resolved chip, a segmented `Keep file · Adopt {provider} · Custom` control, and
> a muted candidates line. When file and a provider agreed, the value was drawn **three times** and the
> candidates line repeated it verbatim; the segmented control also read as a different system from the
> merge chips beside it. That is now collapsed to **one row of source-tagged, single-select value
> chips** — the same `CurationChip` shell as merge fields, distinguished only by its **selector glyph**
> (replace = a leading **● radio dot**, pick one; merge = the **✕-per-chip**, drop any). The sections
> below are updated to the chip model; the segmented-control description is superseded.

---

## Overview

On the media detail page (`/media/[id]`) Metadata section, each **replace (scalar) field** gains an
owner-only **source-of-truth chip row**: the **file** baseline first (anchored, tagged `·file`), one
chip per **distinct** candidate value (tagged with every source that supplies it), and a trailing
**Custom** chip. Selecting a chip names which source is *true* for that field on this item — a
**standing** choice that overrides global mapping precedence and drives both the displayed value and
what writeback commits. The **selected chip *is* the resolved value**; the default (no decision) is the
**file** chip (file-first, RD4). A provider is shown as a sibling *candidate chip*, never the silent
winner. This is the user-facing fix for the F31 refresh-masking bug.

**Merge (set) fields are unchanged** (RD1): they keep today's `CurationFieldRow` chips (union, per-value
include/exclude, manual-add). The source-of-truth **radiogroup** is **replace-only** — but it now shares
the merge fields' chip shell, so the two read as one vocabulary (see "RD1, revised" below).

**Non-negotiable for devs (RD5):** changing a decision is a **DB write only — no file I/O, no spinner**.
The file is written solely by the explicit **Write decisions to file** action, which batches *all*
decided/out-of-sync fields into **one** atomic `WriteBatch` per file via the existing F30 queue. Never
write a file per toggle.

### Design-system fit (the `/design-system` check)

**Zero new primitives.** The refinement deletes the bespoke segmented control and instead reuses the
`CurationChip` shell in a new **radio (pick-one) mode**:

- **Chips + provenance** — `CurationChip` gains a `radio` prop: a leading **● dot** (filled/accent when
  selected, hollow otherwise) replaces the merge `✕`/edit controls; the `·source` suffix, provider-vs-
  baseline colouring, and value/`—`-placeholder rendering are unchanged. One shared shell, one glyph
  swap. No `ProvenanceBadge` and no separate resolved chip — the selected chip carries the provenance.
- **Combined-provenance dedup** — a provider value equal to the file value folds into the file chip
  (`·file + tmdb`) via the same `sources.join(' + ')` provenance `CurationChip` already renders; this is
  what kills the "value shown twice when sources agree" wart. The view-model lives in `f36.ts`
  (`sourceChips`, `selectedChipKey`) — pure, unit-tested.
- **Custom input** — unchanged inline-edit idiom (Enter commits, Escape cancels + returns focus, blur
  commits, empty cancels). The Custom chip is the opener when undecided and the frozen `·manual` value
  chip once set.
- **Write button** — reuse the existing **Write to file** ghost button (relabel + count).
- **Owner gating + refetch-after-mutate** — `activity.effectiveOwner`; reuse `applyMediaDetail`
  (`+page.svelte`) so `resolved[]` reflects the new decision.

`SourceSelect` is now a thin **radiogroup wrapper** around `CurationChip` radios (roving tabindex,
arrow-key debounce, optimistic selection). It uses **no new tokens** — `border-rule` / `bg-surface-2` /
`text-muted` for idle chips, `border-accent` + an accent-filled dot for the selected one, `rounded-full`
chip shape. The selected chip **stays on `bg-surface-2`** (not a filled `bg-accent`) so it doesn't read
heavy in Brutalist; selection reads via the **dot + border + `aria-checked`**, never fill alone.

### RD1, revised — shared chip vocabulary, distinct glyph

The original handoff (RD1) kept replace and merge fields as **different-looking systems** ("one system
per field shape") so the pick-one vs drop-any distinction was obvious. HOLODEX-112 **supersedes that
rationale**: making the two *look* different fought the design system (a segmented control beside chips
read as two unrelated components). Instead the two now **share one chip shell** and are told apart by how
they **behave**, encoded in the **selector glyph**:

| Field shape | Selection model | Glyph | Meaning |
|---|---|---|---|
| **Replace** (scalar) | single-select radiogroup | leading **● radio dot** | pick exactly one source |
| **Merge** (set) | multi-select curation | trailing **✕ per chip** | drop any value from the union |

The file chip stays **anchored first and always tagged `·file`** — the file-first mental model is the
whole point of F36, so the baseline never becomes just another value in the row. Divergence is now
**self-evident** (two different value chips), so the old "providers differ" hint is **deleted**.

---

## Layout

Lives in the existing `Metadata` `<dl>` grid (`grid-cols-1 sm:grid-cols-2`,
`rounded-theme border border-rule bg-surface p-4`). For a **replace** field the `dd` is now a **single
wrapping row** of chips (owner view):

```
Metadata                         [Enrich ▾] [Clear …] [⤓ Write decisions to file · 2 out of sync]
┌────────────────────────────────────────────────────────────────────────────┐
│ Genres:  [ Drama ·file ✎ ✕ ] [ Thriller ·tmdb ✎ ✕ ] [ + Add ]               │ ← merge field: UNCHANGED (CurationFieldRow)
│                                                                              │
│ Title:   [● Blade Runner ·file] [○ Blade Runner: Final Cut ·tmdb] [＋ Custom]│ ← replace field: one chip radiogroup
│                                                       ( file out of sync ⚠ ) │   …+ trailing warn pill when decided ≠ in-file
│                                                                              │
│ Studio:  [● Legendary Pictures ·file + tmdb] [＋ Custom]                     │ ← agreeing sources FOLD to one chip (·file + tmdb)
└────────────────────────────────────────────────────────────────────────────┘
```

- **The chip row** — the file baseline first (anchored, `·file`), one `CurationChip` (radio mode) per
  **distinct** candidate value, then the **Custom** chip. The **selected** chip carries the ● filled
  dot + `border-accent` and *is* the resolved value (no separate resolved chip). `flex flex-wrap` so it
  wraps within the cell at `sm` two-column and on narrow cells.
- **Fold (dedup)** — a provider value equal to the file value folds into the file chip as
  `·file + tmdb`; providers that agree with each other fold together. An agreed value is shown **once**.
- **Out-of-sync pill** — trails the row, `text-warn`, *only when* decided ≠ in-file (RD2; the single
  `text-warn` signal on the field). It sits **outside** the radiogroup (it is not a radio).
- **Empty file baseline** — the file chip renders `—` (em-dash) when the file has no value for the
  field, so file-first "Keep file" stays available and anchored.
- **Visitor / non-owner** — the radiogroup is absent; the field renders read-only exactly as today.
  `displayTitle` and all read-only rendering are unchanged.

### The two signals no longer collide (Open-Q3, resolved by construction)

There is still exactly **one** `text-warn` signal per field: the **out-of-sync** pill. The old
"providers differ" hint is **gone** — divergence is now self-evident (two different value chips), so
there is nothing left to mistake for a second alarm.

---

## The source-of-truth chip radiogroup (`SourceSelect`)

A themed **single-select radiogroup** of `CurationChip` radios. Each chip: `rounded-full border
px-2 py-0.5 text-xs bg-surface-2`, a leading `● dot` (`h-2 w-2 rounded-full`), the value, and the
`·provenance` suffix. **Selected** = `border-accent` + an accent-filled dot + `aria-checked` (the chip
background stays `bg-surface-2` — no filled `bg-accent`, so Brutalist doesn't read heavy). Idle chips
are `text-muted` → `text-ink` on hover.

| Chip | When present | Selecting it |
|---|---|---|
| **file** baseline | always (anchored first, `·file`) | decision → `file` (default; source-pinned to the live file value) |
| **provider value** | one per **distinct** matched-provider value (agreeing sources fold together) | decision → `provider:{name}` (source-pinned to that provider's live value) |
| **Custom** | always (trailing) | opens the inline input; on commit, decision → `manual` + literal; once set it renders as the `·manual` value chip and re-opens on select |

- **No-provider-matched** → just the **file** chip + **Custom** (two chips).
- Selecting a chip issues `PUT …/decision` (or `DELETE` for the **file** chip when reverting to
  default); on success, **refetch** so the row + provenance + sync recompute. **No file write, no
  file-write spinner** (RD5). A brief inline `opacity-60`/`aria-busy` during the DB round-trip is fine;
  it must not look like a file operation.
- **Custom commit** keeps the input's existing affordance: Enter commits, Escape cancels (and returns
  focus to the Custom chip), blur commits; empty cancels.

---

## Writeback affordance

- Rename the header **Write to file** → **Write decisions to file**.
- Append a **count** when any field is out of sync: `Write decisions to file · {n} out of sync`. The
  count is `text-warn` (compact); when `n = 0`, show no count (button stays enabled — a re-write is
  allowed but is a no-op).
- The button keeps its existing ghost treatment + the **download-arrow** icon and the busy spinner
  (this is the **only** place a write spinner appears).
- Behavior note (dev): clicking enqueues **one** durable job → **one** `WriteBatch` per file (RD5/P0-4).

---

## Design Tokens Used

| Token | Usage |
|---|---|
| `bg-surface` | metadata card background (unchanged) |
| `bg-surface-2` | chip background — **both** idle and selected (selection never fills the chip) |
| `border-rule` | card + idle chip borders; input border |
| `text-ink` | selected chip value, hover value, input text |
| `text-muted` | field label, idle chip value + `·provenance` (baseline) |
| `text-accent` / `border-accent` | **selected** chip border + dot; provider `·provenance`; focus ring |
| `text-accent-ink` | primary buttons (unchanged); **not** used on chips |
| `text-warn` / `border-warn` | **out-of-sync** pill + the header out-of-sync count — error/attention only |
| `rounded-full` | chips, dot, pills (intentional shape) |
| `font-ui` / `font-display` | inherited; no per-component font |

No `zinc-/sky-/emerald-/amber-`, no hex, no fixed `rounded-lg/md/sm/xl`, no named fonts. The value span
caps at `max-w-[14rem] truncate` (a layout clamp, not a token) so a long title can't overrun the cell.

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| chip row | undecided (default) | **file** chip selected (● dot, `·file`); provider(s) appear as sibling candidate chips |
| chip row | decided keep-file | same as default (an explicit `file` decision and the default render identically) |
| chip row | decided adopt-provider | the provider value chip selected; `·{provider}` (accent provenance) |
| chip row | decided custom | the Custom chip is the `·manual` value chip, selected |
| chip row | busy (DB round-trip) | row `opacity-60`/`aria-busy`; **no** file-write spinner |
| chip row | no provider matched | only the **file** chip + **Custom** |
| file chip | file has no value | renders `—` (em-dash) so the baseline stays anchored + selectable |
| provider chips | two sources agree | fold to **one** chip tagged `·file + tmdb` / `·tmdb + imdb` (value shown once) |
| Custom chip | undecided | opener: leading `＋` + "Custom"; opens the inline input on select |
| Custom input | open | inline input focused; Enter commit / Esc cancel (+ focus back) / blur commit / empty cancel |
| Out-of-sync pill | decided ≠ in-file | `text-warn`/`border-warn` pill "file out of sync" trailing the row; hidden when in sync |
| Write button | n>0 out of sync | shows `· {n} out of sync` (warn count); spinner while the job is submitted |
| Write button | n=0 | no count; enabled (no-op re-write allowed) |
| Whole control | visitor / non-owner | absent; field renders read-only resolved value as today |

---

## Responsive Behavior

| Breakpoint | Changes |
|---|---|
| ≥ `sm` (two-column `dl`) | the chip row `flex flex-wrap` wraps chips within the cell |
| < `sm` (single column) | unchanged; chips wrap freely; the out-of-sync pill wraps to its own line |

The `dl` grid itself is unchanged. No chip sets a fixed pixel width — chips size to content (the value
span caps at `max-w-[14rem] truncate`) and the row wraps.

---

## Edge Cases

- **Long candidate values** (e.g. a long title) — the chip value truncates at `max-w-[14rem]` with the
  full value on `title`/`aria-label`; the row wraps. Never force the cell wider than its grid column
  (`minmax(0,1fr)` semantics).
- **Provider matched but no value for this field** — that provider contributes **no** chip (you can't
  adopt an empty value); it reappears if a re-enrich populates the field.
- **Provider value equals the file value** — folds into the file chip (`·file + tmdb`); selecting the
  folded file chip still pins `file` (file-first).
- **Custom equals the file value** — allowed; it still records a `manual` decision (the owner may want
  it frozen). Provenance reads `·manual`.
- **All sources empty** — the field doesn't render today (resolver returns nothing); the chip row does
  not appear for a field with no value and no candidates.
- **Decision references a provider later un-matched/cleared** — display falls back to the file chip; the
  stored decision is harmless (clearing the match is the existing F22/F31 path).
- **Out-of-sync after an external edit** — Refresh (F31) re-reads the file; sync recomputes on refetch.

---

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| chip | select / hover | color/border token transition | ~150ms | default `transition` |
| Custom input | open | none required (appears in place) | — | — |
| Write button icon | submitting | reuse existing `animate-spin` | — | — |

No layout-shifting animation; selecting a chip must not jump the grid (the dot is inline, the chip
doesn't resize on select).

---

## Accessibility Notes

- **`SourceSelect` = `role="radiogroup"`** (`tabindex="-1"`) with `role="radio"` **chips** and
  `aria-checked` on the selected one. **Roving tabindex** (cf. [[feedback-keyboard-list-roving-tabindex]]
  / `EnrichPicker`): the group is one Tab stop landing on the checked chip; **Left/Right (and Up/Down)**
  move and change selection; selection applies on arrow (debounced) or Space/Enter — native radio semantics.
- `aria-label` per chip names the value + provenance: `Blade Runner, from file`,
  `Blade Runner: Final Cut, from tmdb`, `Set a custom value for {field}` (opener) /
  `{literal}, from manual` (once set). The group has `aria-label="Source of truth for {field label}"`.
- The out-of-sync pill uses `aria-label="{field label} is out of sync with the file"`.
- Focus-visible ring (`focus:ring-accent`) on chips and the input; the Custom input traps nothing
  (inline), and Escape returns focus to the Custom chip (`await tick()` before refocus).
- **Colour is never the only signal:** selection reads via the **● dot + `aria-checked` + border**, not
  fill/hue; provenance is text (`·source`), not colour. The pick-one vs drop-any distinction is the
  **glyph** (dot vs ✕), not colour.

---

## Addendum — Per-provider match/enrich UI (HOLODEX-119)

Split from HOLODEX-9 (S4) and deferred until a **second real provider** existed. The backend was
already per-provider (`entity_enrichment` keyed by provider; `/enrich/sources` lists every enabled
provider; resolve/enrich/clear all take a `provider`). Only the **SPA** collapsed to the first capable
provider (`provider = sources.find(…)`), so a second matched provider could never be
matched/enriched/cleared from the UI. This widens that single assumption to a **per-provider list** on
both detail pages — no new component, no resolver change.

**Media page (`media/[id]`)** and **People page (`people/[id]`)**, owner view only:

- The Metadata/Details header renders **one `Enrich from {p}`** button per entity-capable provider
  (`sources.filter(s => s.entity_types.includes('video'|'person'))`), each opening its **own**
  `EnrichPicker` targeted at that provider. `pickerProvider` (a provider name, `''` = closed) replaces
  the old boolean `pickerOpen`.
- A **`Clear {p}`** button appears next to a provider **only once it is linked** to the entity — media
  keys this off the raw enrichment store (`enrichedByProvider.has(p)`), people off the resolved
  candidates/items (`providerLinked(p)`, the F37 replacement for the retired `enriched[]`). Each clear is
  independent (`busy`/`enrichBusy` holds the provider being cleared) and refetches so the chips drop that
  provider's candidate too.
- **Raw enrichment disclosures (media only)** — the foot-of-page audit block is now **one collapsible
  per provider** (`Enrichment data: {p} ({n})`), grouped from `enriched` by `provider`; each has its own
  open/closed flag (`openEnriched[p]`).
- **Chips are unchanged.** `SourceSelect` already renders one chip per *distinct* matched-provider value
  and folds agreeing providers into a shared `·{p1} + {p2}` chip (see the chip-row rules above). A
  provider only contributes a chip when it is a configured source for that field in
  `metadata-mappings.yaml` — the same rule as one provider; wiring a second provider into a field's
  `sources:` is an operator config step, distinct from the registry that drives the Enrich/Clear buttons.

**Deliberately unchanged:** no new visual vocabulary, no new tokens, no writeback/curation change; the
button group is the same accent/`rounded-theme` shell repeated per provider, wrapping via
`flex-wrap`. QA all three skins (the button group + folded chips must read in each).
