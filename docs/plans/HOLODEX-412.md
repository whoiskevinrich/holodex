---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-412
status: in-review
release_note: The completeness score now measures the required facets alone (extras are a separate number that overfills the ring), and every entity card shows a completeness ring in owner mode.
---

# HOLODEX-412 · Completeness score v2 (F65)

The F55 score let 12 nice-to-haves out-vote 4 critical facets (all-required-curated = 50,
poster-missing = 88) and blended provenance into doneness. v2: `required` (binary presence over
critical facets) *is* the score; `extras` is separate and only overfills the ring once required is
full; the score is materialized per entity with trigger-fed invalidation so a ring can sit on every
owner-mode card. ADR: [ADR-099](../architecture/ADR-099-completeness-score-required-band.md).
Spec: [entity-completeness-score.md](../specs/entity-completeness-score.md) (F55, amended in place).

## Gates — definition of done

- [x] spec `write-spec` — F55 spec amended in place (F65 RD1–RD8, F65.1–7, v2 § Scoring model + facet tables, worked examples); demoted list ruled 2026-09-18
- [x] architecture `architecture` — ADR-099 (supersedes ADR-081 D3 + D4): required band, extras, `entity_completeness` + `_missing` + `completeness_dirty` with triggers, lazy drain on owner reads
- [x] design `design-handoff` — `docs/design/completeness-ring-badge-handoff.md` + `-mockup.svg`: card = bottom-left chip with Part N shifting right (HOLODEX-389 had taken the corner), rows = trailing before the count, `CompletenessRing` props/markup, numbered three-skin QA
- [x] backend — migration 0049 (tables + triggers, incl. promotions/claims/hints in SQL), `resolver.Complete` Required/Extras, `repo.DrainCompleteness` under writeMu + `StoreCompleteness` self-heal, mark-all on boot/reload, SQL composite sort + missing-facet predicate, owner-only `completeness` on list items, registry demotions; 2026-09-18
- [x] frontend — `CompletenessRing` + `ring.ts` (unit-tested), VideoCard bottom-left chip beside Part, `/people` + `/studios` rows before the count, panel headline `score` + `extras` (studio → extras is the number), `types.ts` v2 shapes; agent QA 2.1–2.5 green on all three skins; 2026-09-18
- [x] testing `testing-strategy` — F65 rows (resolver bands, store triggers/repo/list surfaces, ring badge) + six invariants in `docs/testing-strategy.md`; tests already shipped with the code; 2026-09-18
- [x] security `security-review` — clean 2026-09-18: entityType SQL concat is constants-only, sort whitelisted, missing_facet bound, drain/attach/self-heal all inside the owner check, facets endpoint in the requireOwner group, visitor items omit the key

## Up next — ordered (position = priority)

1. [x] [—] Draft PR #350 open with the ADR + this worklog; Gate-status checkboxes mirrored from HOLODEX-412
2. [x] [—] `/write-spec` amendment — demoted list ruled: video 8 (six TMDB-shaped + director + runtime), person nationality + alternate_names, studio description + country
3. [x] [—] `/design-handoff` — ring badge SVG + three-skin QA checklist
4. [x] [—] Backend per ADR-099 action items 2–6 (2026-09-18)
5. [x] [—] Frontend ring badge per the design handoff (2026-09-18); Kevin's eyeball items 3.1–3.4 + 1.2 (needs a multi-part video) still open on the prod skin
6. [x] [—] `/testing-strategy` F65 block + `/security-review` (2026-09-18)
7. [x] [—] Spec nit: F65.3 reworded to "offers at most" (2026-09-18)
8. [x] [—] Marked ready + HOLODEX-412 → In Review by hand 2026-09-18 (epic-keyed branch, CI fires nothing)
9. [ ] [—] On merge: move HOLODEX-412 to Done by hand; Kevin's eyeball QA 3.1–3.4 + 1.2 on the prod skin

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 → 2026-09-18 · brainstorm → epic → ADR
- skills: product-brainstorming, architecture, write-spec, design-handoff, code-review, testing-strategy, security-review
- handoff: decisions locked via cards (required-only + extras overfill, binary tier, O2 ring
  always-on in owner mode, materialize, registry-level Optional); HOLODEX-412 filed + In Progress;
  branch renamed `HOLODEX-412-completeness-score-v2` and fast-forwarded to main; ADR-099 written;
  Draft PR #350 open.
  Trigger-fed dirty set was the ADR's own call (answers ADR-081 D3's staleness objection) — Kevin
  chose "materialize", not the mechanism. Spec amended in place the same day (F65.1–7, RD1–RD8);
  studio ring is now branding-only 0/100 and person ring photo-only 0/100 — both accepted by Kevin.
  Design handoff landed the same day: bottom-left was NOT free (Part N badge, HOLODEX-389) so the
  ring chip goes first and Part shifts right; people/studios are ROW lists, ring goes before the
  count. All three pre-implementation gates green — next is the backend (ADR-099 action items 2–6).

