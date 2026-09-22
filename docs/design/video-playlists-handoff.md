# Video playlists — list, playlist page, producers, next-up — design handoff

**Ticket:** HOLODEX-442 (S2 pages + producers) · HOLODEX-443 (S3 next-up) · epic HOLODEX-438, F69 ·
**Status:** approved by the owner 2026-09-20 — panels 1, 2, 3, 5 as drawn; panel 4 (*Save as
playlist* placement, spec OQ3) resolved **A** (toolbar) · **Date:** 2026-09-20 ·
**Spec:** [video-playlists.md](../specs/video-playlists.md) P0-5..P0-10 ·
**ADR:** [ADR-104](../architecture/ADR-104-video-playlists-container-and-persistent-player.md) D4/D5

## Decision

Four surfaces, one affordance rule: **everything a playlist offers reuses an idiom the app already
has** — no new component family. The list page is `/people`'s row-card grid; the playlist page is
the browse grid under a title block; the detail rail gains a **PLAYLISTS** row that is the TAGS row
with a different noun; the producers use the `btn-quiet → inline form` pattern of *+ Add tag*; the
next-up strip is a single surface under the player with three states. Mocked against the real
routes and components (`web/src/routes/people/+page.svelte`, `web/src/routes/+page.svelte`,
`web/src/routes/media/[id]/+page.svelte`, `VideoCard`, `SortDropdown`, `ConfirmDialog`):

