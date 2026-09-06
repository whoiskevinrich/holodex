# Design handoff: Films entity (F56)

**Spec**: [films-entity.md](../specs/films-entity.md) (F56, HOLODEX-279) ·
**ADR**: [ADR-085](../architecture/ADR-085-films-entity.md) · **Precedent**: this is an
**addendum** to [studio-entity-handoff.md](studio-entity-handoff.md) and
[field-source-of-truth-handoff.md](field-source-of-truth-handoff.md) — the `SourceSelect`
radiogroup, `CurationFieldRow` merge chips, and roving-tabindex picker mechanics are **inherited
unchanged**. This document specifies only what's new for films: the list/detail pages, the two
asymmetric attach pickers (RD9), the films row on three existing entity pages, and the
suspended-resolver-source display question ADR-085 §5 left open. Tokens-only, **QA all three
skins**.

New surfaces, in build order:
1. `/films` — entity list (poster-forward, unlike the logo-well People/Studio pattern)
2. `/films/{id}` — entity detail (poster header, full-film section, scenes list, Details chips)
3. `/media/{id}` — new Films section + video→film attach picker
4. Film→video attach picker (film-side, bulk, the heavier of the two per RD9)
5. `/people/{id}`, `/studios/{id}`, `/tags/{id}` — new films row
6. `/media/{id}` Details — film candidate chips on Album/Title + the suspended-source display
7. Global search — a Films result group

---

## Overview

A film is browsable like a person or studio, but its membership is **asserted, not derived**
(RD1) — a scene doesn't belong to a film because Holodex inferred it, it belongs because the
owner said so. That distinction has one visible UI consequence worth stating up front: **nothing
on the film page ever looks auto-populated**. Every scene, every full-film file, was put there by
an explicit attach action, so the empty state before any of that happens is not "not enough data
yet" (the Studio page's framing) — it's "attach something to get started," and the empty-state
copy and the picker affordances should read that way.

The second framing to carry into every screen: **`films_enabled` is subtractive while on**
(RD6). Turning the flag on doesn't just add a `/films` nav entry — it can make a full-film video
vanish from browse, search, and every entity page's video grid the moment it's attached as
full-film. The attach flow for a full-film file is the one place in this feature where an owner
action has a side effect elsewhere in the app, and the UI must say so at the moment of attaching,
not let it be discovered later as "where did my video go."

---

## 1. `/films` — list

**Poster-forward, not the logo-well row pattern.** Studio's list (`studio-entity-handoff.md`
§1) uses a compact horizontal row because a studio has no meaningful default image. A film's
default image *is* the portrait movie poster (P1-2) — this list should look like a poster grid,
closer to a media-browse card than a People/Studio row.

### Layout
- Grid: `grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6` — denser than the
  People/Studio 1/2/3-column grid because poster cards are narrow (portrait aspect), matching
  the density of the existing media-browse grid rather than the People grid.
- Card: `<a href={`/films/${f.id}`}>` — `rounded-theme border border-rule bg-surface
  overflow-hidden hover:border-accent`, poster image at `aspect-[2/3] object-cover` (P1-2's
  portrait role) filling the card top, then a text footer: `<div class="p-2">` with name
  (`text-sm text-ink truncate`), year + scene count on one muted line
  (`text-xs text-muted`, e.g. `2019 · 6 scenes`).
- **No-poster fallback**: a monogram plate, same construction as Studio's logo well (§1b of the
  studio handoff) but sized to the poster aspect (`aspect-[2/3]`) instead of the fixed 40×26
  well — first glyph of the name, `font-display`, `text-logo-plate-ink` on `bg-logo-plate`. Keeps
  the grid rhythm identical whether or not a poster exists yet, same rationale as Studio §1b.
- No A–Z jump bar in v1 — a film library plausibly sorts by year or name but poster grids don't
  scan alphabetically the way name-only rows do; defer to a future pass if it's requested. Sort
  reuses `PEOPLE_TAG_SORTS` (`name` | `random`) with a new `films` sort-preference key, same as
  Studio.

### States
| State | Render |
|---|---|
| Loading | `<p class="py-16 text-center text-sm text-muted">Loading…</p>` |
| Error | `<p class="py-16 text-center text-sm text-warn">Couldn't load films: {msg}</p>` |
| Empty | `<p class="py-16 text-center text-sm text-muted">No films yet — attach a video to a film from its media page to get started.</p>` |
| Populated | the poster grid above |

