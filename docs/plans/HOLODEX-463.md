---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-463
status: in-review            # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # new table + owner-gated mutation + UI → every gate applies
depends-on: []
release_note: The halo behind studio logos, icons and posters is now off by default. On a studio's page the owner can turn it on per image, and the choice is saved separately for dark and light palettes, with a black halo on a light palette. The choice carries over to the studio list, studio cards and the completeness queue.
approved:
  design:
    on: 2026-09-26           # Kevin picked option A (one switch per slot) in-session from the rendered mockup
    at: 55c2a13
---

# HOLODEX-463 · Per-studio owner toggle for the studio image halo

The `.logo-halo` glow (HOLODEX-432/437) was on for every studio image. Done means the owner turns it
on per studio and per image role (logo / icon / poster) from the Studio page. It is off by default
and saved independently for dark and light palettes, inverting to black on a light one. The choice
follows the image to every surface that draws it.

**Design package:** spec [studio-images.md § Image halo](../specs/studio-images.md) ·
[ADR-109](../architecture/ADR-109-per-studio-image-halo.md) ·
[handoff](../design/studio-image-halo-handoff.md) + [mockup](../design/studio-image-halo-mockup.svg).

## Decisions — do not re-litigate

- **Light/dark = palette brightness** (Kevin, 2026-09-26). Holodex has no light mode. A custom
  palette whose `--bg` luminance is above 0.179 counts as light, and every shipped skin is dark. The
  SPA sets `<html data-mode>`, and the server stores both modes.
- **Per studio, everywhere** (Kevin, 2026-09-26). The choice follows the image to the /studios row,
  the Film/Media link card (`StudioLogoBox`, using the shown role) and the completeness queue icon.
  It is not instance-wide.
- **Design option A** (Kevin, 2026-09-26). One `Halo · <mode>` switch per slot, saved for the
  palette being viewed. Option B (Dark and Light chips, both always editable) was declined.
- **Keyed on studio + role, not the image row.** A replace is delete + insert, so the choice
  survives re-upload and re-enrich.
- **Film poster keeps the unconditional `.logo-halo`.** It is out of scope; only its colour moves
  to `--logo-halo`.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — studio-images.md § Image halo, H1–H7 (55c2a13)
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-109 + index row (55c2a13)
- [x] design `design-handoff` → `docs/design/**` — option A approved in-session; handoff + committed SVG (55c2a13)
- [x] backend → `{cmd,internal,providers}/**` — migration 0053, repo, `PUT …/halo`, `image_halo`/`icon_halo` (55c2a13)
- [x] frontend → `web/src/**` — `halo.ts`, `theme.mode`/`data-mode`, `EntityImageSlot` switch, `StudioLogoBox`, queue row, `--logo-halo` (55c2a13)
- [x] testing `testing-strategy` — strategy row; `studio_image_halo_test.go`, `halo.test.ts`, `theme.test.ts`; handoff §4 items 1–7 browser/Go-verified
- [x] security `security-review` — no findings: route sits in the `requireOwner` group, SQL is parameterized, values are enum + CHECK constrained

## Up next — ordered (position = priority)

1. [ ] [—] Confirm the PR squash-merged and CI moved HOLODEX-463 to Done
2. [ ] [testing] Human QA (handoff §4 items 8–9): live library dark and light marks; a light custom palette if configured — `docs/design/studio-image-halo-handoff.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-26 · session
- skills: code-review (high --fix), security-review
- filed HOLODEX-463 and renamed the branch; Kevin chose palette-brightness light/dark, per-studio-everywhere scope, and design option A
- built backend + frontend + all docs; driven-browser QA on the AMV testbed (default off, toggle persists, row follows, three skins, `data-mode` flip, visitor view)
- handoff: All seven gates are settled and the PR is open. Kevin asked for it to be merged once CI passes, so the only thing left is confirming the merge and the Done transition, then the human QA on the live library.

## Dropped — newest first (the reason is the point)
