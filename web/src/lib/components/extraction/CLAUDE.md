# Extraction components

The Extraction review queue (F48, ADR-067): filename/tag-derived field suggestions the owner
stages then commits in a batch.

| File | Purpose |
|---|---|
| `ExtractionPreviewDialog.svelte` | Preview-before-write dialog — reuses `WritebackFormDialog`'s chrome but shows a static old → new diff per staged row; each checked row resolves sequentially. |
| `ExtractionQueueRow.svelte` | One field row grouped under its video. Entity fields (People/Studio) render one chip per parsed name with an edit-in-place picker; scalar fields (Title, Release date) keep filename/tag/Edit UI. Staged picks commit together via the preview dialog. |
