---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-452
status: in-progress
approved:
  design:
    on: 2026-09-24
    at: 260484e
release_note: A provider that says two of your people are the same record now shows up in the Duplicates queue, ranked above the name-guesses and labelled with who said it — and a mis-picked entity in the enrich screen queues the pair instead of silently dropping the identity write.
---

# HOLODEX-452 · Detect duplicates by shared provider external id, not just by name

Duplicate detection is entirely name-based. `SeedIdentityReviewQueue`
(`internal/repo/identity_queue.go`) folds canonical names ∪ aliases to a loose key;
`FlagNearMiss`, `queueProviderAliasPair` and `queueFilmSameTitle` are all name-driven too. Nothing
can see two entities carrying the **same provider external id** — the strongest positive merge
evidence there is.

Host probe **2026-09-23** (1577 people, 556 studios, 48 films, 1000 enriched, 33 queued, 185
keep-separate): **15 pairs — 9 person, 6 studio, 0 film — 10 of them new**, and exactly one already
in the name-based queue. Near-disjoint new coverage.

## Decisions — do not re-litigate

- **It queues; it never merges.** ADR-107 D3. Irreversible (ADR-061), providers carry their own
  duplicate entries, and decisively: the colliding data was produced by an **owner picking the
  wrong entity in the enrich UI**, so auto-merge would let one mis-click fold two real people
  together. F70's rule governs the surface — the panel reports, it never adjudicates.
- **The signal is memo ⋈ spine disagreement, not a memo self-join.** ADR-107 D1. The ticket as
  filed proposed the self-join; it makes the memo an identity store (ADR-096 D2's rejected option
  C), cannot say which side the spine already believes, and **measurably produces a false
  positive** the disagreement suppresses — verified on a fixture, see below.
- **The memo column is a witness, not a second source of truth**, and its drop stays deferred to
  **HOLODEX-457**. ADR-107 D2. Do not fold ADR-096 D2's migration into this ticket.
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
- **A shared-xid pair rarely looks alike.** ADR-061's `UNIQUE` nameKey index forbids two entities
  sharing a canonical name — discovered by the fixture rejecting it — so a split identity always
  wears two different names. **Not fully disjoint though:** 1 of the 15 host findings was already
  queued, because a shared-id pair can *also* be a punctuation near-miss. Corrected in the ADR,
  spec and index — the earlier "disjoint by construction" wording was too strong.
- **A merge deletes the loser's enrichment rows** (`identity_ops.go:451`), so a merge can never
  strand a memo. The only staleness source is a re-enrich that wrote a *narrower* field set —
  handled by the newest-`fetched_at` rule.
- **`entity_enrichment.external_id = ''` for the whole `filename` extract population**
  (`extract/store.go:35`). Every query must exclude it.
- **The live library is not reachable from this machine.** `G:/source/holodex/data/holodex.db` is
  a 72-person dev database that has not run migration 0046. The probe must run on the host.

## Probe verification — fixture, 2026-09-23

`scripts/detect_shared_external_id.sql` was run against a database built by applying **all 50 up
migrations** to an empty file, then seeded with the seven cases in its header. Results as designed:

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

## Host probe results — 2026-09-23 (second run, after the claimant-set fix)

| § | result |
|---|---|
| 1 | person **10** pairs (5 new) · studio **7** (7 new) · film **0** |
| 2 | **2 pairs have no spine owner on either side** — found memo-to-memo, invisible to the join this ticket was originally specced with |
| 2b | one studio id claimed by **three** studios |
| 3 | **1** finding already queued — near-disjoint, not disjoint |
| 4a/4b/4c | **all empty** — the memo is perfectly consistent per (entity, provider) and always `<ns>:<id>`. OQ3 closed clean |
| 5 | self-join would find 6 person / 5 studio colliding ids against 10 / 7 real pairs |
| 5b | **23** video ids shared by multiple files — excluding video suppressed more wrong findings than there are right ones |
| 6 | **10 rows for 10 pairs** — the earlier 5-of-9 was a trimmed paste, not a bug (OQ5 closed). 3 pairs share a video |
| 7 + 8 | **the spine is sparse, and that is the bigger story** — see below |

### §8 — the spine is far sparser than the memo layer

| kind | provider | memos | id also in `entity_external_ids` |
|---|---|---|---|
| studio | provider-3 | 436 | **436 — 100%** |
| person | provider-3 | 960 | 491 — 51% |
| person | provider-1 | 851 | **101 — 12%** |
| film | provider-3 | 45 | 17 — 38% |

