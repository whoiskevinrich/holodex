# Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating

**Spec:** [video-owner-mode-editing.md](../specs/video-owner-mode-editing.md) · **Jira:** [HOLODEX-251](https://whoiskevinrich.atlassian.net/browse/HOLODEX-251) / [HOLODEX-252](https://whoiskevinrich.atlassian.net/browse/HOLODEX-252)
**Date:** 2026-08-05 · **Status:** Draft handoff

Four surfaces on [`web/src/routes/media/[id]/+page.svelte`](../../web/src/routes/media/[id]/+page.svelte),
each a layout/extension of existing chrome — **no new visual language**. Tokens only (ADR-021); QA
all three skins. (The person/studio **link picker** is the separate [F40 handoff](person-media-linking-handoff.md) —
this document doesn't repeat it.)

---

## 1. Studio next to the title

**Today:** the studio link renders as a small `text-muted` line *inside* the metadata `dl`, produced
by the `{#if f.canonical === 'studio' && studios.length}` block at
[`+page.svelte:738-747`](../../web/src/routes/media/[id]/+page.svelte#L738), directly under that
field's `SourceSelect`/`CurationFieldRow`.

**Change:** pull the **studio row** (the `SourceSelect` for the `studio` field + its `→ StudioName`
entity link) out of the generic `canonicalResolved` loop entirely and render it once, directly under
the `<header>` block ([L496-507](../../web/src/routes/media/[id]/+page.svelte#L496)). The metadata
`dl` loop skips `canonical === 'studio'` (it already special-cases studio for the link line; this
just moves the special-case earlier and removes the field from the generic list).

```
<header class="space-y-2">
  <h1 class="skin-title text-2xl font-semibold text-ink">{displayTitle}</h1>
  <!-- NEW: studio row -->
  <div class="flex flex-wrap items-center gap-2 text-sm">
    {#if isOwner}
      <SourceSelect field={studioField} decide={(s, mv) => decideField('studio', s, mv)} />
    {:else if studioField?.values?.length}
      <span class="text-ink">{studioField.values[0]}</span>
    {/if}
    {#each studios as s (s.id)}
      <a href={`/studios/${s.id}`} class="text-muted hover:text-accent">→ {s.name}</a>
    {/each}
  </div>
  <div class="flex flex-wrap items-center gap-2 text-sm text-muted"> <!-- existing resolution/duration/year line, unchanged --> </div>
</header>
```

Visitor with no studio value: the row renders nothing (same "field absent → no row" rule as
everywhere else on this page). Owner with no value: `SourceSelect` still renders (consistent with
every other owner-editable replace field — the control itself is the affordance to set one).

## 2. Commentary block

> **Superseded 2026-08-16 ([HOLODEX-115](https://whoiskevinrich.atlassian.net/browse/HOLODEX-115))**:
> this dedicated Commentary section/field was retired — see [video-owner-mode-editing.md](../specs/video-owner-mode-editing.md)'s
> superseded note. `overview` now covers the same "owner-editable long-text field written back to the
> file's Comment tag" need generically, via the shared Metadata `dl`'s `long_text` branch, with no
> bespoke section of its own.

**New**, positioned directly after the studio row (still inside `<header>` or as its own `<section>`
immediately following — either reads fine; **section** is preferred so it gets the same
`space-y-6` rhythm as Tags/People below it):

```
{#if isOwner || commentaryField?.values?.length}
  <section class="space-y-1.5">
    <h2 class="text-xs uppercase tracking-wide text-muted">Commentary</h2>
    <SourceSelect field={commentaryField} decide={(s, mv) => decideField('commentary', s, mv)} />
  </section>
{/if}
```

`commentaryField` is looked up from `canonicalResolved` by canonical key (it stays in the generic
metadata `dl` too, or is filtered out — filtered out is cleaner, mirroring the studio treatment, so
it doesn't render twice). `SourceSelect` already renders a `long_text`-capable manual-entry textarea
for a field whose `Display` is `long_text` (registry `commentary` entry, spec P0-1) — verify this
against `SourceSelect.svelte`'s existing display-mode branch before wiring; if it currently assumes
single-line input for all replace fields, widen it the same way the read-only `long_text` branch at
[L705-710](../../web/src/routes/media/[id]/+page.svelte#L705) already handles block text (this is an
implementation note, not a new design surface — reuse the existing long_text presentation).

**Visitor, no commentary set:** section absent entirely (not empty, not greyed).
**Owner, no commentary set:** `SourceSelect` shows its normal "set a value" affordance — identical to
how Title/Studio look before they're ever decided.

## 3. Poster upload

**Today:** the hover-reveal button row over the player (`.group` wrapper,
[L450-494](../../web/src/routes/media/[id]/+page.svelte#L450)) has exactly one control: a circular
"Regenerate thumbnail" icon button, top-right, `bg-black/60`, opacity-0→100 on hover/focus.

**Change:** add a sibling icon button **left of** Regenerate (so the pair reads as one control
cluster, right-aligned), plus a conditional third:

| Button | Visibility | Icon | Behavior |
|---|---|---|---|
| Upload poster | owner always | upload-tray glyph | Opens a native `<input type="file" accept="image/*">` (hidden, triggered via the button — same pattern as any file-picker trigger, no custom drag-drop for v1); on file select, `POST /media/{id}/poster` multipart; on 201, bump `thumbVersion` (existing cache-bust mechanism used by `regenerateThumbnail` already) so the `<video poster>` and any card thumbnail refetch |
| Regenerate | owner always (unchanged) | existing refresh glyph | unchanged |
| Remove upload | owner **and** `thumbnail_state === 'uploaded'` only | small "×" or trash glyph | `DELETE /media/{id}/poster`; on success, bump `thumbVersion` |

Same chrome as today's button: `rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition
hover:text-ink focus-visible:opacity-100 group-hover:opacity-100`, `aria-label` per action. A
busy/uploading state disables the trigger and spins the icon (mirror `regenerating`'s
`animate-spin` pattern) rather than a separate progress bar — consistent with the existing
Regenerate affordance, and the upload is a single small image so a spinner is sufficient.

**Error path:** on a 400 (oversized/undecodable), show a `text-warn` line under the player (same
placement pattern `tagError`/`refreshStatus` already use elsewhere on this page) rather than a modal.

## 4. File metadata — owner only

**Today:** the File section ([L780-794](../../web/src/routes/media/[id]/+page.svelte#L780)) is an
unconditional `<section>`.

**Change:** wrap it in `{#if isOwner}` — no other change. It disappears for a visitor exactly like
the "Manage" delete controls and raw-enrichment disclosures already do elsewhere on this page (same
`isOwner` derivation, same pattern, zero new chrome).

## Responsive / motion / a11y

No new breakpoints or motion — every element here reuses an existing component
(`SourceSelect`, the hover-button chrome, the section/`dl` rhythm) at its existing responsive
behavior. The only new interactive element is the poster-upload trigger + hidden file input: label
it `aria-label="Upload poster"`, and on the hidden input use `class="sr-only"` (existing utility) —
never `display:none` on a file input inside a button-triggered pattern if any assistive tooling
expects it focusable; triggering via `button.click()` on the input ref is the standard pattern and
keeps the native `<input>` out of the tab order entirely, which is fine since the visible button *is*
the tab stop.

## QA checklist

1. [smoke] Visitor view: File section absent; Commentary section absent when no value; studio shows
   plain text (no edit control) next to the title when set, nothing when unset.
2. [agent] Owner view, `rg 'zinc-|sky-|#'` over any new/changed markup in `+page.svelte` — clean.
3. [agent] Poster upload round-trip: upload → `thumbnail_state` becomes `uploaded` → rescan/backfill
   leaves it untouched → Remove reverts to a re-derived poster.
4. [human] All three skins (Cinémathèque / Broadcast / Brutalist): the new upload/remove icon buttons
   read at the same visual weight as the existing Regenerate button; the studio row doesn't crowd the
   title on narrow viewports (< 640px) — wraps, doesn't truncate awkwardly.
