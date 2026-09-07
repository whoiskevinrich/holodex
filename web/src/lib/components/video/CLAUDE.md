# Video components

The video card/grid primitives and the horizontal shelves built on top of them.

| File | Purpose |
|---|---|
| `RecentlyAddedShelf.svelte` | The 20 newest videos, sliced from the already-loaded grid page (no extra request); landing view only. |
| `RelatedShelf.svelte` | One "More with &lt;name&gt;" shelf; self-omits when there are no items so there's never an empty rail. |
| `VideoCard.svelte` | Single video thumbnail card, with cache-busted thumbnail URL and 404-retry-with-backoff while a thumbnail is still generating. `sceneNumber`/`onEditScene` (Films-only, F56/HOLODEX-326) render a scene-number badge that becomes a real `<button>` — a sibling of the card's `<a>`, never nested inside it — when an edit callback is supplied. |
| `VideoGrid.svelte` | Responsive video grid — `min(density, capForWidth(viewport))` columns, 1 on a phone up to 16 on an ultrawide; card aspect ratio driven by `data-layout`. Opt-in `stageAligned` (film Scenes only) swaps `1fr` tracks for fixed ones sized off the full column count and caps the track count at the card count, so the grid holds the stage width for a short row and grows past it for a long one — track maths in `$lib/stageGrid`, CSS in `app.css`. |
