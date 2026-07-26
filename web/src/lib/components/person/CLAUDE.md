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
