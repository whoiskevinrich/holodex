# Writeback components

Committing resolved values back to source: metadata file tags (F28) and person image crops
(F25.15).

| File | Purpose |
|---|---|
| `CropEditor.svelte` | Promote-with-crop editor: previews a gallery image inside the target core-role aspect frame with zoom/drag, then renders a WYSIWYG canvas crop as the uploaded image. |
| `WritebackFormDialog.svelte` | Batch writeback modal: all writable resolved fields pre-filled, editable, and toggleable; writes sequentially with per-row progress. |
