# Field resolution

This doc owns how an entity's displayed field values are produced: the canonical field registry and
operator mapping, the pure resolver that merges every layer, the owner-authored stores it reads
(source decisions, promotions, claims), computed fields, and how the completeness score is stored and
kept fresh. Where provider values come from belongs to [metadata-providers.md](metadata-providers.md);
how a decided value reaches the file belongs to [writeback.md](writeback.md). What the completeness
score counts is a product rule in the [completeness spec](../specs/entity-completeness-score.md).

## Capture every file tag; map tags to canonical fields in `metadata-mappings.yaml`

The scanner stores every human-meaningful container-level tag of a video in `video_metadata`
(`video_id`, `source_key`, `value`, indexed on `(source_key, value)`), mapped or not, so a mapping
change re-interprets stored rows and never needs a rescan. `metadata-mappings.yaml` (gitignored;
`.example` committed) maps one or more case-insensitive source keys to one canonical field with a
label and `filterable` / `multi` / `merge` flags. A source is bare (a file tag) or namespaced
(`file:`, `filename:`, `<provider>:`). `filterable` fields become browse facets and API query
parameters. The mapping governs video only; person, studio and film field sets are synthesized in
code. Per-field facts that are code rather than deployment config (label, display, `Computed`,
`Criticality`, `OfferWhenEmpty`) live on `registry.FieldDef` in `internal/registry`.

**Rejected:** a JSON column of tags — a key/value table keeps facet queries index-friendly.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## A pure resolver over a `BaselineSource` is the sole merge point

`internal/resolver` is the only place a resolved field is built. `ResolveFields(baseline, enrichment,
curation, fields, opts)` is the entity-agnostic core; `Resolve` is a thin wrapper over
`NewVideoBaseline`. Every input is pre-loaded (baseline, enrichment shadow, value curation,
decisions); the resolver does no I/O and reads no clock. Enrichment is an additive shadow, never
flattened into the baseline, so every layer stays recoverable.

`BaselineSource.Baseline(src)` returns `(vals, true)` for a source in the entity's own baseline
namespace, even when empty, so a baseline source never falls through to a provider, and `(nil,
false)` otherwise. The baseline, not the resolver, owns which namespace is intrinsic: the file layer
for video, the record for person, studio and film. The internal token stays `file` for all of them;
the person/studio API presents it as `record`. A new entity type joins by implementing one
`BaselineSource`.

**Rejected:** passing the baseline as raw video-shaped data — it bakes the video shape into the core.

