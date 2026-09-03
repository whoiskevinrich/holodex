# Spec: Collapse provider aliases into the canonical alias spine (F58)

**Status**: Draft
**Phase**: New epic (Jira [HOLODEX-306](https://whoiskevinrich.atlassian.net/browse/HOLODEX-306))
**Owner**: Project owner
**Date**: 2026-09-02
**Feature block**: **F58** — a person (and studio) has exactly one set of alternate names. Values a
metadata provider supplies as `also_known_as` stop being a display-only field and become real rows
in `entity_aliases`, carrying a `source` and behaving from the moment they arrive exactly like a
name the owner typed: searchable, and load-bearing for scan routing.

**Depends on** (all shipped):
- [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md) / [person-aliases.md](person-aliases.md)
  (F23) — the original alias-as-routing-rule model, `entity_aliases_fts`, and the
  name → alias → create resolution order the scanner walks
- [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md) / [entity-identity.md](entity-identity.md)
  (F43) — the polymorphic `entity_aliases` spine, the **global** `UNIQUE (entity_type, alias_key)`
  (RD1: one alias key belongs to exactly one entity), the per-entity `nameKey` fold, and
  `identity_review_queue` / `entity_keep_separate`
- [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) / [metadata-plugins.md](metadata-plugins.md)
  (F22) — `entity_enrichment`, the shadow store a provider value arrives through, and the enrich
  apply path that writes it
- [ADR-081](../architecture/ADR-081-entity-completeness-score.md) / [entity-completeness-score.md](entity-completeness-score.md)
  (F55) — the scored-facet model and `injectAssetFacet`, the existing seam for a facet the resolver
  does not produce
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

**ADR**: **[ADR-088](../architecture/ADR-088-provider-alias-collapse.md) (Proposed)** records the
collapse as D1–D7: `aliases` leaves the field registry; `entity_aliases` gains a `source` column
(provenance, not privilege); a provider alias is fully live on arrival; deleting one records a
per-entity suppression; a collision against another entity's key skips and enqueues for review;
the migration promotes existing curation rather than dropping it; completeness re-homes to a
synthetic facet. Touches the **enrichment write path** (a provider value now creates identity rows
that route scans) → `/security-review` before merge.

**Design handoff**: [alias-collapse-handoff.md](../design/alias-collapse-handoff.md) — the
"Also known as" curation row is deleted from the person page; `AliasPanel` gains a source badge on
provider-sourced chips, widened subcopy, and a skipped-collision review line. Everything else in
the panel is untouched. Mockup: [alias-collapse-mockup.svg](../design/alias-collapse-mockup.svg).

**Test plan**: [testing-strategy.md](../testing-strategy.md) §4 (five backend rows), §5 (two
frontend rows), and three new entries under *Critical invariants*.

---

## Problem Statement

A person has alternate names in two unrelated places, and only one of them works.

| | Mechanism A — registry field | Mechanism B — identity spine |
|---|---|---|
| Store | `entity_enrichment` (shadow) | `entity_aliases` |
| Source | provider `also_known_as` | owner typing, rename, merge |
| Curation | `metadata_curation` add/suppress | add/delete rows |
| Searchable | **no** | yes |
| Routes scans | **no** | yes |
| UI | "Also known as" row | Aliases panel |

Both render on the person detail page within ~200px of each other, each labelled as alternate
names. The page's own source comments acknowledge the split as deliberate, but the distinction is
invisible to anyone who has not read them. The observable behaviour is that Holodex knows a person
is also called 宮崎駿, displays that fact, and then fails to find them when you search it — and a
file tagged with that name creates a second, duplicate person on the next scan.

ADR-036 anticipated the fix and deferred it, reserving room to "later promote a provider's
`aliases` field into real `person_aliases` rows — a clean future bridge, not a merge of two
concepts." That bridge was never built, so the provider half has sat inert since F22. This spec
builds it, and in doing so removes the second mechanism rather than connecting the two.

## Goals

1. One store for alternate names. `aliases` leaves the resolver registry entirely; provider values
   are written onward into `entity_aliases` as real rows carrying a `source`.
2. A provider-supplied name finds the entity in search **and** routes a file tagged with it on the
   next scan — the same two behaviours an owner-typed alias has always had, with no confirmation
   step and no second tier.
3. Deleting an alias is durable: a re-enrich never resurrects a name the owner removed.
4. A provider name already held by a different entity never merges the two and never fails the
   enrichment — it is skipped and surfaced for the owner to decide, through the near-miss review
   surface that already exists.
5. Owner intent already expressed against the old display-only row survives the change, upgraded
   to a real alias rather than discarded.

## Non-Goals

1. **Tag aliases are out of scope.** Tag has no `AliasPanel` (F43 RD7) and no provider alias
   source. `entity_aliases` is polymorphic, so nothing structurally excludes tag — this spec
   simply does not add a surface for it.
2. **No change to how an owner adds, deletes, or merges aliases.** The panel's add form, merge
   button, homonym `MergeOfferCard`, and near-miss nudge are untouched. This spec changes what can
   *create* an alias row, not what an alias row is or how it is managed.
3. **No matched-alias annotation in search results.** Making provider names findable is this spec;
   showing *which* alias matched a result remains HOLODEX-81.
4. **No new review surface.** Collisions ride `identity_review_queue`, which already exists and
   already has an owner-facing review flow. A dedicated "provider alias inbox" is not built.
5. **No bulk alias management across entities.** One entity at a time, through its own panel.
6. **`metadata_curation` is not removed.** Its `field_key='aliases'` rows are promoted and deleted,
   but the mechanism stays for every other merge field, and the person page's `mergeFields` loop
   stays with it (see Behavior detail).

## Resolved Decisions

*(RD1–RD4 and RD7–RD8 were locked with the owner during the ADR-088 session, 2026-09-02. RD5 and
RD6 were locked via question cards while writing this spec, 2026-09-02.)*

**RD1 — One store, not a bridge between two.** The owner rejected a two-tier design mid-session:
*"I'm looking to collapse the aliases on both the backend and the front end. Aliases should be
attached to the canonical person entity, enrich from enrichment providers, and fully searchable."*
Considered and rejected: a "suggested from TMDB" zone in the panel with per-chip promote/dismiss.
That is honest about the distinction and lower-risk, but it *keeps* both mechanisms and adds a
third state to model, render, and re-suppress on every enrich — it makes the duplication legible
rather than removing it.

**RD2 — A provider alias is fully live on arrival.** The owner chose this over a
searchable-but-not-routing variant carrying a `confirmed` flag. Searchable and scan-routing
immediately, indistinguishable in behaviour from an owner-typed alias. The consequence is
deliberate and is the point: after enriching a person, a file tagged `H. Miyazaki` routes to them
on the next scan. **This widens scan routing, not just search** — the load-bearing consequence for
whoever implements, and the reason RD3 and RD4 are not optional.

**RD3 — Deleting a provider alias suppresses it permanently.** The `×` on a provider chip writes
an `entity_alias_suppressions` row keyed `(entity_type, entity_id, alias_key)`; the enrich path
skips any suppressed candidate for that entity. Deleting an *owner-authored* alias writes nothing —
nothing would re-add it. The suppression stands even if the owner later re-types the name by hand:
the standing suppression gates the enrich path only, never the owner, and leaving it in place means
a name the owner once removed can never come back uninvited.

Considered and rejected: a soft-deleted tombstone row inside `entity_aliases`. It would hold the
globally-unique `alias_key` hostage and block a legitimate claim by another entity.

**RD4 — A collision skips, enqueues, and never fails the enrichment.** `UNIQUE (entity_type,
alias_key)` is global per entity type (F43 RD1), so a name another entity already holds cannot be
inserted. Enrich skips that one candidate, records the pair in `identity_review_queue` with
`variation='provider-alias'`, and completes normally. *Enrich completing* is as load-bearing as the
skip: one awkward AKA must never cost an entity its bio, birthdate, and photo.

**RD5 — Provider input is additive; a name the provider later drops is kept.** *(Card, 2026-09-02.)*
On re-enrich, an alias the provider no longer lists stays in the spine. Considered and rejected:
mirroring the provider's current list, so a dropped name disappears. That keeps the store honest to
the source, but a routine re-enrich could then silently stop routing files that were matching on
that name, with no owner action and no record of it. Only two things ever remove an alias: the
owner, and a merge. This also matches how enrichment behaves everywhere else in Holodex — the
shadow layer is additive and never destructive.

**RD6 — Import every provider AKA except near-duplicates of the canonical name.**
*(Card, 2026-09-02.)* A candidate is skipped when, after stripping every non-alphanumeric character
and lowercasing, it is identical to the entity's canonical name treated the same way — so
`Hayao-Miyazaki` and `Hayao  Miyazaki` are dropped against a canonical `Hayao Miyazaki`, while
`H. Miyazaki`, `Miyazaki, Hayao`, and `宮崎駿` are all kept. This is a deliberately **narrow**
filter: it extends the existing "skip the entity's own `nameKey`" rule by one step and removes
names that add routing surface without adding reach. It is not general deduplication and should
not be built into one — see P2-2.

Considered and rejected: a hard cap (import the first N, report the rest). The provider's ordering
is not meaningful, so which names survived a cap would be effectively arbitrary.

**RD7 — Existing curation is promoted, not dropped.** `metadata_curation` rows with
`field_key='aliases'` carry real owner intent. The migration converts `add` actions to owner alias
rows and `suppress` actions to suppression rows, then deletes them. An owner who curated the old
display-only row keeps the result, now searchable.

**RD8 — Entity-generic, covering person and studio.** `entity_aliases` is polymorphic and
`AliasPanel` is already reused verbatim on studio, so the same behaviour lands on both surfaces
from one implementation. Studio picks up provider aliases the moment a provider supplies studio
alternate names, whether or not one does today; the badge, suppression, and collision paths are
identical either way. Not tag (Non-Goal 1).

## User Stories

- As the owner, I want searching a person's Japanese name to find them, when Holodex is already
  displaying that name on their page — today it displays it and then can't find it.
- As the owner, I want a file tagged with a person's alternate spelling to attach to the person I
  already have, instead of quietly creating a second one on the next scan.
- As the owner, I want to see at a glance which alternate names came from a provider and which I
  typed, without those being two different kinds of thing I have to manage differently.
- As the owner, I want removing a wrong name a provider supplied to be final — if the next enrich
  puts it back, deleting it was pointless.
- As the owner, when a provider hands Holodex a name that already belongs to someone else, I want
  to be told and to decide, not to have two people silently merged or my enrichment fail.

## Requirements

### Must-have (P0)

- **P0-1**: Migration adds `entity_aliases.source TEXT NOT NULL DEFAULT ''`, creates
  `entity_alias_suppressions (entity_type, entity_id, alias_key)` with that composite primary key,
  and promotes `metadata_curation` `field_key='aliases'` rows per RD7 before deleting them
  (ADR-088 D2/D4/D6). Acceptance: an existing `add` curation row for a person becomes an
  `entity_aliases` row with `source=''` that the person's own search now matches; an existing
  `suppress` row becomes a suppression row; pre-existing alias rows are byte-identical afterwards.
- **P0-2**: The enrich apply path writes provider `aliases[]` into `entity_aliases` with
  `source='<namespace>'`, skipping the entity's own `nameKey`, RD6 near-duplicates, and any key
  suppressed for that entity (ADR-088 D3). Acceptance: enriching a person from a provider that
  returns three AKAs produces three alias rows carrying the provider namespace; re-enriching
  produces no additional rows.
- **P0-3**: **The payoff, asserted as a pair.** After P0-2, the provider name (a) surfaces the
  person via the existing alias search path, and (b) routes a file tagged with it to that existing
  person on scan rather than creating a second one. Acceptance: both halves hold in one test; either
  passing alone means the collapse shipped the original bug in a new table.
- **P0-4**: Deleting a provider-sourced alias writes a suppression row; a subsequent full enrich
  does not re-add it (RD3). Acceptance: delete → re-enrich → the alias is still absent. Deleting an
  owner-authored alias writes no suppression.
- **P0-5**: A candidate whose `alias_key` belongs to a different entity is skipped, recorded in
  `identity_review_queue` with `variation='provider-alias'` and ordered `id_lo`/`id_hi`, and the
  enrichment completes (RD4). Acceptance: the entity's other enriched fields all land in the same
  run; the alias is absent; no merge occurred. A candidate colliding with the *same* entity's
  existing alias is a plain no-op with no queue row.
- **P0-6**: `aliases` stops being a resolved field anywhere (ADR-088 D1). Acceptance: a person's
  `resolved[]` contains no `aliases` entry. **Amended during implementation** — the registry was
  one of four places that had to change, and deleting it alone does not satisfy the acceptance
  criterion, it only changes how the row arrives:
  - the `aliases` `FieldDef` leaves `internal/registry/registry.go`;
  - the hardcoded `personFields` synthesis stops emitting it (it never read the registry, so the
    field survived the FieldDef's removal — and it was the person's only merge field, see
    Behavior detail);
  - the block leaves `metadata-mappings.yaml.example`;
  - **the enrich path stops storing the key in `entity_enrichment`.** This is the load-bearing
    one. A stored row does not disappear when its canonical is retired — it is *demoted*, and F39
    auto-registration then renders the now non-canonical key as a display-only "Aliases" row. The
    second list would have survived the collapse, arriving through a different door.
- **P0-6b**: A one-time backfill promotes alternate names already sitting in `entity_enrichment`
  into the spine and deletes those rows, so the acceptance criterion holds for an existing library
  and not only a fresh one. Runs at boot beside the other one-time backfills, gated on a job-run
  marker, and promotes through the same guards as P0-2 (RD6, suppressions, collisions) so old data
  is treated exactly like new. Acceptance: after upgrading a library whose people carry stored
  provider aliases, those names are in `entity_aliases`, the shadow rows are gone, and no
  `aliases` row renders.
- **P0-7**: Completeness gains a synthetic alias facet resolved by querying `entity_aliases`
  directly, replacing the scored facet P0-6 removes (ADR-088 D7, mirroring the studio
  `branding_image` injection). Acceptance: an entity with ≥1 alias of any source scores the facet
  as present; an entity with none scores it missing; a person whose aliases are all
  provider-sourced scores identically to one who typed them.
- **P0-8**: `EntityAlias` carries `Source` on the model and detail reads, and the person and studio
  detail payloads carry `skipped_aliases` for the collision review line. Both owner-gated — the
  key is absent for a visitor, not null or empty, so a visitor never learns a collision exists.
  **Needed one more migration than planned**: the skipped name lives nowhere after the enrich pass
  ends (P0-6 stopped storing provider aliases in the shadow layer) and `identity_review_queue`
  recorded only the pair, so it gained a free-text `detail` column. `skipped_aliases` is then
  derived from the queue rather than stored per-entity, which means resolving the pair — by merge,
  keep-separate, or any other queue action — clears the review line with no extra bookkeeping.
- **P0-9**: The "Also known as" `mergeFields` block is removed from the person detail page.
  Acceptance: no `aliases` row renders; the `mergeFields` loop itself remains (Non-Goal 6).
- **P0-10**: `AliasPanel` renders a source badge on provider-sourced chips, the widened subcopy,
  and the collision review line, per the design handoff — QA'd across all three skins with computed
  contrast checks on the badge.

### Should-have (P1)

- **P1-1**: The review line's `Review` link routes to the existing near-miss review queue filtered
  to this entity. If that filter does not already exist, an unfiltered link to the queue is
  acceptable for v1 rather than building one.
- **P1-2**: Surface the provider namespace on the alias row in the API response as a stable
  identifier alongside the display label, so a future consumer does not have to parse the badge.

### Future considerations (P2)

- **P2-1**: Tag aliases, if a provider alias source for tags ever exists (Non-Goal 1).
- **P2-2**: Near-duplicate detection *between* aliases, not just against the canonical name —
  `Miyazaki, Hayao` and `Miyazaki Hayao` can both be imported today under RD6. Deliberately not
  built now: the fold that would catch them is the tag-style one, and applying it to people would
  contradict F43's per-entity scoping (`"Mary Jane"` ≢ `"MaryJane"` for a person).
- **P2-3**: Matched-alias annotation in search results (HOLODEX-81, Non-Goal 3).

## Behavior detail

- **The `alias_key` comparison is not the RD6 comparison.** `alias_key` is a stored generated
  column — `lower(trim(alias))` for person and studio — and stays exactly as F43 defined it. RD6's
  punctuation-and-spacing fold is an **import-time filter only**, computed in Go, never stored and
  never used for uniqueness. Conflating the two would change what collides, which is an F43
  decision this spec does not reopen.
- **`aliases` was never eligible for a source decision.** It is a `multi:true` merge field, and
  `field_source_decisions` is replace-fields-only (rejected in both `person_decisions.go` and
  `decisions.go`). Its curation lived in `metadata_curation`, which is why RD7 is a
  `metadata_curation` migration and not a `field_source_decisions` one.
- **The `mergeFields` loop stays but goes dead in the default configuration.** `aliases` was the
  person entity's only merge field. Removing it leaves the loop rendering nothing unless an
  operator creates one, which they can: promoting a provider key with the `chips` renderer makes a
  merge field (`field_promotions.go`, "chips ⇒ merge field"). Deleting the loop would be a wider
  change than this spec needs and would have to be undone the moment someone does.
- **Two person-side guards become unreachable in the default configuration, and both stay.**
  `personReplaceField`'s "source decisions apply to replace fields only" branch can no longer fire
  for a person, because `personFields` now yields no `Multi` entry and a promoted field is rejected
  as out-of-schema before that check. The identical branch on the video and studio handlers is
  unaffected and still covered. The guards are structural — the field set could gain a merge entry
  again — so they are kept and the person-side test asserts the reachable behaviour (the 404)
  instead of faking a merge field to reach a branch no request can.
- **Upgrade note for operators.** A live `metadata-mappings.yaml` is gitignored and untouched by
  this change. An operator whose file still carries the `aliases` block will keep seeing a
  display-only "Aliases" row, now non-canonical and unscored, until they delete it. The committed
  `.example` carries this warning in place of the block. The one place we *can* clean up
  automatically — stored shadow rows — is handled by P0-6b.
- **A merge still wins over everything here.** Merging B into A registers B's name as an alias on A
  (F43 RD6). Nothing in this spec changes that, and a provider alias is subject to it like any
  other row — the merge path is not made source-aware.
- **Suppression is per-entity, never global.** Suppressing a name on person 12 leaves person 40
  free to receive or add it. This is the reason RD3 chose a separate table over a tombstone.

## API

- **Existing, unchanged**: `POST /people/{id}/aliases`, `DELETE /people/{id}/aliases/{aliasId}`,
  `POST /people/{id}/merge`, and their studio equivalents. The delete handler gains the RD3
  suppression write as a side effect; its request and response shapes do not change.
- **Extended**: `GET /people/{id}` and `GET /studios/{id}` — each alias in `aliases[]` gains
  `source` (empty for owner-authored), and the payload gains `skipped_aliases` for the collision
  review line. Both are additive; a client ignoring them behaves as it does today.
- **Removed**: nothing. `aliases` disappearing from `resolved[]` is a consequence of P0-6, not a
  route change — the person curation endpoints stay for the other merge fields.

## UI

See the [design handoff](../design/alias-collapse-handoff.md) for full detail. Summary: the
"Also known as" curation row is deleted; the Aliases panel becomes the only list of alternate
names. Provider-sourced chips gain a small text badge between the label and the `×` — a plain
span, deliberately **not** `ProvenanceBadge`, which expands to a source breakdown that has no
meaning for a value with exactly one origin. Chips stay in one case-insensitive list with provider
and owner names mixed; grouping them would re-introduce the two-tier reading this spec removes. A
collision review line renders above the add form when `skipped_aliases` is non-empty, inside the
panel's existing `aria-live` region.

## Success Metrics

Single-owner internal tool — no adoption metrics apply. Success is binary and checkable on a real
library: after enriching a person who has a non-Latin or alternate-spelling AKA, searching that
name finds them, and a file tagged with it attaches to them instead of creating a duplicate.

## Open Questions

*(Resolved by [ADR-088](../architecture/ADR-088-provider-alias-collapse.md): the store, the
provenance column, the live-on-arrival posture, the suppression mechanism, the collision
disposition, the migration's treatment of existing curation, and where the lost completeness facet
goes. Four rejected alternatives are recorded there so they are not re-proposed.)*

*(Resolved by the [design handoff](../design/alias-collapse-handoff.md): chip anatomy, the badge's
component choice, ordering, the review line's copy and placement, the six rendering states, and the
three-skin QA requirement.)*

*(Resolved by question cards while writing this spec, 2026-09-02: RD5 — a name the provider drops
is kept, not removed; RD6 — import every AKA except punctuation/spacing near-duplicates of the
canonical name.)*

**One item deliberately left unanswerable by any test**, recorded in
[testing-strategy.md](../testing-strategy.md) §11: RD2's risk that a junk provider AKA claims files
cannot be closed by a fixture, because no fixture proves a real library's provider AKAs are sane.
It is a post-deploy observation item on the first enrichment sweep, bounded by RD3 and RD4, and
accepted as such rather than treated as covered.

No open questions remain before implementation.

## Timeline / routing

No hard deadline. Per this repo's change-routing rules, the pre-implementation gates are
`/architecture` ([ADR-088](../architecture/ADR-088-provider-alias-collapse.md), green),
`/design-handoff` ([handoff](../design/alias-collapse-handoff.md) + committed SVG, green),
`/testing-strategy` ([testing-strategy.md](../testing-strategy.md), green), and this spec — all on
a Draft PR per [ADR-069](../architecture/ADR-069-draft-prs-for-pre-implementation-gates.md).
`/security-review` follows the enrich write path landing, and `/simplify` runs before each commit;
the PR is marked ready for review only once both are green.

Suggested slice order: P0-1 (migration) first as the standalone schema change, then P0-2/P0-4/P0-5
(the enrich write path and its two guards) as one slice since the guards are meaningless without
the writer, then P0-6/P0-7 (registry removal and the facet that replaces it) together so the
completeness denominator never shifts mid-branch, then P0-8/P0-9/P0-10 (the surface).
