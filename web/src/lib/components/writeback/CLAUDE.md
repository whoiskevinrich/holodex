# Writeback components

Committing resolved values back to source: metadata file tags (F28) and person image crops
(F25.15).

| File | Purpose |
|---|---|
| `CropEditor.svelte` | Promote-with-crop editor: previews a gallery image inside the target core-role aspect frame with zoom/drag, then renders a WYSIWYG canvas crop as the uploaded image. |
| `WritebackFormDialog.svelte` | Batch writeback modal: all writable resolved fields pre-filled, editable, and toggleable; writes sequentially with per-row progress. |
| `WritebackBatchDialog.svelte` | N-video writeback progress dialog (HOLODEX-239): confirm → aggregate pending/running/done/failed progress bar → completion, for a tag-scoped manual sync batch (single or bulk). Sibling of `WritebackFormDialog`, not a mode of it — no per-video field list, only aggregate counts. An `initialBatch` prop (F57, HOLODEX-285) seeds progress directly for a batch a caller already started server-side (the Film-studio cascade), skipping the confirm step and `trigger()` entirely. |