Only **489 of 1000** enriched people hold any spine row. **Root cause: historical, not a defect.**
The enrich-path attach landed `28e2540` (F60, **2026-09-15**) — 8 days before the probe. Studio's
100% is a *different writer*: `ReconcileVideoStudios` → `resolveOrCreateByName` →
`attachExternalID`, fed by the `_studio_external_ids` sidecar since `746a5ac` (ADR-054,
**2026-07-02**), re-derived on every relink, with prune-on-empty guaranteeing coverage. Person's
mirror is optional and an id-less person is orphan-stamped, not pruned. **No migration ever folded
the memo into the spine** (0018, 0038, 0046 each declined). Not `identityShaped` either — it
accepts slug ids cleanly.

Consequences: **HOLODEX-457 blocked** (commented there); the repair pass is **P0-9, a backfill, and
it runs BEFORE the write-time guard** — switching the guard on first would surface almost nothing
from 1700+ historical memos.

### The anonymization rule leaked, and the host run caught it

§2 printed `provider-1:…lor_Luna`. That provider mints **human-readable slug ids**, not UUIDs, so
the `substr(external_id, -8)` the rule prescribed exposed a fragment of a performer's name. No
truncation of an id is safe, because nothing constrains a provider's format. Labels are now an
opaque per-run ordinal (`provider-N:xid-M`); `scripts/CLAUDE.md` corrected with the reason.

**Detector bug the data caught.** §2 showed `…:studio:9e231db3…` claimed by studios 39, 489 and
spine-owner 126. A memo⇔spine join emits (39,126) and (489,126) and **silently drops (39,489)** —
a duplicate the owner could never reach. ADR-107 D1 now pairs over the whole **claimant set**
(spine owner ∪ memo holders). Both halves are load-bearing: 3 person ids have a spine owner holding
no memo (invisible to a self-join) and 1 id has two memo holders (invisible to a spine-anchored
join). Verified on a new three-claimant fixture — the clique closes.

**4 of the 9 person findings are already `entity_keep_separate`** — dismissed against the weakest
evidence the queue produces, before this signal existed. **Kevin's call 2026-09-23: surface those
once, out-of-queue, as a one-time reconciliation** — and at N = 4 the tool is the probe's own §2
output, worked by hand. No code, no UI; ADR-061's durable-no invariant stays intact and the queue
still never re-proposes. Reason-aware dismissals were considered and rejected — reopen only if the
OQ2 repair pass makes the number stop being small.

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
- [ ] backend — **P0-9 done** (migration `0052_backfill_entity_external_ids` + its named-case test);
      P0-1 … P0-5, P0-7, P0-8 open
- [ ] frontend — P0-6 (one chip keyed on `variation`; three-skin QA in the handoff)
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. ~~P0-9 backfill.~~ **Done 2026-09-24** — migration `0052_backfill_entity_external_ids`.
2. **P0-3, the write-time guard.** Three traps recorded in the spec — `RowsAffected() == 0` is
   *not* the signal (benign re-enrich idempotence would flood the queue), the guard belongs on
   `Repo.AttachExternalID` and never on the shared `attachExternalID`, and the check plus queue
   write must stay inside the one `writeMu` critical section (no `AttachExternalIDLocked` exists).
   Then P0-1/P0-2/P0-4 (detector + sweep), P0-7, P0-8, P0-6.
3. **Decide the variation-upgrade question — once, for all three producers.** A pair that is both
   a shared-id finding and a name near-miss keeps its *weaker* variation, because P0-2 specifies
   `INSERT OR IGNORE` and the migration honors that. The host probe's §3 found exactly 1 such pair
   out of 15. Consequence: that pair renders the near-miss label and sorts in the fuzzy band, which
   is the precise failure P0-8 exists to prevent. The alternative is a one-line
   `ON CONFLICT … DO UPDATE SET variation = 'shared-external-id'` — a one-way ratchet, since the
   name detector's own `INSERT OR IGNORE` can never demote it. **Do not implement it in one
   producer only**; recorded in the spec's Success Metrics.
4. **Work the 4 dismissed-but-now-evidenced person pairs by hand** from the probe's §2
   (`kept_separate = 1`) — spec P1-2.
5. ~~File the ADR-096 D2 follow-up.~~ Filed as **HOLODEX-457** (drop `entity_enrichment.external_id`,
   re-home the video re-enrich memo), linked `Relates` to 452 and blocked on the host probe's §7.
   **Still blocked, but the block is now partial:** 0052 has made the spine the record for every
   *uncontested* id, so 457 would no longer destroy those. A **contested** id is deliberately left
   unowned (spec P0-9 rule 1), and for those the queue row — not the memo — is the surviving
   evidence. 457 must not run before the contested pairs are worked.
6. ~~`/implement` to cross into build.~~ Done 2026-09-24 — Kevin signed off on the design handoff
   at `260484e`; Draft PR open.
