# Entity components

Entity-generic (person | studio | tag) building blocks: searching for an entity, merging one
into another, and the shared video-list body for an entity's detail page.

| File | Purpose |
|---|---|
| `EntityPicker.svelte` | Merge picker: search/pick another entity, then an informed confirm showing both video counts (never a silent merge). Generalized from F23's PersonPicker. |
| `EntityPickerDialog.svelte` | Local entity-search picker for the Extraction tab's People/Studio field edits — searches the app's own entities (no external round trip); confirming never writes, just hands back a name. |
| `EntityVideos.svelte` | Shared body for person/[id] and tag/[id] detail pages: back-link, title, video count, and the video grid, with optional `hero`/`detail` snippets. |
| `MergeCanonicalDialog.svelte` | Step two of a multi-select merge: "keep which name?" — the rest fold into the survivor. Shared by /people and /tags. |
