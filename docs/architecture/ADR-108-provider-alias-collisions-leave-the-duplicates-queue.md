# ADR-108: A provider-alias collision is recorded, not reviewed — it leaves the Duplicates queue

**Status:** Proposed (supersedes **ADR-088 D5's "enqueue for review" half only** — the skip and the
never-fail posture stand; relates ADR-061, ADR-107; spec F58 RD4; HOLODEX-453)

**Date:** 2026-09-25

## Context

ADR-088 D5: when a provider offers an alias that another entity already holds, enrich skips that one
name and inserts the pair into `identity_review_queue` as `variation='provider-alias'`, so the owner
decides through the Duplicates queue. The same row does a second job: it is the only record of the
skip, and `SkippedAliasesForEntity` (`internal/repo/provider_aliases.go`) derives the Aliases panel's
owner-only "N names were skipped" line from it.

In practice the queue half does not pay its way.

- **Every one of the 31 open person pairs on the live library (2026-09-22) was `provider-alias`**,
  and **185 person pairs had been dismissed keep-separate**. The dominant verdict on this stream is
  "not a duplicate".
- The owner, working the F70 compare panel (HOLODEX-451) on the live queue, has **not found one
  of these pairs that was actually the same person** (2026-09-25).
- The evidence the probe found points the same way: providers hold *different* external ids for the
  two sides on nearly every pair, birthdates differ on 25 and match on 1, and 3 pairs co-appear in a
  video.
- The strong case this stream was standing in for now has its own detector. When the provider's
  **id** — not just a name — lands on two entities, ADR-107's `shared-external-id` producer queues the
  pair, and its `ON CONFLICT DO UPDATE` **upgrades** an existing `provider-alias` row in place.

A queue whose entries are almost always dismissed trains the owner to stop working it, which is worse
than no queue.

**A correction to the ticket's framing.** HOLODEX-453 described these pairs as "alias-to-alias only"
(`match_kind = alias`). That label comes from the probe script
(`scripts/detect_person_duplicate_evidence.sql`), where `alias` is the `ELSE` fallthrough. A
provider-alias pair can never show a name collision there, because the refused name is by
construction never written to the refused entity. So the "31 of 31 alias-only" figure is an
artifact, and whether the holder held the name canonically or as an alias was never measured.
`entityConflict` does not record it either. This decision therefore covers **every**
`provider-alias` row, not an alias-only subset. The subset is not observable, and the owner's
experience covers the whole stream.

## Decision

### D1 — `ListReviewPairs` does not return `provider-alias` rows

The Duplicates queue — and everything counted from it — excludes `variation = 'provider-alias'`, for
every entity type (person and studio both produce it). One `WHERE` clause at the single read.

### D2 — The row is still written; it is the skip record

`queueProviderAliasPair` is unchanged: enrich still skips the name, still inserts the pair, still
honors `entity_keep_separate`, and still never fails (ADR-088 D5's other half, which stands). The row
remains the source of `SkippedAliasesForEntity`, so the Aliases panel keeps reporting the skip, and it
remains the row ADR-107 upgrades when a shared id later turns up. No migration, no backfill, no data
deleted.

### D3 — The Aliases panel line names and links the holder instead of linking the queue

The line's **Review → /owner/duplicates** link would now land on a queue that no longer lists the
pair. It is replaced by the holding entity's name, linked to its page (design:
`docs/design/provider-alias-skipped-line-handoff.md`). The owner who suspects a real duplicate opens
the holder and merges from its own panel. `SkippedAlias` gains `conflict_name`, joined in the same
read.

## Consequences

- The Duplicates queue shows only the name near-miss detector's rows, film same-title rows and
  `shared-external-id` rows. The 31 open provider-alias pairs drop out on deploy, with no verdict
  recorded against them.
- `identity_review_queue` now holds rows that no reviewer sees. That is a semantic stretch of the
  table name, accepted over a migration (see rejected option B). Any future reader of the table must
  filter `provider-alias` or say why not.
- Keep-separate can no longer be recorded against a new provider-alias pair from the queue. Nothing
  needed it: the row is written only once per pair (`INSERT OR IGNORE`), so a re-enrich adds no new
  noise.
- Reversible by deleting one `WHERE` clause.

## Alternatives considered

- **A. Stop writing the row at all** (the ticket's option 1 as filed). Rejected: the Aliases panel
  line would disappear with it, and so would ADR-107's upgrade path.
- **B. Stop writing it and move the skip record to its own table.** This is the clean data model,
  but it costs a migration and a backfill, and repoints two readers, for no difference the owner can
  see. Worth revisiting only if `identity_review_queue` gains another reader.
- **C. Keep the rows, rank them last, fold them behind "weak matches"** (the ticket's option 2).
  Rejected: it keeps a surface whose answer is always the same.
- **D. Suppress only pairs where the holder holds the name as an alias.** Rejected for now: that
  split is unmeasured (see Context), and the owner's hands-on result covers both halves.
