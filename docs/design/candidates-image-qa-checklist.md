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
- **1.2** `[agent]` Stub personas, selectable by query text:
  (a) **faces** — four person candidates all labelled `Chris Evans`: image on the stub's own host;
  image on the stub's `asset_hosts` entry; image on `img.other.example` (must arrive stripped); no
  `image_url` key. Each with `disambiguation` + `profile_url`, two with `detail`;
  (b) **broken** — one candidate whose `image_url` is on the stub's host but the path returns 404;
  (c) **logos** — two studio candidates, one with a wide (≈ 4:1) logo, one with none;
  (d) **flood** — the existing 25-candidate F61 persona, now every candidate carrying an image;
  (e) **hostile** — `image_url` values: `ftp://…`, `javascript:alert(1)`, `""`, `not a url`,
  `https://stub.example.evil.example/x.jpg` (suffix spoof), a 5000-char URL.

## §2 Smoke — `[smoke]`

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
- **3.6** `[agent]` `logos`: the wide logo `<img>` `naturalWidth/naturalHeight` ≈ 4 and its
  rendered box is 40 wide with rendered height < 60 (letterboxed); the plate `bg-logo-plate` is
  visible above and below (computed background on the wrapper, not the img).
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
- **3.11** `[agent]` `flood`: 25 rows all have slots; at most the rows in / near the viewport
  have requested their image (network log count < 25 immediately after render); scroll to the
  last row — inside the `<ul>` scroll box, dialog overflow 0.
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
  face legibility; this is the one number open to taste.
