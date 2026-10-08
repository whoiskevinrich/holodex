# Design System — People Images (F25) pattern extension

**Status**: Proposed
**Date**: 2026-06-16
**Mode**: extend
**Refs**: spec [People Images (F25)](../specs/people-images.md) · [ADR-038](../architecture/archive/ADR-038-person-images.md) · theming contract [`theming.md`](theming.md) · [ADR-021](../architecture/archive/ADR-021-frontend-theming-and-skins.md)

This extends the existing design system; it introduces **no new color/type/radius tokens**. Person
images reuse the semantic token contract and follow the same "the look attaches to a shared hook
class" model as `.video-frame`. The only system-level addition is **one hook class (`.portrait-frame`)
with three aspect variants** and a small family of components built on it.

---

## Problem

People are rendered as bare text everywhere (lists, person header, video credits). F25 adds four image
**roles** — `headshot` 1:1, `banner` 16:9, `poster` 2:3, and a free-form `extra` gallery — each of which
must: (a) crop cleanly to a fixed ratio, (b) show a themed + gendered **placeholder** when empty, (c)
load without layout shift, and (d) read correctly in the Cinémathèque look. We need a reusable frame, not
per-surface markup.

## Existing patterns (and why they're not enough)

| Related | Shared | Missing |
|---|---|---|
| `.video-frame` (16:9 thumbnail well) | Aspect-locked well, flourishes attach here, lazy fill | Only 16:9; tied to video thumbnails + the `.video-grid` counter; no 1:1 / 2:3; no gendered placeholder |
| `EnrichPicker` / `ProvenanceBadge` (F22) | Owner-gated mutation UI, provenance badging | Not image-aware; no upload/crop |
| Thumbnail `<img>` render (retry-on-404, lazy) | Lazy load, placeholder-first | Single ratio; no role/placeholder resolution |

## Proposed system addition

### Hook class: `.portrait-frame` (+ `--1x1` / `--16x9` / `--2x3`)

A single aspect-locked well, parallel to `.video-frame`. The base class owns the shared mechanics
(token-driven background, overflow clip, `object-fit: cover` on the child image, lazy placeholder
backdrop); the three modifier classes only set the aspect ratio. **Flourishes attach here in
`app.css`** — never in component markup. In Cinémathèque ([theming.md](theming.md), the only look)
the frame is a `--rule` hairline at `--radius`, and the banner variant fades into `--bg` at its foot.

```
.portrait-frame { background: var(--surface-2); overflow: hidden; border-radius: var(--radius); }
.portrait-frame--1x1  { aspect-ratio: 1 / 1; }
.portrait-frame--16x9 { aspect-ratio: 16 / 9; }
.portrait-frame--2x3  { aspect-ratio: 2 / 3; }
.portrait-frame > img { width: 100%; height: 100%; object-fit: cover; display: block; }
```

Tokens used: `--surface-2` (well), `--rule` (border), `--radius` (corner), `--muted`
(placeholder glyph/label), `--accent` (focus/hover ring), `--warn` (error overlay). No literals.

### Components

| Component | Built on | Role(s) | Where |
|---|---|---|---|
| `PersonAvatar` | `.portrait-frame--1x1` | `headshot` | People list cards, person header, (optional) video credit chips |
| `PersonBanner` | `.portrait-frame--16x9` | `banner` | Person page hero |
| `PersonPoster` | `.portrait-frame--2x3` | `poster` | Video detail page credits |
| `PersonGallery` | grid of `.portrait-frame` (natural ratio) | `extra` | Person page only |
| `PlaceholderImage` | any `.portrait-frame` | resolved fallback | Rendered by the frame when a role is empty |
| `ImageUploader` (owner) | dropzone + role select | upload/replace | Person page (owner) |
| `CropEditor` (owner, P1) | zoom/crop on a copy | promote `extra`→core | Person page (owner) |

#### `PersonAvatar` / `PersonBanner` / `PersonPoster` — props

| Prop | Type | Default | Description |
|---|---|---|---|
| `personId` | number | — | Resolves the role's serving URL |
| `role` | `'headshot'\|'banner'\|'poster'` | per component | Fixed per component |
| `src` | string \| null | null | Version-stamped real-image URL (`?v=`) from the read model; null ⇒ placeholder |
| `gender` | `'male'\|'female'\|'nonbinary'\|'unknown'` | `'unknown'` | Drives placeholder bucket (collapses to male/female/neutral) |
| `name` | string | — | `alt` text + placeholder initials/label |
| `size` | `'sm'\|'md'\|'lg'` | `'md'` | Layout size; aspect fixed by the frame |
| `eager` | boolean | false | First-viewport images opt out of lazy loading |

### States (every component)

| State | Visual | Behavior |
|---|---|---|
| Loading | `--surface-2` well + subtle shimmer; placeholder shown immediately | No layout shift (frame reserves the box) |
| Empty (no real image) | Resolved themed/gendered **placeholder** | Not an error; the default resting state |
| Ready | Real image, `object-fit: cover` | Version-stamped URL; `immutable` cached |
| Error (real image failed) | Falls back to placeholder; no broken-image glyph | Logs; never shows a 404 box (matches thumbnail contract) |
| Owner hover (P1) | Edit/replace/delete affordance over the frame | Owner-gated; hidden for viewers |
| Over-cap (gallery) | Disabled add + `--warn` inline message | Server also enforces (F25.8) |

### Placeholder system (programmatic SVG)

Placeholders are **generated SVG**, not bundled binaries (ADR-038 §4). A resolver maps
`(role × gender-bucket)` → an SVG built from the Cinémathèque palette: a role-shaped silhouette on
`--surface-2`, neutral by default. Three buckets only (`nonbinary` + unknown →
`neutral`). The matrix is 3 core roles × 3 buckets = 9 generated cells; an owner override dir
can replace any cell (F25.14, not built yet).

## Accessibility

- **Role**: decorative frames are `img` with meaningful `alt` (the person's name); placeholders carry
  `alt` too (e.g. "No photo of {name}") so they aren't announced as broken.
- **Keyboard**: gallery items are focusable; owner edit affordances reachable by Tab (reuse the roving
  pattern from `EnrichPicker` where a list is involved). The crop editor traps + returns focus (modal).
- **Contrast**: placeholder glyph uses `--muted` on `--surface-2` — verify ≥ 3:1 (a11y
  review covers this).
- **Motion**: shimmer/stagger respects `prefers-reduced-motion` (as `.video-grid` already does).

## Do / Don't

| ✅ Do | ❌ Don't |
|---|---|
| Use `.portrait-frame--*` for any person image well | Hand-roll an `aspect-ratio` box per surface |
| Attach flourishes in `app.css` on the hook class | Put `zinc-*`/hex/named fonts/fixed radii in the component |
| Reserve the box so the placeholder shows first | Let images pop in and shift layout |
| Let the frame fall back to the placeholder on error | Render a broken-image / 404 state |
| Drive placeholder by resolved gender bucket | Branch on raw provider gender codes in the component |

## Resolved decisions

- **Video page uses `PersonPoster` (2:3) cards, not avatars on chips.** The spec names poster as the
  video-page surface; we don't also sprinkle 1:1 avatars onto credit chips elsewhere. (Confirmed at
  design-critique.)
- **Gallery layout = uniform `.portrait-frame--1x1` thumbnails opening a lightbox**, not masonry — keeps
  the grid token-clean and predictable; owner reorder via drag-handle or keyboard
  move-up/down.
