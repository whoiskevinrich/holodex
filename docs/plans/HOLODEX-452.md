---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-452
status: in-progress
approved:
release_note: A provider that says two of your people are the same record now shows up in the Duplicates queue, ranked above the name-guesses and labelled with who said it — and a mis-picked entity in the enrich screen queues the pair instead of silently dropping the identity write.
---

# HOLODEX-452 · Detect duplicates by shared provider external id, not just by name

Duplicate detection is entirely name-based. `SeedIdentityReviewQueue`
(`internal/repo/identity_queue.go`) folds canonical names ∪ aliases to a loose key;
`FlagNearMiss`, `queueProviderAliasPair` and `queueFilmSameTitle` are all name-driven too. Nothing
can see two entities carrying the **same provider external id** — the strongest positive merge
evidence there is.

Live library 2026-09-22 (1591 people, 1000 enriched, 31 queued person pairs): **6 external ids
claimed by more than one person**, none queued, `same_xid = 0` across all 31 queued pairs. New
coverage, not a re-cut.

## Decisions — do not re-litigate

- **It queues; it never merges.** ADR-107 D3. Irreversible (ADR-061), providers carry their own
  duplicate entries, and decisively: the colliding data was produced by an **owner picking the
  wrong entity in the enrich UI**, so auto-merge would let one mis-click fold two real people
  together. F70's rule governs the surface — the panel reports, it never adjudicates.
- **The signal is memo ⋈ spine disagreement, not a memo self-join.** ADR-107 D1. The ticket as
  filed proposed the self-join; it makes the memo an identity store (ADR-096 D2's rejected option
  C), cannot say which side the spine already believes, and **measurably produces a false
  positive** the disagreement suppresses — verified on a fixture, see below.
- **The memo column is a witness, not a second source of truth**, and its drop stays deferred as a
  follow-up. ADR-107 D2. Do not fold ADR-096 D2's migration into this ticket.
- **Person, studio and film. Video and tag excluded by construction, as named tests.** ADR-107 D5.
  Video has no spine row and two files of one movie legitimately share a provider id — queueing
  those would be wrong, not merely noisy.
- **The row names its asserter in an accent chip** (`tmdb says one person`), not the raw variation
  slug: the queue's `text-warn` already means *weak signal*, and this is the strongest one in the
  list. Kevin chose this over the raw slug 2026-09-23. Don't re-propose the slug.

## Structural findings — 2026-09-23

- **The hole is a silent no-op, not a missing query.** `resolveOrCreateByName` is id-first
  (`identity.go:162`) so the scan path is safe. The **enrich** path calls the public
  `AttachExternalID` (`identity.go:274`, from `enrich/service.go:695`) with an entity the *owner*
  picked, and the `INSERT OR IGNORE` writer — whose own comment at `identity.go:253` assumes
  id-first already ran — discards the claim silently while `UpsertEnrichment` stamps the colliding
  id onto every field row anyway. That comment becomes false when the guard lands (spec P0-7).
- **The memo is already namespace-qualified** (`tmdb:1100`) — same grammar as
  `entity_external_ids`. The ticket's `GROUP BY (provider, external_id)` is redundant; a memo
  compares to the spine with no string building. Nothing enforces the grammar at the write,
  though (probe §4c).
- **A shared-xid pair can never look alike.** ADR-061's `UNIQUE` nameKey index forbids two people
  sharing a canonical name — discovered by the fixture rejecting it — so a genuine split identity
  necessarily wears two different names. The two detectors are disjoint by construction, not by
  accident.
- **A merge deletes the loser's enrichment rows** (`identity_ops.go:451`), so a merge can never
  strand a memo. The only staleness source is a re-enrich that wrote a *narrower* field set —
  handled by the newest-`fetched_at` rule.
- **`entity_enrichment.external_id = ''` for the whole `filename` extract population**
  (`extract/store.go:35`). Every query must exclude it.
