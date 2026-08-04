# Design handoff: Studio image roles — icon / logo / poster (F51)

**Spec**: [studio-images.md](../specs/studio-images.md) (F51, HOLODEX-247) ·
**ADR**: [ADR-079](../architecture/ADR-079-studio-image-roles.md)

This is an **addendum** to the [F38 studio-entity handoff](studio-entity-handoff.md). The
list's leading logo well (§1b there) and the detail page's layout are unchanged in shape;
this document specifies only what's new: the well's data source swap, and a new
role-generic image control on the detail page. Tokens-only (no literal palette/radius/
font — [theming.md](theming.md)); QA all three skins.

The nearest existing pattern is Person's per-role image wrappers
(`web/src/lib/components/person/PersonAvatar.svelte`, `PersonBanner.svelte`,
`PersonPoster.svelte`, all thin role-fixed callers of `PersonImageFrame.svelte`) — but
Studio needs **no gallery, viewer modal, or promote/reorder**, so the control is smaller:
upload / replace / remove, nothing else.

---

## 1. `/studios` list — logo well data source change only

No layout change to §1b of the F38 handoff. The well's `<img>` source moves from
`s.logo_url` to `s.icon_url`; the monogram fallback (studio has no icon) is byte-for-byte
the same fallback that already renders when `logo_url` was empty. If a studio has a logo
but no icon (common right after this ships — existing data carries into the `logo` role
only, per ADR-079 §1), the list shows the monogram until the owner sets an icon or a
provider starts supplying one (P2-1) — this is expected, not a bug, and matches how a
freshly-enriched-but-not-yet-logoed studio already renders today.

## 2. `/studios/{id}` detail — role-generic image control

Add a new `StudioImageSlot.svelte` component, rendered three times in the Details section
(order: **logo** first — it's the one thing every studio already has — then **icon**, then
**poster**), each bound to its role:

```svelte
<StudioImageSlot studioId={studio.id} role="logo"   url={studio.logo_url}   label="Logo" />
<StudioImageSlot studioId={studio.id} role="icon"   url={studio.icon_url}   label="Icon" />
<StudioImageSlot studioId={studio.id} role="poster" url={studio.poster_url} label="Poster" />
```

### Layout
- A labeled row per role: `<div class="flex items-center gap-4 rounded-theme border
  border-rule bg-surface p-3">` — a fixed-size preview frame on the left, label +
  provenance/action controls on the right. Mirrors the Details `<dl>` row rhythm already on
  this page (studio-entity-handoff's chip rows), not a new visual language.
- **Preview frame**: `object-contain` inside a role-appropriate fixed box — logo/icon at
  `h-16 w-16` (square-ish crop tolerance, same `bg-logo-plate` token as the list well),
  poster at `h-24 w-16` (2:3, matching Person's poster aspect). Empty slot renders the same
  monogram used elsewhere for that studio (logo/icon) — poster has no existing placeholder
  convention on this page, so an empty poster slot shows a plain dashed-border box with a
  small "+" glyph on the `text-muted` token (the upload affordance itself is the empty
  state — there's no prior poster placeholder to match since poster had no consumer before
  F51).
- **Owner-only controls**: "Replace" (file picker → `POST .../images/{role}`) and "Remove"
  (confirm → `DELETE .../images/{role}`), both `text-xs` buttons under the label, visible
  only when `owner` (existing `isOwner` store this page already gates other mutations
  with). Visitors see the frame and label only — no buttons, no empty-slot upload
  affordance (a visitor seeing a "+" they can't click is worse than seeing nothing; render
  the monogram/empty box with no glyph for visitors).
- **Uploading state**: frame dims to 60% opacity with a small spinner overlay
  (`animate-pulse` on the `bg-surface-muted` token) — reuse whatever in-flight pattern
  `PersonImageFrame` already uses for its own upload, don't invent a second one.
- **Error state**: a `text-warn` line below the frame ("Couldn't save — try a smaller
  image" for a 413/400, "Something went wrong" otherwise), cleared on the next successful
  action. Matches the person upload error pattern.

### Interaction
- "Replace" always available (even on a filled slot) — it's an upload that overwrites,
  matching Person's core-role replace-in-place semantics (no separate "are you sure" for
  overwrite, since the old bytes are still recoverable via a provider re-enrich unless the
  owner also deletes).
- "Remove" only shown on a filled slot; a confirm step (reuse whatever confirm affordance
  Person's image delete uses — inline "Remove? / Cancel" toggle, not a modal) since it's
  destructive and (per ADR-079) unlocks the slot for the next enrich to refill.
- No drag-and-drop requirement — a plain `<input type="file" accept="image/*">` behind the
  "Replace"/"+" trigger is sufficient (matches Person's upload control, no new interaction
  pattern to design).

### Empty vs. populated states (per role)

| Role | Empty | Populated |
|---|---|---|
| Logo | monogram (existing F38 fallback) | served image, replace/remove |
| Icon | monogram | served image, replace/remove |
| Poster | dashed box, owner-only "+" | served image (2:3 frame), replace/remove |

## 3. Provenance (P1, non-blocking)

If the P1 provenance badge lands (spec P1-1), it renders as a small pill in the label row
— "Yours" (upload) vs. "from {provider}" (enrichment) — reusing the existing
`ProvenanceBadge` component/token vocabulary unchanged. Not required for the P0 cut; the
control works identically without it, just without the provenance hint.

## 4. Accessibility & 3-skin QA checklist

1.1 `[smoke]` Each `StudioImageSlot` preview has `alt="{studio.name} {label}"` (or
    `alt=""` when decorative-only per role — logo/icon are meaningful, poster likewise).
1.2 `[smoke]` "Replace"/"Remove" buttons are real `<button>` elements, reachable by Tab in
    document order (no roving-tabindex needed — this is three independent controls, not a
    list).
1.3 `[agent]` Uploading an oversized/invalid file surfaces the error state without a page
    reload or console error.
1.4 `[human]` Open `/studios/{id}` for a studio with all three slots empty, then one with
    all three filled. **What should look right:** the three rows read as one visual group
    (consistent spacing/border, not three unrelated widgets bolted on); the monogram and
    dashed-box empty states are visually distinct from each other (so an owner doesn't
    mistake "no icon yet" for "no poster yet, but uploadable").
1.5 `[human]` Switch skins (Cinémathèque / Broadcast / Brutalist) on a studio detail page
    with images set. **What should look right:** frame borders and the empty-state dashed
    box use the skin's rule/border tokens, not a hardcoded gray — should look native to
    each skin, not like a leftover default.