![Video playlists: the /playlists list with the new nav item and row cards, the playlist page with Play all / sort / visibility / remove / delete, the detail rail's PLAYLISTS row and Add-to-playlist picker, Save as playlist placement A vs B on the browse page, and the next-up strip under the player in its three states plus the Chrome PiP window with Media Session prev/next](video-playlists-mockup.svg)

| Panel | Surface | Idiom reused |
|---|---|---|
| 1 | `/playlists` list + nav item | `/people` row cards (`ul.grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3`, `rounded-theme border-rule bg-surface`), header nav link |
| 2 | `/playlists/[id]` | title block + `SortDropdown` + `.video-grid` of `VideoCard`; `ConfirmDialog` for delete; `.btn-*` treatments |
| 3 | detail rail PLAYLISTS row + picker | the TAGS row (chips + `+ Add …` `btn-quiet`), the tag inline form |
| 4 | *Save as playlist* on browse | **A:** toolbar group beside `Clear filters` → inline form · **B:** count line |
| 5 | next-up strip on `/media/[id]` | a `bg-surface border-rule` strip directly under the player column; Media Session |

## 1 · `/playlists` — list page and nav item

- **Nav:** a `Playlists` link after `Tags` (before `Films`, which is gated the same way) in
  `web/src/routes/+layout.svelte`. Owner: always. Visitor: only when
  `/capabilities.public_playlists > 0` (spec P0-5; ADR-104 D5). The link is a plain anchor like its
  neighbours; no badge, no count.
- **Title row:** `Playlists` in the display font (`skin-title`) + `N playlists` in `text-sm text-muted`
  (owner: total; visitor: public count). Right: **+ New playlist** as `.btn-accent` (outlined —
  affirmative but not the page's primary act; the page has no single primary).
- **Row card** (`li > a`, `rounded-theme border border-rule bg-surface px-4 py-2.5 hover:border-accent`):
  a stacked-frames glyph in `text-muted` (two offset 20px squares — the only playlist-specific mark;
  it stands in for the cover that P2-3 will add), name (`flex-1 truncate`), a visibility chip
  (**owner only** — `Private` outlined `border-muted text-muted`, `Public` outlined
  `border-accent text-accent`; visitors see no chip because every row they see is public), then
  the item count `text-xs text-muted`. Row order: `updated_at DESC`.
- **+ New playlist** expands *in place* (below the title row, not a modal) into
  `input[placeholder="Playlist name"]` + `Create` (`.btn-accent`) + `Cancel` (`.btn-quiet`) —
  the tag-add form verbatim. Enter submits, Escape cancels. On success navigate to the new
  `/playlists/[id]` (an empty playlist is a valid destination — spec story 11).
- **States:** loading = the page's existing skeleton convention (none — `/people` shows nothing
  until data; match it). Empty, owner: `No playlists yet.` + the New playlist form already open.
  Empty, visitor (direct visit while the nav item is hidden): `No playlists shared yet.` centred
  `py-16 text-sm text-muted`, exactly the `/people` empty-state treatment. Error: the page's error
  line, `text-warn`.

## 2 · `/playlists/[id]` — the playlist page

- **Title block:** name in the display font with the owner-only rename pencil (`✎`, same control as
  the video page title), then `N videos · <sort label>` in `text-sm text-muted`. Rename is inline
  (input replaces the title; Enter/blur saves, Escape reverts) — a `PATCH {name}`; there is no
  collision rule (duplicates allowed, spec P0-3).
- **Controls row** (right-aligned, wraps under the title at narrow widths — same `flex items-end
  gap-2` group as the browse toolbar):
  1. **Sort** — `SortDropdown` with a `manual` entry prepended (label `Manual order`), rendered only
     on this page: the component takes an `extra` list (or a `playlist` flag) rather than a fork —
     one dropdown, one `MEDIA_SORTS` source. Changing it `PATCH`es and re-renders in place; switching
     to `Random` mints a seed (the browse `shuffleSeed` helper) and shows `SortReroll` beside it,
     exactly as browse does.
  2. **Visibility** — a two-state segmented control `Private | Public` (`.btn-ghost` pair; the
     active half `border-accent text-ink`, the other `text-muted`). Owner only. `PATCH {visibility}`;
     no confirm — it is reversible in one click and nothing is sent anywhere.
  3. **▶ Play all** — the page's **one solid `bg-accent text-accent-ink`** action. Navigates to
     `/media/{first}?playlist={id}[&seed=]` and playback starts from that gesture. Disabled state
     (empty playlist): withdraw the affordance (`.btn-ghost` look, label unchanged) — never
     `opacity` on `text-muted`.
- **Grid:** `.video-grid` of `VideoCard {video}` in the playlist's order — the browse tile, unchanged,
  so `card_layout` and the skin flourishes apply for free. **Owner-only Remove:** an `×` in the
  tile's top-right corner, `absolute right-2 top-2 rounded-full bg-black/60 text-muted opacity-0
  group-hover:opacity-100 focus-visible:opacity-100` — the poster-upload button's exact treatment
  on the video page. `DELETE /playlists/{id}/videos/{videoId}`, tile leaves the grid without a
  reload, count decrements. No confirm (membership only; the video is untouched).
- **Delete playlist:** `.btn-quiet text-muted` at the bottom of the page, after the grid — a
  destructive act belongs at the end, not in the controls row. Opens `ConfirmDialog`
  (`title="Delete playlist?"`, body names the playlist and says *"The 12 videos stay in your
  library."*, `confirmLabel="Delete"`). On success navigate to `/playlists`.
- **Visitor:** no pencil, no controls except *Play all*, no `×`, no Delete. A private id renders the
  app's standard not-found page — indistinguishable from an unknown id (ADR-104 D5).
- **Empty playlist (owner):** the grid area shows `Nothing here yet — add videos from any video's
  page, or save a browse view as a playlist.` with links to `/` (browse). *Play all* withdrawn.

## 3 · Video detail rail — PLAYLISTS row and the picker

- **Placement:** a new labelled row directly **under TAGS** in the metadata rail
  (`web/src/routes/media/[id]/+page.svelte`, the tag row's block). Label `PLAYLISTS` in the rail's
  small-caps label style. It is content, so it renders for visitors too — with only the
  public-playlist chips, and no button (the visitor/owner rule: gate the *action*, never blanket-hide
  the content).
- **Chips:** one per member playlist, the TAGS chip (`rounded-full border-rule bg-surface px-2
  text-xs`), linking to `/playlists/[id]`. No chip is dismissible here — removal is the picker's or
  the playlist page's job, keeping the chip a link like a tag chip.
- **+ Add to playlist** (`.btn-quiet px-3 py-1.5 text-sm`, owner only) opens the **picker** anchored
  below the row: a `bg-surface border-rule rounded-theme` panel listing every playlist
  (`updated_at DESC`) as rows `✓/blank · name · count`. Clicking a row toggles membership
  (`PUT` / `DELETE …/videos/{id}`; ✓ in `text-accent`); the chip row updates immediately. Divider,
  then **+ New playlist…** in `text-accent`, which expands in place into `input[placeholder="Name"]`
  + `Create & add` (`.btn-accent`) — two requests (`POST /playlists`, then `PUT`), one interaction
  (spec P0-7). Escape or outside-click closes; Enter in the input submits.
- **Busy/error:** the row being toggled shows the rail's inline busy treatment (no spinner
  component exists; disable the row and keep the label at full contrast); a failure shows a
  `text-warn` line under the list, the toggle reverts.
- **Stress:** 40 playlists → the picker scrolls (`max-h-64 overflow-y-auto`), the New playlist row
  is pinned below the scroll region. Long names truncate in the row; the count never wraps.

## 4 · Browse page — *Save as playlist* (spec OQ3 — **A chosen** 2026-09-20)

Owner only. Both options post `from_query` = the F4.7 shareable filter string (`filtersToParams(f,
false)`), then navigate to the new page.

| | Placement | Why / why not |
|---|---|---|
| **A** *(chosen)* | **In the toolbar group** — `flex items-end gap-2` after `SortDropdown` / `SortReroll` / `Clear filters`, as `.btn-quiet` `Save as playlist`. Click → expands in place into `input[placeholder="Playlist name"]` + `Save` (`.btn-accent`) + `Cancel` — the tag-add form again | Sits with the other *result-set* controls, which is what it acts on; visible whether or not filters are active (no filter = save the whole library in the current sort, spec P0-8); one idiom for every producer |
| B | **In the count line** — `48 videos · Save as playlist` as a `text-accent` link under the toolbar | Reads as a footnote; the count line scrolls away with the grid at narrow widths; nothing else in the app puts an action there |

- **Feedback:** on success navigate to `/playlists/[id]`; the new page's count line is the
  confirmation (P1-4's toast is not in v1). On failure: `text-warn` line in the form, form stays open.
- **Responsive:** in A the toolbar already wraps at the tiers the responsive-width handoff set; the
  button wraps with `Clear filters`. The expanded form takes the same slot.

## 5 · `/media/[id]?playlist=…` — the next-up strip

- **Placement:** a single strip **directly under the player**, full player-column width, above the
  title card. `rounded-theme border border-rule bg-surface px-3 py-2`, `flex items-center gap-3`.
  Renders only when `?playlist=` resolves (visitor + private → it does not render and the param is
  inert; no error).
- **State — playing from:** left block `PLAYING FROM` label + playlist name as a `text-accent` link
  to `/playlists/[id]` + `· n of N` in `text-muted`. Centre: the next item's small thumbnail
  (`VideoCard`'s thumbnail derivative, 30×22 at the mock scale — use the list thumbnail tier) +
  `NEXT` label + title `· duration`. Right: `‹ Prev` (`.btn-ghost`) and `Next ›` (`.btn-accent`).
  Both navigate within the playlist order and **do** set the play-on-load intent (they are
  gestures).
- **State — last item:** `LAST IN` + name `· N of N — playback stops here`; `‹ Prev` stays,
  `↺ Start` replaces Next (`.btn-ghost`; navigates to item 1 with intent).
- **State — not in playlist** (stale link / removed): `NOT IN` + name `· this video was removed, or
  the link is stale`; one action `Play from start` (`.btn-accent`). `ended` does nothing.
- **Autoplay intent** is never in the URL; a reload or a shared link shows the strip and waits. The
  copy under the strip in the mock is explanatory, not UI.
- **Media Session:** while the strip is mounted, `navigator.mediaSession.metadata = {title}` and
  `nexttrack` / `previoustrack` handlers are set (cleared on unmount / leaving the context). That is
  what puts ⏮ ⏭ in Chrome's PiP window and on OS media keys — no UI to build for it.
- **Hotkeys (P1-1):** if shipped, `N` / `P` map to the same two actions and appear in F62's sheet.
- **Player element:** per ADR-104 D4 the `<video>` is the same node across items; the strip is a
  sibling of the player, never a wrapper, so it can mount/unmount freely without touching the
  element.

## Design tokens used

| Token | Usage |
|---|---|
| `bg-bg` / `bg-surface` / `bg-surface-2` | page · cards, strip, picker · inline form inputs |
| `text-ink` / `text-muted` | names, titles · counts, labels, secondary copy |
| `border-rule` / `border-accent` | card and chip borders · active segment, hover, focus |
| `bg-accent text-accent-ink` | **Play all only** (the one solid action per page) |
| `.btn-accent` / `.btn-ghost` / `.btn-quiet` | Create/Save/Next · Prev/segment/Start · Add to playlist, Save as playlist, Delete playlist |
| `text-warn` | inline errors |
| `rounded-theme` / `rounded-full` | controls, cards · chips, the tile `×` |
| `font-display` (`skin-title`) / `font-ui` | page titles · everything else |
| `.video-grid` + `.video-frame` | playlist page tiles (skin flourishes for free) |

No new token. No hardcoded value. The mock's hex values are Cinémathèque's `app.css` tokens.

## Interaction summary

| Element | Trigger | Result |
|---|---|---|
| Nav `Playlists` | click | `/playlists` |
| `+ New playlist` (list / picker) | click → Enter | `POST /playlists` → navigate (list) or `PUT` membership (picker) |
| Row card | click | `/playlists/[id]` |
| Rename pencil | click → Enter/blur | `PATCH {name}`; Escape reverts |
| Sort dropdown | change | `PATCH {sort}`; grid re-orders in place; `random` mints a seed |
| Visibility segment | click | `PATCH {visibility}`; chip on the list page updates |
| `▶ Play all` | click | `/media/{first}?playlist=…` + play (gesture) |
| Tile `×` | click | `DELETE …/videos/{id}`; tile leaves, count −1 |
| `Delete playlist` | click → confirm | `DELETE /playlists/{id}` → `/playlists` |
| `+ Add to playlist` | click | picker opens; row click toggles membership |
| `Save as playlist` | click → Enter | `POST /playlists {name, from_query}` → new page |
| `Next ›` / `‹ Prev` / `↺ Start` | click | navigate within order, play-on-load intent set |
| video `ended` (with next) | — | navigate to next, intent set, `play()` on `video.id === route id` |

## Accessibility

- Focus order follows reading order in every panel; the picker and the inline forms trap nothing —
  Escape closes/cancels, focus returns to the button that opened them.
- Picker rows are `button`s in a `ul[role=list]`; the ✓ is text, not colour alone (`aria-pressed`
  on the row).
- Visibility segment: `role=radiogroup`, two `radio`s, labelled *Visibility*.
- Tile `×`: `aria-label="Remove from playlist"`; reachable by keyboard (`focus-visible:opacity-100`).
- Next-up strip: `aria-live="polite"` on the `n of N` text so a hand-off is announced; buttons
  labelled *Previous in playlist* / *Next in playlist*.
- `Play all` is a link styled as a button (it navigates) — `<a>` with the button classes.

## Not in scope (spec Non-Goals)

Reorder handles · tile context menus / multi-select · cover image (the glyph is its placeholder) ·
nav-search rows · "in N playlists" chips on entity pages · a layout-level / mini player.

## QA

Numbered, grouped by tag; smoke rows are the agent's, human rows are the owner's.

**[smoke]**
1. `/playlists` renders for owner with rows + chips; for a visitor with ≥1 public playlist, rows
   without chips; the nav link is absent for a visitor with none.
2. `/playlists/[id]` visitor + private → the standard not-found page (status 404 from the API).
3. *Play all* lands on `/media/{first}?playlist=…` and the strip shows `1 of N`.
4. Reload that URL: strip shows, **no** autoplay.

**[agent]**
5. Three skins × panels 1, 2, 3, 5 — chip/segment/strip contrast, the `×` on hover, the solid
   *Play all* against each accent; `javascript_tool` computed-style check per the skin-QA note.
6. Picker with 40 playlists scrolls; New playlist row stays visible.
7. Long playlist name truncates in the row card, the picker row, and the strip.
8. `document.querySelector('video')` identity holds across `Next ›`.

**[human]**
9. PiP opened on item *n* is still open showing *n+1* after `ended` (Chrome).
10. ⏮ ⏭ present in the PiP window and OS media keys drive them.
11. Safari: unmuted `play()` after the hand-off (spec OQ2).
12. ~~OQ3 pick: A or B.~~ A, 2026-09-20.
