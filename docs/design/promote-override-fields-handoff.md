# Design handoff: In-app promote / override affordance for auto-registered fields (F44)

**Spec**: [promote-override-fields.md](../specs/promote-override-fields.md) (F44, HOLODEX-171) ·
**ADR**: [ADR-062](../architecture/ADR-062-in-app-field-promotion.md) ·
**QA checklist**: [promote-override-fields-qa-checklist.md](promote-override-fields-qa-checklist.md)

This is an **addendum** to the [F39 provider-render-hints handoff](provider-render-hints-handoff.md) (which added
the display-only "Additional details" group and the `chips` render mode) and, through it, to the
[F36 source-of-truth handoff](field-source-of-truth-handoff.md) and the
[F30 metadata-curation handoff](metadata-curation-handoff.md). Everything those documents specify — the
`AutoFieldRows` read-only rows, the `SourceSelect` radiogroup, the `CurationFieldRow` chips, `ProvenanceBadge`,
the `image_url` allowlist fallback — is **inherited unchanged**.

This document specifies only what F44 adds: an **owner-only** control that **promotes** an auto-registered
(display-only) field into a **first-class curatable** field, the **inline editor** that drives promote /
edit / de-promote, and the **partition move** that happens when a field crosses that line. Everything is
**tokens-only** (no literal palette / radius / font — see [theming.md](theming.md)) and must be QA'd in all
three skins.

**Resolved design decisions** (2026-07-07, via mockup):
- **DD1 — Editor is an inline expander.** The editor unfolds in-flow directly beneath the row it targets
  (rows below reflow down), reusing the existing in-flow input idiom (`CurationFieldRow`'s inline "Add",
  `SourceSelect`'s inline "Custom"). **No** popover, focus-trap, outside-click, or viewport-flip machinery —
  there is no popover primitive in the detail pages to reuse, and the inline form is the lower-risk,
  fewer-skins-to-break choice.
- **DD2 — Promote and Edit share one editor component.** A single editor drives both: **Promote** opens it
  empty (tier-3/4 label + render shown as placeholders), **Edit** opens it pre-filled and adds a **Remove
  promotion** action. Both commit via `PUT`; Remove issues the `DELETE`. One component, one validation surface,
  one QA pass.

---

## Overview

An **auto-registered** field (F39) is a provider's extra attribute shown as a clean read-only row under the
"Additional details" divider — for owner and visitor alike. F44 gives the **owner** (Admin mode on, F29
`effectiveOwner`) a way to make such a field first-class **without editing `metadata-mappings.yaml`**: a small
**Promote** control on each auto row opens an inline editor for its label / render mode / group / order. On
commit the field leaves "Additional details" and re-renders **above**, in the curatable field list, with the
full F36 source picker (`SourceSelect`) or F30 chips (`CurationFieldRow`) — exactly as a natively-mapped field.

Promotion presentation (label/render/group/order) is **global per `(entity_type, field_key)`** — renaming
"measurements" renames it for every person that has the key. Value curation stays **per-entity** (the existing
`field_source_decisions` / `metadata_curation` grain). A **visitor sees no F44 controls at all** — only the
promoted label/mode/order and curated values, indistinguishable from a mapped field.

The three entity detail pages share the same seams (F39 already unified them):
- **Person** — `web/src/routes/people/[id]/+page.svelte`
- **Studio** — `web/src/routes/studios/[id]/+page.svelte`
- **Media** — `web/src/routes/media/[id]/+page.svelte`

---

## 1. The Promote control (owner-only, on the auto row)

`AutoFieldRows.svelte` gains an **`isOwner`** prop (it currently takes only `fields`). When `isOwner` is true,
each auto-registered row gains a trailing **Promote** control; when false, the component renders exactly as
F39 today (no visual change for visitors).

**Affordance** — mirror the `CurationFieldRow` "+ Add" button pattern (border-rule pill, tokens only). It sits
**after** the value and the `ProvenanceBadge` on the same row:

```
Measurements: 34-24-36   [tmdb]   [ ↑ Promote ]
```

- Shape: `inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted
  hover:text-accent hover:border-accent focus-visible:text-accent` — **identical** to the `CurationFieldRow`
  Add button, so the two owner affordances read as one system.
- Icon: a small "promote/raise" glyph (an up-arrow / `arrow-up`), `h-3 w-3`, `stroke="currentColor"`,
  `aria-hidden`, followed by the text **Promote**.
- The control is a real `<button type="button">`; activating it (click / Enter / Space) opens the inline
  editor (§2) for that row and moves focus into the editor's first field (the label input).
- One row's editor is open at a time. Opening a second Promote closes any other open editor (the pages already
  hold detail state; a single `promotingKey` string in the page component is enough).

