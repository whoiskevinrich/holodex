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

All four pre-implementation gates are green; implementation is underway. Slice order follows the
spec's own Timeline section. **Next: #4, the enrich write path** — its two guards (suppression,
collision) ship in the same commit as the writer, since reviewing a writer with no brakes is how
one gets approved.

1. [x] [gate] spec — done 2026-09-02, `docs/specs/provider-alias-collapse.md`
2. [x] [gate] `/testing-strategy` — done 2026-09-02
3. [x] [backend] migration 0044 — done 2026-09-02 (P0-1; ADR-088 D2/D4/D6)
4. [x] [backend] enrich write path + both guards — done 2026-09-03 (P0-2/P0-4/P0-5)
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
- skills: architecture, design-handoff, simplify
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
- handoff: **all four pre-implementation gates green** — implementation can start at Up-next #3
  (the migration). The PR **stays Draft**: ADR-069 gates ready-for-review on the whole routing
  table, and backend, frontend, and `/security-review` are still open. Marking it ready now would
  fire the Jira `In Review` transition against a docs-only branch.

### 2026-09-02 · P0-1 migration 0044 landed
- skills: simplify
- `0044_alias_source_and_suppressions.{up,down}.sql` + `internal/db/alias_collapse_test.go`
  (3 tests). Full `go test ./...` green; `openAt`/`mustExec`/`count` reused from `fold_test.go`
  exactly as the test plan predicted, so no new harness.
- **`/simplify`'s altitude pass earned its keep.** 0022 states the invariant *"an alias never
  outlives its entity"* and — because `entity_aliases` is polymorphic, so no FK can express it —
  enforces it with three `AFTER DELETE` triggers on `people`/`studios`/`tags`.
  `entity_alias_suppressions` has the identical shape and I had skipped them, leaving rows that
  nothing would ever prune. Added the three matching triggers.
- That fix carried a second, sharper one: the triggers live on `people`/`studios`/`tags`, **not**
  on the dropped table, so `DROP TABLE` does not remove them — without an explicit `DROP TRIGGER`
  the down migration leaves them pointing at a missing table and the next `DELETE FROM people`
  fails. The down test now deletes a person after migrating down; I verified that assertion is
  non-vacuous by removing the `DROP TRIGGER` lines and watching it fail.
- Not overstated: all three entity tables use `AUTOINCREMENT`, so a deleted id is never reissued
  and an orphaned suppression could not have been inherited by a later entity. The fix is about
  unreachable rows accumulating and about the invariant holding uniformly, not a live data bug.
- Two `created_at` assertions in the test plan were wrong — `entity_aliases` has no such column.
  Corrected in `docs/testing-strategy.md` rather than left to mislead the next slice.
- handoff: next is Up-next #4, the enrich write path. Its two guards (suppression skip, collision →
  review queue) belong in the same commit as the writer.

### 2026-09-03 · P0-2/P0-4/P0-5 enrich write path landed
- skills: simplify
- `repo.ApplyProviderAliases` (`internal/repo/provider_aliases.go`) + the suppression write inside
  `DeleteEntityAlias`, wired into `runEnrich` next to the asset download and sharing its
  best-effort posture. 8 repo tests + 2 enrich tests; full `go test ./...` green.
- **The payoff test is a pair and must stay one**: `TestApplyProviderAliases_LiveOnArrival` asserts
  a provider name both finds the person in search *and* routes a new file to them on scan. Either
  half alone is the original bug in a new table.
- **A guard the spec did not name, found while reading `review_queue.go`**: `FlagNearMiss` checks
  `entity_keep_separate` before queueing. A provider-alias collision must too, or every re-enrich
  re-proposes a pair the owner already dismissed — F43 RD5's "a kept-separate pair never nags"
  applied to a source that repeats on a schedule. `TestProviderAliasCollisionRespectsKeepSeparate`.
- `/simplify` reuse pass: my `aliasHolder` had re-implemented `EntityConflict`'s UNION just to run
  it on a `*sql.Tx`. `identity.go` already defines `queryRower` for exactly that ("the read slice
  both `*sql.Tx` and `*sql.DB` satisfy"), so the query is now one shared `entityConflict` and the
  public method is a wrapper.
- Test-quality fix worth noting: the suppression test's "owner can re-add a suppressed name"
  assertion originally contradicted its own comment — it asserted a *failure*, caused by an earlier
  step having given the name to another person. Rewritten so each asymmetry owns its own name.
- Gotcha for future sessions: a Python round-trip over a source file writes CRLF on Windows, which
  `gofmt -l` then flags in a repo whose `.gitattributes` sets `eol=lf`. Normalize with a binary
  read/write. (Three other files in `internal/` carry the same pre-existing drift; not touched.)
- handoff: next is Up-next #5 — delete the `aliases` FieldDef and add the synthetic completeness
  facet, together, so the completeness denominator never shifts mid-branch. That slice is where
  `TestEnrichmentShadowStore` and `TestPersonFields_Synthesis` get rewritten (and
  `TestServiceResolveEnrichClear`'s `got["aliases"]` assertion, which still passes today).
