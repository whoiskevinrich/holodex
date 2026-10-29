# Design Handoff: Writeback dialog — the Poster row, grouped by source

**Specs**: [Choose the video poster in the writeback dialog (F85)](../specs/writeback-poster-sources.md)
(HOLODEX-599) · [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Sync state ·
**Architecture**: [writeback.md](../architecture/writeback.md) (the write ledger as the image sync
witness, the embed path) · [media-ingest.md](../architecture/media-ingest.md) (poster tiers) ·
[security-perimeter.md](../architecture/security-perimeter.md) (asset-host allowlist)
**Builds on**: [writeback-cockpit-handoff.md](writeback-cockpit-handoff.md) (HOLODEX-400 — every rule
there applies; this doc only covers the image chooser shape and the Poster row's sources) ·
[thumbnail-frame-picker-handoff.md](thumbnail-frame-picker-handoff.md) (F84 — where `·frame` comes from).
**Theming contract**: [theming.md](theming.md) — **tokens only, QA Cinémathèque.**
**Surface**: `web/src/lib/components/curation/SourceImageTiles.svelte` (groups) ·
`web/src/lib/components/writeback/WritebackFormDialog.svelte` (the mount; the upload note goes) ·
`web/src/routes/media/[id]/+page.svelte` (Maintenance button label only).
**Issues**: [HOLODEX-599](https://whoiskevinrich.atlassian.net/browse/HOLODEX-599) (this revision,
absorbs HOLODEX-597) · [HOLODEX-403](https://whoiskevinrich.atlassian.net/browse/HOLODEX-403) (the tile
chooser) · [HOLODEX-600](https://whoiskevinrich.atlassian.net/browse/HOLODEX-600) (several posters per
provider, later).
**Approved**: by the owner, 2026-10-27 (mockup below; at phone width the groups wrap as units).

![The writeback dialog's Poster row in Cinémathèque at real sizes. Tiles sit in captioned groups: In the file, Holodex (frame and upload), Automatic (only when a generated frame exists) and one group per provider, with "no poster" text for a provider that has none. A: arrival after Pick a frame, the frame tile checked and the gutter will-write. B: after Write, collapsed as matches the file. C: a file with no cover art, Automatic shown, a pending provider tile. D: only one source. E: every group at dialog width, whole groups wrapping. F: phone width, groups wrapping as units onto three lines.](writeback-poster-chooser-mockup.svg)

---

## Overview

The Poster row is the one place the owner chooses a video's poster (F85). It offers every image
the owner can see: the file's own cover art, the newest frame they picked, their newest upload,
the automatic frame, and each provider's poster. Those come from different places, so the tiles
are grouped by where each one came from, and each group has a caption. Everything else is the
cockpit's model: picking a tile stages it, Write confirms it, and the gutter says what Write will do.

---

## Decided visual spec

### 1. The image row is a cockpit row

`isCockpitRow` admits `display === 'image_url'`. The row gets the staged pick,
`willWrite` / `savesDecisionOnly`, the `↧` / cylinder / `○` / `=` gutter, the header destination
line (`→ cover.jpg` / `→ QuickTime:CoverArt` per container; `no file tag for this container`
when unmapped), and the `change` toggle on `=`. Nothing image-specific in the row shell.

### 2. Groups

The chooser is a row of **groups**. Each group is a caption above a **list** of tiles.

| Order | Caption | Tiles | Shown when |
|---|---|---|---|
| 1 | `In the file` | `·file` — the cover art embedded in the file, or the **no cover / art** placeholder | always |
| 2 | `Holodex` | `·frame` (newest picked frame), then `·upload` (newest upload) | when either exists; each tile only when it exists |
| 3 | `Automatic` | `·automatic` — the frame Holodex generated | only when a generated frame exists (a file with no cover art) |
| 4… | the provider's name (`provider-1`) | that provider's poster | one group per enabled provider, in the provider order the row uses today |

| Element | Treatment |
|---|---|
| Group row | `flex flex-wrap items-start gap-x-5 gap-y-3` — **whole groups wrap**, at every width (owner decision for phone width) |
| Group | a plain `<div>`; caption above, then the tile list |
| Caption | `text-[0.65rem] uppercase tracking-wide text-muted`, `whitespace-nowrap`, `mb-1` |
| Tile list | `flex flex-wrap gap-3` — a list even when it holds one tile, so HOLODEX-600 adds tiles to a provider group without changing the layout |
| Provider with no poster | its caption, then `no poster` as `text-[0.65rem] text-muted` text. **No tile and no dashed slot** — a dashed image slot is the ghost slot, which means "add something here" |
| Group with no tiles (Holodex, Automatic) | not rendered at all — no caption |

### 3. Tiles

Tiles are unchanged from HOLODEX-403 apart from where they sit.

| Element | Treatment |
|---|---|
| Tile | `<button role="radio" data-seg={key}>`, `flex flex-col items-center gap-1`; an `h-20 w-auto min-w-14 object-contain rounded-theme border` image, caption below in `text-[0.65rem]` with the radio dot + `·{label}` |
| Width | set by the image: a 2:3 poster is ~54px (the 56px floor), a 16:9 frame 142px. The button is as wide as its caption when that is wider (`·provider-1` measured ≈ 63px in the mockup's fallback font; Archivo runs wider) |
| Staged | `border-2 border-accent`, filled dot, caption `text-accent` |
| Pending (RD6) | as staged but `border-dashed`, hollow dot, caption `·provider-1, pending` |
| Unstaged | `border-rule`, hollow dot, caption `text-muted hover:text-ink` |
| `·file` image | **always the file's own cover art** — never the page's served poster. When the file has none, the placeholder: `w-14 border-dashed`, two muted lines `no cover` / `art` |
| `·frame`, `·upload`, `·automatic` images | the stored images Holodex holds for each |
| Provider image | the candidate URL (already allowlist-gated by the resolver; a degraded candidate never reaches a tile) |
| `·frame` hover/focus (P1-1) | `title="frame at 12:40"` — the picked time, `m:ss` or `h:mm:ss` |
| No Custom tile | unchanged — a pasted URL can't pass the asset-host allowlist |
| Busy | the group row is `pointer-events-none` and guarded in the handler — never `opacity` |

### 4. Keyboard

The whole chooser is **one** `role="radiogroup"` (`aria-label="Source of truth for Poster"`) — the
groups are layout, not separate radio groups.

- **One tab stop**: the staged tile has `tabindex="0"`, all others `-1` — the chip row's rule,
  unchanged.
- **Arrows walk every tile in reading order across groups**: Right/Down → next, Left/Up → previous,
  wrapping from the last tile back to `·file`. Moving stages, as in the chip row. Space/Enter stage.
- A provider with **no poster has no tile, so it is skipped**. Its caption is ordinary text inside
  the radiogroup.
- The handler works over the flattened tile list (`groups.flatMap(g => g.chips)`), so HOLODEX-600's
  extra tiles join the order with no keyboard change.

### 5. States

| State | What the row shows | Mockup |
|---|---|---|
| Arrival after **Pick a frame** (or an upload) | `·frame` checked (solid), header `·frame → cover.jpg`, gutter `↧` (will-write) | A |
| After Write | collapsed `=` row: the decided image as the `h-8` thumbnail, `— matches the file`, `change` | B |
| File with no cover art | `·file` placeholder; **Automatic** group shown; with no decision, the RD6 pending tile (here `·provider-1, pending`) | C |
| Only one source | one group, one tile — the row still renders, so the owner can see and decide | D |
| Many sources | whole groups wrap to a second line; where the break falls depends on caption widths | E |
| Phone (375px) | the tile area is ~281px. `Holodex` (208px) can't share the first line with `In the file`, so it starts its own line: three lines in all | F |

### 6. What Write does

| Pick | Gutter | Write |
|---|---|---|
| `·frame`, `·upload` or `·automatic` (new or changed) | `↧` | the decision is saved and the image is written to the file ([writeback.md](../architecture/writeback.md) has the path) |
| provider tile (new or changed) | `↧` | the decision is saved and the provider's poster is written to the file, as today |
| any tile already standing, ledger matches | `=` | nothing (row collapsed, `change` available) |
| `·file` on a decided row | cylinder | decision only — the file keeps what it has |
| untouched, undecided | `○` | nothing |

Picking a frame or uploading on the page sets the decision to `·frame` / `·upload`, so the dialog
opens in state A: the row is in the decided group, `↧`, counted in the footer. Picking a tile here
changes the page's poster once the footer action succeeds (Write, or Save for a decision-only
pick), never on the pick itself.

### 7. Copy

| Where | Text |
|---|---|
| Group captions | `In the file` · `Holodex` · `Automatic` · `{provider}` |
| No-poster line | `no poster` |
| Tile captions | `·file` · `·frame` · `·upload` · `·automatic` · `·provider-1` · `·provider-1, pending` |
| File placeholder | `no cover` / `art` |
| `·frame` title | `frame at {time}` |
| Tile `aria-label` | `{label} from {source}{, pending}` — e.g. `Poster from frame` |
| **Removed** | the note `Your uploaded poster stays on the page; this row is the cover art inside the file.` and the `cover art` / `in file` placeholder. An upload is a tile like any other, and `·file` always shows the file's cover art |
| Media page, Maintenance → Thumbnail | `Regenerate thumbnail` → **`Pick a frame`** (`btn-ghost`, unchanged). While the picked frame saves: `Saving frame…` (was `Regenerating…`) |
| Everything else | the cockpit's |

### 8. Accessibility

- One roving radiogroup (§4): Tab never lands on an unstaged tile, and the dialog's `focusables()`
  exclusion holds.
- Images are `alt=""` inside the button; the caption and `aria-label` carry the meaning. The
  placeholder's words and the group captions are real text.
- Selection is never colour alone: border weight/dash + dot + `aria-checked`.

### 9. Edge cases

- **A provider candidate degraded** (off-allowlist URL): the resolver turns the field into text and
  the row renders as a chip row, not tiles. Unchanged.
- **Removing the chosen upload** on the page ([F85 P0-9](../specs/writeback-poster-sources.md)):
  `·upload` disappears and the row next opens undecided (`○`), with the RD6 pending tile if there
  is one.
- **A chosen image missing from storage** (e.g. after a restore): the spec leaves this to the build.
  The design rule is simply that a tile with no image is not rendered, and the row falls back as for
  a removed upload.
- **Container without a cover target**: header `no file tag for this container`; any pick is a
  decision-only save (cylinder).
- **Write fails**: reported on the page as today; the decision stands, the ledger gains no row, the
  row reads `↧` again next open.

### 10. Verification checklist

Numbered `section.item`, tagged by verifier, grouped by tag.

#### Setup
- 10.0 `[smoke]` `make test` + `cd web && npm run check && npm run test` green.

#### Agent
- 10.1 `[agent]` Vitest: the group builder orders groups file → Holodex → Automatic → providers;
  omits Holodex with neither image, omits Automatic with no generated frame; a provider with no
  candidate yields a group with an empty list; every group's tiles are a list.
- 10.2 `[agent]` On a video whose file has cover art: Pick a frame → Use this frame. The page poster
  is the frame at once. Open the dialog: Poster row is state A — `·frame` checked solid, `↧`, footer
  counts it, the `·frame` tile's `title` is `frame at …`.
- 10.3 `[agent]` Write (MKV, then MP4): when the job lands, the file's cover art is the picked
  frame; re-open → state B (`=`, `— matches the file`, the frame thumbnail).
- 10.4 `[agent]` Upload a poster, then re-open: `·upload` checked and `↧`; `·frame` still present
  (newest of each kind, P0-8); `·file` shows the file's own cover art; **no** upload note and no
  `cover art / in file` placeholder anywhere in the dialog.
- 10.5 `[agent]` A file with no cover art: `·file` is the `no cover / art` placeholder, the
  Automatic group is shown; on a file with cover art there is no Automatic group.
- 10.6 `[agent]` A provider with no poster renders its caption and `no poster`, and no
  `[role=radio]` for it.
- 10.7 `[agent]` Keyboard: Tab lands on the checked tile only; ArrowRight from the last Holodex
  tile stages the first tile of the next group; ArrowRight from the last tile wraps to `·file`;
  a no-poster provider is never focused; Escape closes the dialog.
- 10.8 `[agent]` Geometry at 768px dialog and 375px viewport: groups never split across lines
  (each group's caption and tiles share one line box); no tile caption overlaps a neighbour; at 375px
  `Holodex` starts its own line; nothing scrolls horizontally.
- 10.9 `[agent]` Cinémathèque via `javascript_tool`: checked tile border vs `bg-surface` ≥ 3:1; tile
  caption `text-accent` ≥ 4.5:1; group caption, `no poster` and placeholder `text-muted` ≥ 4.5:1.
- 10.10 `[agent]` Media page Maintenance: the button reads `Pick a frame`, and `Saving frame…` while
  the picked frame posts.

#### Human
- 10.11 `[human]` In Cinémathèque, the group captions should read as labels for the tiles beneath
  them, not as a heading competing with the row's own `Poster` header; the checked tile should be
  unmistakable at a glance.
- 10.12 `[human]` On a phone, the wrapped groups should still read as one chooser, with no group
  looking orphaned on its own line.

---

## Not in scope (filed or noted)

- Several posters per provider, and a `+N more` tile — HOLODEX-600. The group lists above are the
  hook.
- A history of picked frames or uploads — the newest of each kind only (F85 non-goal).
- A poster chooser on the media page — the dialog is the only chooser (F85 non-goal).
- A content-hash sync witness — the ledger stays the witness.
