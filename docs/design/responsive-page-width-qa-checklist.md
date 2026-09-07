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
  **`FILMS_ENABLED=true` must be set** or `/films` 404s and the Films nav link is hidden — it is an
  env var (`internal/config`), not implied by the films config paths. The local `.claude/launch.json`
  sets it; that file is gitignored, so a fresh worktree needs it added. Note this also exposes
  HOLODEX-333 (the header nav overflows at ~768 with five nav links), which is *not* a page-layout
  fault — attribute any 768px overflow by counting overflowing elements **inside** the page's own
  container before blaming the layout.
- **1.2** Start the frontend: `preview_start` with `web`. Confirm it is on :5173.
- **1.3** Confirm the worktree is serving its own code, not the main worktree's: `.claude/launch.json`
  must exist in this worktree. Load a page and confirm a change from this branch is present.
- **1.4** Clear `holodex:media-density` from localStorage before the density tests in section 4.

## 2. Smoke

- **2.1** `[smoke]` `cd web && npm run check` passes with no new type errors.
- **2.2** `[smoke]` `cd web && npm run test` passes.
- **2.3** `[smoke]` `make test` passes (no backend change expected; guards against accidental breakage).
- **2.4** `[smoke]` No page-level width cap uses an arbitrary value — the stage must come from
  `max-w-stage`. `rg 'max-w-\[' web/src --glob '*.svelte'` should return only truncation limits
  (`max-w-[10rem]`-style) and the Films/People `max-w-[50%]` split, never a page wrapper.
- **2.6** `[smoke]` No field list re-declares the grid template — `rg 'minmax\(320px' web/src` should
  match only `app.css`. The four lists use the `field-grid` utility so the floor lives in one place.
- **2.5** `[smoke]` `owner/status` has **no** `mx-auto max-w-*` — its old `max-w-5xl` was identical
  to the layout's and could never bind, so it takes the stage. `owner/keys` and `owner/trash`
  **keep** `mx-auto max-w-4xl` on purpose: at 896px inside a 1024px layout they were the *binding*
  cap, and stage width would strand a row's Delete button ~2400px from its title. Do not "clean
  them up" — removing them is a ~3x widening, not a de-duplication.

## 3. Agent-verifiable geometry

Run each at a **fresh page load** at the stated width. Read values with `javascript_tool`
(`getBoundingClientRect`, `getComputedStyle`) rather than screenshots.

- **3.1** `[agent]` At 1024 on `/media/{id}`: exactly two grid columns. Measured 547 / 390
  (predicted 555 / 397; the gap is scrollbar width).
- **3.2** `[agent]` At 1920 on `/media/{id}`: measured 1069 / 764 (predicted 1078 / 770).
- **3.3** `[agent]` At 5120 on `/media/{id}`: the `<article>` measures exactly 2600px with a left
  offset of 1253px — capped and centred, not left-aligned. Zones 1503 / 1073.
- **3.4** `[agent]` At 768 on `/media/{id}`: a single 705px column; the rail zone's `top` is at or
  below the player zone's `bottom` (stacked, not beside).
- **3.5** `[agent]` Field grid column count on `/media/{id}`, from
  `getComputedStyle(dl).gridTemplateColumns`. Measured on a multi-field list: **1 at 1024** (356px),
  **2 at 1920** (361px each), **3 at 5120** (341px each); 1 at 412 and 768.
  **Read the track widths, not just the count** — `auto-fit` collapses unused tracks to `0px`, so a
  single-field list legitimately reports e.g. `[1039, 0, 0]`. That is correct (it lets one field
  span the row instead of leaving dead space), not a bug. Use a list with 4+ fields to exercise the
  multi-column case.
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
- **3.11** `[agent]` Focus order: DOM order is player zone → rail at both 768 (stacked) and 1920
  (two-zone), and no real `order-*` utility exists inside the `<article>` (computed `order` is `0`
  on both zones). **Beware `[class*="order-"]` — it also matches `border-*`;** match
  `/^(?:[a-z0-9]+:)*order-/` against `classList` entries instead.
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
  `posterColumns()` (the 2:1 ratio, now in `density.svelte.ts`) — 16 columns at 1536+ and 32 at
  3840+. Measured: **78x169px at 1536 (the tightest point on the ladder)**, 102x205px at 1920,
  103x207px at 3840. Names at 14px, no clipping with short names, no horizontal overflow. Confirm
  the count is exactly double the video grid's at the same density and width.
- **3.16** `[agent]` **First row eager-loads on the People poster grid.** `PersonPosterGrid` passes
  `eager={i < cols}`. With more people than one row holds, exactly the first `cols` images carry
  `loading="eager"` and the rest `lazy`. Guards the regression where the old literal `12` stopped
  matching one row once the ceiling moved.

- **3.17** `[agent]` **Rail contents and order** at 1920, owner view: the rail's headings read
  Tags, Films, People, Metadata, Manage, File, Completeness (Films/People self-omit when the video
  has none — see `media-detail-films-people-handoff.md`). The left zone carries only the title and
  the "More with …" shelves.
- **3.18** `[agent]` **Visitor view leaves the rail sparse.** With Owner view off, the rail holds
  only Tags and People — measured 240px tall against a 1102px player column at 1920. The grid
  tracks stay fixed by design (§5c), so the player is the same size for a visitor as for the owner;
  the cost is a tall empty right column. Confirm it renders without collapsing or stretching.
- **3.19** `[agent]` **The metadata fold has a magic ceiling.** `#metadata-fields` animates via
  `max-height: 6000px` when expanded — a sentinel sized when the field list was 864px wide with
  426px columns. In a 356px rail column values wrap far more, so the same list is materially taller.
  On a video with a full field set, expand the fold at 1024 and 412 and assert
  `scrollHeight < 6000`; above that the fold silently clips with no overflow affordance. If it ever
  crosses, replace the sentinel with `grid-template-rows: 0fr → 1fr` on a wrapper plus
  `min-h-0 overflow-hidden` on the inner — same animation, no ceiling.
