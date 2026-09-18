# QA Checklist: Candidate slot shape per entity kind (HOLODEX-414)

Companion to [candidates-image-per-kind-handoff.md](candidates-image-per-kind-handoff.md).
Items are numbered `section.item` and tagged by verifier: `[smoke]` = unit / integration test in
CI, `[agent]` = a Claude session driving the preview with `javascript_tool` (computed style and
`getBoundingClientRect`, not screenshots — they time out on this picker), `[human]` = Kevin, on
the real testbed. Everything in
[candidates-image-qa-checklist.md](candidates-image-qa-checklist.md) (F64) still applies; this
list covers only what the shape change adds or alters.

## §1 Setup

- **1.1** `[agent]` Start `backend-stub` (or `backend-films` for a real film/video) and `web` with
  the `enrich-stub` sidecar (local `launch.json` has all three). The picker is shared, so the
  studio page reaches the logo box and the media page the landscape box; the person page needs
  a seeded person (the AMV testbed has none — use `backend-stress` after `go run
  ./testdata/stressseed`).
- **1.2** `[agent]` Stub renditions (`testdata/enrich-stub/`): `/p/<slug>/thumb/landscape-N.png`
  at 300 × 169 for the backdrop rows; the existing `portrait-N` (80 × 120, the trap size) and
  `wide` (64 × 16 logo) stay. **As built:** no new persona — `twins` reads the request's
  `entity_type` and, for `video`, pictures rows 0–3 with `landscape-N` and row 4 with a
  portrait (the poster fallback); rows 5–7 (404 · foreign · none) are unchanged.

## §2 Smoke — `[smoke]`

- **2.1** `[smoke]` Sidecar (`providers/tmdb`): `TestTMDBResolveImageURL` rows — `video` with
  backdrop → `…/w300/<backdrop>`; `video` without backdrop, with poster → `…/w185/<poster>`;
  `video` with neither → key absent from the JSON; `film` with both → `…/w185/<poster>` (never
  the backdrop); person and studio rows unchanged.
- **2.2** `[smoke]` Sidecar: `movieSearchEntry` and the `/3/find` movie result decode
  `backdrop_path` (a fixture response carrying it round-trips to the candidate).
- **2.3** `[smoke]` Pure helper (vitest, `candidateImage.test.ts`): `slotShape('person')` and
  `('film')` → `portrait`; `('video')` → `landscape`; `('studio')` → `logo`; `SLOT_CLASS` maps
  the three shapes to `w-10 h-15` / `w-27 h-15` / `w-30 h-15`.
- **2.4** `[smoke]` `npm run check`: every `<EnrichPicker` mount passes `entityType`; removing
  it from one mount is a type error (verify once by hand, do not commit the mutation).
- **2.5** `[smoke]` Geometry harness: `candidate-slot-is-40-wide-on-every-row` re-keyed to
  `candidate-slot-is-108-wide-on-every-row` — the stressed picker opens on a **video** page
  (`landscape`, `w-27`); exact bound, not a minimum, so an `aspect-*` regression is caught. The
  76 px row-floor assertions unchanged and still passing. **As run 2026-09-18:** 27/27 across the
  9 skin/width cells; dropping `w-27` fails the width assertion on all 9, dropping `h-15` fails
  the row floor on all 9.

## §3 Agent — live, all three skins — `[agent]`

- **3.1** `[agent]` Media page picker: every row's slot `getBoundingClientRect()` is exactly
  108 × 60; the label `<span>` x is identical on every row (parity holds within the kind).
- **3.2** `[agent]` Media page, backdrop row: `<img>` `naturalWidth/naturalHeight` = 300/169,
  rendered box 106.67 × 60 inside the 108 slot (`object-fit: contain`), plate visible ≤ 1 px each
  side.
- **3.3** `[agent]` Media page, poster-fallback row: `<img>` rendered 40 × 60 centred, plate 34 px
  either side; no crop (rendered height == slot height).
- **3.4** `[agent]` Media page, monogram row: glyph centred in the 108 × 60 plate; same
  `font-display text-sm` computed style as the portrait monogram.
- **3.5** `[agent]` Studio page picker: slot 120 × 60 on every row; the 64 × 16 `wide` logo
  renders 120 × 30 letterboxed (plate 15 px above and below); a square logo fixture, if added,
  renders 60 × 60 with plate 30 px either side.
- **3.6** `[agent]` Person page (stress seed) and film page: slot still exactly 40 × 60 — no
  regression from swapping `aspect-[2/3]` for `h-15`; the 80 × 120 trap portrait does **not**
  widen the box.
- **3.7** `[agent]` Every collapsed row is 76 px on all four kinds; F61 `details` expansion still
  grows downward with the slot top == label top.
- **3.8** `[agent]` Wire check (`/media/{id}/enrich/resolve` against the stub): the video row's
  `image_url` points at the `landscape-N` rendition, the poster-fallback row at `portrait-N`; the
  film page's resolve for the same stub title points at `portrait-N`.
- **3.9** `[agent]` Three skins (`[data-theme]` = cinematheque / broadcast / brutalist): plate
  radius 2 / 0 / 0 on the 108 and 120 boxes, monogram contrast on plate unchanged from F64's
  12.2 / 13.0 / 15.3.
- **3.10** `[agent]` 375 px viewport (`resize_window` mobile, reload): media slot still 108 × 60
  and studio 120 × 60; label truncates; match-strength text intact and right-aligned; no
  horizontal overflow on the dialog (`scrollWidth == clientWidth`).
- **3.11** `[agent]` Keyboard and click: unchanged — Tab from the search field lands on row 0,
  not the slot; clicking the backdrop applies the candidate.

## §4 Human — `[human]`

- **4.1** `[human]` Open the Enrich picker on a media file whose title exists on TMDB (owner view,
  any skin). Each row's picture is a widescreen still from the film, not a poster. Does the still
  help you tell "which release is my file" faster than the poster did?
- **4.2** `[human]` Same picker, find a row for an obscure or very old title with no backdrop on
  TMDB: a narrow poster sits centred in the wide tile with the light plate either side. It should
  read as "this one only has a poster", not as broken.
- **4.3** `[human]` Open the picker on a studio: the logo sits centred on a light tile twice as
  wide as it is tall; a wordmark spans the tile with plate above and below; a symbol fills the
  height with plate either side. Nothing is cropped.
- **4.4** `[human]` On a phone-width window (≈ 375 px), open the media picker and the studio
  picker. The name and the match percentage should both be readable on every row. If names are
  truncating so early the rows stop being scannable, say so — the fix is a narrower tile on
  small screens only, not a shorter one.
- **4.5** `[human]` Open the picker on a person and on a film: nothing has changed from before.
