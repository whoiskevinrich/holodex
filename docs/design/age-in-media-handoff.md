# Design Handoff: Age-in-media badge on the cast poster card (HOLODEX-173)

**Status**: Design decided (developer handoff)
**Date**: 2026-07-12
**Spec**: [`docs/specs/age-in-media.md`](../specs/age-in-media.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[`theming.md`](theming.md) — **tokens only, QA all three skins**

A small corner badge on each cast member's poster card shows their age at the time of the video's
release — a computed, read-only number with no home in the caption below the poster (see "Resolved
decision" below for why not).

## Resolved decision

Three placements were mocked up against the real component structure and tokens; **corner badge,
number only** was chosen — it reuses `VideoCard.svelte`'s existing duration-badge pattern exactly
(same slot, same treatment), and unlike an inline-caption option, it never interacts with the name's
`line-clamp-2` truncation (a long name can't push a known age out of view).

Rejected:
- **Corner badge, "Age N" label** — same slot, but the extra characters get tight at `md:grid-cols-6`
  desktop density for no real gain in clarity (the poster context already establishes this is an age).
- **Inline in the name caption** (`"Harrison Ford · 25"`) — the caption already truncates with
  `line-clamp-2`; a long name can silently drop a known age from view, which fights the "absent means
  no data" invariant (D5) this feature depends on.

## Placement & measurements

**Files:** [`web/src/lib/components/PersonPoster.svelte`](../../web/src/lib/components/PersonPoster.svelte),
[`web/src/routes/media/[id]/+page.svelte`](../../web/src/routes/media/%5Bid%5D/+page.svelte) (cast grid,
lines ~379–400).

| Property | Value | Token / source |
|---|---|---|
| Position | Absolute, bottom-right inside the poster's `.portrait-frame` well | matches `VideoCard.svelte`'s duration badge exactly: `absolute bottom-1.5 right-1.5` |
| Stacking | Above the image | `z-[2]` |
| Fill | Translucent black scrim (not a themed surface color) | `bg-black/70` — **intentional carryover** of the existing `VideoCard` duration-badge literal, not a new tokening violation; see Theming notes |
| Text | Neutral ink, no accent | `text-ink`, `text-xs`, `tabular-nums` |
| Shape | Same radius as every other chrome element | `rounded-theme` (`--radius`) |
| Padding | `px-1.5 py-0.5` | matches `VideoCard` |
| Copy | Whole number only — no `"yrs"`, no `"Age"` prefix, no unit | e.g. `25` |
| Corner | 6px inset from both edges | `bottom-1.5 right-1.5` |

`.portrait-frame` (`web/src/app.css`) is already `position: relative; overflow: hidden`, so the badge
drops in as a sibling of the `<img>` with no new wrapper element.

## Component wiring notes (non-visual, for implementer awareness)

- `PersonPoster` currently takes `{personId, name, version?, eager?}` — no age slot. Add an optional
  `ageInMedia?: number | null` prop and render the badge as a sibling `<span>` inside `PersonPoster`'s
  own markup (next to its `<PersonImageFrame>` call), **not** by threading it through
  `PersonImageFrame`. `PersonImageFrame` is the shared primitive behind avatar/banner/poster roles —
  "age in media" is a poster-only, video-context concept and has no meaning for an avatar or banner
  render, so it stays out of the shared component.
- Per the spec's FR4, `age_in_media` lives on a video-detail-scoped credit shape, not the generic
  `Person` type (`web/src/lib/types.ts`) — the cast-grid loop in `+page.svelte` destructures
  `p.age_in_media` directly from each cast entry already returned by `getMedia`; no separate fetch.

## States

- **Computable** (video's resolved `release_date` × cast member's resolved `birthdate` both present) —
  badge renders with the whole-year number.
- **Missing person birthdate** (per-member, spec D5/FR4) — badge omitted entirely for that one card. No
  placeholder, no dash, no `0`. Other cast members on the same video are unaffected (FR4 independence).
- **Missing video `release_date`** (spec FR3/AC2) — badge omitted for **every** cast member on that
  video's page, regardless of individual birthdate coverage. There is no partial/mixed state at the
  video level, and no `recorded_at` fallback under any circumstance (the hard constraint D4).
- **Invalid combination** (`birthdate` after `release_date` — a data inconsistency, FR2 guard) — treated
  identically to "uncomputable": no badge, never a negative or nonsensical number.
- **Owner vs. visitor** — identical, no gating (matches D5 / F45's precedent).

## Theming notes

- **`bg-black/70` is a deliberate exception**, already established by `VideoCard`'s duration badge for
  exactly this reason: a translucent scrim needs to stay legible over *arbitrary* underlying imagery
  (a person photo here, a video thumbnail there) regardless of which skin is active — a themed surface
  color would fight the photo instead of sitting on top of it. Reuse the literal as-is; don't reinvent
  it as a token.
- Everything else is a token and reacts per skin: `rounded-theme` (2px Cinémathèque / 0 Broadcast /
  0 Brutalist), `text-ink` (per-skin ink color).
- QA note: verify the badge reads against both the `--surface-2` placeholder (person with no photo yet)
  and a busy real poster photo, in all three skins — the black scrim is what keeps contrast stable
  across both cases, same reasoning the duration badge already relies on.

## Edge cases

- **Long names** — untouched by this feature. The badge lives on the poster image, not the caption, so
  `line-clamp-2` name truncation is unaffected (this was precisely Option C's problem, and why it was
  rejected).
- **Dense grids** — at `md:grid-cols-6` (desktop), poster cards run roughly 100–110px wide. A 1–3 digit
  age fits the badge pill at `text-xs` without wrapping across any realistic age range (0–120).
- **Zero computable ages on an otherwise-valid video** (has `release_date`, but no cast member has a
  resolved `birthdate`) — the People section renders normally with no badges anywhere; no special empty
  state needed (FR4 per-member independence already covers this).

## Accessibility

- The badge sits over an image whose `alt` text already carries the person's name
  (`PersonImageFrame`'s `altText`). Don't fold the age into that alt text — give the badge `<span>` its
  own `aria-label`, e.g. `aria-label={\`Age ${age} at time of release\`}`, so a screen reader announces
  context rather than a bare floating number.
- Purely informational overlay — no new focusable/interactive element.

## Out of scope (see spec)

- Search/filter/facet UI for `age_in_media` — that was HOLODEX-176's scope
  (F46 typed-field substrate), separately closed as Won't Do; this feature is render-only.
- Any placement or copy change on the person's own detail page — `age_in_media` is video-context-only;
  F45's existing intrinsic Age on the person page is untouched.