Decided in [`8e8884dc`](https://github.com/whoiskevinrich/holodex/commit/8e8884dc).

## Standing per-field source decisions in `field_source_decisions`, file-first by default

A decision is `(entity_type, entity_id, field_key) → source`, `source ∈ {file, provider:<name>,
manual}` (grammar in `internal/fieldsource`), one row per decided field, `manual_value` used only by
`manual`. Set is an upsert; clear is a delete back to the default.

- **Pin the source, not the value.** `file` and `provider:<name>` follow the live layer, so a rescan
  or re-enrich flows through; only `manual` is a frozen literal.
- **Replace fields only.** Merge fields (`multi`/`merge`) resolve as a deduplicated union and are
  curated per value in `metadata_curation`; there is no winner.
- **Undecided default.** Under `default_source: file` (the default, `holodex.yaml`) the baseline
  wins and providers are candidates; `provider_trust_order` ranks providers among themselves;
  `default_source: mapping` restores first-configured-source-wins.
- One decision drives display and writeback; `ResolvedField` carries the marker, candidates and
  `in_sync`.

**Rejected:** a value-less pin row in `metadata_curation` — that table is keyed by normalized value.

Decided in [`12a984b6`](https://github.com/whoiskevinrich/holodex/commit/12a984b6).

## A cleared field is a `manual` decision with an empty value

"No value" is stored as `source = manual`, `manual_value = ''`; grammar, schema and resolver are
unchanged (a standing decision keeps the empty field in `resolved[]`). The decision API writes it only
on an explicit `clear: true`, only for the `clearableFields` allowlist in `internal/api/decisions.go`
(replace fields that are neither identity keys nor images, each admitted with a real-file tag-delete
test per container: `studio`, `edition`). F76's label-as-editions writes the same cleared decision
inside its own transaction. An empty `manual` without `clear` is still
refused, so `manual` + `''` always means cleared; the SPA tests it through one helper, `isCleared`.

**Rejected:** a new `none` source — "no value" is a value, and a new source widens every switch on it.

Decided in [`4e2e782b`](https://github.com/whoiskevinrich/holodex/commit/4e2e782b).

## A sole linked film is the undecided winner for its fields

An attached film reaches the resolver as a synthetic `film:<id>` namespace (how it is injected is
[entity-relationships.md](entity-relationships.md)). With no standing decision and exactly one
`film:<id>` offering a value, `soleFilmNamespace` puts it first inside the normal
`resolvePrecedence` walk, so manual adds and suppressions still apply. Several films are ambiguous
and nothing auto-wins; any standing decision beats the film. A default film winner reports a real
`in_sync` against the file tag.

**Rejected:** writing a decision at attach time — it outlives a detach where a read-time rule suffices.

Decided in [`ceac6253`](https://github.com/whoiskevinrich/holodex/commit/ceac6253).

## Empty replace fields are offered through the resolver, per registry flag

A replace field with no value, decision or film candidate is dropped from `resolved[]` unless
`resolver.Options.Offer(canonical)` is true; then it keeps the ordinary undecided shape (empty values,
a `file` candidate, a non-standing decision). The API builds `Offer` from
`registry.FieldDef.OfferWhenEmpty` and passes it only for owner requests. Merge, multi, entity-link
and identity fields are never offered. The client never synthesizes a row the server offers.

**Rejected:** building the empty row in the page — a second row builder with no write target.

Decided in [`e375c173`](https://github.com/whoiskevinrich/holodex/commit/e375c173).

## Adoption gates the shadow store; precedence decides over it

Two mechanisms, never merged. **Adoption** decides whether a candidate enters `entity_enrichment`,
judged only against the entity's own baseline; its state is transient review rows and durable
dismissals, and it never stores or compares a competing provider value. **Precedence** is the
standing per-field decision over every stored namespace.

- **A new source is a namespace, not a subsystem.** A provider, filename extraction or future import
  writes `entity_enrichment` under its own namespace and becomes a candidate wherever the mapping
  lists it.
- **Adoption storage is generic only where the question is:** `enrichment_dismissals` is keyed
  `(entity_type, entity_id, provider)`; `metadata_extraction_review` is video-only.
- An adoption that enqueues a file write returns the job id, so callers wait on completion.
- Not covered: tags and categories, merge fields, entity-link fields, identity fields.

**Rejected:** one combined control — the two questions differ in lifetime and comparand.

Decided in [`4a96084d`](https://github.com/whoiskevinrich/holodex/commit/4a96084d).

## In-app promotion (`field_promotions`) is presentation tier 0

A `field_promotions` row (`PRIMARY KEY (entity_type, field_key)`; `label`, `render`, `hint_group`,
`ord`, empty = inherit) makes an auto-registered non-canonical key a curatable field. The app never
writes `metadata-mappings.yaml`. Presentation resolves top-down, first answer wins: (0)
`field_promotions`, non-canonical only; (1) operator mapping; (2) `registry.FieldDef`, sole authority
for canonical keys; (3) persisted provider hint (`provider_field_hints`), non-canonical only;
(4) title-case.

`mergePromotions` materializes each promotion as a synthetic `mapping.Field` (replace-or-append by
canonical) before resolve: `ParsedSources` is one `provider:<ns>` per namespace holding the key for
that entity, `Multi = render == "chips"`, `Filterable` always false. Canonical and `_`-prefixed keys
cannot be promoted. Decisions and curation are keyed by `field_key`, so they survive de-promotion.

**Rejected:** the app editing YAML — video-only, races hand edits, needs a reload.

Decided in [`7cdd9a7f`](https://github.com/whoiskevinrich/holodex/commit/7cdd9a7f).

## Claimed provider keys; auto-registration suppression derives from the merged field set

A canonical field claims a differently named provider key via an operator `sources:` entry or a
`field_claims` row (`PRIMARY KEY (entity_type, provider, field_key)`, plus `canonical`). Both
materialize as a `mapping.Source` appended to the target: `mergeClaims` runs after
`mergePromotions`, appends at lowest precedence sorted by `(provider, field_key)`, and skips (but
keeps) a claim whose target is absent. A claim carries no precedence and no presentation.

`resolver.ClaimedKeys(effective)` derives the claimed `provider:key` set from the merged fields, and
`AutoRegisterFields` suppresses a key that is claimed or that names a rendered canonical; both checks
are needed. Only namespaced provider sources claim. Because suppression reads materialized fields, a
key is never hidden without its value landing somewhere. A key is promoted or claimed, never both,
enforced at write time in one transaction.

**Rejected:** suppressing from `field_claims` rows directly — a dangling claim could hide a value.

Decided in [`4086013a`](https://github.com/whoiskevinrich/holodex/commit/4086013a).

## Computed fields are a pure `Derive(resolved, now)` post-pass; the clock enters at the handler

A computed field is a `registry.FieldDef` with `Computed: true` and `DependsOn`. A closed Go formula
registry (`internal/resolver/derive.go`) computes it from resolved values and inserts a display-only
row after its primary dependency, or no row if an input is missing. Nothing is stored. Rows carry
`WinningSource = computed:<canonical>` and no decision; `computed` stays out of
`fieldsource.Valid()` and the decision API rejects it, so a computed field can never be pinned. The
clock is the `Handlers.now` seam passed into `Derive`; a test asserts `internal/resolver` never calls
`time.Now`.

**Rejected:** storing derived values — stale once written. **Rejected:** a formula DSL.

Decided in [`cf8a42ce`](https://github.com/whoiskevinrich/holodex/commit/cf8a42ce).

## Completeness criticality is registry metadata; not-applicable is its own table

`registry.FieldDef.Criticality` (`critical`, `nice_to_have`, `optional`; empty = unscored) is
compiled in beside the field; `Computed` fields are never tagged. `resolver.Complete` is a pure
post-pass over `[]ResolvedField` reading presence and tier from `WinningSource`; its formula is a
spec rule. An owner's "does not apply here" is a value-less row in `facet_not_applicable` (`PRIMARY
KEY (entity_type, entity_id, canonical_field)`), checked before and independent of any decision.

**Rejected:** a weights file or table — criticality is code-reviewed field metadata.
**Rejected:** `source = 'not_applicable'` — it names no live source, which writeback relies on.

Decided in [`bf329409`](https://github.com/whoiskevinrich/holodex/commit/bf329409).

## Completeness is a materialized cache invalidated by SQL triggers into a dirty set

`entity_completeness` (nullable `required`, `extras`) and `entity_completeness_missing` cache
`resolver.Complete` per `(entity_type, entity_id)`, so list sort is SQL `ORDER BY` with paging and
facet counts are a `GROUP BY`. They are never a source of truth: the detail page computes live and
rewrites a differing row (`selfHealCompleteness`); the remediation queue scans live.

- **Invalidation.** `AFTER INSERT/UPDATE/DELETE` triggers on every input table insert the owner
  entity into `completeness_dirty` (`INSERT … WHERE NOT EXISTS`, not `OR IGNORE`); link tables flag
  both sides, film spelling inputs flag every linked video. A test enumerates input tables, so a new
  input cannot ship without a trigger.
- **Mark-all** on promotion, claim and hint writes (in SQL), on mapping reload, and at boot when the
  fingerprint below moves.
- **Drain** on owner-gated reads, under `writeMu`, best-effort. Visitors never drain, and the
  owner-only `completeness` list field is dropped by the existing visitor redaction seam.

**Rejected:** recompute at each Go write site — ~30 sites, no choke point. **Rejected:** recompute
in the trigger — duplicates the resolver in SQL. **Rejected:** a global generation counter.

Decided in [`e7e765ec`](https://github.com/whoiskevinrich/holodex/commit/e7e765ec).

## Boot re-scores completeness only when an input fingerprint moves, in the background

At boot (`cmd/holodex/completeness_boot.go`) a SHA-256 over the executable's bytes plus
length-prefixed `metadata-mappings.yaml` and `metadata-sources.yaml` is compared with
`settings['completeness.inputs_fingerprint']`. If it differs or is absent, `MarkAllCompletenessDirty`
then record it; if hashing fails, mark dirty and record nothing. It lives in the database so a
restored `/data` from another build re-scores. Before listening, `DrainCompletenessInBackground`
raises a flag; while it runs, request-path drains return immediately and serve the store as is.

**Rejected:** a hand-bumped score version — forgetting fails silently.

Decided in [`dbb4bfcd`](https://github.com/whoiskevinrich/holodex/commit/dbb4bfcd).