**Row-level state while its editor is open**: the row stays rendered (the target must remain visible above its
editor per DD1); optionally de-emphasize the Promote button (swap to an "editing" affordance or hide it while
the editor is open, since the editor's own Cancel/Promote are the actions).

---

## 2. The inline editor (shared promote + edit — DD2)

A single lightweight editor component (suggested `PromoteFieldEditor.svelte`) renders **in-flow**, spanning
both grid columns (`sm:col-span-2`), inside a bordered panel that reads as a focused sub-form:

- Container: `mt-2 rounded-theme border border-accent bg-surface-2 p-3` — the **accent** border is the "you
  are editing this" signal (distinct from the resting `border-rule`), matching `SourceSelect`'s inline-input
  accent-ring convention. `rounded-theme`, never a literal radius.
- A muted caption states the **global** scope so the owner is not surprised it renames everywhere:
  `text-xs text-muted` — e.g. *"Promote "measurements" — shared across all people"* (entity noun pluralized
  per page: people / studios / videos).

**Fields** (all tokens-only; inputs mirror the `CurationFieldRow` Add input styling
`rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:ring-1
focus:ring-accent`):

| Field | Control | Placeholder / default | Notes |
|---|---|---|---|
| **Label** | text input | placeholder = the inherited tier-3/4 label (provider hint or title-case) | Empty ⇒ inherit (the store keeps `label=''`). Cap **64 chars** (matches the API sanitizer); a `maxlength` is a courtesy, the server is the authority. |
| **Render** | `<select>` | current/inherited mode | Options: `text`, `long_text`, `chips`, `url`, `image_url` (the F39 vocabulary — **no new modes**). |
| **Group** | `<select>` | current/inherited group | Options: `primary`, `attributes`, `extended`. |
| **Order** | number input | `0` | `w-16`; integer; controls position within the group. |

- **Layout**: label on its own row (full width of the panel); render + order on one row; group on the next —
  or a simple 2-column `sm:grid-cols-2` of labelled controls. Keep it compact; this is a 4-field form.
- Each control has a visible `<label>` (or `aria-label`) — "Label", "Render mode", "Group", "Order".

**Actions row** (`flex justify-end gap-2 mt-1`):
- **Cancel** — `text-xs text-muted` ghost button; closes the editor, no request, returns focus to the Promote
  (or Edit) control that opened it.
- **Promote** / **Save** — the primary action: `text-xs rounded-theme bg-accent px-3 py-1 text-accent-ink`
  (the one accent-filled control in this sub-form). Label is **"Promote"** in create mode, **"Save"** in edit
  mode. On click → `PUT /admin/field-promotions/{entity_type}/{field_key}` with `{label?, render?, group?,
  order?}` → on 204, call the page's `reloadDetail()` and close the editor.
- **Remove promotion** — **edit mode only** (see §4); left-aligned, rendered with the **warn** token
  (`text-warn hover:border-warn`), never accent — it is the destructive/reverting action.

**Commit + reload**: like every F36/F30 mutation, the editor does not mutate local state optimistically beyond
a busy flag — it awaits the `PUT`/`DELETE`, then awaits `reloadDetail()`, which refetches `resolved[]` so the
partition move (§3) happens from server truth. While in flight: `opacity-60` + `aria-busy` on the panel
(matches `CurationFieldRow`/`SourceSelect`), actions disabled-in-effect (ignore repeat clicks). On error:
a `text-xs text-warn` line with `aria-live="polite"`, editor stays open, nothing moved.

---

## 3. After promotion — the partition move (shared by all treatments)

This is automatic and requires **no new render code** on the pages — it falls out of the existing
`!f.auto_registered` partition once the resolver stops marking the key `auto_registered` (ADR-062 FR3). What
the designer/QA must verify is the **visible** result:

- The field **disappears** from the "Additional details" group (`extraFields` filters `f.auto_registered`,
  now false) and **appears** in the curatable list above:
  - **scalar** render modes (`text` / `long_text` / `url` / `image_url`) → a **`SourceSelect`** replace row
    (owner) / read-only resolved value (visitor), in `compactFields` (or `longFields` for `long_text`).
  - **`chips`** render mode → a **`CurationFieldRow`** merge row (per-value ✕/＋ for the owner), in
    `mergeFields`. (`Multi = render=="chips"`, ADR-062 D-candidate — chips is the only multi surface.)
- It renders **once** — never doubled as both an auto row and a mapped row (ADR-062 FR3; pin with a test).
- Its candidate sources (the F36 chips) are **derived per-entity** from that entity's shadow provenance — one
  `provider:<ns>` chip per provider that supplied a non-empty value, plus the always-present baseline chip
  (`·record` for person/studio, `·file` for video — empty baseline is fine, F36 already shows "keep
  baseline") and the trailing **Custom** chip. Presentation (label/order) is shared; *which providers appear*
  follows what each entity has.
- If the "Additional details" group is now empty (the promoted field was its only member), the divider +
  heading disappear (F39 rule: the group renders only when `extraFields.length`).

Value curation on the promoted field is **per-entity**: adding/suppressing a value on person A does not touch
person B, while the label/render/order (from the global promotion) is shared.

---

## 4. Edit / Remove promotion (owner-only, on the promoted row)

A promoted row is a normal curatable row, so it needs a way back to the editor. Add an owner-only **Edit**
control on the promoted field's `dt` (label) line — a small pencil/edit pill mirroring the Promote button
shape (`rounded-full border border-rule text-xs text-muted`, `pencil`/`edit` glyph):

```
Measurements:  [ ✎ Edit ]
● 34-24-36 ·tmdb    ◦ — ·record    [＋ Custom]
```

- Placement: trailing the `<dt>` label, before the value row — so it reads as "edit this field's setup", not
  "edit a value" (values are curated by the chips themselves).
- Activating **Edit** opens the **same** editor (§2, DD2), **pre-filled** from the current promotion row
  (label/render/group/order), primary button reads **"Save"**, and the **Remove promotion** action is present.
- **Remove promotion** → `DELETE /admin/field-promotions/{entity_type}/{field_key}` → `reloadDetail()`. The
  field **reverts** to its F39 auto-registered display-only row back down in "Additional details"; the shadow
  value and any prior `field_source_decisions` / `metadata_curation` rows are **untouched** (they are keyed by
  `field_key`, independent of the promotion) and re-apply automatically if the field is re-promoted. De-promote
  of an already-removed row is idempotent (204) — no error surface needed.

Confirmation for Remove: a single-user personal server + a fully reversible action (curation survives) means a
**one-click Remove is acceptable** — no modal confirm. (If a confirm is ever wanted, use the existing inline
pattern, not a browser `confirm()`.)

---

## 5. States

| State | Render |
|---|---|
| Visitor, any field | **No F44 controls.** Auto rows and promoted rows both render read-only (promoted = curated label/mode/order + resolved value). Byte-identical in shape to F39 + F36 today. |
| Owner, auto-registered row | Read-only value + `ProvenanceBadge` + trailing **Promote** pill. |
| Owner, editor open (promote) | Inline accent-bordered panel beneath the row; empty fields with inherited placeholders; **Cancel** / **Promote**. Rows below reflow down. |
| Owner, editor open (edit) | Same panel, pre-filled; **Cancel** / **Save** / **Remove promotion** (warn). |
| Committing (`PUT`/`DELETE` in flight) | Panel `opacity-60` + `aria-busy`; repeat activations ignored. |
| Commit error | `text-warn` message under the actions, `aria-live="polite"`; editor stays open; nothing moved. |
| Owner, promoted row | Curatable row (`SourceSelect` or `CurationFieldRow`) + owner-only **Edit** pill on the label line. |
| No promotions on the entity | **Byte-identical** to F39/F36 (the golden no-op). No divider change, no extra DOM. |
| `image_url` promoted, non-allowlisted host | Value renders as **text**, not `<img>` (ADR-039 gate is unchanged by promotion — §7). |
| Empty "Additional details" after promoting its last member | Divider + heading disappear (F39 rule). |

---

## 6. Responsive behavior

The detail Details block is a `grid grid-cols-1 sm:grid-cols-2` `<dl>`. The editor and both affordances live
inside it and follow the same breakpoint:

| Breakpoint | Behavior |
|---|---|
| ≥ `sm` (640px+) | Editor panel spans both columns (`sm:col-span-2`); render + order share a row. Promote/Edit pills sit inline on the value/label line. |
| < `sm` (mobile) | Grid is single-column already; the editor is full-width; the 4 fields stack vertically. Pills wrap under the value if the row is narrow (the value row is `flex-wrap`). No horizontal scroll. |

No new breakpoints. Touch targets: the Promote/Edit/Cancel/Save controls keep the existing pill hit-area
(`px-2 py-0.5` / `px-3 py-1`) used across F36/F30 — adequate for the owner-only tooling context.

---

## 7. Security-visible behavior (do not regress)

These are owner-supplied-but-still-untrusted-value surfaces; the visible degradations must be preserved (they
are enforced server-side, but the UI must render them quietly, never as errors):

- **`image_url` on a non-allowlisted host renders as text** — same quiet fallback as F39 §4 (no broken-image
  icon, no error). Promotion does **not** bypass the ADR-039 image perimeter.
- **Labels render as escaped text** (Svelte default) — never HTML. A label with markup shows the literal
  characters.
- **Over-long / control-char label** is capped (64) + cleaned by the server sanitizer; the UI simply shows the
  cleaned value after reload. An **unknown render mode coerces to `text`** server-side (the `<select>` prevents
  it client-side, but the resolved row must render `text` if one ever slips through).
- A promoted field **never becomes a browse facet** (`Filterable=false`, ADR-062 D-filterable) — there is no
  new filter chip, no new browse control, from a promotion.

---

## 8. Motion

Minimal, tokens/skin-agnostic:

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Editor panel | Open / close | Optional height/opacity fade-in of the inline panel | ~120ms | ease-out |
| Partition move (auto → curatable, and back) | After `reloadDetail()` | **None required** — the field re-renders in its new position on refetch. A crossfade is out of scope; do not animate a DOM move across two sections (jank risk across skins). | — | — |
| Busy state | `PUT`/`DELETE` in flight | `opacity-60` (existing F36/F30 pattern) | — | — |

Keep motion subtle and respect `prefers-reduced-motion` if any fade is added (skip it entirely under reduced
motion). No skin-specific motion flourishes in the component (those live in `app.css` gated by `[data-theme]`).

---

## 9. Accessibility

- **Promote / Edit** are real `<button type="button">`s with discernible names — "Promote {label}" /
  "Edit {label} promotion" (visible text "Promote"/"Edit" + the row label via `aria-label` if the visible text
  alone is ambiguous).
- **Opening the editor** moves focus to the first field (the **Label** input). **Cancel**, **Escape**, or a
  successful commit returns focus to the control that opened the editor (the Promote pill, or the Edit pill) —
  the same focus-return contract `SourceSelect`'s Custom input honors (`focusChip` after `tick()`).
- **Escape** anywhere in the editor cancels (no commit) and returns focus, mirroring the F36 Custom-input and
  F30 Add-input Escape behavior.
- Every editor control has a programmatic label; the `<select>`s announce their options natively.
- The busy panel sets `aria-busy`; the error line is `aria-live="polite"` (matches `CurationFieldRow` /
  `SourceSelect`).
- **Focus order** within the editor: Label → Render → Group → Order → (Remove promotion, edit mode) → Cancel →
  Promote/Save. Tab reaches every control (no roving-tabindex here — this is a form, not a radiogroup; contrast
  `SourceSelect`, which is a radiogroup).
- The promoted curatable row inherits `SourceSelect`'s roving-tabindex radiogroup a11y or `CurationFieldRow`'s
  chip a11y **unchanged** — F44 adds only the Edit pill, which is a plain button in the tab order.

---

## 10. Three-skin QA (required)

Render every new surface in **Cinémathèque, Broadcast, and Brutalist** (header picker), in the
loading / empty / populated / error states, tokens only. Full numbered list in
[promote-override-fields-qa-checklist.md](promote-override-fields-qa-checklist.md); the essentials:

1. **Promote pill** matches the `CurationFieldRow` Add pill in every skin (border-rule → accent on hover);
   no collision with the `ProvenanceBadge` on the same row (the F36/F38 badge-vs-chip regression class).
2. **Editor panel** accent border + `bg-surface-2` reads as "editing" and is distinct from resting rows in all
   three skins; the inputs/`<select>`s use `rounded-theme` (2px Cinémathèque, 0px Broadcast/Brutalist) — no
   literal radius.
3. **Primary Promote/Save** button uses `bg-accent` + `text-accent-ink` and is legible in all three
   (Brutalist lime, Broadcast cyan, Cinémathèque gold); **Remove promotion** uses `text-warn`, clearly
   distinct from accent.
4. **Partition move**: after promote, the field appears once in the curatable list and is gone from
   "Additional details"; after Remove, it returns. Verified on **person, studio, and media**.
5. **Chips vs scalar**: a `chips`-render promotion becomes a `CurationFieldRow` (✕/＋); every other mode becomes
   a `SourceSelect` scalar row.
6. **Visitor view**: no Promote, no Edit, no editor — promoted rows show curated label/mode/order + value only.
7. **`image_url` non-allowlisted** promoted value renders as legible text (no phantom image frame) in all skins.
8. **Golden no-op**: an entity with no promotions is visually identical to pre-F44.

---

## 11. What is explicitly not in this handoff

- **A "manage all promotions" admin table** — the slice is the per-row affordance + edit/remove; a
  cross-entity management screen is a follow-up (spec Non-Goals, ADR-062 Consequences).
- **Per-entity presentation overrides** — label/render/group/order is global per `(entity_type, field_key)`;
  per-entity variance is value curation only (F36/F30), unchanged.
- **Promoting canonical or `_`-prefixed keys** — the Promote control appears only on auto-registered
  (non-canonical) rows; there is no affordance to promote `bio`. (The API also rejects it 422.)
- **New render modes** — reuse F39's five; the `<select>` offers exactly those.
- **Browse-facet ("promote to filter")** — `Filterable=false` in v1 (ADR-062 D-filterable); no filter UI.
- **Writeback of promoted fields to files** — out of scope (spec Non-Goals); a promoted field participates in
  decisions + curation, not file writeback.
- **New skins / tokens** — pure token reuse; no new CSS variable, no `[data-theme]` flourish.
