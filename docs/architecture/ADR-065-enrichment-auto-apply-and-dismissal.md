# ADR-065: Enrichment auto-apply-with-revert — provider-contract posture change + durable `enrichment_dismissals` store

**Status:** Proposed
**Date:** 2026-07-12
**Deciders:** Project owner

**Extends:** [ADR-033](ADR-033-metadata-source-plugins.md) (metadata source plugins — the
`entity_enrichment` shadow store and the [provider HTTP contract](../specs/metadata-provider-contract.md)
this ADR amends at §2.3) · [ADR-052](ADR-052-baseline-source-contract.md)/[ADR-051](ADR-051-per-field-source-of-truth-decisions.md)
(entity-generic resolver + per-field decision model — auto-apply writes through both unchanged; Keep/Revert
is the entire safety net, no new undo mechanism). **Relates to:** [ADR-055](ADR-055-enrichment-unique-key-invariant.md)
(the namespaced `external_id` this ADR's dismissals key on, and that Refresh/Refresh-all read directly) ·
[ADR-061](ADR-061-unified-entity-name-identity.md) (the closest structural precedent: a durable
**dismissal**/keep-separate store paired with a review queue, same shape as this ADR's
`enrichment_dismissals`). **Spec:** [enrichment-review-workflow.md](../specs/enrichment-review-workflow.md)
(F47). **Issue:** [HOLODEX-186](https://whoiskevinrich.atlassian.net/browse/HOLODEX-186).

---

## Context

Today, resolving a candidate for a Person/Studio/Media entity always routes through
`EnrichPicker`: `/enrich/resolve` returns candidates, the owner clicks one, `/enrich` applies it.
The [metadata provider contract](../specs/metadata-provider-contract.md) states this as policy —
"Holodex always shows the owner a picker and never auto-applies a candidate in v1, so `confidence`
is advisory" (§2.3) — and the frontend already computes a `confidence >= 0.85` **"Strong match"**
label (`EnrichPicker.matchLabel`) that today does nothing but color a badge.

F47 ([enrichment-review-workflow.md](../specs/enrichment-review-workflow.md)) turns that dormant
signal into a trigger: opening a queue row (or a detail-page picker) whose resolve returns
**exactly one** strong-confidence candidate applies it immediately, no confirm click. Two facts
push this from "just wire up the threshold" to something worth its own ADR:

1. **It reverses a documented contract posture.** §2.3's "never auto-applies... in v1" is not
   incidental prose — it is the stated v1 behavior every provider author reads. Silently starting
   to auto-apply would be an undocumented breaking change to what "Holodex's behavior" means from
   a provider's point of view, even though the wire format (`confidence`'s shape) is untouched.
2. **It surfaces a state the system has never had to store.** Telling the picker "none of these
   are right" today produces no durable effect — the same wrong candidates resurface on every
   future resolve. A queue-driven workflow makes that loss visible immediately (the row just
   re-prompts forever), so F47 needs a new persisted "not matched" verdict, not just a UI tweak.

### Forces

- **Speed for the common case, without a new safety mechanism.** The owner's stated goal (F47
  Goal 1) is one click for an unambiguous match — but the fix must not require a second undo
  system. [ADR-051](ADR-051-per-field-source-of-truth-decisions.md)'s per-field decision model
  (Keep/Revert, source-pinning) already treats every field value — however it arrived — uniformly;
  an auto-applied value must be **indistinguishable, after the fact, from a manually-confirmed
  one**, so it inherits Keep/Revert for free.
- **The threshold must not invent a new confidence scheme.** [ADR-055](ADR-055-enrichment-unique-key-invariant.md)
  and the provider contract are explicit that `confidence` stays provider-native, non-normalized,
  advisory. Auto-apply cannot require providers to emit a calibrated score — it must reuse
  whatever bar the frontend already draws.
- **Ambiguity must never resolve itself.** Auto-apply is a affirmative trust decision for the
  *single-strong-match* case specifically; two-or-more strong candidates, or only possible/weak
  ones, carry real judgment and must still stop at the owner, exactly as today.
- **A rejection needs to be as durable as an acceptance.** The `entity_enrichment` shadow store
  already gives an *accepted* candidate permanence; there is no symmetric record for a *rejected*
  one. Without it, "not matched" is not a workflow feature — it is a slower way to see the same
  candidates twice.
