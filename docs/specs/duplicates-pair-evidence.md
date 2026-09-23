# Spec: Duplicates pair evidence — expand a flagged pair into a symmetric side-by-side compare panel (F70)

**Status**: Draft
**Phase**: Phase 3 (presentation) — rides the F43 duplicates queue (ADR-061), person images
(F26), the resolver (F27) and the F68 card endpoint; adds one component and no new endpoint
**Owner**: Project owner
**Date**: 2026-09-22
**Jira**: [HOLODEX-451](https://whoiskevinrich.atlassian.net/browse/HOLODEX-451)
**Feature block**: **F70** — a pair row in the owner's Duplicates queue expands in place into a
**two-column compare panel**: an image strip, name, counts, aliases and provider-link badges for
each side, an evidence line for the facts that are actually comparable, and the two verdicts.
The verdict emphasis is **swapped** so Keep separate — the dominant real-world answer — is the
primary action.

## Problem Statement

`/owner/duplicates?type=person` gives the owner two names, two video counts, a variation kind
and a match kind. That is not enough to decide whether a flagged pair is one person or two, so
the owner is the entire inference engine on a surface built for batch work.

Measured on the live library 2026-09-22 (1591 people, 1000 enriched, `scripts/detect_person_duplicate_evidence.sql`):
**all 31 queued person pairs are `provider-alias` / `alias`** — the weakest match kind the
detector produces — and **185 person pairs have already been dismissed as keep-separate.** The
cost of not solving it is a queue the owner stops working, which silently accumulates split
identities forever.

## Goals

1. **Decide a pair without leaving the page.** Face, counts, aliases and provider links for both
   sides visible simultaneously, never from memory.
2. **Make the common answer the cheap one.** Keep separate is the dominant verdict; it should be
   the primary action and reachable in one click from the collapsed row.
3. **Report evidence, never adjudicate it.** The panel says who asserted what. It never renders
   a computed "same person" / "different people" verdict.
4. **Cost nothing when collapsed.** The queue stays a dense list; evidence is fetched per pair,
   on expand.

## Non-Goals

| Not doing | Why |
|---|---|
| A computed confidence score or recommended verdict | The probe killed its backing: `same_xid` is 0 across all 31 pairs, and `diff_xid` — the only signal with coverage — means nothing (RD3). A score built on it would be confidently wrong at 100% coverage. |
| Auto-merging any pair | No queued pair shares an external id, so there is nothing to auto-merge. Merge is irreversible (ADR-061). |
| Changing what gets flagged | Whether alias-only provider-alias pairs belong in the queue at all is [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453). |
| Shared-external-id detection | [HOLODEX-452](https://whoiskevinrich.atlassian.net/browse/HOLODEX-452) — a different, higher-confidence detector; disjoint from this queue. |
| Mounting the F68 hover card here | RD2. It is a one-at-a-time affordance and this is a comparison task. |
| Studio, tag and film panels | Person first, where the evidence is richest. The row stays as-is for other entity types (RD10). |

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
| RD7 | **Verdict emphasis swaps**, in the collapsed row and the panel: **Keep separate** takes `.btn-row .btn-pill .btn-accent`, **Merge** takes the `.btn-ghost px-2` that Keep separate vacates — a literal swap of the two existing classes. **Not `.btn-quiet`**, which `app.css:300–306` documents as *"a UI-only toggle with no side effect (Cancel, Undo)"*; Merge is the least reversible action on the page and stays bordered and equally hit-targetable. Design, 2026-09-22. | 185 dismissals against 31 open. The common action currently wears the ghost styling. Merge stays two-step — it is irreversible. **`app.css:281–292` and `:313–324` name these two buttons by name in their doc comments; the swap makes them wrong and they must be updated in the same commit**, or the role vocabulary drifts for `ExtractionQueueRow` and `EnrichQueueRow`, which share the classes. |
| RD8 | **Zero new endpoints.** The panel fetches `GET /people/{id}/card` (F68) and `GET /people/{id}/images` (F26) per side, on expand, cached for the session. | Both already exist and are already owner-safe on this surface. `GET /people/{id}` is not an option — it ships up to 500 videos. This is why `architecture` is `[~]` n/a for this epic. |
| RD9 | **Attributed provider facts are P1, not P0.** | `PersonCard` returns the *resolved* nationality and age, not per-provider values; attribution needs enrichment rows the card does not carry. P0 ships without it rather than growing a new read. |
| RD10 | **Person only.** Studio/tag/film rows keep today's behaviour, with no disclosure rendered — **and, for the same reason, keep their match-kind label in the row.** | Person is the only entity with images, aliases and provider facts rich enough to justify a panel. Tags dominate the queue by count but have a single field. The label moves into the panel on a person pair (OQ3); a non-person pair has no panel to move it to, so gate the move on entity type rather than removing the label — otherwise a studio pair loses the only thing explaining why two unalike names are paired. |
| RD11 | **The link row leans on provider link templates and `website`, not `_source_url`.** | `_source_url` exists for 86 of 1591 people; `website` for 688. A row keyed on `_source_url` would be empty on most pairs. |
| RD12 | **One panel open at a time**, and resolving a pair removes the row (and its panel with it). | A second open panel doubles the fetch and halves the width; the queue is a sequence, not a dashboard. |

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

## Requirements

### Must-Have (P0)

**P0-1 · Disclosure on person rows.** `DuplicatePairRow` renders a disclosure control when
`pair.entity_type === 'person'`. Collapsed is the default.

- [ ] Given a person pair, when the row renders, then a disclosure control is present and the row is collapsed
- [ ] Given a studio, tag or film pair, when the row renders, then no disclosure is present and the row is byte-identical to today's
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

**P0-3 · Image strip.** Headshot first, then gallery images in `sort_order`, capped at **5 frames
at 44 px (`w-11`), `gap-1`**. **There is no `+N`** — the strip is a sample, not an index (OQ2).

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

**P0-7 · Three-skin QA.** Tokens only; no hardcoded colour, radius or shadow.

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

- **P2-1 · Names become links**, which would let `PersonLinkChip` wrap them and bring the F68
  card back as the escape hatch (RD2).
- **P2-2 · Panels for studio, film and tag** (RD10), if their evidence ever justifies it.
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
- **Lagging**: `entity_keep_separate` keeps growing while merges stay rare, confirming the
  queue's real character and feeding the [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453)
  decision on whether to keep flagging these at all.

## Open Questions

| # | Question | Who | Blocking |
|---|---|---|---|
| ~~OQ1~~ | ~~Can co-appearance be derived without a new read?~~ **Resolved 2026-09-22:** yes, via two `api.videos({ personId })` calls intersected client-side — but that is two paged list fetches per expand for a marker that fires on 3 of 31 pairs. Demoted to **P1-0** rather than either adding an endpoint or paying that cost. `architecture` stays `[~]` n/a. | — | Closed |
| ~~OQ2~~ | ~~Does the `+N` affordance open `PersonGalleryModal` in place, or navigate?~~ **Resolved 2026-09-22 (Kevin): neither — there is no `+N`.** Five frames are the whole strip: it is a sample, not an index, and if five faces don't settle it the panel has failed anyway. Also removes a second dismissable layer competing with Escape inside an expanded row. The profile stays reachable via the link row's `Videos` / `Films`. | — | Closed |
| ~~OQ3~~ | ~~Keep the `via alias` / `alias match only — weak signal` labels once the panel exists?~~ **Resolved 2026-09-23 (Kevin), superseding the 2026-09-22 answer: keep both, but a person pair carries them in the panel, not the row.** The row keeps `· {variation}` only and stays one 40 px line; every non-person pair keeps its label in the row, since the panel is person-only (RD10). The first answer put them in the row against a mockup that drew them fitting — measured at real type sizes they need ~836 px and wrapped every row onto a second line, and the probe found 100 % of the queue carries one. They still explain why the *detector* flagged the pair, so they now open the panel that holds the evidence. [HOLODEX-453](https://whoiskevinrich.atlassian.net/browse/HOLODEX-453) may change what gets flagged; if the queue ever becomes mixed, revisit whether the scan line needs the signal back. | — | Closed |
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
