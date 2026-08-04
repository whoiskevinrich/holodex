# Entity components

Entity-generic (person | studio | tag) building blocks: searching for an entity, merging one
into another, and the shared video-list body for an entity's detail page.

| File | Purpose |
|---|---|
| `CategoryPicker.svelte` | Category assign/remove picker (HOLODEX-240): search-or-create (`mode="add"`) or search-only (`mode="remove"`) a category for the selected tags — single-step, no informed confirm (assign/remove are reversible, unlike merge). |
| `EntityPicker.svelte` | Merge picker: search/pick another entity, then an informed confirm showing both video counts (never a silent merge). Generalized from F23's PersonPicker. |
| `EntityPickerDialog.svelte` | Local entity-search picker for the Extraction tab's People/Studio field edits — searches the app's own entities (no external round trip); confirming never writes, just hands back a name. |
| `EntityVideos.svelte` | Shared body for person/[id], studio/[id], and tag/[id] detail pages: back-link, title, video count, the video grid, and scroll restoration (HOLODEX-248, keyed by the `scrollKey` prop), with optional `hero`/`detail` snippets. |
| `MergeCanonicalDialog.svelte` | Step two of a multi-select merge: "keep which name?" — the rest fold into the survivor. Shared by /people and /tags. |
| `PickerShell.svelte` | Shared dialog chrome for `EntityPicker`/`CategoryPicker`: backdrop, `role="dialog"` wrapper, focus trap, trigger-focus save/restore, Escape-to-close, rise-in animation. Title/body are step-specific, passed as snippets. |
| `SearchResultsPanel.svelte` | Grouped/tabbed search results (NS1/NS5, HOLODEX-249) — the All/People/Videos/Studios/Tags tab row + capped result rows, roving tabindex. Shared by the nav box's live-typing dropdown (`variant="dropdown"`) and `/search`'s page body (`variant="page"`), per "reuse, don't fork." Spans video results too, not just person/studio/tag. |
