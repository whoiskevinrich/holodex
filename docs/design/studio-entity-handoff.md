# Design handoff: Studio entity pages (F38)

**Spec**: [studio-entity.md](../specs/studio-entity.md) (F38, HOLODEX-11) ·
**ADR**: [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md) ·
**Impl design**: [studio-entity-implementation.md](../plans/studio-entity-implementation.md)

This is an **addendum** to the [F36 source-of-truth handoff](field-source-of-truth-handoff.md). The
editable-field chip mechanics — `SourceSelect` radiogroup (● per replace value), `CurationFieldRow`
merge chips (✕ per value), provenance fold/dedup, roving-tabindex a11y — are **inherited unchanged**
from F36/F37; this document specifies only what is new for studio: two new pages, the baseline label,
the media-detail link, the facet switch, and the search group. Everything is **tokens-only** (no
literal palette/radius/font — see [theming.md](theming.md)) and must be QA'd in all three skins.

New surfaces, in build order:
1. `/studios` — entity list (mirrors `/people`)
2. `/studios/{id}` — entity detail (name + video grid + Details chips)
3. Media-detail — studio value becomes a link to its entity
4. Browse facet — studio block switches from raw-string values to entity counts
5. Global search — a studio result group

---

## Overview

Studio becomes navigable identity. A visitor can browse studios, open one, and see its films; an
owner can curate a studio's enriched fields with the same chips they use on videos and people. The
studio page is deliberately **sparse before enrichment** — until the TMDB company slice (S3) lands
(or the owner sets a decision), a studio has only a `name`, so the Details section is hidden and the
page is just a header and a video grid. This is correct, not a gap: the page grows chips as there is
something to curate.

---

## 1. `/studios` — list

**Reuse `/people/+page.svelte` structure verbatim**, minus the merge-selection mode and minus
avatars (studios have no headshot in v1). Concretely: page `<section class="space-y-4">`, a header
row with `<h1 class="skin-title text-2xl font-semibold text-ink">Studios</h1>` and the
`SortToggle`/`SortReroll` controls, then the same loading/error/empty/grid state machine.

### Layout
- Grid: `grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3` (identical to People).
- Row: an `<a href={`/studios/${s.id}`}>` card —
  `flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent`.
  A **leading logo well** (§1b, HOLODEX-126) opens the row; then name, then count.
  - Well: `<span class="flex h-[26px] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate">` (see §1b)
  - Name: `<span class="flex-1 truncate">{s.name}</span>`
  - Count: `<span class="text-xs text-muted">{s.video_count}</span>`
- A–Z jump-nav (`sort === 'name'`): keep it — same `computeLetterAnchors` over studio names, same
  sticky letter bar. It is layout-mode-agnostic and studios benefit identically.

### Sort
- Reuse `PEOPLE_TAG_SORTS` (`name` | `random`), `readSort('studios', …, 'name')`,
  `writeSort('studios', …)`, and the SP2 client `seededShuffle`. A new `studios` sort-preference key;
  no new sort semantics.

### States
| State | Render |
|---|---|
| Loading | `<p class="py-16 text-center text-sm text-muted">Loading…</p>` |
| Error | `<p class="py-16 text-center text-sm text-warn">Couldn’t load studios: {msg}</p>` |
| Empty | `<p class="py-16 text-center text-sm text-muted">No studios indexed yet.</p>` |
| Populated | the grid above |

The empty state is reachable on a fresh library or before the backfill (ADR-053 §5) has run.

---

## 1b. Leading logo well (HOLODEX-126, F38 follow-up)

