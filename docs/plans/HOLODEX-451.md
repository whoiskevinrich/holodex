---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-451
status: in-progress
approved:
  design:
    on: 2026-09-23
    at: b340a94
release_note: A flagged duplicate pair in the owner's Duplicates queue can now be opened side by side — both headshots, counts, aliases and provider links at once — so you can tell one person from two without leaving the page.
---

# HOLODEX-451 · Duplicates queue: side-by-side evidence so a person pair is decidable

`/owner/duplicates?type=person` shows two names, two video counts, the variation kind and the
match kind. That is not enough to decide whether a flagged pair is one person or two: the owner
is the entire inference engine, every pair arrives at equal weight, and the row carries no
supporting evidence. This epic gives the row enough to decide on.

**Shape: expand-to-compare.** Clicking a pair expands it in place into a two-column panel
showing both entities at once — headshot, counts, aliases, attributed provider facts, external
link badges — with the Merge / Keep-separate verdicts at the bottom. Profile images are the
fastest discriminator for a human reader.

## Decisions — do not re-litigate

- **The F68 person hover card is not the primary treatment here.** It is a one-at-a-time
  discovery affordance being asked to do a comparison job: one card open app-wide by design
  (so comparison happens from memory), 250 ms hover intent twice per row on a batch surface,
  hidden under `pointer: coarse`, and the pair row's names are `<span>`s rather than `<a>`s so
  `PersonLinkChip` is not free to adopt. It belongs later as the escape hatch on a hard pair,
  once the names become links.
- **Positive evidence can be decisive; negative evidence never is.** Providers carry their own
  duplicate entries, so "different ids on the same provider" does not mean two people.
- **Facts are only comparable inside a provider namespace.** Cross-provider disagreement
  describes the providers, not the people — `Nepal` vs `Federal Democratic Republic of Nepal`
  is a country-name update, `1990-01-01` vs `1990-03-17` is a fidelity difference. The panel
  reports who said what and never adjudicates: **no conflict chip.**
- Nationality normalization, if any, reuses whatever `NationalityFlags` maps names to flags with.

## Structural finding

`entity_external_ids` has `PRIMARY KEY (entity_type, external_id)` (migration 0046), so two
people can never share a `provider:id` there by construction. A shared external id is only
observable via `entity_enrichment.external_id`, which is not globally unique. Any auto-merge
rule depends on that being non-zero in practice — currently unverified.

## Probe results — live library, 2026-09-22 (1591 people, 1000 enriched)

`scripts/detect_person_duplicate_evidence.sql`, run on the host. Both blocking questions closed:

- **Symmetric panel.** 29 of 31 pairs have a headshot on **both** sides, 2 have one side. The
  asymmetric layout is not needed.
- **A strip, not one image.** Most people carry 40–50 images, so a short strip per side is
  clearly affordable and more discriminating than a single provider headshot.

What the numbers changed:

- **The queue is 100% weakest-signal.** All 31 pairs are `provider-alias` / `alias` — no
  whitespace, no punctuation. The F43 fuzzy set has been worked through.
- **Keep separate is the dominant verdict**: 185 person pairs already dismissed against 31 open.
  The current row styles `Merge` as the accent primary and `Keep separate` as a ghost — **the UI
  is optimized backwards and the verdict buttons should swap emphasis.**
- **The auto-merge rule is dead.** `same_xid` is 0 across all 31 pairs; nothing shares an
  external id. The panel is evidence-for-a-human, never a verdict engine.
- **`diff_xid` is 2 for nearly every pair** — both providers hold different ids for the two
  sides. Under the pre-correction ranking this would have stamped a confident "different people"
  on essentially the whole queue from a signal that means nothing. The negative-evidence rule
  above is load-bearing, not pedantry.
- Facts lean "different people": `birthdate` same-provider differs on 25 pairs, matches on 1;
  `nationality` 18 differ / 11 exact / 6 one-contains-other (the Nepal case is real and
  measurable); 3 pairs co-appear in a video.
- **Link row must not lean on `_source_url`** — only 86 of 1591 people have it. `website` covers
  688; provider link templates carry the rest.

Spun out: **HOLODEX-452** (detect duplicates by shared provider external id — the probe found 6
such collisions in `entity_enrichment`, none of them ever queued, which the name-based detector
structurally cannot see) and **HOLODEX-453** (should alias-only provider-alias pairs be flagged
at all). Scope call 2026-09-22: **panel first, detector follow-up.**

