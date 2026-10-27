# Spec: Duplicates lanes and pair evidence — one lane per entity type, and a flagged pair expands into a symmetric side-by-side compare panel (F70)

**Status**: Shipped for people ([HOLODEX-451](https://whoiskevinrich.atlassian.net/browse/HOLODEX-451));
the lanes, the People rework and the Studios panel are in design
([HOLODEX-598](https://whoiskevinrich.atlassian.net/browse/HOLODEX-598)). Requirements that epic adds
are tagged *(598)*.
**Phase**: Phase 3 (presentation) — rides the F43 duplicates queue (ADR-061), person images
(F26), the resolver (F27) and the F68 card endpoint
**Owner**: Project owner
**Date**: 2026-09-22 · revised 2026-10-27
**Jira**: [HOLODEX-451](https://whoiskevinrich.atlassian.net/browse/HOLODEX-451),
[HOLODEX-598](https://whoiskevinrich.atlassian.net/browse/HOLODEX-598)
**Feature block**: **F70** — the owner's Duplicates page shows **one lane per entity type**
(Videos, People, Studios, Tags, Films), and a lane reads only its own pairs. A person or studio pair
expands in place into a **two-column compare panel** that works like the Videos lane: the evidence
for each side sits in its own column, and the owner picks the survivor with **Keep this one** under
that column. Keep separate stays one click from the collapsed row.

## Problem Statement

`/owner/duplicates?type=person` gives the owner two names, two video counts, a variation kind
and a match kind. That is not enough to decide whether a flagged pair is one person or two, so
the owner is the entire inference engine on a surface built for batch work.

Measured on the live library 2026-09-22 (1591 people, 1000 enriched, `scripts/detect_person_duplicate_evidence.sql`):
**all 31 queued person pairs are `provider-alias` / `alias`** — the weakest match kind the
detector produces — and **185 person pairs have already been dismissed as keep-separate.** The
cost of not solving it is a queue the owner stops working, which silently accumulates split
identities forever.

The owner's review on 2026-10-27, after working the shipped panel (*598*):

- Every entity type on one page is visually overloading, and the page is slow to load.
- The person images are too small to compare faces.
- Merge sits in an unexpected place.
- Only the chevron opens a row.
- Following a name or a Videos link loses the owner's place in the queue.
- A studio pair has no evidence at all. Two similar studio names usually mean one spelling came from
  a file tag and the other from a provider, and the file tag is often the misspelling or
  abbreviation. Nothing on the page says which is which.

The Videos lane drew none of these complaints, and the owner is happy with it. It is the template.

## Goals

1. **Decide a pair without leaving the page.** Face, counts, aliases and provider links for both
   sides visible simultaneously, never from memory.
2. **Make the common answer the cheap one.** Keep separate is the dominant verdict; it should be
   the primary action and reachable in one click from the collapsed row.
3. **Report evidence, never adjudicate it.** The panel says who asserted what. It never renders
   a computed "same person" / "different people" verdict.
4. **Cost nothing when collapsed.** The queue stays a dense list; evidence is fetched per pair,
   on expand.
5. **One lane at a time, and only that lane's cost.** *(598)* The owner works one entity type at
   a time, and opening a lane reads only that lane's pairs. The per-lane counts never wait on
   another lane's pairs.
6. **Say where a studio name came from.** *(598)* For each side of a studio pair, the panel shows
   whether the name came from file tags or from a provider, and on how many videos each.
7. **One verdict grammar across lanes.** *(598)* Wherever a pair has a panel, the survivor is
   picked under its own column, as in Videos.

## Non-Goals

| Not doing | Why |
|---|---|
| A computed confidence score or recommended verdict | The probe killed its backing: `same_xid` is 0 across all 31 pairs, and `diff_xid` — the only signal with coverage — means nothing (RD3). A score built on it would be confidently wrong at 100% coverage. |
| Auto-merging any pair | No queued pair shares an external id, so there is nothing to auto-merge. Merge is irreversible (ADR-061). |
| Changing what gets flagged | Whether alias-only provider-alias pairs belong in the queue at all is [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453). |
| Shared-external-id detection | [HOLODEX-452](https://whoiskevinrich.atlassian.net/browse/HOLODEX-452) — a different, higher-confidence detector; disjoint from this queue. |
| Mounting the F68 hover card here | RD2. It is a one-at-a-time affordance and this is a comparison task. |
| Tag and film panels | Their rows are the evidence: a tag has a single field. They keep the inline Merge step (RD10). |
| A two-column image lightbox, with one person's gallery beside the other's | *(598)* The larger lead image (P0-3) covers most of the need. It is a candidate follow-up if a lead image plus a 5-frame strip still leaves pairs undecided. |
| A separate "display name" for studios | *(598)* A studio has one name, plus aliases. Show as (P0-11) is a rename, not a new kind of name (RD14). |

## Resolved Decisions

Locked during the 2026-09-22 brainstorm and confirmed against the live probe.

| # | Decision | Rationale |
|---|---|---|
| RD1 | **Expand-to-compare**: the row gains a disclosure; clicking expands a two-column panel in place. Not inline row chips, not a focus mode, not a modal. | Kevin's call over four alternatives: *"A profile image is the easiest for me as a human to distinguish."* In place keeps the queue as the index and costs nothing when collapsed. |
| RD2 | **The F68 person hover card is not mounted here.** | One card open app-wide by design, so comparison happens from memory; 250 ms intent ×2 per row on a batch surface; hidden under `pointer: coarse`; and the row's names are `<span>`s, so `PersonLinkChip` is not free to adopt. It may return later as the escape hatch once the names become links (P2). |
| RD3 | **Positive evidence can be decisive; negative evidence never is.** No "different ids → different people" inference, ever. | Kevin, 2026-09-22: providers carry their own duplicate entries. `diff_xid` is 2 on nearly every queued pair — the pre-correction ranking would have stamped a false verdict across the whole queue. |
| RD4 | **Facts are only comparable inside a provider namespace, and the panel never adjudicates.** No conflict chip, no "mismatch" styling. | Cross-provider disagreement describes the providers: `Nepal` vs `Federal Democratic Republic of Nepal` is a country-name update, `1990-01-01` vs `1990-03-17` a fidelity difference. Measured: nationality 18 differ / 11 exact / 6 one-contains-other. |
| RD5 | **Symmetric layout.** Both columns identical; a side with no image shows the themed placeholder `PersonImageFrame` already returns. | 29 of 31 pairs have a headshot on both sides. Designing around asymmetry would optimise for 2 pairs. |
| RD6 | **An image strip per side, not a single headshot**, headshot first, capped at 5 visible. | Most people carry 40–50 images; the provider headshot is not always the recognisable one. |
| RD7 | **Verdict emphasis swaps**, in the collapsed row and the panel: **Keep separate** takes `.btn-row .btn-pill .btn-accent`, **Merge** takes the `.btn-ghost px-2` that Keep separate vacates — a literal swap of the two existing classes. **Not `.btn-quiet`**, which `app.css:300–306` documents as *"a UI-only toggle with no side effect (Cancel, Undo)"*; Merge is the least reversible action on the page and stays bordered and equally hit-targetable. Design, 2026-09-22. | 185 dismissals against 31 open. The common action currently wears the ghost styling. Merge stays two-step — it is irreversible. *(598: on a pair with a panel, Merge leaves the row and the survivor is chosen under its own column, per RD15. The design handoff restyles those verdicts to match Videos.)* **`app.css:281–292` and `:313–324` name these two buttons by name in their doc comments; the swap makes them wrong and they must be updated in the same commit**, or the role vocabulary drifts for `ExtractionQueueRow` and `EnrichQueueRow`, which share the classes. |
| RD8 | **Zero new endpoints.** The panel fetches `GET /people/{id}/card` (F68) and `GET /people/{id}/images` (F26) per side, on expand, cached for the session. | Both already exist and are already owner-safe on this surface. `GET /people/{id}` is not an option — it ships up to 500 videos. This is why `architecture` is `[~]` n/a for this epic. |
| RD9 | **Attributed provider facts are P1, not P0.** | `PersonCard` returns the *resolved* nationality and age, not per-provider values; attribution needs enrichment rows the card does not carry. P0 ships without it rather than growing a new read. |
| RD10 | **Person and studio get panels; tag and film do not.** *(Revised 2026-10-27, 598 — was person only.)* Tag and film rows keep today's behaviour: no disclosure, the match-kind label in the row, and the inline Merge step. A studio pair moves its label into the panel, as a person pair does. | Person has images, aliases and provider facts. The 2026-09-22 call was that a studio has "no evidence worth two columns". The owner's review showed that the evidence was there all along, just never shown: **where each name came from**. Two similar studio names are usually a file-tag spelling against a provider's. Tags dominate the queue by count but still have a single field. |
| RD13 | **One lane per entity type, and a lane reads only its own pairs.** *(598)* Each lane shows its count whether or not it is open. The existing per-type link (the entity lists' Duplicates banner) opens that lane. | The owner found all types on one page visually overloading. Today's page reads every entity type's pairs on every load, and again after every verdict. Splitting the view without splitting the read would leave the wait in place behind a tab bar. |
| RD14 | **Show as is a rename, not a display-name layer.** *(598)* A studio has one name, plus aliases (F43 RD6). Merging always keeps the losing name as an alias. Show as then renames the survivor to the chosen spelling, so both spellings stay on the studio: one as its name, the other as an alias. It has every consequence a rename already has, including the name offered for writeback. | A second "display name" would give a studio two places that decide its name. The per-field source decisions already own which source wins for a field, and an adoption or identity surface must not compete with them. Identity (which record survives, with its links and history) and spelling (which name shows) are separate choices. The owner named both. |
| RD15 | **The survivor is chosen under its own column; Merge leaves the collapsed row.** *(598)* On a pair with a panel, the collapsed row carries only Keep separate. Merging needs the panel open. Tag and film rows keep the inline Merge, which leads to the survivor picker. | This is the Videos grammar ("Keep this one" under each file), which the owner calls intuitive. The old Merge, then "Keep: A / B", put an irreversible choice away from the evidence that informs it. |
| RD16 | **The whole row toggles its panel.** *(598)* A click anywhere on a row with a panel opens or closes it. Its buttons and links do their own thing and never toggle. | Only a 28 px chevron opened the row. |
| RD17 | **Names and profile links open in a new tab, and look like they do.** *(598)* An entity's name in a pair row or panel links to its detail page in a new tab, and so do the panel's Videos and Films links. Each carries the new-tab mark used in the Videos lane (Open ↗). | The owner checks a profile and comes back. Leaving the page loses their place in the queue. The mark keeps a link from reading as part of the row's toggle. Revises P2-1, which kept names as plain text so that they would not compete with the row. |
| RD11 | **The link row leans on provider link templates and `website`, not `_source_url`.** | `_source_url` exists for 86 of 1591 people; `website` for 688. A row keyed on `_source_url` would be empty on most pairs. |
| RD12 | **One panel open at a time**, and resolving a pair removes the row (and its panel with it). | A second open panel doubles the fetch and halves the width; the queue is a sequence, not a dashboard. |
| RD18 | **The lead image is large enough to compare faces.** *(598)* Each side shows one large lead image above its strip, and choosing a strip frame makes it that side's lead. Both leads are always visible at once. No hover-zoom. | At 44 px no frame was large enough to compare. Hover-zoom shows one face at a time, and comparing means looking at both. |

## User Stories

1. As the owner, I want to see both people's faces side by side so that I can tell one person
   from two in a couple of seconds.
2. As the owner, I want to keep a pair separate in one click from the collapsed row, so that the
   common answer costs nothing.
3. As the owner, I want to see each side's provider links so that I can open the provider's own
   page when the faces are ambiguous.
4. As the owner, I want to see when two flagged people appear in the same video, because that is
   near-proof they are different people.
5. As the owner, I want the panel to tell me which provider asserted a fact rather than telling
   me the facts conflict, so that I am not misled by a country-name change or a date fidelity
   difference. *(P1)*
6. As the owner working a long queue, I want the panel to open and close by keyboard so that I
   never leave the home row.
7. As the owner, I want to work one entity type at a time, and have it load quickly, so that the
   page is not a wall of every kind of pair. *(598)*
8. As the owner, I want to click anywhere on a row to open it, so that I am not hunting for the
   chevron. *(598)*
9. As the owner, I want a person's or studio's name, and the Videos and Films links, to open in a
   new tab, so that checking a profile never loses my place in the queue. *(598)*
10. As the owner, I want the faces big enough to compare, both at once, so that I am judging
    people and not thumbnails. *(598)*
11. As the owner, I want to pick the person or studio to keep from under its own column, as I do
    for videos, so that the irreversible choice sits next to its evidence. *(598)*
12. As the owner, I want to see whether each studio name came from my files or from a provider,
    and on how many videos, so that I can spot the misspelled or abbreviated file tag. *(598)*
13. As the owner, I want to merge two studios, keep the better-linked record and still show the
    other spelling, so that identity and spelling are two separate choices. *(598)*

## Requirements

### Must-Have (P0)

**P0-1 · Disclosure on person and studio rows.** *(Studios and the whole-row toggle added by 598.)*
A pair with a panel (person, studio) gets a disclosure. Collapsed is the default. A click anywhere
on the row toggles it (RD16).

- [ ] Given a person or studio pair, when the row renders, then a disclosure control is present and the row is collapsed
- [ ] Given a tag or film pair, when the row renders, then no disclosure is present and the row is byte-identical to today's
- [ ] *(598)* Given a collapsed person or studio row, when the owner clicks anywhere on it outside a button or link, then its panel opens; a second click closes it
- [ ] *(598)* Given a click on a button or link inside the row, then the panel does not toggle
- [ ] The control is a `<button>` with `aria-expanded` and `aria-controls`, never a nested interactive element inside another control
- [ ] Given the widest person pair in the queue, when the row renders collapsed, then it occupies a single line with both verdicts on that same line — the match-kind label lives in the panel (OQ3), and the names truncate rather than wrapping
- [ ] Given a person pair with a match kind of `alias` or `mixed`, when the row renders collapsed, then no match-kind label appears in the row; when expanded, then it is the panel's first line

**P0-2 · Symmetric two-column panel.** Expanding fetches and renders both sides identically:
image strip, display name, video count, film count, aliases, nationality flags, provider-link
badges.

- [ ] Given a pair with a headshot on both sides, when expanded, then two columns render with equal width and the same field order
- [ ] Given a side with no images, when expanded, then that column shows the themed placeholder and every other field still renders
- [ ] Given a field the card omits, when expanded, then the segment is dropped — no `—`, no "unknown" (the F68 rule)
- [ ] At the mobile breakpoint the two columns stack, first side above second, with an explicit visual separator

**P0-3 · Lead image and strip.** *(Lead image added by 598, RD18.)* Each side shows a **lead
image** large enough to compare faces, above a strip of headshot first, then gallery images in
`sort_order`, capped at **5 frames**. The lead starts as the headshot. Choosing a strip frame makes
it that side's lead, and the other side does not change. Sizes are set by the design handoff.
**There is no `+N`** — the strip is a sample, not an index (OQ2).

- [ ] *(598)* Given an expanded person pair, then both sides' lead images are visible at the same time, at the same size
- [ ] *(598)* Given the owner chooses the third strip frame on one side, then that frame becomes that side's lead and the other side's lead is unchanged
- [ ] *(598)* Given a side with no images, then its lead is the themed placeholder and no strip renders
- [ ] *(598)* Nothing on the panel enlarges on hover

- [ ] Given a person with more than 5 images, when expanded, then exactly 5 render and nothing indicates there are more
- [ ] Given a person with 1 image, when expanded, then 1 renders and the remaining slots are not reserved
- [ ] Given a person with no images, when expanded, then exactly one placeholder frame renders — not five empty wells
- [ ] Slot 1 is `role="headshot"`; slots 2–5 are `gallery` filtered to `img.role !== 'headshot'`, in `sort_order`
- [ ] Images route through `PersonImageFrame` so the skin-aware, cache-busted URL and placeholder behaviour are unchanged

**P0-4 · Verdict emphasis swap.** In the collapsed row and the panel footer, **Keep separate**
carries the accent pill and **Merge** the bordered-neutral `.btn-ghost` styling (RD7 — not
`.btn-quiet`). Merge still opens the survivor picker.

- [ ] Given any pair row of any entity type, when it renders, then Keep separate is the accent action
- [ ] Given any pair row, when it renders, then Merge is bordered — never borderless
- [ ] Given Merge is pressed, when the survivor buttons appear, then both names are shown in full and Cancel returns to the verdicts
- [ ] Given Merge is pressed while the panel is open, when the survivor buttons appear, then the panel stays open — the survivor choice is exactly when the evidence is wanted on screen
- [ ] Merge remains two-step; no single click can merge
- [ ] The `.btn-ghost` / `.btn-accent` doc comments in `app.css` are updated in the same commit

*(598)* On a pair with a panel (person, studio), the criteria above that place Merge in the row
are replaced by RD15:

- [ ] Given a collapsed person or studio row, then its only verdict is Keep separate
- [ ] Given an expanded person or studio pair, then each column ends with a Keep this one action, and Keep separate is still offered for the pair
- [ ] Given Keep this one under a column, then the owner confirms before anything merges, and the confirmation names the survivor and the name that becomes an alias
- [ ] Given a tag or film pair, then the row's verdicts and two-step Merge are unchanged

**P0-8 · One lane per entity type.** *(598, RD13)* The page shows lanes for Videos, People,
Studios, Tags and Films. Only the open lane's pairs are shown.

- [ ] Given the page opens, then every lane shows its pair count, including lanes the owner has not opened
- [ ] Given the owner opens a lane, then only that type's pairs are read and shown, and paging stays within that lane
- [ ] Given a link from an entity list's Duplicates banner, then the page opens on that type's lane
- [ ] Given no lane is named, then the page opens on the first lane, in page order, that has pairs
- [ ] Given a lane has no pairs, then it still appears with a count of 0, so the lanes never shift position
- [ ] Given a verdict in one lane, then only that lane's pairs and the counts are re-read
- [ ] A visitor never sees any lane, or any sign of one

**P0-9 · Names and profile links open in a new tab.** *(598, RD17)*

- [ ] Given a person or studio pair, collapsed or expanded, then each name links to that entity's detail page in a new tab
- [ ] Given a person panel, then its Videos and Films links open in a new tab
- [ ] Every new-tab link carries the same new-tab mark as the Videos lane's Open link
- [ ] Opening a new tab never changes the queue page: the open lane, the open panel and the scroll position stay where they were

**P0-10 · Studio compare panel.** *(598, RD10)* Expanding a studio pair shows both sides in the
same layout: the name, **where the name came from**, a few sample video posters, and the names it
is also known as.

- [ ] Given a studio pair, when expanded, then each side lists its name sources: file tags or each provider, with the number of videos for each
- [ ] Given a studio whose name came from file tags on 3 videos and from provider-1 on 38, then that side shows both sources with those counts
- [ ] Given a side with only one source, then it shows that one source and no empty slots
- [ ] Given a side, then up to a small fixed number of its videos' posters render, and a studio with no videos shows none
- [ ] Given a side with aliases, then they are listed; a side with none says so in words rather than leaving a gap
- [ ] The panel describes where names came from and never says which spelling is wrong (Goal 3)
- [ ] Evidence loads on expand and follows P0-5: a collapsed studio row issues no request

**P0-11 · Show as.** *(598, RD14)* After Keep this one on a studio, the owner chooses which of
the two spellings the surviving studio shows. The default is the survivor's own name.

- [ ] Given Keep this one on studio A, when the owner confirms with Show as A, then B's videos and links move to A, and B's name is an alias of A
- [ ] Given Keep this one on studio A, when the owner confirms with Show as B, then the merge happens as above, A is renamed to B's spelling, and A's former name is an alias, so both spellings still route to the studio
- [ ] Given either outcome, then no name or alias either studio had before is lost
- [ ] Given Show as B, then the rename has the same consequences as renaming the studio from its own page, and the confirmation says so
- [ ] Cancel at any point merges nothing

**P0-5 · Per-pair fetch on expand.** Both sides load via `api.personCard` and
`api.personImages` when the panel first opens, cached for the session; a collapsed row issues no
request.

- [ ] Given a collapsed queue of N pairs, when the page loads, then zero card/image requests are made
- [ ] Given a panel is expanded twice, when it reopens, then no second request is made
- [ ] Given a fetch fails, when the panel renders, then it shows an inline error for that side only and the verdicts stay usable
- [ ] A failure is not cached

**P0-6 · Keyboard and dismissal.** Enter/Space toggles the disclosure, Escape collapses an open
panel, and focus returns to the disclosure.

- [ ] Given a panel is open, when Escape is pressed, then it collapses and focus lands on its disclosure
- [ ] Given a pair is resolved, when the row is removed, then focus moves to the next row's disclosure — or to the group heading when it was the last row in the group. **There is no fade**: `routes/owner/duplicates/+page.svelte:47–49` is an unanimated `pairs.filter()` and always has been, so focus would otherwise fall to `<body>`. The stale "the row fades out" comment at `DuplicatePairRow.svelte:7` is corrected in the same commit.
- [ ] Opening a second panel collapses the first (RD12)

**P0-7 · Cinémathèque QA.** Tokens only; no hardcoded colour, radius or shadow.

### Nice-to-Have (P1)

- **P1-0 · Co-appearance marker.** When the two sides appear together in at least one video, the
  panel says so plainly — descriptive, never a verdict ("they appear together", not "they are
  different people"), and silent when they do not. **Demoted from P0 by OQ1**: it is derivable
  from two `api.videos({ personId })` calls intersected client-side, but that is two paged list
  fetches per expand to serve the 3 pairs in 31 that actually co-appear. Revisit if
  [HOLODEX-452](https://whoiskevinrich.atlassian.net/browse/HOLODEX-452) adds a cheap
  co-appearance read, and keep it out of the backend gate until then.
- **P1-1 · Attributed provider facts.** Per-provider `birthdate` / `nationality` rows rendered as
  `<provider> · <value>` for each side, never compared or flagged (RD4, RD9). Needs a read that
  carries enrichment rows; decide then whether it widens `/people/{id}/card` or rides a new
  parameter.
- **P1-2 · Nationality normalization** reusing whatever `NationalityFlags` maps names to flags
  with, so `Nepal` and `Federal Democratic Republic of Nepal` present identically.
- **P1-3 · First-seen / provenance line** per side — file-born vs provider-born is a strong
  duplicate tell and the probe shows enrichment coverage on 1000 of 1591 people.

### Future Considerations (P2)

- ~~**P2-1 · Names become links.**~~ Promoted to P0-9 by 598 (RD17). Whether the F68 hover card
  returns on those links is still open (RD2).
- **P2-2 · Panels for film and tag** (RD10), if their evidence ever justifies it. *(Studio was
  promoted to P0-10 by 598.)*
- **P2-4 · Two-column image lightbox** *(598)*: one person's gallery beside the other's, if the
  lead image (P0-3) still leaves pairs undecided.
- **P2-3 · Keyboard verdicts** (`K` / `M`) and auto-advance, if the queue ever grows past a size
  where clicking is the bottleneck.

## Data model

None. No migration, no schema change.

## API

None added. The panel composes two existing owner-safe reads per side:

| Call | Supplies |
|---|---|
| `GET /people/{id}/card` (F68) | display name, headshot version, video + film counts, age, nationality, aliases, `external_links` |
| `GET /people/{id}/images` (F26) | the image strip |

**P0 therefore touches no Go code at all.** Co-appearance was the only P0 candidate that would
have needed one, and OQ1 resolved by demoting it (P1-0) rather than adding a read.

*(598)* That no longer holds for the 598 requirements, which need backend work:

- a lane read that returns one type's pairs, plus counts for every lane that don't read any pairs (P0-8);
- name sources and sample posters for a studio (P0-10);
- a merge that can also rename the survivor (P0-11).

The design handoff and the build own the shapes. None of them is a technology fork, so
`architecture` stays `[~]`: lanes and studio evidence reuse the existing queue and identity model,
and Show as reuses the existing merge and rename.

## UI

`DuplicatePairRow` gains a disclosure and a panel; the panel itself is a new
`DuplicateComparePanel.svelte` in `web/src/lib/components/duplicates/`, per that folder's
`CLAUDE.md` (whose table gains a row for it in the same commit). Reused verbatim:
`PersonImageFrame`, `NationalityFlags`, `ProviderLinkBadge` — and **F68's `#videos` / `#films`
profile anchors**, which the link row keeps. With `+N` gone (P0-3) and the pair-row names still
`<span>`s until P2-1, those two links are the panel's only in-app path to a profile; they cost
nothing, since F68 already added the ids. `.btn-row` / `.btn-pill` shapes are unchanged — only
which verdict carries `btn-accent` moves.

**Each column is the F68 hover card's anatomy laid flat** — same fields in the same order, the
same absent-is-absent rule, with the image strip where its single 48 px headshot was. That is
the concrete form of RD2: the hover card was never the wrong content, only the wrong container.
Full layout, states, tokens and accessibility in
[`duplicates-pair-evidence-handoff.md`](../design/duplicates-pair-evidence-handoff.md).

## Success Metrics

- **Leading**: the open person queue is worked to zero at least once within two weeks of ship.
  Measured by `SELECT count(*) FROM identity_review_queue WHERE entity_type='person'`.
- **Leading**: no pair is resolved by navigating away to a profile and back — the panel is
  sufficient on its own. Measured by the owner's own report during QA, not instrumentation.
- **Leading** *(598)*: opening a lane is measurably faster than today's whole-page load on the
  live library. The baseline is taken before the build (OQ5), and the target is set from it.
- **Leading** *(598)*: the open studio queue is worked to zero at least once within two weeks of
  ship, with no studio pair resolved by leaving the page to look up where a name came from.
- **Lagging**: `entity_keep_separate` keeps growing while merges stay rare, confirming the
  queue's real character and feeding the [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453)
  decision on whether to keep flagging these at all.

## Open Questions

| # | Question | Who | Blocking |
|---|---|---|---|
| ~~OQ1~~ | ~~Can co-appearance be derived without a new read?~~ **Resolved 2026-09-22:** yes, via two `api.videos({ personId })` calls intersected client-side — but that is two paged list fetches per expand for a marker that fires on 3 of 31 pairs. Demoted to **P1-0** rather than either adding an endpoint or paying that cost. `architecture` stays `[~]` n/a. | — | Closed |
| ~~OQ2~~ | ~~Does the `+N` affordance open `PersonGalleryModal` in place, or navigate?~~ **Resolved 2026-09-22 (Kevin): neither — there is no `+N`.** Five frames are the whole strip: it is a sample, not an index, and if five faces don't settle it the panel has failed anyway. Also removes a second dismissable layer competing with Escape inside an expanded row. The profile stays reachable via the link row's `Videos` / `Films`. | — | Closed |
| ~~OQ3~~ | ~~Keep the `via alias` / `alias match only — weak signal` labels once the panel exists?~~ **Resolved 2026-09-23 (Kevin), superseding the 2026-09-22 answer: keep both, but a person pair carries them in the panel, not the row.** The row keeps `· {variation}` only and stays one 40 px line; every non-person pair keeps its label in the row, since the panel is person-only (RD10). The first answer put them in the row against a mockup that drew them fitting — measured at real type sizes they need ~836 px and wrapped every row onto a second line, and the probe found 100 % of the queue carries one. They still explain why the *detector* flagged the pair, so they now open the panel that holds the evidence. [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453) may change what gets flagged; if the queue ever becomes mixed, revisit whether the scan line needs the signal back. | — | Closed |
| OQ5 | *(598)* What does the page take to load on the live library today, from opening it to the first usable lane? This is the baseline for the load-time metric. Probe it before the build, with output anonymized. | Data | Yes, for the target |
| OQ6 | *(598)* Can a studio's name sources (file tags against each provider, with video counts) be read cheaply enough to load on expand? The video's studio field already knows which source won. The question is the cost per studio. | Engineering | Yes, for P0-10 |
| OQ7 | *(598)* How many sample posters should a studio side show, and which videos should they be: the most recent, a random sample, or one per source? | Design | No |
| ~~OQ4~~ | ~~Is 5 the right strip cap?~~ **Resolved 2026-09-22 (Kevin): 5 frames at 44 px (`w-11`), `gap-1`** — now measured, not guessed: `5 × 44 + 4 × 4 = 236 px` inside a 296 px column (`field-grid`'s 320 px minimum less `p-3` both sides). 4 × 52 px wastes 43 px of column; 6 × 40 px drops each face below the 44–48 px the F68 card already proved recognisable. | — | Closed |

## Timeline / routing

No hard deadline. Routing per `CLAUDE.md`: spec (this document) → **`/design-handoff` — done
2026-09-22**, [`duplicates-pair-evidence-handoff.md`](../design/duplicates-pair-evidence-handoff.md)
+ `duplicates-pair-evidence-mockup.svg`, closing OQ2/OQ3/OQ4; its four deltas are applied above
and approved → `/implement` for the design sign-off and the draft PR → build →
`/testing-strategy` → `/security-review` (owner-gated surface, re-confirm the existing gate
covers the reused reads) → `/code-review high --fix`.

`architecture` is `[~]` n/a: the panel adds no endpoint, no migration and no cross-cutting
decision, and OQ1 closed without forcing one. The **backend gate is `[~]` n/a for P0 as well** —
P0 is frontend-only. Both rulings are void if a later slice grows a read.

**598 revision.** Spec (this edit) → `/design-handoff`, which revises the handoff and its SVG for
the lanes, the People rework and the Studios panel, and answers OQ7 → OQ5 and OQ6 probed →
`/implement` → build → `/testing-strategy` → `/security-review` → `/code-review high --fix`.
598 grows reads, so its **backend gate is in scope**. `architecture` stays `[~]` (see API).