The empty-state copy is deliberately directive (per Overview) — there is no backfill that will
ever populate this page on its own, unlike Studio's "before the backfill has run."

---

## 2. `/films/{id}` — detail

**Two regions below the header, per RD4 — never merge them.**

### 2a. Header
- Poster (portrait, `aspect-[2/3]`, same fallback monogram as the list card) alongside name
  (`skin-title text-2xl font-semibold text-ink`), year, and description as a paragraph beneath —
  same "poster + metadata" composition as a movie detail page, not the name-only header Person/
  Studio use.
- Chips inherited from RD2/RD3: studio/people/tag values render as the **read-only union of the
  film's scenes** (linking to their own entity pages, same link treatment as the media-detail
  studio/person links) — these are *not* editable chips, they're derived display, so don't route
  them through `SourceSelect`.
- **Details section** (name/description/release date/poster — the film's own enrichable fields,
  RD2 "film-owned fields resolve like Person/Studio"): same `SourceSelect`/`CurationFieldRow`
  chips as Studio, same **`·record`** baseline label (`baselineKey='record'`), same hidden-when-
  no-resolved-field-beyond-name rule, same owner-gating. No new component work — parameterize
  exactly as Studio does.

### 2b. Full-film file section (RD4, P0-10)
- A **list**, not a slot — a film can hold multiple full-film files (a 2160p remux and a 1080p
  copy). Each row: filename, resolution/codec badge (reuse whatever the video card already shows
  for this), and a writeback control (P0-11) — this is the *only* place a film-page writeback
  button appears, and only on rows in this section.
- Empty when no full-film file is attached — render nothing (not a placeholder row); the section
  heading itself only appears once there's at least one row, mirroring Studio's "hide the whole
  Details section, don't show an empty box" convention.

### 2c. Scenes list (RD4)
- The existing media-grid component, fed by the film's owned (non-full-film) videos, each card
  additionally showing its scene number (or an "unnumbered" treatment — a muted `—` badge, not a
  blank space, so the owner can distinguish "no number" from "not loaded yet").
- Sort by scene number ascending, unnumbered scenes after all numbered ones (RD5: no ordering
  guarantee among unnumbered scenes — placing them last, in whatever order the API returns, is
  sufficient; don't invent a secondary sort).
- Empty state: `<p class="py-8 text-center text-sm text-muted">No scenes attached yet.</p>` plus
  the attach affordance (§4) — never a dead end with no picker in sight.
- **Edit in place (HOLODEX-326)**: owner-only, the scene-number badge itself becomes the edit
  trigger — a real `<button>`, not nested inside the card's link (a sibling overlay in the same
  corner, so clicking it never also navigates). Opens §8's shared dialog. A visitor, and the
  full-film section (§2b, which never shows a scene-number badge), see no change.