- **The live library is not reachable from this machine.** `G:/source/holodex/data/holodex.db` is
  a 72-person dev database that has not run migration 0046. The probe must run on the host.

## Probe verification — fixture, 2026-09-23

`scripts/detect_shared_external_id.sql` was run against a database built by applying **all 50 up
migrations** to an empty file, then seeded with the six cases in its header. Results as designed:

| case | expected | got |
|---|---|---|
| memo disagrees with spine (person) | finding | ✅ pair (1,2) |
| memo agrees with spine | no finding | ✅ |
| stale narrow re-enrich (old memo → other owner) | no finding | ✅ suppressed by newest-`fetched_at` |
| `filename` provider, memo `''` | no finding | ✅ |
| kept-separate pair | found, suppressed from `genuinely_new` | ✅ |
| two videos sharing `tmdb:841` | never a finding | ✅ (and §5b counts it as what the naive query *would* have taken) |
| one pair colliding on **two** providers | counted once | ✅ after a fix — `/code-review high` caught §1 counting `pairs` distinctly but `genuinely_new` per row |

**The self-join comparison (§5) found 2 colliding ids where §1 found 2 pairs** — the extra is the
stale re-enrich. That is RD1's measured justification, not a theoretical one.

## Gates — definition of done

- [x] spec `write-spec` — **F71**, `docs/specs/duplicates-shared-external-id.md`. 8 P0 criteria,
      2 P1, 3 open questions (all three need the host probe)
- [x] architecture `architecture` — [ADR-107](../architecture/ADR-107-shared-external-id-duplicate-detection.md),
      5 decisions; index row added. Amends the **timing** of ADR-096 D2's memo-column drop, not its
      substance
- [x] design `design-handoff` — `docs/design/duplicates-shared-external-id-handoff.md` +
      committed `duplicates-shared-external-id-mockup.svg`, measured in the browser at real type
      sizes (no row collisions, gaps match the component's `gap-x-2`). **Chip vs. raw slug chosen
      by Kevin 2026-09-23; sign-off on the artifact itself is still outstanding** — `/implement`
      records it
- [ ] backend — P0-1 … P0-5, P0-7, P0-8
- [ ] frontend — P0-6 (one chip keyed on `variation`; three-skin QA in the handoff)
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. **Run `scripts/detect_shared_external_id.sql` on the host.** It closes all three spec open
   questions: the studio/film numbers (OQ1), how many memos have no spine row at all (OQ2 — if
   large, a repair pass is a P0 not a P1), and whether any memo breaks the `<ns>:<id>` grammar
   (OQ3 — a bare id would make the detector silently blind for that provider).
2. File the ADR-096 D2 follow-up (drop `entity_enrichment.external_id`, re-home the video
   re-enrich memo) — ADR-107 action item 4 says before this merges, so the deferral has an owner.
3. `/implement` to cross into build — it puts the design handoff in front of Kevin and records the
   sign-off, then opens the Draft PR.
4. On merge: 452 is a **Task**, not an Epic, so CI fires its Jira transitions itself — no hand
   sweep needed (same as 451).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-23 — design phase, complete
- skills: code-review

Renamed the branch from `claude/holodex-452-db79fe` to
`HOLODEX-452-duplicates-shared-external-id` and fired **In Progress**. Reserved **F71** and
checked **ADR-107** free. Traced the write paths, found the silent no-op and the ADR-096 D2
collision, and reframed the ticket: it is not "add a name-free detector", it is "a dropped
identity write has never been visible". Kevin answered four decisions (queue not merge;
memo ⋈ spine; person+studio+film; asserter chip). Shipped ADR-107, spec F71, the design handoff
plus a browser-measured SVG mockup, and `scripts/detect_shared_external_id.sql` — verified against
a database built from all 50 migrations with a six-case fixture, including the false positive the
ticket's original query would have produced.

**Handoff:** every design gate is closed. The one thing standing between this and `/implement` is
running the probe on the host — all three spec open questions are its output, and OQ2 could
promote a repair pass from P1 to P0.