- **Dismissal cannot silently suppress interest in an entity.** The owner still wants to *try
  again* later (a provider's catalog changes, or the owner mis-clicked); the record must be an
  explicit, reversible verdict — not a value in `entity_enrichment` (which means "linked") and not
  a soft-delete.
- **This is a contract change, not just an implementation detail.** [metadata-provider-contract.md](../specs/metadata-provider-contract.md)
  is a spec other provider authors (including the `providers/tmdb` sidecar and any future
  provider) read as ground truth. A behavior change to how Holodex *consumes* `confidence` belongs
  in that document, mirroring how [ADR-039](ADR-039-provider-asset-urls.md)/[ADR-055](ADR-055-enrichment-unique-key-invariant.md)
  each amended the same contract for their own posture changes.

---

## Decision

Two sub-decisions, scoped exactly to what the F47 spec's Resolved Decisions (RD1/RD4) require and
no further — Refresh/Refresh-all (RD7/RD8) are **not** architectural changes: they call the
existing `apply()` path with an `external_id` already on hand, so no new decision is needed there.

### D1 — Auto-apply is a threshold-gated *routing* change, not a new confidence model

A `/resolve` call that returns **exactly one** candidate with `confidence >= 0.85` (the existing
`EnrichPicker.matchLabel` "Strong match" cutoff, unchanged and un-recalibrated) routes straight to
the existing `apply()` call — the same HTTP call and the same code path a manual pick already
takes, just triggered by the resolver instead of a click. Any other outcome (0, 2+ strong, or only
possible/weak candidates) is unchanged: it stops at the picker for the owner to confirm.

This amends [metadata-provider-contract.md](../specs/metadata-provider-contract.md) §2.3 from
*"Holodex always shows the owner a picker and never auto-applies... `confidence` is advisory"* to
document the threshold-gated auto-apply behavior. The amendment is **documentation of client
behavior only** — `candidates[].confidence`'s wire shape, range, and provider-native/non-normalized
semantics are unchanged, so this does **not** warrant a `protocol_version` bump (closes spec Open
Question Q4). A provider that already emits a well-calibrated `confidence` (as the contract has
always recommended) needs no change at all; a provider whose scores cluster near 0.85 for
genuinely-ambiguous matches now has a behavioral incentive to be conservative, which the contract
should say explicitly.

Auto-apply carries **no new undo mechanism.** It writes through `entity_enrichment` and the F36
decision tables exactly as a manual `apply()` does — the row it produces is indistinguishable from
a manually-confirmed one, so Keep/Revert ([ADR-051](ADR-051-per-field-source-of-truth-decisions.md))
covers it without modification. This is the load-bearing safety property: speed (no confirm click)
costs nothing in reversibility (revert is exactly as easy as reverting any other field decision).

### D2 — `enrichment_dismissals`: a durable, per-`(entity, provider)` "not matched" verdict

A new table records an explicit owner rejection, symmetric to how `entity_enrichment` records an
explicit owner acceptance:

```sql
enrichment_dismissals
  entity_type    TEXT      -- 'person' | 'studio' | 'video'
  entity_id      INTEGER
  provider       TEXT
  dismissed_at   DATETIME
  PRIMARY KEY (entity_type, entity_id, provider)
```

A dismissal excludes that `(entity, provider)` pair from "needs review" state and from the queue's
zero-cost membership count (spec RD2), and blocks `/resolve` from being called again for that pair
until the owner explicitly clears it ("Try again" — a `DELETE` on the same row). No TTL/expiry: the
spec's non-goal is deliberate (avoiding any background re-check polling), and matches this
workflow's on-demand-only posture ([ADR-033](ADR-033-metadata-source-plugins.md)'s "no scheduled
enrichment" decision, which this ADR does not revisit).

This is the direct structural sibling of [ADR-061](ADR-061-unified-entity-name-identity.md)'s
`entity_keep_separate` table: both are a **persisted negative assertion** ("don't ask again about
this pair") that exists solely to stop a review workflow from re-nagging. `enrichment_dismissals`
is scoped `(entity_type, entity_id, provider)` rather than `entity_keep_separate`'s `(id_lo,
id_hi)` because the thing being rejected is a *provider's candidate set for this entity*, not a
relationship between two entities — the shapes differ because the questions differ, not because
the pattern does.

**"No data yet" needs no row of its own** (spec RD5): it is the absence of both an
`entity_enrichment` row and a dismissal for that pair, distinguished purely by whether an owner
verdict exists. This ADR does not add an attempt-log or "last checked" timestamp (deferred, spec
P2-2/Open Question Q2) — adding one later is additive (a new nullable column or a separate table),
not a migration of this one.

---

## Options Considered

### D1 — where the auto-apply threshold lives

#### A — Reuse the existing frontend `matchLabel` cutoff, server-triggered (chosen)

**Pros:** Zero new configuration surface; the 0.85 bar is already shipped, understood, and visible
to the owner as "Strong match" today — this ADR just wires an existing signal to an existing
action. No coordination burden on provider authors (the contract already asks for
well-calibrated confidence). **Cons:** A single global threshold cannot account for a provider
whose scoring is systematically more/less conservative than another's. Accepted: per-provider
calibration is explicitly out of scope (spec Non-Goals — "cross-provider confidence calibration");
revisit only if a real provider's scores prove miscalibrated in practice.

#### B — Per-provider configurable threshold (operator setting)

**Pros:** Lets an operator tune trust per provider. **Cons:** A new settings surface
([ADR-060](ADR-060-runtime-owner-settings.md)-shaped) for a problem with no evidence yet — no
provider has shown miscalibration, and the contract already asks for calibrated scores as a
baseline expectation. Rejected as premature: adds machinery ahead of a demonstrated need, the
project's stated anti-pattern for "flexibility that wasn't requested."

#### C — Confidence-weighted partial auto-apply (apply high-confidence fields, review the rest)

**Pros:** Finer-grained trust. **Cons:** `confidence` is a per-*candidate* identity-match score
(§2.3), not a per-*field* score — there is no wire-format signal to weight individual fields by.
Would require a new contract field every provider must emit. Rejected: solves a problem the
contract doesn't have data for, and blurs the clean "one identity decision, then all its fields
apply together" model every other enrich path already uses.

### D2 — dismissal storage shape

#### A — Dedicated `enrichment_dismissals` table, mirrors `entity_keep_separate` (chosen)

**Pros:** Matches the existing pattern for "durable negative assertion" ([ADR-061](ADR-061-unified-entity-name-identity.md));
trivial to query for the queue's exclusion filter; a `DELETE` is the entire "Try again" mechanic.
**Cons:** One more entity-typed table in the growing family (`entity_enrichment`,
`entity_aliases`, `entity_keep_separate`, now this). Accepted: the family already establishes this
as the idiom for polymorphic per-entity-type state — adding one more instance is consistent, not
novel design.

#### B — A `dismissed` boolean/state column on `entity_enrichment`

**Pros:** No new table. **Cons:** `entity_enrichment` rows mean "a provider's data is *stored* for
this field" — overloading it to also mean "the owner *rejected* this provider" conflates two
opposite states in one table, and a dismissal has no field-level data to store (there was no
accepted candidate). Rejected: breaks the shadow store's single meaning and complicates every
existing reader that assumes a row implies enriched data.

#### C — Reuse `entity_keep_separate` directly (generalize its `(id_lo, id_hi)` shape)

**Pros:** Zero new tables. **Cons:** A dismissal isn't a relationship between two *entities of the
same kind* — it's a relationship between one entity and one *provider*, a different arity and a
different key shape. Forcing it into `(id_lo, id_hi)` would require encoding "provider" as a
synthetic pseudo-entity-id, an awkward hack for a one-column saving. Rejected: the pattern
(durable negative assertion) is worth reusing; the schema is not.

---

## Trade-off Analysis

**Documenting the posture change vs. silently shipping it.** The contract explicitly states
"never auto-applies... in v1" as a promise to provider authors. The alternative to amending §2.3
is to ship the behavior and let the doc go stale — cheaper today, but it is exactly the kind of
drift [ADR-039](ADR-039-provider-asset-urls.md) and [ADR-055](ADR-055-enrichment-unique-key-invariant.md)
both treated as worth an ADR + spec amendment rather than a silent code change. The contract is
the interface a third-party provider author reads without seeing Holodex's source; a v1→v1.1
behavior note costs one paragraph and keeps that document trustworthy.

**No new undo system vs. a dedicated "enrichment history."** A tempting alternative to "reuse F36
Keep/Revert" is a dedicated auto-apply audit log (what was auto-applied, when, easily bulk-revertable).
Rejected because the F36 decision model already gives per-field revert with provenance
(`file` / `provider:<name>` / `manual`) — an auto-applied value's provenance is simply
`provider:<name>`, identical to a manual pick. A second history mechanism would duplicate that
without adding a capability the owner asked for (spec Non-Goals is explicit on this point); it's
the "three similar lines vs. a premature abstraction" call, applied to a subsystem instead of a
function.

**A dismissal table vs. teaching `entity_enrichment` to mean "tried and failed."** The overloading
option (D2-B) saves a table but corrupts a well-understood invariant — every existing reader of
`entity_enrichment` (resolver, writeback, the F36 decision layer) assumes a row means "this
provider's data is available to resolve." Making some rows mean "explicitly rejected, no data"
would require every one of those readers to filter on a new discriminator column, whereas a
disjoint table requires none of them to change at all. The one-table cost of D2-A is strictly
smaller than the blast radius of D2-B.

---

## Consequences

**What becomes easier**
- The dormant `confidence >= 0.85` signal now does real work instead of only coloring a badge —
  the "one obvious match" case collapses from a search-and-confirm to a single click, matching
  F47 Goal 1.
- A rejected candidate set stops resurfacing; the owner's judgment is now durable, matching F47
  Goal 3.
- Both new mechanisms plug into existing, proven patterns (F36 decisions; the
  `entity_keep_separate`-shaped durable-negative-assertion idiom) rather than inventing new ones —
  the review-queue implementation (F47 S1–S4) has no new undo or persistence design to do, only
  wiring.

**What becomes harder**
- The provider contract now carries an implicit trust requirement: a provider whose `confidence`
  is uncalibrated (e.g. always reports 0.9) will cause incorrect auto-applies that the owner must
  catch via revert rather than a confirm click. This is a real behavior change for provider
  authors, which is exactly why it is documented in the contract amendment rather than left
  implicit.
- One more entity-typed table to keep in the family's migration/backfill/cleanup discipline
  (delete-cascade on entity removal, mirroring how `entity_enrichment`/`entity_aliases` are
  cleaned up today).

**What we'll need to revisit**
- **Per-provider threshold tuning** (D1-Option B) — only if a real provider's confidence proves
  miscalibrated in practice; no evidence yet, explicitly deferred.
- **"Last checked" annotation** on the no-data-yet state (spec P2-2) — would need a new
  attempt-log write on every resolve, independent of this ADR's tables; revisit only if owners
  report confusion between "never tried" and "tried, found nothing" (spec Open Question Q2).
- **Auto-resurfacing / TTL for dismissals** (spec P2-4) — deliberately out of scope to avoid
  reintroducing background polling; revisit only if "Try again" proves too manual in practice.

---

## Action Items

1. [ ] Add this ADR to `docs/architecture/README.md`.
2. [ ] `docs/specs/metadata-provider-contract.md` §2.3: replace the "never auto-applies... in v1"
   language with the threshold-gated auto-apply behavior (D1); note explicitly that
   `candidates[].confidence`'s wire shape is unchanged (closes spec Open Question Q4) — F47 slice
   S5.
3. [ ] Migration `NNNN_enrichment_dismissals.up.sql` (+ matching `.down.sql`): the table in D2,
   entity-delete cleanup wired the same way `entity_enrichment`/`entity_aliases` are today — F47
   slice S1.
4. [ ] `internal/enrich` (or `internal/api`): the auto-apply routing check (single strong
   candidate → `apply()` directly) sits at the same call site `/resolve` responses are already
   inspected, ahead of returning candidates to the client — F47 slice S3.
5. [ ] Owner-gated (`requireOwner`, [ADR-030](ADR-030-access-control-gating-seam.md)) dismiss/undismiss
   endpoints per entity type, mirroring the existing per-entity-type enrich route shape
   (`/people|studios|media/{id}/enrich/{provider}/dismiss`) — F47 slice S1.
6. [ ] `/design-handoff` (spec-tracked, not this ADR): the queue's "not matched"/"Try again"
   affordance and the auto-applied row state.
7. [ ] `/testing-strategy` (spec-tracked): auto-apply threshold boundary (exactly one strong vs.
   two-or-more), revert-after-auto-apply parity with manual apply, dismissal persistence + the
   "does not re-trigger `/resolve`" invariant, dismissal cleanup on entity delete.
8. [ ] `/security-review` before merge: confirm the new dismiss/undismiss endpoints are
   `requireOwner`-gated like every existing enrich mutation; no new externally-influenced input
   here (`profile_url` scheme validation, the spec's one new untrusted-input surface, is P1-1 and
   orthogonal to this ADR's two decisions).
