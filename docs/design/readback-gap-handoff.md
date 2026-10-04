# Design Handoff: Read-back gap — ledger-witnessed rows, the dialog hint, and Mapping checks

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Sync state ·
**ADRs**: [ADR-119](../architecture/archive/ADR-119-ledger-witness-for-readback-gaps.md) (this change) ·
[ADR-101](../architecture/archive/ADR-101-ledger-witnessed-image-sync.md) ·
[ADR-093](../architecture/archive/ADR-093-writeback-readback-and-tristate-in-sync.md)
**Builds on**: [writeback-cockpit-handoff.md](writeback-cockpit-handoff.md) (HOLODEX-400). Every
rule there applies, with one narrowed (§2).
**Theming contract**: [ADR-021](../architecture/archive/ADR-021-frontend-theming-and-skins.md) as amended by
[ADR-115](../architecture/archive/ADR-115-cinematheque-only-skin.md). Tokens only; QA Cinémathèque.
**Surface**: `web/src/lib/components/writeback/WritebackFormDialog.svelte` (hint line) ·
`web/src/routes/owner/status/+page.svelte` (Mapping checks + reload toast) · `web/src/lib/api.ts` ·
backend `internal/resolver`, `internal/api`, `internal/writeback` (ADR-119 D1, D2, D4).
**Issue**: [HOLODEX-489](https://whoiskevinrich.atlassian.net/browse/HOLODEX-489).

![Three panels. 1: the writeback dialog after title and release date were written once; both rows show the equals gutter and the header reads 0 out of sync. 2: a decided release-date row Holodex has never written; the file chip reads not read back, and one muted hint line under the chips names Year as the key to add. 3: System Activity, where the reload toast reports 2 fields that can't be read back and a Mapping checks block lists release_date → Year and title → Title with the key to add](readback-gap-mockup.svg)

## Overview

A field writeback can write but the mapping can't read back (`title` → `Title` and `release_date` →
`Year`, mapped only to provider and filename sources) was `in_sync`-unknown for ever. So the dialog led
it and rewrote it on every open. ADR-119 moves its sync to the write ledger, the same witness posters
already use. Most of the visible fix therefore needs **no frontend change**: `rowClass` already reads a
standing `in_sync: true` as `matches` (panel 1). The frontend work is the two places the gap becomes
visible: the hint on a row that has never been written (panel 2) and the owner's Mapping checks
(panel 3).

Owner decisions (2026-09-28): ledger witness (not an implicit read-back source), hint **A** (a muted
line under the chips, not folded into the chip), diagnostics **A** (System Activity, not Owner ›
Fields).

## Decided visual spec

### 1. Written gap rows read `=` (no UI change)

The backend returns `in_sync: true` once the newest ledger row holds the decided value. The row falls
into the `=` tier like any matching row, and the header's out-of-sync count drops it. The file chip
still reads `not read back`: the ledger witnesses Holodex's write, not a fresh read of the file, and
the chip describes the file. Check this live (§6); don't rebuild it.

### 2. The hint line: decided, unknown, never written

- **When:** `isUnverifiable(field)` (a writable cockpit row with `in_sync === undefined`) **and** the
  field carries a standing decision **and** the detail payload names it as a gap (§4). This is exactly
  a decided gap row with no ledger row. An undecided row gets no hint: the resolver reports undecided
  rows as in sync by construction, so they never reach this state.
- **Where:** one `<p>` directly under the chooser (chip row or stacked list), inside the row.
- **Copy:** `Holodex can't read {write_tag} back yet. After this write it'll track what it wrote; to
  check the file itself, add {key} to {canonical}'s sources.` `{key}` is the first `add_one_of`
  entry, in a `<code>` span. With more than one candidate key, the text reads `add one of Year, Date`.
- **Style:** `text-xs text-muted`. Never `text-warn`, never an icon, never a border. This **narrows**
  HOLODEX-400's "no warning line on a not-read-back row". The line is setup guidance, and it disappears
  once the field has been written (panel 1).
- It carries no link: the owner's fix is in `metadata-mappings.yaml`, and Mapping checks (§3) is where
  they re-check it.

### 3. System Activity: Mapping checks

- **Where:** in the Actions area of `/owner/status`, under the button row that holds *Reload config*.
  Owner + Admin mode only, the same gate as the buttons.
- **When:** rendered only when the gap list is non-empty. No empty state, because a clean mapping has
  nothing to report.
- **Block:** heading `Mapping checks` (`text-sm text-ink`), then the lead line `Written to the file but
  not read back, so their sync is tracked from Holodex's own writes:` (`text-xs text-muted`), then one
  line per gap: `{canonical} → {write_tag} · add {key}` (canonical `text-sm text-ink`, the rest
  `text-xs text-muted`, key in `<code>`), then the tail `in metadata-mappings.yaml, then reload
  config.`. The tail links `Why this matters` to `docs/reference/canonical-fields.md#writeback-round-trip`,
  the same anchor the startup WARN prints.
- **Reload toast:** after a gap count greater than 0, `Config reloaded — {n} fields · {g} fields can't
  be read back` (use `field` when g is 1). The existing `Config reloaded — {n} fields.` is unchanged
  when g is 0. After a reload the block re-fetches, so fixing the mapping and reloading clears it
  without a page reload.

### 4. Data

- `GET /api/v1/owner/readback-gaps` → `[{canonical, write_tag, add_one_of: string[]}]`, owner-gated,
  sorted by canonical. It is `writeback.ReadbackGaps` over the live mapping, with `write_tag` added.
- `POST /api/v1/admin/reload-config` response gains `readback_gaps: number`.
- The media detail payload needs to tell the dialog which rows are gaps. Add
  `readback_gap: { write_tag, add_one_of }` on each resolved field in the gap set (owner response
  only). The dialog must not fetch the owner endpoint per open.

### 5. Accessibility

- The hint is plain text in the row, so it is read in document order after the chooser. No
  `aria-describedby` is needed, and there is no focus stop.
- Mapping checks is a `<section aria-labelledby>` with an `<h3>` (the page's existing heading levels),
  and the gap lines are a `<ul>`.
- Toast text is unchanged in mechanism (the existing live region).

### 6. Verification checklist

#### Setup
1. `[smoke]` Worktree backend + web dev server on a library with an MKV and an MP4. The mapping gives
   `title` and `release_date` provider/filename sources only.

#### Agent
2. `[agent]` Decide `release_date` from tmdb and open the dialog: the row leads with `↧`, the file chip
   reads `not read back`, and the hint names `Year`.
3. `[agent]` Write, then reopen: both rows read `=`, the header count excludes them, and there is no hint.
4. `[agent]` Re-point the decision to a different value: the row reads `↧`, `in_sync: false`, and the
   page pill counts it.
5. `[agent]` `/owner/status` in Admin mode: Mapping checks lists `release_date → Year` and `title →
   Title`. Add `Year` to the mapping, then Reload: the toast reports 1 gap, the block drops
   release_date, and the release-date row now compares against the file tag.
6. `[agent]` Visitor view of `/owner/status`: no Mapping checks. `GET /owner/readback-gaps` without
   the owner token returns 401.
7. `[agent]` Computed style: the hint and the lead line use `--muted`, not `--warn`, on Cinémathèque.
8. `[agent]` 375 px: the hint wraps inside the row, and `scrollWidth == innerWidth` on `/owner/status`.

#### Human
9. `[human]` Look at the hint and the Mapping checks block on Cinémathèque. Confirm they read as
   guidance, not alarm.

## Not in scope

- Reading `Title`/`Year` back automatically (ADR-119 D5, rejected).
- Mapping diagnostics other than read-back gaps (claims, inactive keys). The block is named *Mapping
  checks* so it can grow, but only gaps ship here.
- The spec's stale F36 line about "pre-checked" rows (§Write-back, pre-cockpit wording). It is noted,
  not rewritten here.
