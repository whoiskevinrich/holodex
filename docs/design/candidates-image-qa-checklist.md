# QA Checklist: Candidate thumbnail in the Enrich picker (HOLODEX-406)

**Handoff**: [candidates-image-handoff.md](candidates-image-handoff.md) ·
**Spec**: [candidates-image.md](../specs/candidates-image.md) AC 1–11 ·
**Contract**: [metadata-provider-contract.md](../specs/metadata-provider-contract.md) §2.3 / §5 / §6

Tags: `[smoke]` runs in CI (Go / vitest); `[agent]` an agent runs live against the stub in all three
skins by computed style and geometry (screenshots time out on this picker); `[human]` needs eyes.
Items are numbered `section.item`.

## §1 Setup

- **1.1** `[agent]` Start `backend-films` (or `backend-stub`) and `web` with the `enrich-stub`
  sidecar configured for `person` and `studio`, its own host in `asset_hosts`. Owner view on — the
  picker is owner-only. **Verify the served bundle matches this worktree** (curl the dev server,
  grep for `image_url`) — a worktree with no local `.claude/launch.json` silently previews `main`.
- **1.2** `[agent]` Stub personas (`testdata/enrich-stub/`). **As built:** rather than three new
  personas (each needs `personas.json` + `sources.yaml` + seeder registration), every slot state
  rides the existing adoption personas —
  (a) **twins** (eight same-label rows): rows 0–3 a portrait each on the stub's own host
  (`/p/twins/thumb/portrait-N.png`, 80 × 120 so the slot cannot borrow the image’s own size, colour from the id); row 4 a wide 64 × 16 "logo"
  (`/thumb/wide.png`); row 5 `/thumb/missing.png` (the stub 404s it); row 6
  `https://img.other.example/…` (core must strip it); row 7 no key (also F61's no-detail member);
  (b) **flood** — the 25-candidate F61 persona, every row pictured;
  (c) **hostile** values (`ftp://`, `javascript:`, `""`, malformed, suffix spoof, over-cap) are
  the Go sanitizer table (2.1) — the stub does not need to emit them.

## §2 Smoke — `[smoke]`

> **Reconciled 2026-09-17 (frontend gate).** 2.1–2.4 and 2.6 are Go tests
> (`internal/enrich/candidate_image_test.go`, `internal/api/enrich_hints_test.go`,
> `providers/tmdb/tmdb_test.go`); 2.5 is `web/src/lib/candidateImage.test.ts`. §3 ran live
> against the stub on the studio page (the AMV testbed has no people; the picker is shared, so
> the surface is the same) in all three skins — results in `docs/plans/HOLODEX-406.md` and
> `docs/testing-strategy.md` §5 once the testing gate lands.

- **2.1** `[smoke]` `sanitizeCandidates` table: base host kept; `asset_hosts` entry kept; foreign
  host cleared; suffix-spoof cleared; `ftp:` / `javascript:` cleared; malformed cleared; `""`
  cleared; absent stays absent; > 4096 chars cleared. Existing rows for `label` /
  `disambiguation` / `profile_url` / `detail` unchanged.
- **2.2** `[smoke]` The gate is `Service.ImageURLAllowed` — assert `sanitizeCandidates` calls the
  same function `gateImageURL` (ADR-056) calls, not a second allowlist.
- **2.3** `[smoke]` Resolve handlers (person, film): a `Fake` candidate with `ImageURL` on the
  Fake's allowed host round-trips to the JSON as `image_url`; on another host the key is absent
  from the JSON.
- **2.4** `[smoke]` Sidecar builders (`providers/tmdb`): `profile_path` / `poster_path` /
  `logo_path` → `https://image.tmdb.org/t/p/w185/<path>`; null path → no key; never `original`.
- **2.5** `[smoke]` Pure helper (vitest): slot-state decision — `image_url` present + not failed →
  image; absent → monogram; present + failed → monogram; `monogram("chris evans") === "C"`;
  `monogram("") === "?"`.
- **2.6** `[smoke]` `image_url` never appears in `entity_enrichment`, the `/enrich` response, a
  writeback payload, or an activity-log line (grep the Fake-driven API tests' outputs).

## §3 Agent — live, all three skins — `[agent]`

> **Reconciled 2026-09-17 (testing gate).** 3.3 (x-offset parity) is the §12 assertion
> `candidate-slot-is-40-wide-on-every-row` — asserted by construction (every slot 40 wide), since
> the harness has no cross-element metric; 3.4 (height parity) is `collapsed-detail-row-costs-one-line`
> re-based to the equality 76 plus the new `collapsed-detail-text-block-costs-one-line` (60), both
> mutation-tested. 3.1–3.2, 3.5–3.10 and 3.12–3.14 were measured live on `twins`/`flood` and the
> numbers are the record in `docs/testing-strategy.md` §5. The harness sees only pictured rows
> (`flood`); the monogram rows are live-only (§12.5).

- **3.1** `[agent]` `faces`: every row has exactly one `[aria-hidden="true"]` slot as the `<li>`'s
  first child, `getBoundingClientRect()` width 40 ± 0.5 and height 60 ± 0.5, on every row.
- **3.2** `[agent]` `faces`: rows (a) and (b) contain an `<img>` with `alt=""`,
  `loading="lazy"`, `referrerpolicy="no-referrer"`, computed `object-fit: contain`; rows (c) and
  (d) contain no `<img>` and a monogram span reading `C`. Row (c) proves the server stripped the
  foreign host: the DOM never held that URL (check the fetched JSON too).
- **3.3** `[agent]` **x-offset parity**: the label `<span>` of every `faces` row has the same
  `getBoundingClientRect().x` (± 0.5) whether the slot holds an image or a monogram.
- **3.4** `[agent]` **Height parity**: every collapsed `faces` row has the same `offsetHeight`;
  record it (expected 76). With `flood`, `#enrich-opt-2` height still satisfies the F61 assertion
  `collapsed-detail-row-costs-one-line` (see the handoff's implementation note on re-basing it).
- **3.5** `[agent]` `broken`: after the 404 (await `img.complete` / the `error` event), the row
  shows the monogram and no `<img>`; `document.querySelector('img[src*="404path"]')` is null.
- **3.6** `[agent]` `twins` row 4: the wide logo `<img>` has `naturalWidth/naturalHeight` = 64/16
  while its element box is the full 40 × 60 and computed `object-fit: contain` — the painted
  image letterboxes inside the box, so the plate `bg-logo-plate` (computed background on the
  wrapper, not the img) shows above and below.
- **3.7** `[agent]` F61 interplay: on a `faces` row with `detail`, click `details` — the dialog
  stays open, the `<ul>` appears **inside the text block** (its left edge ≥ the label's x), the
  slot's `y` is unchanged (pinned to the label line), the row grows by the lines' height only.
- **3.8** `[agent]` Keyboard: Tab from the search field reaches row 0, not the slot; ↓ moves the
  roving focus row by row; Enter on a row confirms; the `trapTab` stop list is identical to F61's
  (no new stops).
- **3.9** `[agent]` Clicking on the image itself confirms the candidate exactly as clicking the
  label does (same handler, same result).
- **3.10** `[agent]` New response resets state: with `broken` showing the monogram, retype a
  query that returns the same candidate with a good path — the row shows the image again.
- **3.11** `[agent]` `flood`: 25 rows all have slots and `loading="lazy"`; scroll to the last
  row — inside the `<ul>` scroll box, dialog overflow 0. (How many thumbs the browser requests at
  render is its lazy-load threshold's call, not ours — Chrome's ~1250 px margin covers the whole
  1900 px list on a tall viewport; record the count, don't assert it.)
- **3.12** `[agent]` **Three skins** (`[data-theme]` = cinematheque / broadcast / brutalist):
  monogram contrast `text-logo-plate-ink` on `bg-logo-plate` ≥ 4.5 : 1 in each; plate corner
  radius = 2 / 0 / 0 px; the plate reads the same on the active row (`bg-surface-2` behind it)
  and the resting row. Record the three contrast numbers in `docs/testing-strategy.md` §5.
- **3.13** `[agent]` 375 px viewport (`resize_window` mobile, reload): slot still 40 × 60,
  label truncates, match-strength text intact, no horizontal scroll on the dialog.
- **3.14** `[agent]` No `image_url` string in the rendered DOM of the entity page after applying a
  candidate (the field list, `ProvenanceBadge`, the identity card) — the thumb did not become a
  field.

## §4 Human — `[human]`

- **4.1** `[human]` Open the Enrich picker on a person with a common name (owner view, any skin),
  type the name, wait for results. What looks right: each row starts with a small
  portrait-shaped tile; rows with a photo show the face, rows without show a single letter on a
  light tile; every name starts at the same left edge; nothing jumps when photos finish loading.
- **4.2** `[human]` Same on a film with a remake (e.g. "Heat"): the tile is the poster; the year
  in the grey line plus the poster together make the right row obvious without reading further.
- **4.3** `[human]` Same on a studio: a wide logo sits centred on the light tile with tile showing
  above and below — not stretched, not cropped.
- **4.4** `[human]` Click a row's `details`: the extra lines appear under the name, the tile stays
  where it was, the dialog stays open. Then click the photo itself: the candidate is applied.
- **4.5** `[human]` Switch skins (Cinémathèque / Broadcast / Brutalist): the tile corners follow
  the skin (slightly rounded / square / square) and the letter on the tile is easy to read in all
  three.
- **4.6** `[human]` Does the 76 px row feel too tall on the 25-candidate list? Say if the slot
  should drop to 32 × 48 (the text stack would then set the height) — the spec chose 40 × 60 for
  face legibility; this is the one number open to taste. **Decided 2026-09-17: keep 40 × 60** — reviewed
  both sizes side by side in three skins; 32 × 48 hands row height back to the text stack and
  thins a wide logo's letterbox to ~13 px.
