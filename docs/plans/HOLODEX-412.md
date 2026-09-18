---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-412
status: in-progress
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
- [ ] backend — migration 0048 (tables + triggers), `resolver.Complete` Required/Extras, drain + upsert, mark-all on boot/reload/promotions/claims, SQL sort, owner-only `completeness` on list items, detail self-heal, registry demotions
- [ ] frontend — ring badge on VideoCard / people / studio cards (owner only), panel shows `extras`
- [ ] testing `testing-strategy` — trigger-coverage enumerating test, formula tables, sort composite, visitor redaction
- [ ] security `security-review` — owner gating of the new list field; trigger migration

## Up next — ordered (position = priority)

1. [x] [—] Draft PR #350 open with the ADR + this worklog; Gate-status checkboxes mirrored from HOLODEX-412
2. [x] [—] `/write-spec` amendment — demoted list ruled: video 8 (six TMDB-shaped + director + runtime), person nationality + alternate_names, studio description + country
3. [x] [—] `/design-handoff` — ring badge SVG + three-skin QA checklist
4. [ ] [—] Backend per ADR-099 action items 2–6, then frontend
5. [ ] [—] On ready: this is an epic-keyed branch — CI fires nothing; move HOLODEX-412 to In Review by hand, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 → 2026-09-18 · brainstorm → epic → ADR
- skills: product-brainstorming, architecture, write-spec, design-handoff
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
