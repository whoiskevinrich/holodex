# Curation components

Per-field source-of-truth and value display for resolved fields (F30/F36/F39/F44): value chips,
source selection, read-only auto-registered field rows, and the promote-to-canonical flow.

| File | Purpose |
|---|---|
| `AutoFieldRows.svelte` | Read-only rows for display-only auto-registered non-canonical fields, shared by video/person/studio detail pages. Owner also sees "Attach to…" / "Promote" pills opening `ClaimFieldEditor` / `PromoteFieldEditor`, plus the post-attach confirmation strip with Undo. |
| `ChipValueList.svelte` | Read-only pill list for a `chips`-display field's values — the control-free cousin of `CurationChip`. |
| `ClaimFieldEditor.svelte` | Inline editor behind an auto-registered row's "Attach to…" pill: picks the canonical field a provider key attaches to (F49), with the per-provider checklist and the outcome preview. |
| `CurationChip.svelte` | One value chip in a curated field: value + provenance + (owner) inline edit/remove/"don't write" toggle. Radio mode takes an optional `placeholder` for an empty value (default `—`; the writeback dialog passes `not read back` for an ADR-093 baseline chip). |
| `CurationFieldRow.svelte` | One curated field row: renders value chips plus an owner "+ Add" affordance; entity-generic since F37 (video/person). |
| `FacetFilter.svelte` | Typeahead multi-select for a facet (people or tags), filtering the pre-fetched option list client-side. |
| `MappedFacets.svelte` | Loads the mapped-facet list and lets the browse page bind selected values per canonical facet. |
| `PromoteFieldEditor.svelte` | Shared inline editor driving promote/edit/de-promote of a non-canonical field (label/render/group/order). |
| `PromotedFieldEdit.svelte` | Owner-only Edit/Remove-promotion affordance on an already-promoted field row; opens `PromoteFieldEditor` in edit mode. |
| `SourceBadge.svelte` | Tier-2 per-field source-of-truth control (F56) — collapsed `ProvenanceBadge` at rest, click-to-expand `CurationChip` radio row with staged (not auto-committing) selection + explicit Confirm/Cancel. Used on Video (Metadata fields), Person (all replace fields but bio), Studio and Film (all replace fields). The `name` field's mount is `entity/DisplayNameLine.svelte` (HOLODEX-378), which passes `showValue={false}` — the only mount that omits the resting value span, because the page heading already renders it; every field-list mount keeps the default. |
| `SourceChipRow.svelte` | The Tier-2 chip row as a staged, embeddable `role="radiogroup"` (HOLODEX-400): `CurationChip` radios + the Custom opener/inline input, roving tabindex, arrow keys stage, Escape inside the Custom input cancels only the input (and stops propagating). Binds `stagedKey`/`stagedCustomValue`; never commits — the embedder owns Confirm. `baselinePlaceholder` relabels an empty baseline chip. Lifted from `SourceBadge`'s expanded state for the writeback dialog; `SourceBadge` still carries its own copy (folding it onto this component is a follow-up). |
| `SourceEditModal.svelte` | Tier-2 per-field source-of-truth control, modal variant (HOLODEX-303) — same staged-then-Confirm contract as `SourceBadge`, but a `ConfirmDialog`-chrome modal whose body is `SourceRadioList` (one full-width radio row per candidate source plus an inline Custom textarea) instead of an inline chip row. Standard pattern for `long_text` fields going forward (chip-expand doesn't work for paragraph-length values): used by the Person page's header bio, the Video page's Overview block (HOLODEX-365) and the Film page's Description block (HOLODEX-364), each behind an owner-only pencil in the heading. Values clamp to four lines via `SourceValueClamp` (HOLODEX-417). |
| `SourceImageTiles.svelte` | Image-tile chooser for an `image_url` replace field (HOLODEX-403, ADR-101 D3) — the third chooser shape: one `role="radio"` tile per `SourceChip` (the entity's own image as `·file`, each provider's), no Custom; `baselineImage` supplies what the `·file` tile shows (the served poster) and `baselinePlaceholder` two lines for when there is nothing to show (an owner upload — ADR-049). Chip-row keyboard contract verbatim; binds `stagedKey`; the embedder owns Confirm. Used by the writeback dialog. |
| `SourceRadioList.svelte` | Stacked full-width candidate rows for a `long_text` replace field (HOLODEX-400): one native radio row per `SourceChip` plus the inline Custom textarea. Binds `stagedKey`/`stagedCustomValue`; the embedder owns Save/Confirm. `baselinePlaceholder` relabels an empty baseline row (`No value` by default). Lifted out of `SourceEditModal`, which now renders it inside `ConfirmDialog`; the writeback dialog embeds it directly for its `long_text` rows. |
| `SourceSelect.svelte` | Per-field source-of-truth control (F36) — one row of source-tagged, single-select value chips plus a Custom chip and "file out of sync" warning. Kept alive only for the one Tier-1 field it still owns: Video's Studio field (Tier-1 per spec, pending HOLODEX-271's relationship popover) — the Person name moved to `NameEditControl` (HOLODEX-269) and its display decision to `DisplayNameLine` (HOLODEX-378). `SourceBadge` supersedes it everywhere else. |
| `SourceValueClamp.svelte` | A source's paragraph-length value inside `SourceEditModal`, clamped to four lines with a `btn-quiet` Show more / Show less toggle that is absent when the text fits (HOLODEX-417). Deliberately not `ExpandableText`: that is the one look for *displayed* prose (muted, no styling props, HOLODEX-365); this is evidence under a radio and keeps the row's ink tone. |
| `UrlValueList.svelte` | Renders a `url`-display field's values as scheme-gated links, with optional hostname-only text and a leading provider brand icon. |

## Rules

- **`SourceBadge` renders its badge for every field it is mounted on, single-source included**
  (F60 RD12, HOLODEX-377). It used to hide itself unless 2+ sources were on offer, which left a
  field with exactly one candidate — a filename-only edition, a tag-only anything — with no way to
  type a custom value. Mounting the component is the page's "this field is curatable" decision;
  don't add a second gate inside it. The empty `—` baseline chip in the row is the F37 RD3
  blank-pin and stays.