## Gates — definition of done

- [x] spec `write-spec` — F70, `docs/specs/duplicates-pair-evidence.md`. The design's four deltas applied and approved by Kevin 2026-09-22 (no `+N`, Merge takes `btn-ghost` not `btn-quiet`, no fade, F68 `Videos`/`Films` anchors); OQ2/OQ4 struck through as closed. **OQ3 re-answered 2026-09-23** — the match-kind label moves into the panel on a person pair; two P0-1 criteria and RD10 updated with it
- [~] architecture `architecture` — n/a (Kevin, 2026-09-22): no endpoint, no migration, no cross-cutting decision; OQ1 closed without forcing one
- [x] design `design-handoff` — `docs/design/duplicates-pair-evidence-handoff.md` + committed `duplicates-pair-evidence-mockup.svg` (5 panels, Cinémathèque + a Brutalist radius-0 panel). **Amended 2026-09-23**: panel 1 redrawn at real type sizes as single-line 40px rows, panel 2's well opens with the label. **Signed off by Kevin 2026-09-23 at `b340a94`** (`approved.design` in the frontmatter)
- [~] backend — n/a for P0: the panel composes `GET /people/{id}/card` (F68) + `GET /people/{id}/images` (F26), both existing. P0 touches no Go code
- [ ] frontend — disclosure in `DuplicatePairRow` + new `DuplicateComparePanel.svelte`; verdict emphasis swap
- [ ] testing `testing-strategy`
- [ ] security `security-review` — owner-gated surface; re-confirm the existing gate covers the new fields
- [ ] `code-review high --fix`

## Up next — ordered (position = priority)

1. [ ] [—] Build the frontend (P0 is frontend-only) — the handoff's build checklist is the task list; note it also touches `+page.svelte` (first keyboard handling on that page) and the `.btn-ghost`/`.btn-accent` doc comments in `app.css`. **Three things the OQ3 reversal added:** drop `flex-wrap` from `DuplicatePairRow.svelte:77` (keep it on `:73`), gate the match-kind label at `:84–92` on `entity_type !== 'person'`, and export `matchKindLabel` so the panel reuses the strings rather than forking them
2. [ ] [—] Settle the `CardLease` question in code: does `release()` evict or refcount? P0-5 ("reopening issues no second request") depends on the answer
3. [ ] [—] Then testing and security; mark the PR ready only once every gate is green

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-23 · the sign-off was refused, the mockup was the reason, and the redraw carried it
- skills: implement, design-critique, design-handoff
- `/implement` put the design package up for sign-off. **Kevin pushed back** — "the collapsed
  queue has overlapping text" — so nothing was recorded: no `approved:`, no rebase, no PR.
- The overlap was real and measurable: the weak-signal label ran 28–32 px under the
  `Keep separate` pill in both person rows. But it was not a drawing slip. At the component's
  **real** type sizes (`text-sm` names, `text-xs` meta) that row needs **~836 px** to stay on
  one line, so `:77`'s `flex-wrap` wraps it — and the probe already said **100 % of the queue
  carries that label**, making the two-line row the *default*, not an edge case, on a surface
  built for scanning. The mockup had drawn the label overlapping the pill instead of wrapping,
  which is exactly why the cost was invisible when OQ3 was answered on 2026-09-22.
- **Kevin chose: move the label into the panel, keep rows 40px.** OQ3 reversed. It now opens
  the panel well above both columns — it describes *the pair*, so it can't sit in a column, and
  RD3/RD4 keep it out of the footer.
- Verifying that turned up a trap: `matchKindLabel` (`:84–92`) is **not** gated on entity type,
  and the panel is person-only (RD10), so a naive move would have deleted the label from
  studio/tag/film rows. Non-person rows keep it in the row — which also keeps P0-1's existing
  "byte-identical to today's" criterion true.
- Fixed two **pre-existing** defects in the same artifact while redrawing: panel 4's title ran
  145 px into panel 5's, and three captions (two of them untouched by this change) ran off the
  720 px canvas and were being clipped. Panel 1's group rect was also 18 px too short for its
  own tag row.
- Verified by measurement, not eyeball: 0 text collisions and 0 clipped captions across all 82
  text elements, SVG parses clean.
