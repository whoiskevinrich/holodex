# QA checklist: Responsive page width (HOLODEX-331)

**Handoff:** [responsive-page-width-handoff.md](responsive-page-width-handoff.md)
**Jira:** [HOLODEX-331](https://whoiskevinrich.atlassian.net/browse/HOLODEX-331)

Verifier tags: `[smoke]` runs in CI or a single command · `[agent]` an agent can verify via
`javascript_tool` computed styles and geometry · `[human]` needs eyes on a real display.

Widths under test: **412, 768, 1024, 1280, 1536, 1920, 2560, 3840, 5120**.
Skins under test: **Cinémathèque, Broadcast, Brutalist**.

> **Reload at each width — do not resize.** `viewportTierCap` reads `window.innerWidth` at
> construction, so a resized window and a freshly-loaded one take different code paths. Resizing
> and then reading gives stale column counts.

---

## 1. Setup

- **1.1** Start the backend: `preview_start` with `backend-films` (poster layout — the tallest
  card shape, and the worst case for a wide grid). Confirm it is on :7800.
- **1.2** Start the frontend: `preview_start` with `web`. Confirm it is on :5173.
- **1.3** Confirm the worktree is serving its own code, not the main worktree's: `.claude/launch.json`
  must exist in this worktree. Load a page and confirm a change from this branch is present.
- **1.4** Clear `holodex:media-density` from localStorage before the density tests in section 4.

## 2. Smoke

- **2.1** `[smoke]` `cd web && npm run check` passes with no new type errors.
- **2.2** `[smoke]` `cd web && npm run test` passes.
- **2.3** `[smoke]` `make test` passes (no backend change expected; guards against accidental breakage).
- **2.4** `[smoke]` No `max-w-[` arbitrary-value utility anywhere in the diff — the stage cap must
  come from the `--container-stage` token. Grep the diff for `max-w-[`.
- **2.5** `[smoke]` No `mx-auto max-w-*` remains in `owner/status`, `owner/keys`, or `owner/trash`;
  only `owner/+layout.svelte` sets the stage.

## 3. Agent-verifiable geometry

Run each at a **fresh page load** at the stated width. Read values with `javascript_tool`
(`getBoundingClientRect`, `getComputedStyle`) rather than screenshots.

- **3.1** `[agent]` At 1024 on `/media/{id}`: the page has exactly two grid columns; player column
  ≈555px, rail ≈397px (±10px for scrollbar).
- **3.2** `[agent]` At 1920 on `/media/{id}`: player column ≈1078px, rail ≈770px.
- **3.3** `[agent]` At 5120 on `/media/{id}`: the stage element's width is 2600px, and its left
  offset is ≈1260px — confirming it centres rather than left-aligns.
- **3.4** `[agent]` At 768 on `/media/{id}`: the layout is a single column; the rail's first card
  has a `top` greater than the player's `bottom` (it is stacked below, not beside).
- **3.5** `[agent]` Field grid column count on `/media/{id}`, from
  `getComputedStyle(dl).gridTemplateColumns.split(' ').length`: 1 at 412, 2 at 768, 1 at 1024,
  1 at 1280, 2 at 1920, 3 at 5120.
- **3.6** `[agent]` A long-text field (Overview) reports `gridColumn` resolving to full width at
  every column count — not `span 2`.
- **3.7** `[agent]` Browse grid column count at **default** density (4) is unchanged by the ladder
  extension: 1 at 412, 2 at 768, 3 at 1024, 4 at 1280 and above. The ladder only raises the
  *ceiling*, so a default-density user sees no difference at any width.
- **3.8** `[agent]` At 5120 the browse grid is **not** capped by the stage — its width should be
  ≈5072px, not 2600px.
- **3.9** `[agent]` At 320px width, `document.documentElement.scrollWidth <= 320` on `/media/{id}` —
  no horizontal scrolling (WCAG 1.4.10).
- **3.10** `[agent]` With a 200-character file path injected into the File section, the player
  column's width is unchanged at 1920 — `minmax(0, 1.4fr)` is clamping min-content.
- **3.11** `[agent]` Focus order: tab from the player through to the rail and confirm the DOM order
  is player zone → rail at both 768 (stacked) and 1920 (two-zone), with no `order-*` class present
  on either zone.
- **3.12** `[agent]` Contrast: `--color-muted` on `--color-surface` (field labels on rail cards)
  meets AA in all three skins. Read computed colors and compute the ratio.
- **3.13** `[agent]` **Max density reaches 8 columns** (shipped; handoff §1b-i). With
  `holodex:media-density` = `8`: 8 columns at 1920 (~220px cards) **and** at 1536 (~172px cards —
  the rung where 8 columns begins). Check both `wide` and `poster` layouts and all three skins;
  assert no horizontal overflow. Confirm 1280 still gives 4 and 1024 still gives 3.
- **3.14** `[agent]` **Slider ends map correctly.** Setting the range input to its `min` position
  stores the highest density (most columns); setting it to `max` stores `DENSITY_MIN` (fewest).
  The inversion is deliberate — dragging right means bigger cards.
- **3.15** `[agent]` **People poster grid tracks the ceiling at 2:1.** `PersonPosterGrid` uses
  `min(density, cap) * 2`, so max density gives 16 columns at 1536+ — measured at 1920:
  102x205px cards, names at 14px, no clipping with short names, no horizontal overflow. Confirm the
  count is exactly double the video grid's at the same density and width.
- **3.16** `[agent]` **First row eager-loads on the People poster grid.** `PersonPosterGrid` passes
  `eager={i < cols}`. With more people than one row holds, exactly the first `cols` images carry
  `loading="eager"` and the rest `lazy`. Guards the regression where the old literal `12` stopped
  matching one row once the ceiling moved.

## 4. Density ladder

The column-count model is retained (handoff §2e), so there is **no** preference migration to test —
a stored `holodex:media-density` keeps its existing meaning. What needs testing is the extended
ladder and the viewport-tracking slider range.

- **4.1** `[agent]` With no stored preference, the grid renders at the default density (4).
- **4.2** `[agent]` A stored value of `6` still yields six columns at a viewport whose cap allows
  it — the meaning of existing preferences is unchanged.
- **4.3** `[agent]` Seed a garbage value (`"abc"`), reload: falls back to the default without
  throwing. Console clean.
- **4.4** `[agent]` **Ladder steps at each new rung**, at max density and a fresh load:
  8 columns at 1536 and 1920, 12 at 2560, 16 at 3840 and 5120. Card width stays within roughly
  170–320px at every one — assert it never exceeds ~350px, which is the ballooning this fixes.
- **4.5** `[agent]` **No dead slider stops.** At each of 1024 / 1280 / 1920 / 2560 / 5120, the range
  input's `max` equals `capForWidth(innerWidth)`, so every reachable position changes the column
  count. Moving the slider one stop must always change `gridTemplateColumns`.
- **4.6** `[agent]` **Inverted direction survives a dynamic max.** At each width above, dragging
  right yields *fewer* columns and dragging left yields *more* — `invertDensity` must invert
  against the current cap, not a fixed `DENSITY_MAX`.
- **4.7** `[agent]` **Preference does not ratchet down across displays.** Set max density at 5120
  (16 columns), reload at 1920 (clamped to 8), then reload at 5120 again: it must return to 16, not
  stay at 8. The raw preference is stored; clamping happens at render.

## 5. Human review

Do these on the real 5120x1440 display and on the Pixel 7 Pro, not in an emulator. Navigate to a
media detail page with a poster, several tags, at least one linked film, and a populated metadata
list.

- **5.1** `[human]` **On the ultrawide, does the page look deliberate rather than broken?** The
  content sits in a centred block with large empty margins on both sides. That is intended — the
  question is whether it reads as a designed choice or as a page that failed to fill the screen.
  If it reads as broken, the stage cap needs to go up.
- **5.2** `[human]` **On the ultrawide, can you read a metadata field comfortably?** Pick a field
  in the rail. The grey label and the lighter value should sit close enough together to read as one
  pair. If your eye has to travel to connect them, the rail is too wide.
- **5.3** `[human]` **On the ultrawide browse page, how many complete rows of covers can you see?**
  You should see several full rows, not one giant row. Change the size slider from smallest to
  largest and confirm the covers grow and shrink smoothly with no sudden jumps.
- **5.4** `[human]` **On the phone, scroll a media page top to bottom.** Order should be: video,
  title, description, tags, films, people, metadata, then the file details. Nothing should be
  hidden behind a tab or a section you have to open. Nothing should be cut off at the right edge.
- **5.5** `[human]` **On the phone, is anything uncomfortably small?** Tag chips and any buttons in
  the rail should still be easy to tap with a thumb.
- **5.6** `[human]` **Switch skins on both devices** using the skin picker in the header. All three
  should keep the same layout — only colors, fonts and corner rounding change. Watch for text that
  becomes hard to read against its background in any one skin.
- **5.7** `[human]` **On a laptop, drag the window slowly from wide to narrow.** The rail should
  move below the video at around half-screen width. The change should be a clean jump, not a
  stutter or a flash of overlapping content.
- **5.8** `[human]` **Follow a deep link to a field.** Open a link ending in `#field-title` on the
  phone; the page should scroll to that field and highlight it, with the field visible rather than
  hidden.

- **5.9** `[human]` **Long person names at max density.** `PersonPosterCard`'s name is
  `-webkit-line-clamp: 1` with `overflow: hidden`, so at 102px a long name is cut to one line
  (verified: card height stays 205px, nothing overflows — the layout does not break). What is
  *not* verified is whether the cut renders with a trailing ellipsis or as a hard mid-word chop.
  Look at a person with a genuinely long name at max density and judge whether it reads as
  deliberate truncation. Pre-existing, but the 16-column ceiling makes it bite at roughly 70% of
  the previous card width, so it is newly noticeable.

## 6. Regression

- **6.1** `[agent]` The metadata fold still expands and collapses, with its 200ms transition intact.
- **6.2** `[agent]` The empty-section collapse from `media-detail-films-people-handoff.md` still
  applies inside the rail: an empty Films section renders a text CTA and no heading.
- **6.3** `[agent]` Owner pages (`/owner/status`, `/owner/keys`, `/owner/trash`) render their tables
  full-width to the stage cap, with no nested inner cap shrinking them.
- **6.4** `[human]` The film detail page (`/films/{id}`) hero banner still looks right at 1920 and
  5120 — its aspect ratio differs from the video player (handoff §9.3).