### 2d. Film → video attach entry point
The bulk picker (§4) opens from a persistent "Attach videos…" action in this region — not
gated behind a menu, since this is the primary way a film gets built out (RD9's film-side picker
is the heavier, more-used-in-practice surface per the spec's framing, not a rare action).

---

## 3. `/media/{id}` — Films section + video→film attach

New section on the video detail page, positioned alongside the existing Studio/People chips
(same vertical rhythm, own heading "Films").

### 3a. Current attachments
- Each attached film renders as a small chip/row: poster thumb (tiny, `aspect-[2/3]`) + film
  name + scene-number badge (or the full-film badge, see below) + a detach control (✕, owner-
  only) — modeled on the existing person/studio chip-with-remove pattern already on this page,
  not a new interaction.
- A row attached as **full-film** gets a distinct badge (e.g. `Full film` pill using the
  `.btn-quiet`-style neutral treatment, never a raw color) — this is the one place the owner is
  reminded, every time they look at this video, that it's the reason the file doesn't show up in
  browse. Don't bury this as a tooltip; render it inline.
- **Edit in place (HOLODEX-326)**: the scene-number badge itself is the edit trigger for a
  non-full-film row (owner-only) — opens §8's shared dialog. The `Full film` badge never becomes
  a button; a full-film attachment has no scene number to edit.

### 3b. Attach picker (P0-8, RD9 "video → film")
**New component**, `FilmAttachDialog.svelte`, modeled on `EntityPickerDialog.svelte`'s dialog
chrome (backdrop, `role="dialog"`, focus-trap `trapTab`, Esc-to-close, trigger-focus restore,
rise-in animation) and its **roving-tabindex** `role="listbox"`/`role="option"` result list —
but *not* a direct reuse, because the result shape and the confirm step both differ:
- **Result rows carry a poster thumb + name + year** (`GET /api/v1/films?q=`), not
  `EntityPickerDialog`'s plain name + count row — this is the "similar to the people enrichment
  modal" requirement from the original ask; picture a `EnrichPicker`-style result card scaled
  down to list-row height (thumb left, name/year stacked right), not the plain text rows Studio's
  merge picker uses.
- **Confirming a film doesn't attach immediately** — selecting a result (click or Enter, same
  roving-tabindex `onOptionKey` pattern) advances the dialog to a second, small step *within the
  same dialog* (don't reopen a new modal): the chosen film's poster/name at top, then two
  optional fields — a scene-number input (numeric, empty = unnumbered) and a "This file
  represents the entire film" checkbox. A short inline note under the checkbox states the
  subtractive consequence plainly: *"This file will no longer appear in Browse, search, or
  [person/studio/tag] pages while Films is enabled — its own page stays reachable."* Confirm
  commits `POST /media/{id}/films`; Back returns to the result list without losing the query.
- **Scene-number collision** (RD5): a 409 from the API renders inline, under the scene-number
  field, naming the occupant by title — e.g. *"Scene 6 is already {other video's filename}."* —
  never a silent overwrite, never a toast that disappears before it's read. The field stays
  focused/editable so the owner can pick a different number without restarting the flow.

---

## 4. Film → video attach (film-side, bulk) — RD9's heavier picker

**New component** (no existing picker is close enough to derive from — this is genuinely new
scope per RD9's explicit call-out). Opens from §2d.

### 4a. Why this isn't `FilmAttachDialog` again
The search space is the entire video library (tens of thousands of files, often meaningless
names) versus a few hundred films — a single-select, film-scale dialog doesn't work here. This
picker needs, all in one screen:
- **Default scope**: unattached-only videos (a checkbox or segmented toggle to widen to "all
  videos," off by default — the common case is populating a film from files that aren't spoken
  for yet).
- **Filters**: the film's own resolved studio and cast, offered as one-click filter chips above
  the results (pre-populated from the film's Details, not manually typed) — narrowing "which of
  my 40,000 files might belong to this film" is the entire point of this surface.
- **Free-text filename search** — a plain search input, since candidate filenames are often the
  only signal ("scene_04_final.mkv").
- **Multi-select**: checkboxes on each result row, a running "N selected" count, and a "Select
  all visible" affordance for the filtered set.
- **Already-attached-elsewhere flag**: a candidate already attached to a *different* film shows
  an inline badge (e.g. `Also in: {other film name}`, muted, non-blocking) — legal per RD4 but
  worth surfacing since it's usually accidental from this direction.

### 4b. Layout
- Full-screen-ish dialog (wider than `FilmAttachDialog`'s `max-w-lg` — this needs room for a
  results table, not a narrow list): filter chip row, search input, then a scrollable results
  list with checkboxes, each row showing filename + resolution badge + the already-attached
  flag if present.
- **Roving-tabindex still applies** to the results list (arrow-key navigation between rows,
  Space to toggle a checkbox) — don't regress the a11y contract just because this picker is
  denser; it's the same convention as every other selectable list in this codebase.

### 4c. Commit step
- A footer bar, sticky at the bottom of the dialog, shows the selection count and a **starting
  scene number** input (numeric, optional — omitted means every selected video attaches
  unnumbered) plus the bulk-attach button. Selected videos number sequentially from that
  starting value in the order they appear in the results list (not selection order — state that
  explicitly in the UI copy: *"Numbered sequentially from N in the order shown below."*).
- **All-or-nothing** (per spec P0-9/repo semantics): if any file in the batch collides on a
  scene number, the whole commit is rejected — surface this as a single error naming the first
  colliding number/occupant, not a partial-success state. The dialog stays open with the
  selection intact so the owner can adjust the starting number and retry.
- No full-film toggle here — bulk attach is for scenes; marking a file full-film is a one-at-a-
  time decision made from the video's own attach flow (§3b) or (future) a per-row toggle if this
  proves too restrictive in practice — not scoped for v1's bulk flow.

---

## 5. Films row — Person/Studio/Tag detail pages (P0-5, RD6 consequence)

- New section, same visual tier as the existing Details section — a small horizontal row of
  poster-thumb cards (reuse the `/films` list card at a smaller fixed size, not a new component),
  each linking to `/films/{id}`.
- **Shown only when `films_enabled` is true and at least one film matches** — absent otherwise,
  never an empty "Films" heading with no cards (the project's standing rule against an
  informational dead end with no affordance applies in reverse here too: don't show the heading
  if there's nothing to show and nothing to *do* about it — this row has no attach affordance of
  its own, unlike the picks above).
- Placement: below the existing video grid, since it supplements rather than replaces it —
  this is explicitly compensating for full-film videos disappearing from that grid, so it reads
  naturally as "and here's what you're not seeing above."

---

## 6. Album/Title film candidate chips + the suspended-source display (ADR-085 §5)

### 6a. Film candidates on the video's Details section
- When one or more films are attached to a video, each contributes a candidate chip to that
  video's `album` field row (and `title`, only from the video's full-film-flagged attachment, if
  any) inside the **existing** `SourceSelect` radiogroup — no new chip component. The chip
  **label is the film's name**, not a generic "film" label — with two films attached to one
  video, the row must show two distinguishable chips (e.g. `The Great Escape` and `Anthology Vol.
  2`), not two chips both labeled `film` with no way to tell them apart. This is what makes RD7's
  "two films = a normal decision-UI conflict" actually legible.
- Exact source-key wiring (`provider:film:<id>`) is a resolver concern already settled in
  ADR-085 §4 — nothing new for design here beyond the label rule above.

### 6b. Resolved decision: no new "source unavailable" visual state
ADR-085 §5 flagged this as an explicit design action item: when `films_enabled` flips off, a
video's Album/Title decision that names a suspended film source resolves to nothing rather than
silently falling back to `file`, and the ADR asked design to give that an explicit treatment "so
it doesn't read as data loss."

**Resolved: it doesn't get one.** This isn't a gap — it's a deliberate decision not to
special-case films. The mechanism ADR-085 §4/§5 chose (film candidates as caller-injected,
gated enrichment rows; suspension as simply not injecting them) means a suspended film decision
hits the **exact same code path** as any other decided-but-currently-unmatched provider — the
case `field-source-of-truth-handoff.md`'s Edge Cases already documents: *"Decision references a
provider later un-matched/cleared — display falls back to the file chip."* A film-decided Album
field behaves identically: if the file itself still carries a value, that's what shows (the
existing fallback-to-file chip, unchanged markup); if the file is also empty, the row simply
doesn't render (the existing "all sources empty" edge case). Inventing a distinct "source
unavailable" badge for films specifically would be new, bespoke UI for a state that is otherwise
indistinguishable from an existing, already-shipped, already-undesigned edge case — the
inconsistency of treating one decided-source-lost-its-match case specially and not the others
would be a worse outcome than the "counterintuitive but matches everything else" behavior this
closes on. This closes ADR-085 §5's action item.

---

## 7. Global search — Films result group

- Add a **Films** group to the global mixed-entity search results (`SearchResultsPanel.svelte`),
  mirroring the People/Studios/Tags groups — same header + row styling, backed by `films_fts`
  (excluded entirely when `films_enabled` is false, per RD6 — the group doesn't render empty, it
  doesn't exist).
- Result row shows the poster thumb (small) + name + year, not just name — the one group where a
  thumbnail materially helps disambiguation (two films can share a name across different years).
- Full-film video files never appear as *video* search hits while the flag is on (RD6) — no
  change needed to the Videos group beyond what P0-4's hiding already guarantees.

---

## 8. Scene number edit (HOLODEX-326) — shared by §2c and §3a

Attach (§3b, §4) and detach (§3a) already covered the film↔video link's lifecycle; this closes
the gap in between — correcting an already-attached video's scene number without a
detach+reattach round trip.

- **Trigger**: the scene-number badge itself, wherever it already renders (§2c's scenes grid,
  §3a's Films chip row) — owner-only, no separate pencil icon. Clicking it opens the same small
  dialog, `EditSceneNumberDialog`, regardless of which page it was opened from.
- **Dialog**: a single numeric field (empty = unnumbered, matching §3b's attach-step field
  exactly — same placeholder, same validation message), labeled with whichever name the calling
  page *isn't* already showing on screen (the video's title from §2c, since the film name is the
  page header; the film's name from §3a, since the video's title is the page header). Chrome is
  the shared `ConfirmDialog` (focus trap, Esc/backdrop cancel, focus-restore-on-close) rather than
  a new modal shell — this is a one-field form, not a search-then-confirm flow like §3b/§4.
- **Collision** (RD5, same rule as attach): a 409 renders inline as a plain error string naming
  the occupant — *"Scene {n} is already {title}."* — no silent swap, no auto-bump, dialog stays
  open with the value intact so the owner can pick a different number without restarting.
  Re-submitting the video's own current number is a no-op success, not a collision, since nothing
  actually changed.
- **On success**: both pages patch the one affected item in their already-loaded local list
  (mirroring how detach already updates §3a's list without a reload) — §2c's `scenes` array is
  itself the sort input (`$derived` sort), so patching it in place re-sorts automatically with no
  stale-order risk and no extra round trip.

---

## Design tokens used

All inherited — no new tokens (unlike Studio's `logo-plate-ink`, which films reuse as-is for the
monogram fallback). Reference (from [theming.md](theming.md)):

| Token | Usage here |
|---|---|
| `bg-logo-plate` / `text-logo-plate-ink` | poster-fallback monogram (list card, films row, film-candidate-less states) |
| `text-ink` / `text-muted` | names/descriptions / secondary metadata (year, scene count, badges) |
| `text-warn` / `border-warn` | scene-number collision error, load errors |
| `bg-surface` / `bg-surface-2` | cards / hover / picker rows |
| `border-rule` / `border-accent` | card border / hover + focus + selected picker row |
| `bg-accent` / `text-accent-ink` / `text-accent` | primary attach actions, active picker selection |
| `.btn-accent` / `.btn-ghost` / `.btn-quiet` | attach/confirm (accent), detach/cancel (ghost), full-film pill / filter chips (quiet) |
| `rounded-theme` | all card/chip/dialog radii |
| `skin-title` / `font-display` | film `<h1>`, monogram glyph |
| `.video-grid` (+ `data-layout`) | scenes list body |

**Token guard**: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` must stay empty. **Muted-disabled guard**: `rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'` must stay empty (applies to the bulk-picker's disabled commit button, the full-film pill, etc. — withdraw the affordance instead of dimming the label, per the theming rule).

---

## Responsive behavior

| Breakpoint | `/films` grid | Film→video bulk picker |
|---|---|---|
| `<640px` (mobile) | 2 columns | Filter chips wrap; results list single-column; sticky footer bar remains full-width |
| `640–1024px` (sm/tablet) | 3 columns | Same as mobile, more visible rows |
| `≥1024px` (lg/desktop) | 4–6 columns (`md:4`, `lg:6`) | Two-pane feel is unnecessary — single scrollable list is enough at this scale |

The scenes list and full-film section inherit `.video-grid`'s own responsive rules unchanged.
`FilmAttachDialog` stays `max-w-lg` at every breakpoint (it's a narrow single-select flow); the
bulk picker is intentionally wider (`max-w-3xl` or similar) since it's a results-table
interaction, not a short list.

---

## Edge cases

- **Film with zero videos** (created via enrichment/search before anything is attached) —
  detail page shows poster header + Details (if enriched) + both "no full-film files" and "no
  scenes attached yet" empty states, each with its own attach entry point. Never a single
  generic "empty film" message that hides which region to act on.
- **Film with only a full-film file, no scene rips** — the scenes list empty state and the
  full-film section both render; cast/tags still populate from RD2's union (the full-film file
  is a baseline source even though it's excluded from the scenes list display).
- **Two films attached to the same video** — both film-name chips appear on that video's Album
  row (§6a); if the owner has decided neither, the field resolves per the normal precedence
  chain (ADR-085 §4) with no additional UI beyond the two chips being present and selectable.
- **`films_enabled` toggled off mid-session** — the SPA should treat this as it treats any other
  capability flag flipping (existing `mcp_enabled`-style precedent): `/films` nav/routes
  disappear, films rows on person/studio/tag pages disappear, and any open film-related dialog
  should close gracefully rather than error against a now-absent endpoint.
- **Scene-number collision on bulk attach** — see §4c; the whole batch is rejected, not a
  partial commit, and the dialog stays open with selections intact.
- **Long film name / international title** — `truncate` on card/list rows; full name wraps in
  the detail `<h1>` and the attach-dialog confirm step (no truncation once it's the single
  focused subject of a step).
- **No poster yet (pre-enrichment or P1-1 not yet shipped)** — monogram fallback everywhere a
  poster would render (list card, detail header, films row, picker result rows). Never a broken
  `<img>` or a blank rectangle.

---

## Accessibility

- **List/films-row cards**: each is a single `<a>`; poster `<img>` (when present) carries
  `alt="{name} poster"`; the monogram fallback is `aria-hidden` (name is adjacent text), same
  convention as Studio §1b.
- **`FilmAttachDialog`**: inherits `EntityPickerDialog`'s full a11y contract — focus-trapped
  dialog, `role="dialog"`/`aria-modal`, `role="listbox"`/`role="option"` roving tabindex,
  Esc-to-close, trigger-focus restore on close. The second "scene number + full-film" step stays
  inside the same dialog element (focus moves to the first field of that step on advance, not
  reset to the dialog root) so Tab order stays predictable across the two-step flow.
- **Bulk film→video picker**: roving tabindex across result rows; checkboxes are real
  `<input type="checkbox">` elements (never a div-with-click) so they carry native
  checked-state semantics; the sticky commit footer's inputs are reachable via the same trap
  (don't let the footer sit outside the dialog's tab loop).
- **Full-film pill / already-attached-elsewhere badge**: convey meaning via label text, never
  color alone — both are small text badges, not colored dots.
- **Film candidate chips on Album/Title**: unchanged `SourceSelect` a11y (radiogroup/radio,
  `aria-checked`, roving tabindex) — the only change is chip *label* text (film name instead of
  provider name), which is already how the component renders any chip label.

---

## QA checklist (3-skin)

Conventions ([[feedback-qa-checklist-numbering]]): every item numbered `section.item`, tagged by
verifier — `[smoke]` automated, `[agent]` agent-driven live QA, `[human]` needs human eyes.
Skins: **Cinémathèque · Broadcast · Brutalist**, switched via the header picker.

### §1 Setup
- **1.1** `[agent]` Enable `films_enabled` on a preview stack
  ([[reference-holodex-preview-testbeds]]). Create ≥2 films: one with a full-film file + no
  scenes, one with ≥3 numbered scenes + ≥1 unnumbered scene + no full-film file.
- **1.2** `[agent]` Attach one video to two different films (to exercise the two-chip Album
  conflict, §6a) and confirm a genuine scene-number collision candidate exists (a taken number)
  for the collision-error path.
- **1.3** `[agent]` Have ≥1 video in the library that is a candidate for the bulk picker
  (unattached, filename-only, no useful metadata) to exercise §4's filename-search path.

### §2 Smoke
- **2.1** `[smoke]` `/films*` routes return 404-family (not 401/403) when `films_enabled=false`;
  fully absent from the router, not merely gated (P0-3).
- **2.2** `[smoke]` `film_videos` rows are byte-for-byte unchanged across a full scan/enrich/
  decision cycle (P0-2's regression test) — a design-adjacent smoke check worth re-confirming
  here since it's what makes "nothing on this page is auto-populated" (Overview) actually true.
- **2.3** `[smoke]` Scene-number collision on both single-attach and bulk-attach returns the
  occupant-naming 409; bulk attach is all-or-nothing (no partial rows committed on collision).
- **2.4** `[smoke]` Toggling `films_enabled` off does not delete any `films`/`film_videos` row
  and does not revert a previously-written Album/Title file value.
- **2.5** `[smoke]` (HOLODEX-326) `PATCH /films/{id}/videos/{videoId}` re-numbering a scene to a
  number ANOTHER scene already holds 409s naming that occupant; re-numbering it to its OWN
  current number succeeds (not a collision); 404 on a video not attached to that film.

### §3 Agent live QA (preview tools against §1 stack)
- **3.1** `[agent]` `/films` renders the poster grid; no-poster films show the monogram
  fallback at identical card size to poster films (alignment parity, mirroring Studio's logo
  well). **All 3 skins.**
- **3.2** `[agent]` Open the scenes-only film: header, empty full-film-section (no heading
  rendered), populated scenes list ordered by scene number then unnumbered-last, each unnumbered
  card showing the muted `—` badge (not blank). **All 3 skins.**
- **3.3** `[agent]` Open the full-film-only film: full-film section shows the file with a
  writeback control; scenes list shows its empty state with an attach entry point; Details
  cast/tags are still populated from that file (RD2 union still applies).
- **3.4** `[agent]` On the scenes-only film, open the bulk film→video picker: default scope
  excludes already-attached videos; studio/people filter chips are pre-populated from the film
  and narrow results on click; filename search finds the filename-only candidate from §1.3;
  select 3, set a starting scene number, commit — all 3 attach sequentially in list order with
  the stated numbering. **All 3 skins**, roving-tabindex keyboard-only pass on at least one skin.
- **3.5** `[agent]` From `/media/{id}` on an unattached video, open `FilmAttachDialog`: search
  returns poster+name+year rows; selecting one advances to the scene-number/full-film step
  in-place (no new modal); checking "entire film" shows the subtractive-consequence copy;
  confirm attaches; the video's Films section now shows the chip with a "Full film" pill.
  **All 3 skins.**
- **3.6** `[agent]` Trigger a scene-number collision from both `FilmAttachDialog` (single) and
  the bulk picker: inline error names the current occupant by title in both cases; the dialog
  stays open and editable, not dismissed.
- **3.7** `[agent]` On the two-films-attached video (§1.2), the Album field's `SourceSelect` row
  shows **two distinct film-name chips** (not two identically-labeled `film` chips). **All 3
  skins** (confirm both chip labels stay legible/truncate correctly at `max-w-[14rem]`).
- **3.8** `[agent]` Suspended-source check (§6b): decide the Album field to one of the two film
  chips, then toggle `films_enabled` off. Confirm the field's display matches the existing
  "decided source unmatched" behavior exactly (falls back to the file chip if the file has a
  value, or the row disappears if not) — **no** new badge/pill appears. Toggle the flag back on;
  confirm the decision is restored with no owner action.
- **3.9** `[agent]` Visit a person/studio/tag page whose only video is a full-film-flagged file:
  confirm the video grid is empty (or shows only other, non-full-film videos) while the new
  films row surfaces that film with a working link. **All 3 skins.**
- **3.10** `[agent]` Global search for a film name (with `films_enabled` true) returns a Films
  group with poster thumb + name + year; searching the same term with the flag off returns no
  Films group at all (not an empty one).
- **3.11** `[agent]` (HOLODEX-326, §8) On the scenes-only film, click a scene's number badge:
  `EditSceneNumberDialog` opens pre-filled with its current number, labeled with that scene's
  video title. Save a new number — the card re-sorts into position with no page reload. Repeat
  from `/media/{id}`'s Films chip row on that same video: the dialog now labels with the film's
  name instead. Trigger a collision from each entry point and confirm the inline "Scene {n} is
  already {title}" error (same string §3.6's attach-time collision already uses). **All 3 skins.**

### §4 Human
- **4.1** `[human]` Visit `/films` in each skin. It should read as a movie-library grid — denser
  and more poster-forward than the People/Studio pages, but still unmistakably part of the same
  app (same borders, same hover treatment, same fonts reacting to the skin).
- **4.2** `[human]` Open a film with several scenes. The two-region layout (full-film section vs.
  scenes list) should be immediately legible — you shouldn't have to read a label to know which
  list is which.
- **4.3** `[human]` Attach a video to a film from the video page, checking "entire film." The
  warning that the file will disappear from Browse should be impossible to miss before you
  confirm — read it as a genuine heads-up, not fine print.
- **4.4** `[human]` Open the bulk film→video picker on a film and try to find 3-4 files for it
  using the studio/people filter chips plus filename search. It should feel faster than
  attaching one at a time from the video page — if it doesn't, something about the filtering or
  layout needs another pass.
- **4.5** `[human]` Turn `films_enabled` off, then back on, on a library that already has film
  decisions made. Nothing should look broken or reset in between — the Album fields on affected
  videos should end up exactly where they started.
