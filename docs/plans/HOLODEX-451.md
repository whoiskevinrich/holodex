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

## Open question blocking the design

**How many queued person pairs have a headshot on both sides?** Both sides → a symmetric
compare panel. Mostly one side only → the empty frame is itself the primary signal (the thin,
imageless, un-enriched half is the junk record) and the panel must be designed around
asymmetry, which is a materially different layout. `scripts/detect_person_duplicate_evidence.sql`
answers it; Kevin runs it on the host.

Also open: one image per side, or a short strip — the provider headshot is not always the
recognisable one.

## Gates — definition of done

- [ ] spec `write-spec` — blocked on the probe numbers (symmetric vs asymmetric panel)
- [ ] architecture `architecture` — only if the panel needs a new endpoint rather than widening `/owner/duplicates`
- [ ] design `design-handoff` — committed SVG mockup next to the handoff doc
- [ ] backend — evidence fields on the duplicates read
- [ ] frontend — expand-to-compare panel in `DuplicatePairRow`
- [ ] testing `testing-strategy`
- [ ] security `security-review` — owner-gated surface; re-confirm the existing gate covers the new fields
- [ ] `code-review high --fix`

## Up next — ordered (position = priority)

1. [ ] [Kevin] Run `scripts/detect_person_duplicate_evidence.sql` on the host, paste the output
2. [ ] [—] Decide symmetric vs asymmetric panel from section 1 of the probe, then `/write-spec`
3. [ ] [—] `/design-handoff` with a committed SVG mockup once the panel shape is settled
4. [ ] [—] Decide one image per side vs a short strip
5. [ ] [—] Mark the PR ready only once every gate above is green

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-22 · brainstormed the gap, ruled out the hover card, shipped the probe
- skills: product-brainstorming
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
- handoff: probe is on the branch and waiting on Kevin's host run; section 1 (headshot coverage
  per pair) decides whether the compare panel is symmetric or asymmetric, and the spec is
  blocked until that number exists.