**Decided via `/design-critique` (2026-07-03).** Once studios can carry a `logo` enrichment field (S3,
[HOLODEX-121](https://whoiskevinrich.atlassian.net/browse/HOLODEX-121)), surface it in the list for
scannability — but as a **fixed leading well with a monogram fallback**, not a logo-card grid.

- **The well is a constant ~40×26 plate** at the start of every row, on the `bg-logo-plate` token.
  Its consistency is the **load-bearing constraint**: rows stay aligned whether or not the studio has
  a logo, which is the common case in a fresh library. (Rejected: a logo-tile grid — mostly-monogram
  tiles add scrolling and weight for little gain until logos are common.)
- **Enriched studio → its real logo.** `<img src={s.logo_url} alt="{s.name} logo" class="h-full w-full
  object-contain p-0.5" />` — `object-contain` so any logo aspect ratio fits the well without cropping.
- **Every other studio → a monogram**: the name's real first glyph, upper-cased (so "24 Frames" shows
  `2`, "東宝" shows `東` — not the A–Z jump bar's catch-all `#`), in `font-display`, `text-logo-plate-ink`.
  The monogram is **decorative** (`aria-hidden`) — the studio name is adjacent, so it adds nothing for
  a screen reader. The real logo `<img>`, by contrast, keeps a meaningful `alt`.
- **New token**: `--logo-plate-ink` (a dark neutral tuned per skin) for the monogram glyph — the plate
  is a light neutral in all three skins, so its ink must be dark to read. Lives in `app.css` alongside
  `--logo-plate`, mapped through `@theme inline` to the `text-logo-plate-ink` utility.

**Known edge (documented, acceptable):** `logo_url` is the *stored* provider logo, not the *resolved*
one — an owner who blank-pins the logo would still see it in the list thumbnail. The detail page stays
authoritative; the list is a scannability aid. A later, heavier option is a per-studio resolve.

| Well state | Render | a11y |
|---|---|---|
| `logo_url` present | `<img>` real logo, `object-contain` | `alt="{name} logo"` |
| no logo (common) | monogram (first glyph, upper-cased) | `aria-hidden` (name adjacent) |
| empty name (shouldn't occur) | `?` monogram | `aria-hidden` |

---

## 2. `/studios/{id}` — detail

**Reuse `/people/[id]/+page.svelte` as the template**, dropping the headshot/gallery,
aliases-management, rename, and merge sections (all out of scope, RD4). What remains:

### 2a. Header
- Studio name: `<h1 class="skin-title text-2xl font-semibold text-ink">{studio.name}</h1>`.
- No sub-line, no logo in v1 (logo arrives with S3 enrichment as a P1 nicety; spec it then).
- The name is **read-only** — no chips, no rename affordance (RD5: name materializes nothing and
  studios have no rename in v1). Do **not** render `name` as a `SourceSelect` row.

### 2b. Video grid
- The existing media grid component + paging used on the person page, fed by `studio.videos`.
- Same `.video-grid` hook class so skin flourishes and `data-layout` apply unchanged.

### 2c. Details section (the chips)
- **Hidden entirely** when the studio has no resolvable field beyond `name` — i.e. no enrichment and
  no decisions. Guard: render the section only if `resolved` (excluding `name`) is non-empty.
- When present, render each replace field as the inherited **`SourceSelect`** radiogroup with the
  **baseline label `·record`** (not `·file`) — pass the entity-generic `baselineKey='record'` exactly
  as the person page does. The empty-record chip is `— ·record` and is selectable (a standing
  "keep blank" decision, RD3/RD5).
- If/when a merge field exists for studios (none in v1 unless enrichment adds one), it uses
  `CurationFieldRow` — same as F30/F37.
- **Owner-gated**: only render the interactive chips when `activity.effectiveOwner`. Visitors see the
  resolved values as static text (the same read-only rendering the person page uses for visitors).
- **No writeback affordances**: no "Write decisions to file" button, no out-of-sync pill anywhere —
  a studio has no file (RD5, mirrors F37 Non-Goals).

### Baseline label — the one visual delta from F36
| Entity | Baseline chip label | Payload source token |
|---|---|---|
| video | `·file` | `file` / `file:<key>` |
| person | `·record` | `record` |
| **studio** | **`·record`** | **`record`** |

This is already parameterized (`baselineKey`); studio passes `'record'`. No new component work.

---

## 3. Media detail — studio value → entity link

- Wherever the media detail renders the resolved `studio` field value, it becomes a **link to the
  studio entity**: `<a href={`/studios/${sid}`}>` styled like the existing actor/director person
  links (same chip/link token treatment — do not invent a new style).
- **Link target source**: the media payload carries a `studios: [{ id, name }]` array derived from
  `video_studios` (ADR-053), so the link id always matches the *displayed* resolved value — that
  identity is the whole point of RD1. Match each rendered studio value to its `studios[]` entry by
  name; if (transiently) unmatched, render the value as plain text (no dead link).
- Multi-valued studio (operator-mapped multi): each value links to its own entity.

---

## 4. Browse facet — string values → entity counts

- The studio **facet block** switches from listing raw `video_metadata` string values to listing
  **studio entities with counts** (from `GET /studios`). Visual treatment of the block is unchanged
  (same facet list styling); only the data source and the click behavior change.
- Facet entry click → filtered browse via **`?studio_id={id}`** (was `?studio=Acme`).
- Each entry also links to the studio **page** (secondary affordance) — match how other entity facets
  expose "filter here" vs "go to page" if such a pattern exists; if not, the primary click filters and
  the entry label is fine as filter-only for v1.
- The legacy `?studio=` string filter still works at the API level (back-compat) but the **facet UI
  no longer generates it**. Do not surface both.

---

## 5. Global search — studio result group

- Add a **Studios** group to the global mixed-entity search results, mirroring the People/Tags
  groups exactly (same group header + result-row styling). Backed by `studios_fts` (ADR-017).
- Result row → `/studios/{id}`.

---

## Design tokens used

All inherited except `logo-plate-ink` (new — HOLODEX-126). For reference (from [theming.md](theming.md)):

| Token | Usage here |
|---|---|
| `bg-logo-plate` / `text-logo-plate-ink` | leading logo well plate / monogram glyph (§1b) |
| `text-ink` / `text-muted` | names / counts + secondary text |
| `text-warn` / `border-warn` | error state; (no attention pills on studio) |
| `bg-surface` / `bg-surface-2` | cards / hover |
| `border-rule` / `border-accent` | card border / hover + focus |
| `bg-accent` / `text-accent-ink` / `text-accent` | (only if an owner action button appears) / active |
| `rounded-theme` | all card/chip radii |
| `skin-title` / `font-display` | the page `<h1>` |
| `.video-grid` (+ `data-layout`) | the detail video grid hook |

**Token guard** must stay clean: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` empty (`rounded-full` pills OK).

---

## Responsive behavior

Identical to People (the grid is the only responsive element):

| Breakpoint | Grid |
|---|---|
| `<640px` (mobile) | 1 column |
| `640–1024px` (sm/tablet) | 2 columns |
| `≥1024px` (lg/desktop) | 3 columns |

The detail video grid inherits the media grid's own responsive rules. The A–Z jump bar wraps
(`flex-wrap`) and is sticky (`top-0`) as on People.

---

## Edge cases

- **Empty library / pre-backfill** → list empty state (§1).
- **Studio with 1 video** → count reads `1`; opening it shows a single-item grid.
- **Long studio name** (e.g. "Metro-Goldwyn-Mayer Pictures / United Artists") → `truncate` on the
  list row; full name wraps in the detail `<h1>` (no truncation on the header).
- **International text** → FTS uses `remove_diacritics 2`; names render as stored, no transliteration.
- **Studio pruned mid-session** (owner fixes the last video crediting it) → it 404s on refetch; the
  detail page shows the standard not-found state (reuse the person/tag 404 handling).
- **Pre-enrichment studio** → Details section hidden; page is header + grid only. Not an error state.
- **Slow connection** → the list/detail loading text (`Loading…`) covers it; no skeletons needed
  (parity with People).

---

## Accessibility

- **List**: the row is a single `<a>`; name + count are its content. Focus ring via `border-accent`
  (already token-driven). A–Z buttons carry `aria-label` (`Jump to {L}`), as on People.
- **Detail chips**: inherit the F36/F37 a11y contract wholesale — `role="radiogroup"` +
  `role="radio"`/`aria-checked`, **roving tabindex** (Tab reaches the group, arrows move within),
  selection conveyed by the ● dot + aria (never color alone). No new a11y work; do not regress it.
- **Search group**: the Studios group header is a heading in the results landmark, consistent with
  People/Tags groups.
- **Reduced motion**: A–Z jump respects `prefers-reduced-motion` (reuse the People `jumpTo`).

---

## QA checklist (3-skin)

Conventions ([[feedback-qa-checklist-numbering]]): every item numbered `section.item`, tagged by
verifier — `[smoke]` automated, `[agent]` agent-driven live QA, `[human]` needs human eyes. Skins:
**Cinémathèque · Broadcast · Brutalist**, switched via the header picker.

### §1 Setup
- **1.1** `[agent]` Start `backend-films` + `provider-tmdb` ([[reference-holodex-preview-testbeds]]).
  Have ≥2 videos resolving to the same studio (a shared TMDB production company) and ≥1 whose
  `studio` decision adopts a TMDB value differing from its file tag.
- **1.2** `[agent]` Ensure ≥1 studio with **no** enrichment (name-only) so the hidden-Details case is
  exercisable.

### §2 Smoke
- **2.1** `[smoke]` List/detail endpoints return name-sorted studios with active, non-deleted counts;
  a pruned (zero-link) studio never appears.
- **2.2** `[smoke]` `studioBaseline`: `name` resolves from the record; every other field empty
  baseline; flows through `ResolveFields` with the **resolver core unmodified** (build fails if the
  video path changed).
- **2.3** `[smoke]` `PUT /studios/{id}/fields/name/decision` → 400; unknown field → 404; unmatched
  provider → 400; visitor → 401/403 on all mutating endpoints.

### §3 Agent live QA (preview tools against §1 stack)
- **3.1** `[agent]` `/studios` renders the grid; counts match; A–Z jump works; sort toggle + Random
  reroll behave as on People. **All 3 skins.**
- **3.1b** `[agent]` Leading logo well (§1b, HOLODEX-126): an enriched studio shows its real logo
  (`object-contain`, `alt="{name} logo"`); a name-only studio shows a monogram (first glyph,
  `aria-hidden`); logo rows and monogram rows have **identical height** (the alignment constraint).
  Confirm the well plate (`bg-logo-plate`) and monogram ink (`text-logo-plate-ink`) read cleanly in
  **all 3 skins** (plate is light in every skin; ink must stay dark/legible).
- **3.2** `[agent]` Open a studio → name header + video grid; a name-only studio shows **no** Details
  section (not an empty box). **All 3 skins.**
- **3.3** `[agent]` On an enriched studio (post-S3, or with a decision set), Details renders
  `SourceSelect` rows with the **`·record`** baseline chip; `— ·record` selectable as blank-pin.
  **All 3 skins** (confirm the Brutalist filled-accent selected chip and any `text-warn` read
  cleanly, per the F36 regression history).
- **3.4** `[agent]` Media detail: the studio value is a **link**; clicking lands on the matching
  studio page; the linked studio equals the *displayed* resolved value even after adopting a TMDB
  decision (RD1 — no rescan).
- **3.5** `[agent]` Facet: the studio block lists entities with counts; clicking filters via
  `?studio_id=`; grouping matches displayed values (the adopted-decision video groups under the
  adopted studio, not its file value).
- **3.6** `[agent]` Global search for a studio name returns a Studios group linking to the page.
- **3.7** `[agent]` Absences: no "Write decisions to file" button, no out-of-sync pill, zero
  `/writeback` calls anywhere on studio surfaces.

### §4 Human
- **4.1** `[human]` Visit `/studios` in each skin. It should feel like the People page's sibling —
  same rhythm of cards, same fonts and borders reacting to the skin. Nothing should look like a
  different app or use a stray color.
- **4.2** `[human]` Open a studio that has films from a real company. The name reads as a title; the
  film grid below looks like the rest of the library. If the studio has curated fields, the chips
  look and behave exactly like the ones on a video's or person's page.
- **4.3** `[human]` On a video, click its studio. You land on that studio's page and see that video
  among its films. Going back and forth feels like clicking a person or a tag.