### 2026-09-18 · backend
- skills: code-review high --fix
- handoff: backend gate green, all Go tests pass. Migration 0049 = three tables + 20 input-table
  trigger sets; `INSERT … WHERE NOT EXISTS` not `OR IGNORE` (SQLite lets the firing statement's
  conflict clause override the trigger's — an upsert turned OR IGNORE into an abort, caught by the
  existing repo suite); `videos` update trigger is scoped `OF title, active, deleted_at` + a WHEN
  clause because UpsertVideo rewrites every row every scan. Promotions/claims/hints mark-all is done
  in SQL triggers rather than the ADR's four Go hooks (same effect, covers future writers); boot +
  mapping reload are the two Go mark-alls. Code review found and fixed a lost-update race: the
  drain now lives in `repo.DrainCompleteness` under writeMu (compute callback), and the detail
  self-heal writes the row but never clears the dirty flag. `ListPeople/ListStudios(bool)` kept as
  wrappers over `*Filtered(NamedListFilter)` (50 test call sites). Next: frontend ring badge.

### 2026-09-18 · frontend
- skills: code-review high --fix (no findings)
- handoff: ring badge shipped per the handoff — `completeness/CompletenessRing.svelte` over a pure
  `ring.ts` (reading + arc, 6 vitest cases), VideoCard bottom-left flex row (ring chip first, Part
  shifts right; chip needed `inline-flex` or it inherited the line box and ran 30 px tall), people +
  studio rows trailing before the count, panel headline = required with `extras N%` beside it and
  extras standing in when required is null. Live QA on backend-films: tokens/overfill/radii/level
  bottom edge/8-col gap/visitor-zero all green on all three skins; row heights unchanged. Not
  exercised: a multi-part card (no `part` video in the fixture) — human 1.2 + 3.x remain. Next:
  `/testing-strategy` F65 block + `/security-review`, then mark ready.

### 2026-09-18 · gates → ready
- skills: testing-strategy, security-review
- handoff: all seven gates green. Testing-strategy F65 rows + invariants landed (the "never blended",
  "null not 100", "migration is the list", "drain under writeMu / self-heal never clears", "NOT EXISTS
  not OR IGNORE", "visitor has no key" rules); security review clean. F65.3 reworded to "offers at
  most". PR #350 marked ready, HOLODEX-412 → In Review by hand. Open: Kevin's eyeball QA on the prod
  skin (handoff 3.1–3.4, 1.2 needs a multi-part video) and the by-hand Done sweep on merge.

### 2026-09-18 · post-ready code review (xhigh)
- skills: code-review xhigh --fix
- handoff: nine findings, seven fixed, one no-change, one skipped (the bool list wrappers). Fixed:
  drain failure no longer 500s the owner list pages (logs, serves the store as-is, ids stay dirty);
  `/people` Poster View card was unwired — ring now rides the caption's count line under the row
  rule (handoff § People Poster View + QA 3.5, live-verified 53/53 cards); link triggers flag the
  person/studio side too (listability, not facets — ADR-099 implementation note); shadow-table
  triggers skip unscored entity types; chunked DELETEs in the drain; stale v1 doc comments; a
  visitor-never-drains test. PR #350 stays ready; nothing else open on the branch.
