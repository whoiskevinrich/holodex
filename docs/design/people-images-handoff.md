# Design Handoff — People Images (F25)

**Status**: Proposed
**Date**: 2026-06-16
**Spec**: [People Images (F25)](../specs/people-images.md) · **ADR**: [ADR-038](../architecture/ADR-038-person-images.md) · **System pattern**: [people-images-design-system.md](people-images-design-system.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Stack: SvelteKit SPA + Tailwind v4 (CSS-first, `@theme inline`). All values below are **token
> references**, never literals. "Owner" = the ADR-030 capability flag; non-owners never see mutation UI.

---

## Overview

Give people faces. Four image **roles** per person — `headshot` (1:1), `banner` (16:9), `poster` (2:3),
and a free-form `extra` gallery — render across three surfaces (people list, person page, video page).
Empty roles fall back to a **themed + gendered placeholder** (never a broken-image box). The owner can
upload, replace, delete, reorder gallery extras, and **promote** a gallery extra into a core slot with a
zoom/crop step. Everything is built on the shared `.portrait-frame` hook class so skins own the look.

## Surfaces & layout

### A. People list (`/people`) — headshot

- Each person card gains a **`PersonAvatar` (1:1)** above the existing name + video count.
- Grid unchanged; the avatar sits in a `.portrait-frame--1x1` well at the card's full width; name/count
  below, existing spacing tokens.
- **Lazy**: only first-viewport avatars `eager`; the rest load on scroll. The frame reserves the square
  so the grid never reflows.

### B. Person page (`/people/[id]`) — banner hero + gallery + owner tools

Top-to-bottom:
1. **`PersonBanner` (16:9)** hero spanning the content width, with the **`PersonAvatar` (1:1)** overlapping
   bottom-left (avatar overlaps the banner's lower edge by ~`size-md`/2), name + alias panel to its right.
   On empty banner the hero shows the themed 16:9 placeholder; the page never looks "missing."
2. Existing **Aliases** (F23) and **Enrichment** (F22) panels, unchanged.
3. **`PersonGallery`** — the `extra` images in a responsive grid (see Responsive). Person-page only.
4. **Owner tools** (owner flag only): an upload control per core slot (over the banner/avatar on hover)
   + a gallery "Add image" tile; each gallery item gets hover affordances (set-as-headshot/banner/poster,
   delete, drag-handle to reorder).

### C. Video page (`/media/[id]`) — poster cards

- The existing text "People" chips become **`PersonPoster` (2:3)** cards in a horizontal wrap, each
  linking to `/people/{id}` with the name beneath. Missing poster ⇒ 2:3 placeholder.
- This is the spec'd video-page surface; do **not** also add avatars to credit chips elsewhere.

## Design tokens used

| Token (utility) | Usage |
|---|---|
| `--surface-2` (`bg-surface-2`) | Image well background / placeholder field |
| `--rule` (`border-rule`) | Frame border (visible in mono skins) |
| `--radius` (`rounded-theme`) | Frame corners (0 on Broadcast/Brutalist) |
| `--ink` (`text-ink`) | Person name |
| `--muted` (`text-muted`) | Counts, placeholder glyph/label, role captions |
| `--accent` (`bg-accent`/`text-accent`/`text-accent-ink`) | Focus/hover ring, owner action buttons, drag-active |
| `--warn` (`text-warn`/`border-warn`) | Over-cap + upload-error messages (never the accent) |
| `--font-display` (`.skin-title`) | Person name heading |

No new tokens. Skin flourishes (letterbox/scanline/hairline) attach to `.portrait-frame` in `app.css`
under `[data-theme]`, mirroring `.video-frame`.

## Components

| Component | Variant/Role | Key props | Notes |
|---|---|---|---|
| `PersonAvatar` | headshot 1:1 | `personId, src, gender, name, size, eager` | List card + person header |
| `PersonBanner` | banner 16:9 | `personId, src, gender, name` | Person hero |
| `PersonPoster` | poster 2:3 | `personId, src, gender, name` | Video credits |
| `PersonGallery` | extra | `personId, items[], owner` | Grid + (owner) add/reorder/promote |
| `PlaceholderImage` | resolved | `role, gender, skin` | Rendered by the frame; programmatic SVG |
| `ImageUploader` (owner) | — | `personId, role` | Dropzone/file-pick; client-validates then POSTs multipart |
| `CropEditor` (owner, P1) | promote | `sourceImageId, targetRole` | Zoom/crop a **copy** to the target ratio |

## States & interactions

| Element | State | Behavior |
|---|---|---|
| Any frame | Loading | Placeholder shown immediately; subtle shimmer on the well; **no layout shift** (box reserved) |
| Any frame | Empty | Themed/gendered placeholder (resting state, not an error) |
| Any frame | Ready | Real image, `object-fit: cover`; URL is version-stamped (`?v=`) + `immutable` cached |
| Any frame | Error (img 404/decoded fail) | Silently falls back to placeholder; **never** a broken-image glyph |
| Core slot (owner) | Hover | Edit/replace + delete affordances fade in over the frame; `--accent` ring |
| Upload | In progress | Control disabled + spinner; optimistic frame swap on success |
| Upload | Rejected (type/size/decode) | Inline `text-warn` words (not color-only); nothing written; control re-enabled |
| Gallery (owner) | At 20 extras | "Add" tile disabled + `border-warn`/`text-warn` "Gallery is full (20 max)." (server also enforces) |
| Gallery item (owner) | Drag | Drag-handle reorders; `--accent` drop indicator; persists `sort_order` |
| Promote (owner) | Crop | Modal crop editor on a **copy**; saving creates/replaces the core role; gallery original untouched |

## Responsive behavior

| Breakpoint | Changes |
|---|---|
| Desktop (>1024px) | List: existing column count + avatar. Person: banner full width, avatar overlap, gallery 4–6 cols. Video: poster cards ~`size-lg` wide. |
| Tablet (768–1024px) | Gallery 3 cols; banner keeps 16:9; avatar overlap reduced. |
| Mobile (<768px) | Gallery 2 cols; avatar drops below the banner (no overlap) or centers; poster cards scroll horizontally; upload affordances become an explicit "Edit" button (no hover on touch). |

## Edge cases

- **No images at all** (new person): all three core surfaces show placeholders; gallery shows just the
  owner "Add image" tile (owner) or nothing (viewer). Page reads complete, not broken.
- **Very long name**: truncates to 2 lines with ellipsis under avatars/posters; full name in `title`/`alt`.
- **Non-Latin / accented names**: `alt` + placeholder initials must render real glyphs in the blocky
  Broadcast/Brutalist fonts (CJK tofu check).
- **Slow connection**: placeholder-first guarantees immediate paint; real image fades in when ready.
- **Stale cache after replace**: the new `?v=` stamp guarantees the browser fetches the new image; verify
  no old image lingers.
- **Wrong-ratio upload**: stored normalized; `object-fit: cover` crops to the frame — uploader is
  responsible for a sane source crop (except promote, which offers the crop editor).

## Animation / motion

| Element | Trigger | Animation | Duration | Easing | Notes |
|---|---|---|---|---|---|
| Frame | Image ready | Cross-fade placeholder→image | 150–200ms | ease-out | Disabled under `prefers-reduced-motion` |
| Well | Loading | Shimmer sweep | ~1.2s loop | linear | Reduced-motion → static well |
| Owner affordances | Hover | Fade-in overlay | 120ms | ease | Touch → no hover; explicit Edit button |
| Gallery reorder | Drag | Position transition | 150ms | ease | Reduced-motion → instant |

## Placeholder system (programmatic SVG)

A resolver maps `(skin × role × gender-bucket)` → an SVG composed from the active skin's tokens: a
role-shaped silhouette on `--surface-2`, accented per skin, neutral by default. **Three buckets only**
(`male`/`female`/`neutral`); `nonbinary` and unknown → `neutral`. 27 generated cells; an owner override
dir may replace any cell. The placeholder is requested with the active `?skin=` (gender resolved
server-side). Placeholders carry `alt="No photo of {name}"` so they read as intentional, not broken.

## Accessibility notes

- **Focus order**: list avatar is part of the card link (single tab stop). On the person page: banner →
  avatar/name → aliases → enrichment → gallery (each gallery item focusable) → owner controls. Crop modal
  traps focus and returns it to the trigger on close.
- **ARIA/roles**: images are `<img>` with meaningful `alt` (person name; placeholders "No photo of …").
  Gallery is a list; owner action buttons are real `<button>`s with `aria-label` (e.g. "Set as headshot",
  "Delete image", "Reorder"). Reuse the `EnrichPicker` **roving-tabindex** pattern for any list-select.
- **Keyboard**: upload control is keyboard-openable; gallery reorder offers a keyboard alternative
  (move-up/move-down buttons, not drag-only); crop editor zoom/pan operable by keyboard.
- **Contrast**: placeholder glyph `--muted` on `--surface-2` must hit ≥3:1 in each skin (a11y review).
- **Motion**: all transitions respect `prefers-reduced-motion` (as `.video-grid` does today).
