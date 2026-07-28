# Curation components

Per-field source-of-truth and value display for resolved fields (F30/F36/F39/F44): value chips,
source selection, read-only auto-registered field rows, and the promote-to-canonical flow.

| File | Purpose |
|---|---|
| `AutoFieldRows.svelte` | Read-only rows for display-only auto-registered non-canonical fields, shared by video/person/studio detail pages. Owner also sees "Attach to…" / "Promote" pills opening `ClaimFieldEditor` / `PromoteFieldEditor`, plus the post-attach confirmation strip with Undo. |
| `ChipValueList.svelte` | Read-only pill list for a `chips`-display field's values — the control-free cousin of `CurationChip`. |
| `ClaimFieldEditor.svelte` | Inline editor behind an auto-registered row's "Attach to…" pill: picks the canonical field a provider key attaches to (F49), with the per-provider checklist and the outcome preview. |
| `CurationChip.svelte` | One value chip in a curated field: value + provenance + (owner) inline edit/remove/"don't write" toggle. |
| `CurationFieldRow.svelte` | One curated field row: renders value chips plus an owner "+ Add" affordance; entity-generic since F37 (video/person). |
| `FacetFilter.svelte` | Typeahead multi-select for a facet (people or tags), filtering the pre-fetched option list client-side. |
| `MappedFacets.svelte` | Loads the mapped-facet list and lets the browse page bind selected values per canonical facet. |
| `PromoteFieldEditor.svelte` | Shared inline editor driving promote/edit/de-promote of a non-canonical field (label/render/group/order). |
| `PromotedFieldEdit.svelte` | Owner-only Edit/Remove-promotion affordance on an already-promoted field row; opens `PromoteFieldEditor` in edit mode. |
| `SourceSelect.svelte` | Per-field source-of-truth control (F36) — one row of source-tagged, single-select value chips plus a Custom chip and "file out of sync" warning. |
| `UrlValueList.svelte` | Renders a `url`-display field's values as scheme-gated links, with optional hostname-only text and a leading provider brand icon. |
