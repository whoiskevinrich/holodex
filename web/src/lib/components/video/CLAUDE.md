# Video components

The video card/grid primitives and the horizontal shelves built on top of them.

| File | Purpose |
|---|---|
| `RecentlyAddedShelf.svelte` | The 20 newest videos, sliced from the already-loaded grid page (no extra request); landing view only. |
| `RelatedShelf.svelte` | One "More with &lt;name&gt;" shelf; self-omits when there are no items so there's never an empty rail. |
| `VideoCard.svelte` | Single video thumbnail card, with cache-busted thumbnail URL and 404-retry-with-backoff while a thumbnail is still generating. `sceneNumber`/`onEditScene` (Films-only, F56/HOLODEX-326) render a scene-number badge that becomes a real `<button>` — a sibling of the card's `<a>`, never nested inside it — when an edit callback is supplied. |
| `VideoGrid.svelte` | Responsive video grid (1–5 columns by viewport); card aspect ratio driven by `data-layout`. |
