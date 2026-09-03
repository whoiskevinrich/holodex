---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-306
status: in-progress
depends-on: []
release_note: Alternate names from metadata providers now find a person in search and match them on scan, instead of only being displayed.
---

# HOLODEX-306 · Collapse provider aliases into the canonical entity_aliases spine

Done means: a person (and studio) has exactly one set of alternate names. Provider
`also_known_as` values land in `entity_aliases` as real rows carrying a `source`, are searchable
and scan-routing on arrival, and the display-only "Also known as" curation row is gone from the
person page.

**Design package:** [ADR-088](../architecture/ADR-088-provider-alias-collapse.md) · [handoff](../design/alias-collapse-handoff.md) · [mockup](../design/alias-collapse-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/provider-alias-collapse.md` (F58) — 8 resolved decisions,
      10 P0 requirements, sliced build order
- [x] architecture `architecture` → `docs/architecture/ADR-088-provider-alias-collapse.md`
- [x] design `design-handoff` → `docs/design/alias-collapse-handoff.md` + committed SVG
- [ ] backend
- [ ] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 five backend rows, §5 two frontend
      rows, three Critical invariants, one Known Gaps bullet)
- [ ] security `security-review` → docs-only so far; required once the enrich write path lands

## Up next — ordered (position = priority)

**All four pre-implementation gates are green — the PR can leave Draft and implementation can
start.** Slice order below follows the spec's own Timeline section.

1. [x] [gate] spec — done 2026-09-02, `docs/specs/provider-alias-collapse.md`
2. [x] [gate] `/testing-strategy` — done 2026-09-02
3. [ ] [backend] migration: `entity_aliases.source`, `entity_alias_suppressions`, promote
       `metadata_curation` `field_key='aliases'` rows (P0-1; ADR-088 D2/D4/D6)
4. [ ] [backend] enrich apply writes provider aliases; skip own nameKey, RD6 near-duplicates, and
       suppressed keys; collision → `identity_review_queue` `variation='provider-alias'`
       (P0-2/P0-4/P0-5 — one slice, the guards are meaningless without the writer)
5. [ ] [backend] delete `aliases` FieldDef + `metadata-mappings.yaml.example` block; synthetic
       completeness facet mirroring studio `branding_image` (P0-6/P0-7 — together, so the
       completeness denominator never shifts mid-branch)
6. [ ] [backend] `EntityAlias.Source` on the model + detail read; `skipped_aliases` on the person
       and studio detail payloads (P0-8)
7. [ ] [frontend] remove the "Also known as" `mergeFields` block from the person page
       (`+page.svelte:656-677`) — keep the loop itself (P0-9, Non-Goal 6)
8. [ ] [frontend] `AliasPanel` source badge, widened subcopy, collision review line — then QA all
       three skins with computed-contrast checks on the badge (P0-10)
9. [ ] [gate] `/security-review` once the enrich write path exists

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-02 · ADR-088 + design handoff landed; direction set to a full collapse
- skills: architecture, design-handoff
- The owner rejected a two-tier "suggested chips → promote" design mid-session and asked for a
  genuine collapse across backend and frontend, then chose **fully live on arrival** over a
  confirm-before-routing variant. Both rejected alternatives are recorded in ADR-088 so they are
  not re-proposed.
- The load-bearing detail for whoever implements: alias rows drive `resolveOrCreateByName`, so
  this widens **scan routing**, not just search. That consequence is the accepted risk of D3 and
  the reason D4 (suppression) and D5 (collision → review queue) are not optional.
- handoff: two of four pre-implementation gates are green (ADR, design). Next session should close
  the spec gate and run `/testing-strategy` before any code, per ADR-069.

### 2026-09-02 · testing gate closed
- skills: testing-strategy
- Surveyed existing coverage first, which changed two rows materially. The collapse has **no
  existing test pulling against it** — `TestEnrichmentShadowStore` already stores provider
  `aliases` in `entity_enrichment` and asserts the multi-value split — so the new tests are the
  specification, not regression cover. Two existing tests must be *rewritten*, and those edits are
  the D1 guard: `TestEnrichmentShadowStore` and `TestPersonFields_Synthesis` (which asserts
  `aliases` is the person merge field, false after D1).
- Two reusable precedents found, so no new machinery is needed: `openAt` +
  `TestMigration0022FoldsCaseDuplicates` (`internal/db/fold_test.go`) is the only other migration
  test asserting on transformed *data* and is a direct template for D6; the
  `injectAssetFacet`/`branding_image` quartet is the template for D7.
- The frontend row was written down rather than up: `web/package.json` has no
  `@testing-library/svelte`/`jsdom`, so **no Svelte component can be mounted at all** today. What
  is actually automatable is `addPersonAlias`/`deletePersonAlias` in `api.test.ts` plus Go API
  tests; the rest is a manual QA checklist, recorded as such instead of promised.
- handoff: three of four pre-implementation gates green. Only the spec entry remains — write it
  next, then the PR can leave Draft and implementation can start at Up-next #3.

### 2026-09-02 · spec landed — all four pre-implementation gates green
- skills: write-spec
- `docs/specs/provider-alias-collapse.md` (F58), a new spec rather than an amendment to the F43
  identity spec — this has its own ADR, epic, and gates, and folding it into a shipped spec would
  blur what F43 actually delivered.
- Two genuinely open questions went to the owner as cards, and both are now RDs the ADR did not
  cover. **RD5**: on re-enrich, a name the provider has *dropped* is **kept** — provider input is
  additive, and the rejected alternative (mirror the provider's current list) would let a routine
  re-enrich silently stop routing files with no owner action and no record. **RD6**: import every
  AKA **except** punctuation/spacing near-duplicates of the canonical name — a hard cap was
  rejected because the provider's ordering is not meaningful, so which names survived would be
  arbitrary.
- RD6's trap, now pinned in the test plan: its fold is an **import-time Go filter only** and must
  never touch the stored `alias_key` generated column. Conflating them would change what collides,
  reopening an F43 decision this epic explicitly does not touch. The test asserts the false-positive
  half (`H. Miyazaki`, `Miyazaki, Hayao`, `宮崎駿` all kept) harder than the true-positive half.
- ADR-088's `Spec:` line now points at the file and names RD5/RD6 as refinements of D3, since both
  post-date the ADR. The ADR itself is unchanged otherwise — it stays Proposed, not rewritten.
- handoff: **all four gates green.** PR #288 can be marked ready for review and implementation can
  start at Up-next #3 (the migration). `/security-review` is still owed once the enrich write path
  exists — it is gate #9, not a pre-implementation blocker.