- Re-asked with the redraw rendered inline — rows back to one 40px line (widest run ends at
  x=489, the pill starts at x=540), the weak-signal label opening the panel's well, tag rows
  keeping their own. **Kevin approved.** `approved.design` recorded at `b340a94`, the rebase
  onto `0dfea65` (`1.16.1`) was clean, and the draft PR is open.
- handoff: **Crossed into build — design signed off at `b340a94`; draft PR open.** Start at
  `Up next` 1: build the frontend from the handoff's build checklist, and settle whether
  `CardLease.release()` evicts or refcounts before writing P0-5. Nothing in the design is open
  any more; the next gates are frontend, testing and security.

### 2026-09-22 · brainstormed the gap, ruled out the hover card, shipped the probe
- skills: product-brainstorming, implement, write-spec, design-handoff
- Kevin asked whether the new person hover card would fix the undecidable rows. Argued it
  would not — wrong shape of affordance for a comparison task — and put five treatments up as
  an inline mockup; he picked expand-to-compare on the strength of profile images. He then
  corrected two of my evidence claims (providers self-duplicate; cross-provider facts are not
  comparable), which demoted the whole auto-verdict line and produced the standing rules above.
  Chasing that down found a third error of my own: the `entity_external_ids` PK makes a shared
  external id impossible in that table, so the auto-merge rule I had ranked first is not even
  reachable there.
- Probe written, anonymized in the same shape as `detect_entity_collisions.sql`, and verified
  against a seeded fixture covering canonical/mixed/alias matches, both/one/neither headshots,
  same vs different external ids, and both cross-provider fact-disagreement shapes.
- Kevin ran the probe on the host. Both blocking questions closed (symmetric panel, image strip)
  and the queue turned out to be 100% weakest-signal with keep-separate as the dominant verdict.
  Spun out HOLODEX-452 and HOLODEX-453; he chose panel-first.
- handoff: nothing is blocked any more — start at `/write-spec` for the symmetric two-column
  panel with an image strip per side and the verdict emphasis swapped, then `/design-handoff`,
  then `/implement` to cross and open the draft PR.

### 2026-09-22 · design handoff — the column is the F68 card laid flat
- skills: design-handoff
- **The organising idea:** each column is `PersonHoverCard`'s anatomy, same fields in the same
  order with the same absent-is-absent rule, with a strip of five 44 px frames where its single
  48 px headshot was. That makes RD2 concrete — the hover card was never the wrong *content*,
  only the wrong *container*. Everything else in the panel falls out of reusing it.
- All three open questions closed by Kevin against an inline mockup: **OQ2 — there is no `+N`
  at all** (he rejected both the modal and the navigate; the strip is a sample, not an index),
  **OQ3 keep both match-kind labels** in the collapsed row only, **OQ4 five frames at 44 px**
  (`5 × 44 + 4 × 4 = 236` inside a 296 px column — measured, not guessed).
- Four design calls the spec didn't make: Merge takes **`.btn-ghost`, not `.btn-quiet`** (that
  class is documented as "no side effect" and Merge is irreversible); the link row **keeps F68's
  `Videos`/`Films` anchors**, which is what stops the panel being a navigational dead end now
  that `+N` is gone and the names are still `<span>`s; **each column is its own bordered card
  with no divider between them**, because `--surface-2` is ~2 % off `--surface` in Cinémathèque
  and the stacked layout would otherwise need a border-orientation flip `field-grid` can't
  signal; and **no summary line in the footer** — a first draft had one and it was exactly the
  cross-side adjudication RD3/RD4 forbid.
- Three things the exploration turned up that the spec had wrong or missing: **the row never
  fades** (`+page.svelte:47–49` is an unanimated filter; both the spec and the component's own
  header comment claim a fade), **`app.css` names these two buttons by name** in the
  `.btn-ghost`/`.btn-accent` doc comments so the swap makes them wrong, and **`api.videos({personId})`
  does not exist** — the P1-0 co-appearance call would be `api.listMedia({ person: [id] })`.
- Kevin approved the four spec deltas the same session; they are applied, OQ2/OQ3/OQ4 are struck
  through as closed, and RD7/RD12 reworded. The spec and the handoff no longer disagree.
- handoff: **every design-phase gate is green** (spec `[x]`, architecture `[~]` n/a, design
  `[x]`). Next is `/implement` — record the sign-off, rebase, push, open the draft PR. Then
  build straight from the handoff's build checklist; the first unknown to settle in code is
  whether `CardLease.release()` evicts or refcounts, because P0-5 depends on it.
