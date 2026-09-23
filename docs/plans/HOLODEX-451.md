---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-451
status: in-progress
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

- [/] spec `write-spec` — F70, `docs/specs/duplicates-pair-evidence.md`
- [~] architecture `architecture` — n/a (Kevin, 2026-09-22): no endpoint, no migration, no cross-cutting decision; OQ1 closed without forcing one
- [ ] design `design-handoff` — committed SVG mockup next to the handoff doc
- [~] backend — n/a for P0: the panel composes `GET /people/{id}/card` (F68) + `GET /people/{id}/images` (F26), both existing. P0 touches no Go code
- [ ] frontend — disclosure in `DuplicatePairRow` + new `DuplicateComparePanel.svelte`; verdict emphasis swap
- [ ] testing `testing-strategy`
- [ ] security `security-review` — owner-gated surface; re-confirm the existing gate covers the new fields
- [ ] `code-review high --fix`

## Up next — ordered (position = priority)

1. [ ] [—] `/design-handoff` with a committed SVG mockup next to the handoff doc — answers OQ2 (`+N` in place vs navigate), OQ3 (keep the weak-signal labels?) and OQ4 (strip cap of 5)
2. [ ] [—] `/implement` — puts the mockup in front of Kevin, records the sign-off, opens the draft PR
3. [ ] [—] Build the frontend (P0 is frontend-only), then testing and security
4. [ ] [—] Mark the PR ready only once every gate is green

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-22 · brainstormed the gap, ruled out the hover card, shipped the probe
- skills: product-brainstorming, implement, write-spec
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
