# Design Handoff: Writeback dialog — the Poster row becomes an image-tile chooser

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Sync state ·
**ADRs**: [ADR-101](../architecture/ADR-101-ledger-witnessed-image-sync.md) (the sync witness
and the tile chooser) · [ADR-093](../architecture/ADR-093-writeback-readback-and-tristate-in-sync.md) ·
[ADR-049](../architecture/ADR-049-manual-image-precedence.md) · [ADR-039](../architecture/ADR-039-provider-asset-urls.md)
**Builds on**: [writeback-cockpit-handoff.md](writeback-cockpit-handoff.md) (HOLODEX-400 — every rule
there applies; this doc only adds the third chooser shape and the image rows' sync) ·
[writeback-poster-and-decision-legibility-handoff.md](writeback-poster-and-decision-legibility-handoff.md)
(HOLODEX-245 — whose read-only comparison this replaces).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Surface**: `web/src/lib/components/writeback/WritebackFormDialog.svelte` (image rows join the
cockpit) · new `web/src/lib/components/curation/SourceImageTiles.svelte` · `web/src/lib/writebackCockpit.ts`
(`isCockpitRow` admits `image_url`) · backend `internal/resolver` + `internal/repo` + `internal/api`
(ADR-101 D1/D2).
**Issue**: [HOLODEX-403](https://whoiskevinrich.atlassian.net/browse/HOLODEX-403) (parent HOLODEX-167).

![Four states of the Poster row: undecided with a dashed pending tmdb tile; picked, with the accent tile and the will-write gutter; after one Write, collapsed to "matches the file" behind change; and with an owner upload on the page, where the file tile is a placeholder and a note explains the upload keeps the page](writeback-poster-chooser-mockup.svg)

---

## Overview

HOLODEX-400 removed the last checkbox from the dialog: deciding is the check action, and a field
with nothing to decide is read-only. That left the poster with **no writeback path at all** — it
had nothing to decide only because it had no chooser. This handoff gives it one, as the third
chooser shape (tiles beside chips and stacked rows), and closes the trap that would otherwise
follow: with nothing reading cover art back, a decided poster would lead the dialog on every
open and be re-embedded on every Write. ADR-101 makes the write ledger the witness instead.

Everything else is the cockpit's existing model — applied vs. on file, golden record with two
destinations, gutter = what Write will do. Read that handoff first.

---

## Decided visual spec

### 1. The image row is a cockpit row

`isCockpitRow` admits `display === 'image_url'`. The row therefore gets: the staged pick,
`willWrite` / `savesDecisionOnly`, the `↧` / cylinder / `○` / `=` gutter, the header destination
line (`→ cover.jpg` / `→ QuickTime:CoverArt` per container; `no file tag for this container`
when unmapped), and the `change` toggle on `=`. Nothing image-specific in the row shell.

### 2. The chooser: image tiles

| Element | Treatment |
|---|---|
| Container | `role="radiogroup"`, `aria-label="Source of truth for {label}"`, `mt-1 flex flex-wrap gap-3`; roving tabindex, arrow keys move **and** stage, Space/Enter stage — the `SourceChipRow` handler verbatim (`data-seg` on each tile) |
| Tile | `<button role="radio" data-seg={key}>`: a `h-20 w-14` (56×80, the HOLODEX-245 box) `rounded-theme border object-cover` image, caption below in `text-[0.65rem]` with the radio dot + `·{labels}` |
| Staged | `border-accent border-2` on the image, filled dot, caption `text-accent` |
| Pending (RD6, staged) | as staged but `border-dashed`, caption `·tmdb, pending` (the chip row's vocabulary) |
| Unstaged | `border-rule`, hollow dot, caption `text-muted hover:text-ink` |
| File tile image | the served poster (`/api/v1/media/{id}/poster?v=…`) when `!video.poster_uploaded` — that *is* the extracted cover art or frame grab; otherwise the **placeholder** (§4) |
| Provider tile image | the candidate URL (already allowlist-gated by the resolver's `gateImageDisplay`; a degraded candidate never reaches the tile) |
| Empty file candidate | the file tile still renders — the file's cover art is what it shows, whatever the mapping's `file:` source says. Its *value* for the pick stays the candidate's (`''`), so `·file` is the blank pin, as everywhere |
| No Custom tile | a pasted URL would have to pass the provider asset-host allowlist (ADR-039), and the owner upload has its own control on the page. `sourceChips()`'s trailing Custom chip is dropped for image fields |
| Busy | tiles `pointer-events-none`, guarded in the handler (chip-row idiom) — never `opacity` on the caption |

### 3. What Write does

| Pick | Gutter | Write |
|---|---|---|
| provider tile (new or changed) | `↧` | `PUT …/fields/poster_url/decision {source: provider:x}`, then the writeback with `values: [url]` — the existing `IsImage` download + embed path, unchanged |
| provider tile already standing, ledger matches | `=` | nothing (row collapsed, `change` available) |
| provider tile already standing, no ledger row | `↧` (unknown sync) | written — once; the ledger then witnesses it |
| `·file` tile on a decided row | cylinder | decision only (`source: file`) — the file keeps whatever it has |
| untouched, undecided | `○` | nothing |

### 4. The owner-upload case (ADR-049)

When `video.poster_uploaded` is true the served poster is the upload, not the file's cover art,
and the upload overwrote the extracted tier on disk — there is no image of the file's own cover
to show. The file tile renders a **placeholder** (`border-dashed`, `text-muted`, two lines: `cover
art` / `in file`) and the row carries one muted note under the tiles:

> Your uploaded poster stays on the page; this row is the cover art inside the file.

The upload is never a tile: it is a local file, and the write path takes an `https://` URL.

### 5. Copy

| Where | Text |
|---|---|
| Tile captions | `·file` · `·tmdb` · `·tmdb, pending` |
| File placeholder | `cover art` / `in file` |
| Upload note | `Your uploaded poster stays on the page; this row is the cover art inside the file.` |
| Tile `aria-label` | `{label} from {source}{, pending}` — e.g. `Poster from tmdb, pending` |
| Everything else | the cockpit's |

### 6. Accessibility

- The tiles are the same roving radiogroup as the chip row: one `tabindex="0"`, the rest `-1`,
  so the dialog's `focusables()` exclusion holds and Tab never lands on an unstaged tile.
- Images are `alt=""` inside the button — the caption + `aria-label` carry the meaning; the
  placeholder's two words are real text.
- Selection is never colour alone: border weight/dash + dot + `aria-checked`.

### 7. Edge cases

- **Provider candidate degraded** (off-allowlist URL): the resolver already turns the field into
  text (`gateImageDisplay`); the row then renders as a chip row, not tiles. No tile-side handling.
- **Two providers**: two provider tiles; tiles wrap at the dialog's width.
- **Container without a cover target** (unmapped): header `no file tag for this container`;
  picking a tile is a decision-only save (cylinder), exactly like an unmapped text field.
- **Write fails** (download refused, remux error): reported on the page as today (ADR-091); the
  decision stands, the ledger gains no row, the row reads `↧` again next open — the honest state.
- **Poster written before ADR-101 shipped**: the ADR-041 audit row exists → witnessed → `=`.

### 8. Verification checklist

Numbered `section.item`; tagged by verifier; grouped by tag.

#### Setup
- 8.0 `[smoke]` `make test` + `cd web && npm run check && npm run test` green.

#### Agent
- 8.1 `[agent]` Backend: `TestReplaceMarkers_ImageSyncFromLedger` — decided `poster_url` with a
  matching `LastWritten` entry → `in_sync: true`; different value → `false`; no entry → `nil`;
  a text field with a `LastWritten` entry is unaffected. `TestLastWrittenValues` returns the
  newest row per field.
- 8.2 `[agent]` Open the dialog on an enriched video with no poster decision: Poster row in the
  undecided disclosure, `○`, file tile + dashed `·tmdb, pending` tile, no Custom, no checkbox.
- 8.3 `[agent]` Click the tmdb tile: `○` → `↧`, footer +1. Write → `PUT …/poster_url/decision`
  then a writeback whose payload carries `poster_url: [url]`; the page's poster refreshes.
- 8.4 `[agent]` Re-open after the job lands: Poster reads `=` ("matches the file"), collapsed
  behind `change`; API shows `in_sync: true` for `poster_url`.
- 8.5 `[agent]` `change` → pick `·file` → cylinder, `Save 1 decision`; Write → one decision PUT
  (`source: file`), no writeback POST.
- 8.6 `[agent]` Upload a poster on the page, re-open: file tile is the placeholder, the note is
  present, the served poster is the upload.
- 8.7 `[agent]` Keyboard: Tab lands on the staged tile only; ArrowRight stages the next tile;
  Escape closes the dialog.
- 8.8 `[agent]` Three skins via `javascript_tool`: staged tile border vs. `bg-surface` ≥ 3:1;
  caption `text-accent` ≥ 4.5:1; placeholder text ≥ 4.5:1.

#### Human
- 8.9 `[human]` In each skin, the picked tile should be unmistakable at a glance (border + dot),
  and the dashed pending tile should read as "suggested, not chosen". The placeholder should
  not look like a broken image.

---

## Not in scope (filed or noted)

- Writing an owner-uploaded poster into the file — needs a local-file write path (ADR-101 D4).
- Frame grab as a candidate — same reason.
- A content-hash sync witness — ADR-101 leaves the door open.
- The page-side `poster_url` `SourceBadge` (`METADATA_ELSEWHERE`) — unchanged; the dialog is
  the poster's chooser for now.