## 4. Density ladder

The column-count model is retained (handoff §2e), so there is **no** preference migration to test —
a stored `holodex:media-density` keeps its existing meaning. What needs testing is the extended
ladder and the viewport-tracking slider range.

- **4.1** `[agent]` With no stored preference, the grid renders at the default density (4).
- **4.2** `[agent]` A stored value of `6` still yields six columns at a viewport whose cap allows
  it — the meaning of existing preferences is unchanged.
- **4.3** `[agent]` Seed a garbage value (`"abc"`), reload: falls back to the default without
  throwing. Console clean.
- **4.4** `[agent]` **Ladder steps at each new rung**, at max density and a fresh load. Verified:
  8 columns at 1536 (172px) and 1920 (220px), 12 at 2560 (195px), 16 at 3840 (222px) and 5120
  (302px). Card width must stay under ~350px at every rung — that ceiling is the ballooning this
  fixes. At 5120 a poster row is 453px tall, not 1878px.
- **4.5** `[agent]` **No dead slider stops.** At each of 1024 / 1280 / 1920 / 2560 / 3840 / 5120,
  the range input's `max` equals `capForWidth(innerWidth)`, so every reachable position changes the
  column count. Moving the slider one stop must always change `gridTemplateColumns`.
- **4.5a** `[agent]` **The slider hides where there is no choice.** No range input renders at 800px
  (cap 2, a single position) or 412px (cap 1, which would be an invalid `min > max` range). The
  grid still renders — 2 and 1 columns respectively.
- **4.6** `[agent]` **Inverted direction survives a dynamic max.** At each width above, dragging
  right yields *fewer* columns and dragging left yields *more* — `invertDensity` must invert
  against the current cap, not a fixed `DENSITY_MAX`. Test the smallest cap explicitly: at 1024
  (cap 3) the slider spans 2–3, position 2 gives 3 columns and position 3 gives 2.
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

- **5.10** `[human]` **Is 78px too small?** At 1536 wide and max density the People poster grid
  renders 16 columns of 78x169px cards — the smallest cards anywhere on the ladder. Open the
  People index in poster mode at roughly that window width and judge whether a face is still
  recognisable and the name still useful. If not, the fix is a People-specific rung, not a change
  to the video ladder.
- **5.11** `[human]` **Films and People side by side inside the rail.** When a video has *both* a
  linked film and linked people, `media/[id]/+page.svelte` renders them as
  `max-w-[50%] flex-none` beside `min-w-0 flex-1` — markup written for the old ~896px column that
  now lives in a 320–1073px rail. Structurally safe (the `min-w-0` and the 50% cap prevent
  overflow), but at the narrowest rail (390px at a 1024px viewport) that is roughly a 195px film
  column beside 171px of people, about two 80px tiles each. Open such a video at 1024 and judge
  whether it reads as a deliberate two-up or as two squeezed columns that should stack instead.
## 6. Regression

- **6.1** `[agent]` The metadata fold still expands and collapses, with its 200ms transition intact.
- **6.2** `[agent]` The empty-section collapse from `media-detail-films-people-handoff.md` still
  applies inside the rail: an empty Films section renders a text CTA and no heading.
- **6.3** `[agent]` Owner pages (`/owner/status`, `/owner/keys`, `/owner/trash`) render their tables
  full-width to the stage cap, with no nested inner cap shrinking them.
- **6.4** `[human]` The film detail page (`/films/{id}`) hero banner still looks right at 1920 and
  5120. It is an 8:3 band in the 1.4fr subject column — measured 401px tall at 1920 and 563px at
  5120. Confirm it reads as a hero band rather than a wall, and that the header still overlaps its
  lower third (the `-mb-14` coupling).
- **6.5** `[agent]` **Film detail zoning** (handoff §2c-i). At 1920: zones 1078 / 770, the rail's
  headings read Cast → Details → Full film (scene coverage appears only when a provider has billed
  a cast), and the Scenes `<section>` spans the full 1872px beneath both zones — **not** the subject
  column. At 5120 the section is exactly 2600px, left offset 1260px, with Scenes at 2600px.
- **6.6** `[agent]` **`stage-grid` drives both detail pages.** `getComputedStyle('.stage-grid')`
  reports `display: grid`, `gap: 24px`, and two tracks at ≥1024px on both `/media/{id}` and
  `/films/{id}`; a single track at 1023. The ratio and the 320px rail floor must exist only in
  `app.css` — grep that no route repeats `grid-cols-[minmax(`.

- **6.7** `[agent]` **Stage-aligned Scenes grid** (handoff §2c-ii). At 5120 with a film of ≤8
  scenes: the grid is 2600px wide and its left edge equals the stage section's left edge (1260px),
  with 302px cards — *not* 148px, and *not* stranded at x=24. `grid-template-columns` reports one
  fixed `302px` track per card, never more tracks than cards.
- **6.8** `[agent]` **The mode is inert below the stage.** At 2560 the grid is 2512px at x=24 with
  194px cards, and at 1920 it is 1872px at x=24 with 220px cards — both identical to the behaviour
  before `stageAligned` existed. At 412 a single 364px track. No overflow inside `main` at any width.
- **6.9** `[human]` **Does the growth read as deliberate?** A film with 9–12 scenes puts the grid
  partly outside the stage on both sides while the hero above stays centred. Unit tests pin the
  arithmetic, but whether the page reads as intentional or as a misaligned container needs eyes —
  the dev fixture's only film has 2 scenes, so this needs a film with a real scene list.