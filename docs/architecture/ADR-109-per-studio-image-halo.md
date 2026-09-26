# ADR-109: The studio image halo is a per-studio, per-role, per-palette owner choice

**Status:** Proposed (amends HOLODEX-432/437's "a contained image always wears `.logo-halo`" for
studio images only; relates ADR-079 studio image roles, ADR-102 instance skin; HOLODEX-463)

**Date:** 2026-09-26

## Context

HOLODEX-432 gave every bare studio logo and icon a `.logo-halo`: a three-layer `drop-shadow`
(1/3/6px) in `--logo-plate`. It was added because a dark mark on a transparent ground vanished
into the dark rows once HOLODEX-411/437 removed the plate. HOLODEX-437 extended it to every
`contain` image in `EntityImageSlot`. The halo is unconditional, so an image that reads fine
without it (a light or full-colour mark, a poster) still gets a glow it doesn't need. The owner
asked (2026-09-26) for the halo to be their choice.

Holodex has **no light mode**. `:root` is `color-scheme: dark`, and all three shipped skins are
dark. A page is only light when an operator configures a custom palette (ADR-102 D4) with a bright
`--bg`. The owner wants the choice saved separately for light and dark, with the halo inverted
(black) on a light palette.

## Decision

- **D1 — Storage: a presence table keyed on studio + role + mode.**
  `studio_image_halo(studio_id, role, mode)` (migration 0053), `WITHOUT ROWID`, with a composite
  primary key, CHECKs on the `role` and `mode` enums, and `ON DELETE CASCADE` from `studios`.
  - A row means "on" and no row means "off", so the requested default needs no backfill and no
    default column.
  - It is keyed on the studio + role, **not** the `studio_images` row. A replace there is a
    delete + insert, so a column on that row would silently reset the choice on every re-upload
    or re-enrich.
  - A studio merge deletes the loser studio, so the loser's choices cascade away with its images.
    That is consistent: images are not carried across a merge either.
- **D2 — API.** `PUT /studios/{id}/images/{role}/halo` takes `{mode: "dark"|"light", on: bool}`.
  It is owner-gated (`requireOwner`, in `mountStudioImages`) and idempotent, and returns 204.
  Role and mode are validated against enums; an unknown studio is 404.
  - Reads carry `image_halo: {role: [modes]}` on every `Studio` payload that carries image URLs
    (list, detail, video studios, film studios).
  - The completeness queue row carries `icon_halo`.
  - The choice is public data. It changes how a visitor's page renders, like the image itself.
- **D3 — Light/dark is the SPA's call, from the palette's `--bg`.**
  - `theme.svelte.ts` sets `<html data-mode>`: `light` when the active palette is custom and its
    `--bg` has WCAG relative luminance above 0.179 (the point where black and white text reach
    equal contrast), otherwise `dark`.
  - Every shipped skin is `dark`.
  - The server does not classify palettes. It stores both modes, and the client decides which
    one applies.
- **D4 — The rendering is pure CSS.**
  - A studio image emits `.halo-dark` / `.halo-light` for the modes its role is on for.
  - `app.css` applies the filter only to the class that matches `data-mode`.
  - The colour is a new token, `--logo-halo`: `var(--logo-plate)` by default and `#000000` under
    `[data-mode='light']`.
  - Changing the palette re-renders nothing in JS.
- **D5 — Scope.** The choice follows the studio's image everywhere it renders: the Studio page
  slots, `StudioLogoBox` (the /studios rows and the Film/Media link cards; the choice for
  whichever role the box shows), and the completeness queue icon.
  - Non-studio callers of `EntityImageSlot` (the film poster) pass no `halo` and keep the
    unconditional `.logo-halo`. Its colour now comes from `--logo-halo` too, so it also inverts
    on a light palette.
- **D6 — UI (design option A).**
  - Each Studio-page image slot gets one owner-only `Halo · <mode>` switch
    (`role="switch"`, `aria-checked`).
  - It saves for the palette being viewed. The owner only sets what they can see.
  - The other mode's choice stays as it was until the owner views a palette in that mode.

## Consequences

- Existing studios lose the halo on upgrade, because the default is off. This was deliberate and
  asked for. The owner re-enables it per image where a dark mark needs it.
- A light-mode choice can only be made while a light custom palette is active. With today's
  all-dark skins that is a no-op, and it is the intended trade (option B, both modes editable
  anywhere, was declined).
- QA check 11.11b in `docs/design/studio-list-logo-handoff.md` (the halo on every image) no
  longer holds for studio images; the halo QA moves to `docs/design/studio-image-halo-handoff.md`.

## Rejected

- **A column on `studio_images`.** It resets on every replace (D1).
- **An instance-wide toggle.** The owner chose per studio. Whether a mark needs a halo is a
  property of that image, not of the instance.
- **Building a light mode first.** That is a separate epic; nothing here blocks it. A future light
  mode only has to set `data-mode`.
- **Server-side light/dark classification.** It would duplicate the palette logic (the
  `internal/theme` mirror) for a value only CSS consumes.
