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

- [ ] spec `write-spec` → behaviour is fully described in ADR-088 + handoff; a spec entry still
      owed under `docs/specs/` (or an amendment to the F43 identity spec) before ready-for-review
- [x] architecture `architecture` → `docs/architecture/ADR-088-provider-alias-collapse.md`
- [x] design `design-handoff` → `docs/design/alias-collapse-handoff.md` + committed SVG
- [ ] backend
- [ ] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 five backend rows, §5 two frontend
      rows, three Critical invariants, one Known Gaps bullet)
- [ ] security `security-review` → docs-only so far; required once the enrich write path lands

## Up next — ordered (position = priority)

1. [ ] [gate] spec entry for the collapse (or amend the F43 identity spec) — **the last open
       pre-implementation gate**; nothing else may start until it lands (ADR-069)
2. [x] [gate] `/testing-strategy` — done 2026-09-02
3. [ ] [backend] migration: `entity_aliases.source`, `entity_alias_suppressions`, promote
       `metadata_curation` `field_key='aliases'` rows (ADR-088 D2/D4/D6)
4. [ ] [backend] enrich apply writes provider aliases; skip own nameKey + suppressed; collision →
       `identity_review_queue` `variation='provider-alias'` (D3/D5)
5. [ ] [backend] delete `aliases` FieldDef + `metadata-mappings.yaml.example` block; synthetic
       completeness facet mirroring studio `branding_image` (D1/D7)
6. [ ] [backend] `EntityAlias.Source` on the model + detail read; `skipped_aliases` on the person
       and studio detail payloads
7. [ ] [frontend] remove the "Also known as" `mergeFields` block from the person page
       (`+page.svelte:656-677`)
8. [ ] [frontend] `AliasPanel` source badge, widened subcopy, collision review line — then QA all
       three skins with computed-contrast checks on the badge

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