7. On merge: 452 is a **Task**, not an Epic, so CI fires its Jira transitions itself — no hand
   sweep needed (same as 451).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-24 (later) — P0-9, the spine backfill
- skills: code-review

Shipped **migration `0052`** (0052 was free — checked every local and remote branch, and `main` is
at 0051). Data-only: no table, no column, just rows into `entity_external_ids` and
`identity_review_queue`. Being a migration is what buys the ordering the ADR calls load-bearing —
migrations run before the enrich service exists, so P0-9 cannot race P0-3's guard or P0-4's sweep.

**The decision this session actually made: a contested id is left UNOWNED.** The spine PK gives an
id one owner per kind, so folding one of two claimants would decide which entity the provider record
names — and id-first resolve would then route every future credit that way. That is adjudication,
which ADR-107 D3 forbids; the pair is queued instead and the owner's merge assigns the id. Three
supporting rules landed with it (explicit entity-exists guard, since `entity_external_ids` has no FK;
`identityShaped` cut at the first colon so a slug id with a colon in its own half passes; an
asymmetric down on 0044's precedent — review rows deleted, folded spine rows left, because they are
indistinguishable from what `attachExternalID` writes).

`internal/db/external_ids_backfill_test.go` names all fourteen fixture cases — the probe's seven
plus the three-claimant clique, the memo-only contest, the three unshaped id shapes, and the orphan
memo. **Mutation-checked the one rule that could have silently passed:** inverting the
fetched_at-tie preference (agree-with-spine `DESC` → `ASC`) makes the test fail on exactly the pair
it should, so rule 2's tiebreak is genuinely exercised rather than incidentally satisfied.

Three stale spec statements fixed in the same commit, all contradicting P0-9: Data model said "No
migration"; Non-Goals still listed the backfill as out of scope; Success Metrics claimed probe §3
returns 0 when the host measured 1.

- handoff: P0-9 is done and pushed; next is **P0-3, the write-time guard**, carrying its three
  recorded traps (`RowsAffected() == 0` is not the signal; guard `Repo.AttachExternalID` never the
  shared `attachExternalID`; check + queue write in one `writeMu` critical section). Before writing
  P0-2's queue helper, get Kevin's answer on **`Up next` item 3** — whether a shared-id finding
  should *upgrade* an existing weaker `variation` — because it has to be one answer for all three
  producers, and the migration currently honors P0-2's literal `INSERT OR IGNORE`, which leaves 1
  host pair rendering the near-miss label in the fuzzy sort band.

### 2026-09-24 — crossed into build
- skills: implement, code-review

Reconstructed the branch state first: the previous entry's handoff was written as bold
`**Handoff:**` rather than `- handoff:`, so the SessionStart detector read it as missing — and it
was stale anyway, naming three things (probe §6, §8, OQ4) that `4e207fd`, `51c1145` and `260484e`
had since closed. All three design gates settled, branch already on `origin`, `main` an ancestor
(no merge needed). Put the design handoff and its committed SVG in front of Kevin; **approved**,
recorded under `approved.design` pinned to `260484e`. Draft PR opened; the build gates are the
ones still out.

The PR guard then caught a second drift worth recording: **this session log had been written
oldest-first**, but `parseWorklog` reads entries top-down and treats the *first* `###` as the
newest (`flightplan/lib/worklog.mjs:337`) — so appending at the bottom hides a handoff from the
guard and the SessionStart banner no matter how well it is written. Every other worklog in
`docs/plans/` is newest-first. Reordered. **Prepend, never append.**

- handoff: Design is signed off and the crossing is done — start building at `Up next` item 1,
  P0-9 backfill first (the spine is historically sparse, so switching the P0-3 write-time guard on
  ahead of it would surface almost nothing from 1700+ existing memos). Carry the guard's three
  recorded traps into the code: `RowsAffected() == 0` is not the signal, the guard goes on
  `Repo.AttachExternalID` and never the shared `attachExternalID`, and the check plus the queue
  write stay in one `writeMu` critical section.

### 2026-09-23 (later) — host probe, and the detector was wrong

Kevin ran the probe on the host. It closed OQ1 and OQ3, half-closed OQ2 into something bigger
(1219 spine-less person memos), and **found a real bug in the detector**: a memo⇔spine join drops
the memo⇔memo pair of a three-claimant id. D1 now pairs the whole claimant set. Also corrected the
"disjoint by construction" claim — 1 of 15 findings was already queued.

**Handoff:** design gates are still closed and the shape holds; two follow-up reads (probe §6 and
the new §8) and Kevin's answer to OQ4 stand between this and `/implement`. §8 is the one that can
still move work: if the attach almost never lands, a repair pass becomes P0 and HOLODEX-457 gets
blocked behind it.

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
