# Design handoff: Provider render hints — auto-registered non-canonical fields (F39)

**Spec**: [provider-render-hints.md](../specs/provider-render-hints.md) (F39, HOLODEX-128) ·
**ADR**: [ADR-056](../architecture/ADR-056-provider-field-render-hints.md)

This is an **addendum** to the [F36 source-of-truth handoff](field-source-of-truth-handoff.md) and the
[F37 people](people-source-of-truth-handoff.md) / [F38 studio](studio-entity-handoff.md) handoffs. The
canonical-field rendering — `SourceSelect` radiogroup, `CurationFieldRow` chips, `ProvenanceBadge`,
`UrlValueList`, the compact/long/merge field buckets — is **inherited unchanged**. This document specifies only
what is new for F39: **one new read-only render mode (`chips`)**, a **read-only "Additional details"
grouping** for auto-registered non-canonical fields, and the **`image_url` allowlist fallback**. Everything is
**tokens-only** (no literal palette/radius/font — see [theming.md](theming.md)) and must be QA'd in all three
skins.

Auto-registered fields are **display-only** (ADR-056 §D4): no source chips, no curation, no writeback — for
owner and visitor alike. They are the provider's extra attributes, shown well, not edited.

---

## Overview

A provider's non-canonical fields (e.g. `gender`, `trivia`, `credited_as`, `home_page`) that have a stored
value appear as clean read-only rows **after** the curatable canonical fields, under a subtle **"Additional
details"** divider, each badged with its source. They render in the mode the provider hinted
(`text`/`long_text`/`chips`/`url`/`image_url`), falling back to plain text when a hint is absent or a value
fails the safety gate. The page is unchanged when an entity has no non-canonical values — the divider and the
group only appear when there is something to show.

---

## 1. The `chips` render mode (new, read-only)

A new `display: 'chips'` value renders a field's values as a **static pill list** — the read-only cousin of
`CurationFieldRow`'s chips, with no ✕/＋ controls.

- Pills: `inline-flex items-center rounded-full border border-rule px-2.5 py-0.5 text-sm text-ink` in a
  `flex flex-wrap gap-1.5` container. (`rounded-full` is the intentional pill shape, not a hardcoded radius.)
- Multi-value: the resolver splits/dedupes the value like a merge field; each surviving value is one pill.
- No provenance per-pill (unlike `CurationChip`) — the **row** carries a single `ProvenanceBadge` (§3).
- Reused by both the person and media read-only branches; add as a small `ChipValueList.svelte` mirroring
  `UrlValueList.svelte` (values in → pills out), so all three pages share one component.

This is the only new render primitive. `text`/`long_text`/`url`/`image_url` reuse the existing branches.

---

## 2. Placement — "Additional details" grouping

Auto-registered fields render in a **new sub-group at the end of the existing Details section**, on each entity
detail page, separated by a hairline divider + a muted label:

```
Details
  Bio: …                     ← canonical / mapped (curatable, unchanged)
  Born: …
  Also known as: [chips]
  ────────────────────────   ← divider: border-t border-rule
  Additional details         ← <p class="text-xs text-muted">, sentence case
  Gender: Female                      from tmdb
  Also credited as: [chip][chip]      from tmdb
  Trivia: <paragraph>                 from tmdb
  Home page: example.org ↗            from tmdb
```

- The divider + heading render **only when ≥1 auto-registered field is present** (mirrors the studio page's
  "sparse before enrichment" rule — grow chips only when there's something to show).
- Rows reuse the existing `dt`/`dd` grid: `<dt class="text-muted">{label}:</dt>` + a `<dd class="text-ink">`
  whose contents switch on `display`. `long_text` gets the block treatment (`mt-1 block leading-relaxed`);
  `text` stays inline; `chips` uses §1; `url` uses `UrlValueList`; `image_url` uses the gated `<img>` (§4).
- Ordering within the group follows the resolver (group rank → order → key); the SPA renders in received
  order, no client sort.

**Per-page seams** (all three inherit the same group component):
- **Person** — after `longFields` in `web/src/routes/people/[id]/+page.svelte`. Auto-registered fields are
  neither `compactFields` nor `mergeFields` nor `longFields` (those filter on the curatable set); add a fourth
  derived list `extraFields = resolved.filter(f => f.auto_registered)` and render it in the new group. (Add an
  `auto_registered` boolean to `ResolvedField`, or infer from "no `items`/`decision` and non-canonical".)
- **Studio** — the equivalent Details block in `web/src/routes/studios/[id]/+page.svelte`.
- **Media** — after the mapped fields in `web/src/routes/media/[id]/+page.svelte` (which already switches on
  `f.display`); add the group + the `chips` branch.

---

## 3. Provenance

Each auto-registered row carries one `ProvenanceBadge` (existing component) for the supplying provider —
identical to how a provider-won canonical field is badged today. No new badge; auto-registered fields are
**always** provider-sourced (they come from the shadow store), so the badge is always present. File-sourced
fields never auto-register, so there is no "from file" case here.

---

## 4. `image_url` safety fallback (visible behavior)

An `image_url` value that is **not** on the asset-host allowlist (ADR-039) must **not** render as an `<img>` —
the resolver marks it, and the SPA falls back to rendering the value as plain `text`. This is a deliberate,
visible degradation, not an error: the URL string shows as text with the provenance badge, and no image loads.
An allowlisted value renders the thumbnail exactly like `poster_url`/`logo`:
`max-h-64 rounded-theme border border-rule`. Do not show a broken-image icon or an error state for the
non-allowlisted case — plain text is the correct, quiet fallback.

---

## 5. States

| State | Render |
|---|---|
| No auto-registered fields | The divider, heading, and group are **absent**. Page identical to today. |
| ≥1 auto-registered field | Divider + "Additional details" + read-only rows in resolver order. |
| `image_url`, non-allowlisted host | The value renders as `text` (no `<img>`, no error). |
| `url`, non-http value | Renders as `text` (existing `UrlValueList` behavior). |
| Owner vs visitor | **Identical** — auto-registered fields have no owner controls. |

---

## 6. Three-skin QA (required)

Render each mode in **Cinémathèque, Broadcast, and Brutalist** (header picker), in the loading/empty/populated
states, tokens only:

1. `text` and `long_text` rows read correctly against `bg-surface`; `long_text` paragraph uses `text-ink`
   with `leading-relaxed`.
2. `chips` pills use `border-rule` + `text-ink`; confirm no collision with the `ProvenanceBadge` on the same
   row in any skin (the F36/F38 badge-vs-chip regression class).
3. `url` links use `text-accent`; the external-link affordance reads in all three skins.
4. `image_url` thumbnail uses `rounded-theme`/`border-rule`; the **non-allowlisted text fallback** is legible
   (no phantom image frame).
5. The "Additional details" divider (`border-rule`) and heading (`text-muted`, `text-xs`, sentence case) sit
   correctly below the curatable fields; the group is absent when empty.
6. Auto-registered rows show **no** owner controls in owner mode (no `SourceSelect`, no ✕/＋).

See [provider-render-hints-qa-checklist.md](provider-render-hints-qa-checklist.md) for the numbered, tagged QA
items.

---

## 7. What is explicitly not in this handoff

- **Editing / promotion UI.** No affordance to promote an auto-registered field to a mapping (spec Non-Goal);
  promotion is a YAML edit. Auto-registered fields never show curation controls.
- **Per-pill provenance.** The `chips` mode is read-only with one row-level badge, not the `CurationChip`
  per-value provenance.
- **New skins / tokens.** No new CSS variable, no `[data-theme]` flourish — F39 is pure token reuse.
