# Design handoff — Entity identity card (F60)

**Status:** Ratified 2026-09-12 — OQ1 = deep link, OQ2 = keep 378
**Epic:** [HOLODEX-373](https://whoiskevinrich.atlassian.net/browse/HOLODEX-373) · stories with UI:
[374](https://whoiskevinrich.atlassian.net/browse/HOLODEX-374) reference chip ·
[377](https://whoiskevinrich.atlassian.net/browse/HOLODEX-377) edition ·
[378](https://whoiskevinrich.atlassian.net/browse/HOLODEX-378) Display as
**Owner:** Kevin Rich
**Date:** 2026-09-12
**Spec:** [entity-identity-card.md](../specs/entity-identity-card.md) (F60, RD1–RD12)
**ADR:** [ADR-096](../architecture/ADR-096-entity-identity-card.md) — D5 struck Tag from §4 (see 4d).
**Builds on:** [entity-identity-handoff.md](entity-identity-handoff.md) (F43 — AliasPanel,
EntityPicker) · [field-source-of-truth-handoff.md](field-source-of-truth-handoff.md) (ADR-051
chip row) · [film-enrichment-handoff.md](film-enrichment-handoff.md) (F59 film header)
**Theming contract:** [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — tokens only, QA all three skins.

![Entity identity card mockup](entity-identity-card-mockup.svg)

## Overview

Every entity gets a uniform, *surfaced* identity so that a person, an agent, and a provider all
mean the same row. Three of the epic's five stories touch the UI; this document specifies those.
The other two ([375](https://whoiskevinrich.atlassian.net/browse/HOLODEX-375) external-id
unification, [376](https://whoiskevinrich.atlassian.net/browse/HOLODEX-376) films into the spine)
add **no new surface** — §5 records what they reuse so the decision is visible.

Two things changed between the brainstorm mockups and this grounded handoff, both because the
codebase already had the pattern:

1. **Edition edits ride `SourceBadge`, not a pencil.** HOLODEX-268's Tier-2 model is implemented
   as a badge-click (`curation/SourceBadge.svelte:227-241`), not a docked pencil. The pencil is
   `NameEditControl`'s rename affordance. Reusing the badge means the edition row is *literally* a
   canonical field row with zero new components — and it means the film page must not duplicate
   the curation mount (OQ1).
2. **"Display as" is the name field's `SourceBadge`, not a second button.** The brainstorm drew
   two buttons. Grounded: person pages already synthesise a `name` field with file + provider
   sources (`person_decisions.go:117` is where the decision is *rejected*). Lifting the rejection
   turns the name into an ordinary Tier-2 field whose badge opens the ADR-051 chip row — file
   spelling, each provider's spelling, custom. The two verbs then map onto two *existing*
   affordances: **pencil = rename in files**, **badge = display as**. This is a tighter split than
   two buttons and it also gives the owner the provider's spelling without typing it.

## 1. Reference chip — every entity page (HOLODEX-374)

### 1a. Placement

The chip is the **last item of the existing meta line** under the name. There is no shared header
component (each page is bespoke), so it mounts once per page:

| Page | Mount point | Line today |
|---|---|---|
| people | end of `EntityVideoMeta` row | `people/[id]/+page.svelte:544-548` |
| studios | hero meta line, after the video count | `studios/[id]/+page.svelte:302-329` (hero snippet) |
| tags | hero meta line | `tags/[id]/+page.svelte:379-407` |
| films | after the "Release date says …" line | `films/[id]/+page.svelte:437-439` |
| media | end of the `WxH · duration · year` meta line, after the resolution pill | `media/[id]/+page.svelte:1254-1262` |

Not in the `NameEditControl` `trailing` snippet — people already use that slot for
`NationalityFlags`, and a mono chip inline with a display-serif h1 fights it.

### 1b. Component: `RefChip.svelte` (new, `web/src/lib/components/entity/`)

| Prop | Type | Notes |
|---|---|---|
| `ref` | `string` | `"person:1234"` — comes from the API payload's `ref` field, never assembled client-side |

Markup: `<button type="button" class="btn-pill font-ui text-xs text-ink">` containing the ref in a
mono span (`font-mono` is not a token today — use `font-ui` and accept proportional digits, **or**
add `--font-mono` to `app.css` `@theme inline` as part of this story; recommended: add it, one
line, three skins inherit the system mono stack) plus a copy glyph (inline SVG, 12px,
`text-muted`).

Behaviour: click / Enter / Space → `navigator.clipboard.writeText(ref)`; label swaps to
**Copied** for 1500 ms then restores; `aria-live="polite"` region announces "Copied person:1234".
If the clipboard API rejects (insecure context, permissions), fall back to selecting the text
inside the chip and leaving it selected — the chip must never silently do nothing.

Visible to **visitors** — a ref is not a mutation and a visitor pasting `film:42` into a bug
report is the point.

### 1c. States

| State | Appearance |
|---|---|
| Rest | `btn-pill` outline, `border-rule`, text `text-ink`, glyph `text-muted` |
| Hover / focus-visible | `ring-1 ring-accent` (same ring `SourceBadge` uses) |
| Copied | label "Copied", glyph swaps to a check, 1500 ms |
| Clipboard denied | text selected in place; no toast, no error copy |

### 1d. API / MCP contract (frontend-relevant part)

Every entity payload carries `ref`. Routes accept `film:42` wherever they accept `42`; a
mismatched kind (`/people/film:42`) is **400**, not 404. The chip never parses or builds refs.

## 2. Edition field row — media page (HOLODEX-377)

### 2a. It is a canonical field, nothing more

`metadata-mappings.yaml` gains:

```yaml
- canonical: edition
  sources:
    - Edition                 # container tag — baseline, wins
    - filename:edition        # parsed {edition-X} (Plex grammar, strict in v1)
```

List order is precedence (`release_date` at `metadata-mappings.yaml.example:89-96` is the
precedent). No provider source in v1 — TMDB has no edition concept. Field kind: `replace`, single
value, manual allowed.

The media page then renders it through the generic field row
(`media/[id]/+page.svelte:1865-1890`): `<dt>Edition:</dt>` + `SourceBadge` for the owner,
`ProvenanceBadge` for visitors. Deep link `#field-edition` already works (`id="field-{canonical}"`).

### 2b. The one `SourceBadge` change

`SourceBadge` renders the clickable badge only when `isMultiSource`
(`SourceBadge.svelte:227`). An edition parsed from the filename alone is single-source, so the
owner would have **no affordance** to type a custom value. Change: the badge renders when the
field is *curatable* (manual allowed), regardless of candidate count; the chip row then shows the
one file candidate plus the custom input. This is a one-condition change in `SourceBadge`, and it
applies to every other single-source curatable field too — record it in `curation/CLAUDE.md`.

### 2c. Chip row contents

| Chip | Label | Present when |
|---|---|---|
| tag candidate | value · `file · tag` | tag present in file |
| filename candidate | value · `file · name` | `{edition-X}` parsed |
| custom | text input, placeholder `Director's Cut` | always (2b) |

No *new* empty chip is drawn for edition. **Corrected in build (2026-09-14):** the existing chip
row does render one empty chip — the anchored baseline `— · file` — and that is the F37 RD3
blank-pin, a deliberate choice that stays. So a filename-only edition reads `— · file` ·
`Theatrical · filename` · `Custom`. Confirm / Cancel exactly as `SourceBadge` today;
`use:dismissable` on outside click / Esc.

~~Helper line under the row (`text-xs text-muted`): *Typed values are curated. Use Write to file… to
store it in the file's tag.*~~ **Not built:** §2a says edition is a canonical field and nothing
more, and the generic field row has no per-field helper precedent — a helper keyed on one
canonical inside that loop is the special case the row exists to avoid. Reopen if the
out-of-sync pill (which the row already shows after a custom value) proves insufficient.

### 2d. Writeback

`WritebackFormDialog` (`writeback/WritebackFormDialog.svelte`) lists Edition like any other
field — no bespoke control. Writing it sets the container tag; on the next re-extract the tag
becomes the `file · tag` candidate and out-ranks the filename. Tag keys are fixed by spec RD8:
MKV/WebM `EDITION`, MP4/MOV `XMP-prism:Edition` — both read back as `Edition`.

## 3. Full-film list — film page (HOLODEX-377)

### 3a. Pill

Each row in `films/[id]/+page.svelte:641-670` gains a pill **directly beside the title** (built
2026-09-14 — the pre-build draft placed it next to the resolution pill; see §2–§3 as built): `rounded-full border border-rule bg-surface px-1.5 py-0.5 text-[10px] text-muted`, text =
the video's resolved `edition`. The video summary payload the film page already receives carries
`edition` (resolved value only — the film page never needs the candidates).

~~Truncate at 18 characters with the full value in `title`.~~ **Never truncated** (owner ruling
2026-09-14): the title group wraps, so a pill that doesn't fit beside the title drops beneath it,
and a value wider than the row wraps inside the pill. Visitors and owners see the same pill.

### 3b. Set edition — the missing-value affordance

When `edition` is empty **and** the viewer is the owner, the pill slot shows a dashed link:
`rounded-full border border-dashed border-muted px-1.5 py-0.5 text-[10px] text-accent`, text
**+ Set edition**, `href="/media/{id}#field-edition"` (built as spec RD11's bare hash — landing on
any `#field-<canonical>` expands that field, no query flag). Landing there opens the row with
the badge already expanded (`expandedField` store, `SourceBadge.svelte:34`), so the owner is one
Confirm from done. Visitors see an empty slot, not the link.

**OQ1 — deep link vs inline.** The brainstorm mockup opened the chip row *inline* on the film
page. Grounded recommendation: **deep link**. Inline would mean mounting `SourceBadge` (and its
single-slot `expandedField` store, `dismissable`, the curation `decide` plumbing) on a second page
for one field, and the film page would need each video's full `ResolvedField`, not just the value.
The cost is one navigation; the benefit is one curation mount. Owner to ratify.

### 3c. Stressed states

| Case | Rendering |
|---|---|
| Two full-film files, neither with an edition | Two dashed links. Nothing else changes — no banner, no "which is which?" prompt. The list is already the prompt. |
| One file, no edition | One dashed link. Not an error; most films have one file. |
| Edition set on some, not others | Mixed pills and links, same row shape. |
| Very long edition | Wraps beneath the title, then inside the pill — never truncated. |
| Edition present but identical on two files | Two identical pills — genuinely the owner's problem to fix via Set edition; do not dedupe or warn. |

## §2–§3 as built (HOLODEX-377, 2026-09-14)

Panels 2 and 3 of the figure above are the pre-build design; this figure is drawn from the shipped
components and supersedes them where they differ. "Director's Edition" on both pages, every state:

![Edition as built — media detail row and film detail pills](entity-identity-card-edition-as-built.svg)

| Panel | What it shows | Where it differs from the pre-build figure |
|---|---|---|
| A · header | The edition reads **directly beside the title** when present — a read-only pill after the title control, owner and visitor alike. **Never truncated:** on a narrow screen the pill drops beneath the title (the title keeps its line and its docked pencil), and a value wider than the viewport wraps inside the pill. | New (owner rulings 2026-09-14: "the edition, when present, should appear near the Title"; "wrap vs. truncate on narrow screens"). Built as a flex-wrap row on the media page around `NameEditControl` + pill, not in the control's `trailing` slot — that row is deliberately non-wrapping (HOLODEX-356) and a pill there squeezes the title. The Metadata row below stays the curation mount. |
| A · at rest | The one Edition row in its three provenances — container tag (`file`), filename marker (`filename`), typed value (`custom` + `file out of sync`). Owner and visitor see the same value and badge; only the owner's badge is clickable. | Badge labels are `ProvenanceBadge`'s real ones (`file` / `filename` / `custom`), not the drafted "file · name" / "file · tag". |
| A · landed | Arriving via `/media/{id}#field-edition`: row scrolled into view, badge expanded, chip row = `— · file` (the F37 blank-pin), the filename candidate selected, `+ Custom`, Confirm / Cancel. | The blank-pin chip **stays** (§2c corrected). A file with neither tag nor marker nor decision has no row at all, so the landing renders one from its completeness facet — `— · file` and `+ Custom` only — and Confirm makes it real. |
| A · write | The `Write decisions to file…` dialog line: `Edition · custom → Edition, was: —`. | No helper line under the row (owner ruling, §2c). |
| B · owner | Full film rows: the pill sits **directly beside the file's title**, resolution and `Write to file…` stay right-aligned; `+ Set edition` dashed link in that same slot on a file with none. **Never truncated:** the title group wraps, so a long pill drops beneath the title and a value wider than the row wraps inside the pill. Mixed states share one row shape. | Pill moved from the resolution side to the title side, and the 18ch truncation dropped (owner rulings 2026-09-14): the edition is a property of *this file*, so it travels with the file's name, whole. |
| B · visitor | Same pill, beside the title; no link, no `Write to file…`. | As above. |

Not visible in the figure but part of the same build: `edition` is an **optional** completeness
facet — listed so the landing row can be built, never scored, never queued (owner ruling, since on
the requesting library most media has no edition and that is not a gap).

## 4. Display as — entity headers (HOLODEX-378)

### 4a. Model

`name` becomes an ordinary resolved field on person / studio / tag / film: sources are the file
spelling (baseline), each provider's spelling, and custom. The **rendered** name is the resolved
value; the **canonical** `name` column is untouched by any decision and stays the file / writeback
/ identity / alias-routing truth.

### 4b. Header, at rest

The h1 (or `NameEditControl` heading) renders the resolved name. **Only when resolved ≠
canonical**, a line appears beneath it:

`In files as <canonical, mono> <SourceBadge>`

The badge is the name field's `SourceBadge` (its `ProvenanceBadge` shows `tmdb` / `custom`). When
resolved = canonical there is no line and no badge — the header looks exactly as it does today.
This keeps the visitor view and the at-rest owner view identical, per HOLODEX-268.

### 4c. Two verbs, two existing affordances

| Verb | Affordance | What it changes | Copy under it |
|---|---|---|---|
| **Display as** | the badge on the "In files as" line (or, when no decision stands yet, a quiet `btn-quiet text-xs` **Display as…** link in the same slot) | a per-field decision; DB only | *Changes how the name is shown. Never touches the file, aliases, search, or writeback.* |
| **Rename in files** | `NameEditControl`'s docked pencil, unchanged | canonical `name`; old spelling → alias; offers writeback | unchanged |

Search matches canonical, resolved, and aliases. Writeback payloads carry canonical, never the
display value — a test in the story enforces this.

**OQ2 — the go/no-go.** HOLODEX-378's kill criterion is "a first-time reader picks the right
verb." The reviewable artefact is panel 4 of the mockup: pencil next to the big name = rename;
badge on the small "In files as" line = display. If that doesn't read at a glance, cut 378 — the
other four stories don't depend on it.

### 4d. Tags and films

Tags: **excluded** — ADR-096 D5 upholds ADR-061's rule that tags carry the identity spine only, never
the field-resolution model, and tags are always lowercase by owner policy (0034). Films: the
film title has **no** `NameEditControl` today (`films/[id]/+page.svelte:391`, rename is a 376
deliverable) — 378 for films is gated on 376 landing the pencil first.

## §4 as built (HOLODEX-378, 2026-09-15)

Panel 4 of the figure above is the pre-build design; this figure is drawn from the shipped
components and supersedes it where they differ:

![Display name as built — person hero in four states, and a search row](entity-identity-card-display-name-as-built.svg)

| State | What it shows | Where it differs from panel 4 |
|---|---|---|
| A1 · owner, no decision | Heading + docked pencil exactly as before; beneath, a quiet link (`.btn-quiet text-xs`), owner only, that **names the spelling on offer**: `Display as Ana Keßler (tmdb)…` when a provider spelling differs from the record, bare `Display as…` otherwise. | §4c's "same slot" link, with the copy change from QA 4.3: the first pass reached for the provider's Refresh button, because at rest nothing said a second spelling existed. |
| A2 · link clicked | The name field's `SourceBadge` mounts **already expanded** in the link's slot: `record` / provider spelling / `Custom` chips, Confirm / Cancel, and the helper copy under it. Focus hands from the link to the badge; dismissing hands it back. | Helper copy rewritten: search **does** match the display spelling (spec §378, ADR-096 D5), so it reads "Changes how the name is shown here and in search. Files, aliases, and writeback keep the record spelling." |
| A3 · decision standing | h1 = resolved spelling; `In files as <canonical, mono> <badge>`. The badge renders **without repeating the value** (`SourceBadge showValue={false}`) — the heading already shows it. Pencil prefills canonical (`NameEditControl editValue`). **For the owner the record spelling on the line is itself the rename trigger** — dotted-underlined, `aria-label="Rename this person — in files as Ana Kessler"`, opens the same rename form (`NameEditControl.open()`), prefilled and selected. | Films read **On record as** — a title is owner-asserted, never read from a file. Studios read "In files as" like people (the record comes from file tags). The trigger is a QA 4.3 follow-up (owner, 2026-09-15): with a decision standing the docked pencil is hover-hidden beside a heading that no longer shows the record spelling, so there was no visible way to change what the files say. Owner chose the value-as-trigger over an always-visible pencil. |
| A4 · visitor | Heading + the "In files as" line; no badge, no link, no pencil — a content line, not a control (routes rule). | Panel 4 drew the owner only. |
| B · search row | The nav search row labels with `display_name ?? name`; canonical, display and alias spellings all match. | New: the row is the one list surface in scope. **Pickers keep the canonical name** — they send `name` back for linking. |

Scope ruling (2026-09-15, options rendered side by side): **headers + search only.** List cards,
cast tiles, link cards and pickers keep the canonical name; a `display_name` on cards is a
follow-up story if 378 survives QA 4.3. Confirming the `record` chip leaves a standing record
decision (the F37 blank-pin) — resolved = canonical, the line goes, the link returns. A decided
provider with no stored spelling drops the resolved row (resolver rule), so the page falls back to
canonical and shows no line; a re-enrich restores it.

## 5. Out of scope / no new surface

- **375 external-id unification** — the ADR-083 external-id badge reads the new table; no visual
  change.
- **376 films into the spine** — `AliasPanel` (`person/AliasPanel.svelte`, entity-generic since
  F43 S2) mounts under the film header exactly as on `studios/[id]/+page.svelte:363`; provider
  alternative titles land as `source='provider'` rows. `NameEditControl` on the film title
  reuses the studio wiring with `MergeOfferCard` as the verdict.
- Slugs, a Release/Version sub-entity, file renaming, un-lowercasing tags (policy: always lowercase) — set aside
  by the epic.

## 6. Accessibility

- `RefChip` is a real `<button>`; label "Copy reference person:1234"; `aria-live="polite"` on the
  Copied announcement; focus ring is the shared `ring-accent`.
- Edition and name chip rows inherit `SourceBadge`'s roving-tabindex radiogroup
  (`role="radiogroup"`, `aria-label="Source of truth for Edition"`), Esc/outside-click dismiss,
  and `aria-busy` during Confirm. The custom input is in the tab order after the last chip.
- **+ Set edition** is an `<a>` with a full href — no `onclick`-only link; screen readers get
  "Set edition, link".
- "In files as" line: the mono canonical value is plain text, not a button; only the badge is
  interactive.
- Nothing here relies on colour alone: the dashed border + the "+" carry the missing-state.

## 7. Theming

Tokens only. New class usage: `btn-pill` (exists), `border-dashed` (Tailwind utility, no token
needed), `font-mono` **only if** `--font-mono` is added to `app.css` `@theme inline` (1c). Contrast
targets to verify per skin (Cinémathèque / Broadcast / Brutalist): chip text on `bg-surface` ≥ 4.5:1;
`text-accent` on the dashed link ≥ 4.5:1; pill `text-muted` on `bg-surface` ≥ 3:1 (it's a label,
not body text — but check Broadcast, whose muted is the lightest).

## 8. QA checklist (3-skin)

### §1 Setup

- **1.1** [smoke] `backend-films` testbed, owner session (`X-Admin-Token`), one film with two
  full-film files: one named `… {edition-Final Cut}.mkv`, one with no edition marker.
- **1.2** [smoke] One person with a TMDB enrichment whose `name` differs in case from the file
  spelling.

### §2 Smoke — automated

- **2.1** [smoke] `GET /people/{id}` and `GET /people/person:{id}` return byte-identical bodies;
  `GET /people/film:{id}` → 400.
- **2.2** [smoke] Resolver: file with tag `Edition=Final Cut` and filename `{edition-Theatrical}`
  resolves `edition = Final Cut` with source `file · tag`.
- **2.3** [smoke] Writeback payload for a person with a standing display decision carries the
  canonical name, not the display value.
- **2.4** [smoke] `SourceBadge` renders the badge for a single-candidate curatable field (2b).

### §3 Agent live QA

- **3.1** [agent] Ref chip present on all five page kinds; click → clipboard contains the ref;
  label reads "Copied" then restores (use `javascript_tool` to read `navigator.clipboard` after a
  user-gesture click).
- **3.2** [agent] Media page: Edition row at rest shows value + `file · name` badge; expanded row
  shows one file chip + custom input, no empty-source chip; Confirm on a typed value → badge reads
  `custom`; reload → persists.
- **3.3** [agent] Film page: file with edition shows the pill; file without shows **+ Set edition**
  (owner) / nothing (visitor); following the link lands on `#field-edition` expanded.
- **3.4** [agent] Person header: no "In files as" line when resolved = canonical; after choosing
  `tmdb` in the badge, the line appears with the mono canonical and a `tmdb` badge; the h1 shows
  the TMDB spelling; the pencil still opens rename with the **canonical** value prefilled.
- **3.5** [agent] Contrast per §7 in all three skins via computed styles; no horizontal overflow
  at 375px on the media page with a 40-character custom edition.
- **3.6** [agent] Display name (378): visitor sees the h1 in the display spelling and the "In files
  as" line with no badge/link/pencil; owner with no decision sees the **Display as…** link and
  clicking it opens the chip row with focus on the badge; confirming `record` restores canonical,
  removes the line and returns focus to the link; `/search?q=<display>` returns the row with
  `display_name`; a 70-character unbroken canonical does not scroll the page at 375px.

### §4 Human

- **4.1** [human] Open any person page. Under the name, at the end of the small grey line, there's
  a pill like `person:1234`. Click it. It should say "Copied" for a moment. Paste somewhere — you
  get exactly that text. Looks right = the pill is quiet, doesn't compete with the name.
- **4.2** [human] Open a film with two full-film files. One row has a small label like "Final
  Cut" before the resolution badge; the other has a dashed "+ Set edition". Click it — you land on
  that file's page with the Edition row already open. Type "Theatrical", Confirm. Go back to the
  film: both rows now have labels.
- **4.3** [human] **The 378 go/no-go.** Open a person whose provider spelling differs from the
  file. Without reading any help text: which control would you use to *change how the name looks
  without touching the file*, and which to *rename the person for real*? If you hesitate, tell
  the agent — that's the signal to cut 378.
  **Run 2026-09-15 — pass, after one iteration.** First pass: pencil = rename (right); "change how
  it looks" → the tmdb Refresh button (wrong — the adoption layer). Diagnosis: at rest nothing on
  the page said another spelling existed, and "Display as…" didn't say what. Re-run with the two
  verbs stated in plain terms: straight to Display as. Fix shipped: the link names the offered
  spelling (`Display as Ana Keßler (tmdb)…`). 378 stays. Follow-up from the same run: in the
  decision-standing state the owner saw "no way to change the name written to files" (the pencil
  is hover-only) — the record spelling on the "In files as" line is now the rename trigger.

## Open questions for the owner

- **OQ1** (§3b) Set edition: deep link to the media page (recommended) vs inline chip row on the
  film page.
- **OQ2** (§4c) Does the pencil-vs-badge split read at a glance? This is the story's kill criterion.
