# Design handoff: studio image halo toggle (HOLODEX-463)

**Status:** approved 2026-09-26. The owner picked **option A** (one switch per slot) over option B
(Dark and Light chips).
**Refs:** [ADR-109](../architecture/ADR-109-per-studio-image-halo.md), spec
[studio-images.md § Image halo](../specs/studio-images.md), prior halo work in
[studio-list-logo-handoff.md](studio-list-logo-handoff.md) (HOLODEX-432) and
[image-plate-handoff.md](image-plate-handoff.md) (HOLODEX-437).

![Option A: a Halo switch under each studio image slot](studio-image-halo-mockup.svg)

## 1. What changes

| Surface | Before | After |
|---|---|---|
| Studio page image slots (`EntityImageSlot`, row variant) | every contained image wears `.logo-halo` | the halo shows only for the modes saved for that role. The owner gets a `Halo · <mode>` switch under Replace/Remove |
| `/studios` rows and Film/Media studio link cards (`StudioLogoBox`) | logo or icon always haloed | the choice for whichever role the box shows (logo first, then icon) |
| Completeness queue studio icon | always haloed | the icon's choice |
| Film poster slot | always haloed | unchanged. The glow colour is now `--logo-halo`, so it inverts on a light palette |

## 2. The switch

This is a **knob** in [ui-vocabulary](../reference/ui-vocabulary.md) terms: an owner-set display
choice on a value. It is not a new field.

- `<button role="switch" aria-checked>` in the slot's text column, under the Replace/Remove row.
  It only renders for the owner, only when the slot has an image and the fit is `contain`.
- Label: `Halo` then a muted `· dark` or `· light` (the current `theme.mode`). The `title` is
  "Glow behind the logo on dark palettes".
- The track is `h-3 w-5 rounded-full border` and the knob is `h-2 w-2 rounded-full`.
  - Off: `text-muted`, `border-muted`, knob `bg-muted` at the left, with `hover:text-ink`.
  - On: `text-accent`, `border-accent`, knob `bg-accent` at `left-2.5`.
- Tokens only; `rounded-full` is the intentional pill shape.
- While saving, the switch is disabled with no opacity dimming. The label stays at full contrast.
- If the save fails, the slot's existing `text-warn` error line shows the message and the switch
  stays in its old state.

## 3. Behaviour

- Clicking saves `{mode: theme.mode, on: !current}` for that role. The page then patches its
  local `studio.image_halo` with no refetch.
- The other mode is never written from here. The owner sets what they can see (option A).
- The mode comes from the palette. Shipped skins are dark; a custom palette is light when its
  `--bg` luminance is above 0.179.
- Colour: `--logo-halo`, which is the skin's `--logo-plate` on a dark palette and `#000000` on a
  light one. The strength is unchanged (1/3/6px, from HOLODEX-432).

## 4. QA

Numbered, tagged, grouped by tag.

### [smoke]
1. `/studios/{id}` as owner with a filled logo: the `Halo · dark` switch renders, and
   `aria-checked="false"` by default.

### [agent]
2. Default: no studio `<img>` on the Studio page, the /studios row or the queue has a computed
   `filter` other than `none`.
3. Toggling the logo on sets `aria-checked="true"`, gives the `<img>` the class `halo-dark`, and
   gives it a computed filter of three `drop-shadow`s in that skin's `--logo-plate`:
   `#e9e0d0` / `#e4ebf8` / `#f0f0f0` for Cinémathèque / Broadcast / Brutalist. It also makes
   `GET /studios/{id}` return `image_halo.logo = ["dark"]`.
4. The /studios row for the same studio shows the same filter. The icon (not toggled) shows `none`.
5. With `<html data-mode="light">`, a `halo-dark`-only image shows `none`. Adding `halo-light`
   makes it glow `rgb(0, 0, 0)`.
6. Visitor view (Owner view off): no switch; the saved halo still renders.
7. Re-uploading the logo keeps `image_halo.logo`.

### [human]
8. On the live library, pick a studio with a dark wordmark and one with a light mark. Turn the
   halo on for the dark one only, then check that both read well on all three skins.
9. If a light custom palette is configured: switch to it, confirm the label reads `· light`, and
   that a black halo on a light mark reads.

**Verified 2026-09-26** (driven browser, AMV testbed, Cinémathèque): 1, 2, 3 (all three skins
checked by computed style), 4, 5 (via the `data-mode` attribute) and 6. Item 7 is asserted by the
Go test `TestStudioImageHalo_DefaultOffThenPerModeToggle`. Items 8 and 9 are open.
