# Person components

Person-detail UI: identity (aliases/merge), the headshot/banner/poster image system, and the
gallery/viewer stack. Some pieces are reused verbatim by the studio page (not tag — RD7).

| File | Purpose |
|---|---|
| `AliasPanel.svelte` | Owner-curated alternate names (drives search + scan routing); hosts the merge picker and the homonym-collision card. Entity-generic (person + studio); studio adds a Rename affordance. |
| `NationalityFlags.svelte` | Renders a person's nationality flag(s) beside the hero name; renders nothing when none resolve. |
| `PersonAvatar.svelte` | 1:1 headshot — thin wrapper over `PersonImageFrame` fixing the `headshot` role. |
| `PersonBanner.svelte` | Wide 8:3 hero banner over `PersonImageFrame`, with a scroll-driven parallax shift. |
| `PersonGallery.svelte` | Horizontally-scrollable extra-image row; owner can add/promote (via `CropEditor`)/delete/reorder. |
| `PersonGalleryModal.svelte` | Full-page read-only gallery grid, opened from `PersonGallery`'s "Gallery (N)" trigger. |
| `PersonImageFrame.svelte` | Shared frame backing Avatar/Banner/Poster — builds the skin-aware, cache-busted image URL; server always returns a themed placeholder so there's never a broken-image glyph. |
| `PersonImageViewer.svelte` | Full-page single-image viewer modal with prev/next, opened from a gallery thumbnail. |
| `PersonPoster.svelte` | 2:3 poster card for the video-credits surface — thin wrapper over `PersonImageFrame`. |
| `PersonPosterCard.svelte` | Poster-grid card for the People index (F55) — `PersonPoster`'s frame + a name/count block below, mirroring `VideoCard`'s title-below-thumbnail layout; conditional border/hover-lift/focus-ring chrome. |
| `PersonPosterGrid.svelte` | Responsive grid of `PersonPosterCard`s (F55) — mirrors `VideoGrid`'s density→column computation, doubled (RD8). |
| `StudioImageSlot.svelte` | Studio-only (F51, ADR-079): upload/replace/remove control for one of a studio's three core image roles (icon/logo/poster) — no gallery/promote/viewer, unlike Person's image system. Filed here per this folder's "image system" mechanism grouping, not by entity. |
