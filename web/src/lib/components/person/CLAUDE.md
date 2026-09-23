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
| `PersonHoverCard.svelte` | The hover card body (F68, HOLODEX-431, design handoff): header block = the profile `<a>` (48 px headshot, display name + `NationalityFlags`, meta line `age · N videos · N films` with absent segments dropped, up to three aliases), the owner's F65.8 `CompletenessRing` as a **sibling** of that link, then Videos (`#videos`) · Films (`#films`, only when > 0) · `ProviderLinkBadge`s. `card` null = loading (name line + empty frame). Resets `normal-case tracking-normal font-normal` because a trigger can sit inside RelatedShelf's uppercase heading. Mounted only by `PersonLinkChip`. |
| `PersonImageFrame.svelte` | Shared frame backing Avatar/Banner/Poster — builds the skin-aware, cache-busted image URL; server always returns a themed placeholder so there's never a broken-image glyph. |
| `PersonImageViewer.svelte` | Full-page single-image viewer modal with prev/next, opened from a gallery thumbnail. |
| `PersonLinkChip.svelte` | The one person-link component (F68) and the card's only mount: a **transparent wrapper** — the consumer renders its own `<a>` as children (classes, title, handlers, roving tabindex all untouched) and the chip finds it in the DOM. Opens the card on hover-intent (250 ms) or at once on focus; 150 ms leave grace across the gap; Escape/outside click via `use:dismissable`; one card open app-wide; one measure on open flips above / clamps inside the viewport (`placeCard`); never mounts under `pointer: coarse`. `wrapClass` is `relative inline-block` by default, `relative block` for block consumers (PosterTile, search rows). Consumers: film billed chips, `/search` page rows (`SearchResultsPanel` page variant), `RelatedShelf` person title, `PosterTile` via `PeopleGrid`. |
| `PersonPoster.svelte` | 2:3 poster card for the video-credits surface — thin wrapper over `PersonImageFrame`. |
| `PersonPosterCard.svelte` | Poster-grid card for the People index (F55) — `PersonPoster`'s frame + a name/count block below, mirroring `VideoCard`'s title-below-thumbnail layout; conditional border/hover-lift/focus-ring chrome. Owner list items carry `completeness` (F65.5); the caption's count line then leads with a `completeness/CompletenessRing` (size `row`) before the count — the /people row rule applied to the poster caption. |
| `PersonPosterGrid.svelte` | Responsive grid of `PersonPosterCard`s (F55) — mirrors `VideoGrid`'s density→column computation, doubled (RD8). |
| `PersonViewToggle.svelte` | List/Poster segmented control for the People index (F55) — same shell as `sort/SortToggle.svelte`, `aria-pressed` since it's a 2-way toggle rather than a 3-way sort. |
| `personImages.ts` | `/people/{id}/images` cached per session — the same contract as `personCard.svelte.ts` (one request per person, a rejected promise evicted so a Retry re-requests), added because `api.getPersonImages` had none of its own and the F70 compare panel re-fetched the set on every reopen. Also `stripGallery`, the compare strip's slice rule: gallery minus the headshot, capped at 4, **no `+N`**. Pinned by `personImages.test.ts`. |
| `personCard.svelte.ts` | The chip's plumbing (F68): per-session card cache (`loadPersonCard`, one request per person; failures not cached; `invalidatePersonCard` after a ring refresh), the app-wide open latch (`hoverCard.openToken`), `cannotHover()`, and the pure `placeCard` (flip above when `spaceBelow < cardH + 6` *and* the card fits above — neither side fits → stay below, HOLODEX-444; clamp x by the overshoot past the 16 px gutter, never past the left gutter) pinned by `personCard.test.ts` to the OQ1 probe numbers. |

## Rules

- **The hover card never owns the link (F68).** `PersonLinkChip` wraps the consumer's own `<a>`;
  it does not render one. If a surface's link carries semantics (the film page's dashed pill, the
  search row's `role="option"` + roving tabindex), those survive the wrapper untouched. A new
  surface adopts the card by wrapping its existing link, never by swapping it for a chip-owned one.
- **Card facts come from `GET /people/{id}/card` only** (`api.personCard`), which runs the same
  `personResolve` the profile does — never assemble age/nationality client-side, and never feed
  the card the detail read. Absent keys are absent segments: no "—", no "unknown".
- **The ring inside the card is a sibling of the header link** (F65.8: a button may not nest in
  an anchor); `onrefreshed` invalidates the cache and the chip re-fetches so the card redraws.
