---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-452
status: in-review
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
- [x] backend — **P0-9** (migration `0052_backfill_entity_external_ids`), **P0-3** (the write-time
      guard), **P0-2** (`queueSharedExternalIDPair`, the one writer both producers call), **P0-7**
      (both stale comments), **P0-1 + P0-4** (`sharedExternalIDPairsSQL` +
      `Repo.SweepSharedExternalIDs`, wired ungated in `cmd/holodex/main.go` as
      `shared-id-sweep`), **P0-5** (video and tag excluded, asserted at both producers),
      **P0-8** (the `-2` sort branch). Plus the `detail` read path the chip needed
- [x] frontend — **P0-6** — `sharedIdChip` in `queue.ts` + the chip branch in `DuplicatePairRow`.
      Three-skin QA passed on the live preview by computed style and geometry (screenshots time out
      against this preview); **Kevin's eyeball on the prod skin is the one open QA item**
- [x] testing `testing-strategy` — `docs/testing-strategy.md` **§17 / §17.1 / §17.2 / §17.3**, over
      8 Go tests and 4 new SPA cases in three files. **22 mutations run; 20 are caught**, and the
      two survivors are documented rather than hidden: rule 1's *outer* clause and rule 3's `IN`
      list both fail nothing on their own, because the shape test and the per-kind exists-guard
      already cover them — neither may be deleted as redundant, and the reason is now in both
      copies of the SQL. The pass also found a **genuinely unpinned rule**: rule 1's clause
      *inside* the winner subquery guards a **false negative** (a group whose newest memo carries
      no id but whose older one names a real one), which no fixture exercised. Added as case 16 of
      the migration fixture and the person-11/12 pair in the sweep fixture; both copies of the SQL
      now fail when it goes
- [x] security `security-review` — run 2026-09-24. **No new endpoint, no new parameter, no new
      privilege boundary**, and the owner gate was *checked* rather than assumed: `mountDuplicates`
      registers inside `handlers.go`'s `requireOwner` group (line 465, group opened at 399) and
      `duplicates_test.go` already pins a tokenless list at **401**, so the new field rides an
      existing, tested gate. `detail` carries a provider **namespace** only — both producers cut at
      the first colon, so a provider-internal id cannot reach the client. The sweep's `job_runs`
      row and both log lines carry a bare count, and no error string on the new paths includes an
      external id or an entity name. **One finding, fixed:** `ListReviewPairs` selected `q.detail`
      raw, and that column means different things per variation — on a `provider-alias` row it is a
      **skipped person name** whose only correct reader (`SkippedAliasesForEntity`) deliberately
      returns it to the *denied* side of the pair alone. The `SELECT` now projects it for
      `shared-external-id` and `''` otherwise (`TestListReviewPairsDetailScopedToSharedID`,
      mutation-checked). **And a correction:** this epic recorded that 0045's `detail` column "had
      no reader anywhere" — wrong; it has one, and it is owner-gated per-field. What had no reader
      was the review-queue *row*. Fixed in the spec, this worklog and the session log below

## Up next — ordered (position = priority)

1. ~~P0-9 backfill.~~ **Done 2026-09-24** — migration `0052_backfill_entity_external_ids`.
2. ~~P0-3, P0-1 + P0-4, P0-8, P0-6.~~ **All done 2026-09-24. Every P0 is in.**
   ~~Then the testing and security gates.~~ **Both closed 2026-09-24 — every gate is green.**
   ~~Then Kevin's eyeball, then `gh pr ready`.~~ **PR #385 marked ready 2026-09-25 at Kevin's
   call** — no merge was needed, `origin/main` was already an ancestor (GitHub: MERGEABLE / CLEAN).
   **Now watch that `jira-sync` actually fired `In Review`**; 452 is a Task, so CI owns it.
3. ~~Decide the variation-upgrade question.~~ **Kevin decided 2026-09-24: upgrade.** Recorded as
   spec **RD8**, which amends P0-2. The write is
   `ON CONFLICT (entity_type, id_lo, id_hi) DO UPDATE SET variation = 'shared-external-id'` — a
   one-way ratchet, since `SeedIdentityReviewQueue`'s own `INSERT OR IGNORE` can never demote it.
   Shipped in 0052. **P0-3's guard and P0-4's sweep must use the same upsert** — a producer left on
   `INSERT OR IGNORE` reintroduces the weak label for exactly the pairs this decision was about.
4. ~~Work the 4 dismissed-but-now-evidenced person pairs by hand.~~ **DONE 2026-09-25 — spec P1-2
   is closed.** Tool: `scripts/review_kept_separate_shared_id_pairs.sql`. Kevin ran it on the host
   and decided all four: **3 merged, 1 re-affirmed as distinct.**

   | pair | decision | outcome |
   |---|---|---|
   | 167 ↔ 862 | **merged**, canonical 167 | as recommended — two providers agreed, no dissent |
   | 858 ↔ 1280 | **merged**, canonical 858 | as recommended |
   | 333 ↔ 911 | **NOT merged — genuinely two people** | the dismissal was right and is now re-affirmed against the *strongest* signal the system has. The `dissenting` column predicted this: both sides carry 2 providers and only 1 asserted, so the other had them apart |
   | 836 ↔ 1429 | **merged**, canonical 836 | the co-appearance was the credit-list artifact, not two performers — the second reading. **Kevin notes it may recur via an unrelated bug**; see below |

   **333 ↔ 911 needs no action and must not be re-raised.** Its `entity_keep_separate` row already
   stands, every detector honors it, and it has now been weighed against a shared provider id and
   upheld. A future session that finds it in a probe should read this row, not re-litigate it.

   **If 836 ↔ 1429 recurs, F71 catches it by itself** — and that is the one thing this epic changes
   about a recurrence. A re-created duplicate gets a **new** person id, so no keep-separate marker
   covers the new pair, and the boot sweep queues it on the next start with the asserting provider
   named in the chip. The old `(836, 1429)` marker is now dangling (HOLODEX-458) but inert, since
   `people.id` is `AUTOINCREMENT`.

   Evidence the decisions were taken on (`co` = shared videos, `vids`/`als`/`flds` = per side):

   | pair | asserts | dissent | co | vids | als | flds | read |
   |---|---|---|---|---|---|---|---|
   | **167 ↔ 862** | **2** | 0 | 0 | 4 / 2 | 13 / 0 | 17 / 17 | recommended merge → **merged** |
   | 858 ↔ 1280 | 1 | 0 | 0 | 1 / 1 | 6 / 0 | 4 / 4 | recommended merge → **merged** |
   | 333 ↔ 911 | 1 | **1?** | 0 | 5 / 2 | 9 / 4 | 19 / 17 | recommended hold → **not merged, distinct** |
   | 836 ↔ 1429 | 1 | 0 | **1** | 2 / 1 | 2 / 0 | 19 / **0** | recommended a look → **merged** |

   **What the evidence was worth, now that the answers are known.** `providers = 2` with no dissent
   was decisive and right. The `dissenting` signal was right too, and it is the one this pass
   *added* — 333 ↔ 911 was the pair that looked mergeable on every other column. **`shared_videos`
   was the misleading one**: it read as the strongest negative, and on the only row that had it the
   answer was still merge, because two credits on one file are two people *unless the file's own
   credit list named one performer twice* — which is how these duplicates arise here in the first
   place. It is a flag to investigate, not a verdict; the script's wording overstated it and has
   been corrected.

   Nothing has to be cleared first: a keep-separate marker does **not** block a merge
   (`IsKeptSeparate` has no production caller), the merge repoints the spine id to the survivor and
   drops any queue row touching the loser, and **the loser's name is preserved as an alias of the
   survivor** (`name → alias` step), so no spelling is lost and old filenames still route.

   **Two probe bugs the live run exposed, both fixed 2026-09-25.** `spine_side` was not correlated
   on the *shared* external id, so it reported whichever side owned any id at all — it printed 836
   for 836↔1429 where the true owner is **1429**. And there was no `dissenting` column, so a pair
   two providers actively disagree about looked identical to one no second provider has a view on.
   Both now have a fixture case; the other columns were unaffected.
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

### 2026-09-25 (latest) — P1-2 is closed: 3 merged, 1 upheld as distinct

Kevin ran the probe on the host and decided all four. **167↔862 merged into 167. 858↔1280 merged
into 858. 836↔1429 merged into 836. 333↔911 NOT merged — genuinely two people.** Three of the four
went as recommended; the fourth went the other way and is the most useful result of the pass.

**`shared_videos` was the column the probe got wrong, and the live answer is what proved it.** It
was written as the strongest *negative* — two credits on one file are two people in that scene — and
836↔1429 was the only pair carrying one, so the sheet sent it to a human look rather than
recommending a merge. It was a real duplicate. The reason is the reading the probe failed to weigh:
**a single file's credit list naming one performer under two spellings**, which is a primary way
these duplicates get created in this library, not an exotic case. The column is now documented as a
flag to investigate with the artifact tell spelled out (no enrichment, no aliases, only video is the
shared one — exactly 1429's shape), and the sort comment no longer claims it ends the question.

**`dissenting` is the column that earned its place**, and it did not exist until the live run.
333↔911 looked mergeable on everything else — two substantial records, no co-appearance across seven
videos — and what held it back was a second provider holding ids for both sides and keeping them
apart. Kevin's answer confirms it. Recorded in the spec so it is not re-litigated: that pair has now
been weighed against the strongest signal the system has and upheld.

**A recurrence is F71's job now.** Kevin notes 836↔1429 may come back via an unrelated bug. It will
not need another hand pass: a re-created duplicate takes a **new** person id, so no keep-separate
marker covers the new pair, and the boot sweep queues it on the next start with the provider named
in the chip. That is the gap this epic exists to close, and the first thing it will catch in anger.

**HOLODEX-458 is no longer theoretical** — the three merges left three dangling
`entity_keep_separate` rows in production, which is what that ticket predicted. Still inert
(`people.id` is `AUTOINCREMENT`, so those ids are never re-issued), still hygiene; commented on the
ticket with the actual pairs.

- handoff: **P1-2 is closed and every gate on this epic is green.** Nothing on HOLODEX-452 is
  waiting on anyone: PR #385 is ready for review. The only open threads are other tickets —
  **HOLODEX-458** (dangling keep-separate rows, now real but inert) and **HOLODEX-457** (the memo
  column drop, which was blocked on these contested pairs being worked and **is now unblocked for
  the three that merged**; it must still not destroy the evidence for any pair left unowned). If
  836↔1429 reappears, that is the unrelated bug surfacing, not a regression in this feature —
  **ask Kevin which bug and file it** if it is not already tracked.

### 2026-09-25 — P1-2: the decision sheet is built, the four decisions are Kevin's

**What this session could not do, stated first: the four merges.** They need the live library, which
this machine cannot reach (`data/holodex.db` here is a 72-person dev database), and a merge is
irreversible and is by design the owner's adjudication — ADR-107 D3 is the whole reason this feature
queues instead of merging. So the deliverable is the tool and the evidence, not the outcome.

**Recovered the four pairs** from the 2026-09-23 host run's §2 output — the run's *anonymized* table,
and the ids only. The pre-fix table in that same transcript carries real names, which is the
anonymization leak already recorded above; nothing from it is reproduced here. Distinct kept-separate
pairs: **167↔862 (asserted by TWO providers), 333↔911, 836↔1429, 858↔1280.** The count matches the
recorded "4 of 9". 167↔862 appears twice in §2 — once per asserting provider — which is why the row
count is 5 and the pair count is 4.

**Built `scripts/review_kept_separate_shared_id_pairs.sql`.** The detector's §2 lists these pairs but
not what *decides* them. This one re-derives the population as an intersection (shared-id pairs ∩
`entity_keep_separate`), so it stays correct on a later run rather than hardcoding today's four, and
prints **ids and counts only** — `scripts/CLAUDE.md` is explicit that a query needing a name is the
wrong question for a probe, and reading names is the app's job.

**The column that matters is `shared_videos`, and it leads the sort.** Two credits on one file are
two people in that scene, so a co-appearance means the provider conflated two performers and the
dismissal was *right* — the one fact that settles a row without opening the app. Its converse is
`providers = 2`: two independent providers minting one id for both sides is not one provider's
bookkeeping error, which is what makes 167↔862 the strongest of the four. Verified against a
throwaway DB built from all 51 up migrations and seeded with five branches, including the pair that
must **not** appear and an output check that nothing leaked a name.

**Two things about acting on a decision, verified rather than assumed.** A keep-separate marker does
**not** block a merge — `Repo.IsKeptSeparate` has no production caller, only tests — so nothing has
to be cleared first. And the merge handles this feature's own state correctly: it repoints
`entity_external_ids` to the survivor and drops any review-queue row touching the loser.

**One follow-up filed: [HOLODEX-458](https://whoiskevinrich.atlassian.net/browse/HOLODEX-458).**
`entity_keep_separate` has an `AFTER DELETE` cleanup for **film only** (migration 0047); person,
studio and tag have none, so a merge leaves a dangling marker — and this hand pass will create up to
four. It is **hygiene, not correctness**, and the reason is load-bearing: `people.id` is
`AUTOINCREMENT`, so a deleted id is never re-issued and no future entity can inherit a stale marker.
The ticket says to verify that for studios and tags before fixing, because the whole severity call
rests on it.

- handoff: **P1-2 is half-done by construction — the tool is committed, the four decisions are
  Kevin's.** Run
  `sqlite3 -readonly <library> < scripts/review_kept_separate_shared_id_pairs.sql` on the host, then
  for each pair either merge in the app or leave the dismissal standing, and **record the outcome in
  `Up next` item 4** — otherwise the next session cannot tell "decided to keep separate" from "never
  looked". Everything else on this epic is closed: PR #385 is ready for review with every gate green.

### 2026-09-25 — marked ready for review. No merge was needed.

**`origin/main` was already an ancestor of the branch** — 0 behind, 18 ahead, and GitHub reports
MERGEABLE / CLEAN — so there was nothing to merge and no merge commit was made. The usual
merge-main-first rule exists because `gh pr ready` on a *conflicting* PR drops the `jira-sync`
event; that risk was absent here, and it was checked rather than assumed.

One trap worth recording for the next session in this worktree: the **local `main` ref is 52 commits
behind `origin/main`**, so `git diff main...HEAD` overstates this branch enormously (it showed 117
changed files under `web/` alone). Compare against `origin/main`, or against the epic's first commit
(`c36e354~1..HEAD`) for the code-only view. The PR's real scope is **24 files over 18 commits.**

Marked ready at Kevin's call, which closes the last open item — the prod-skin look at the chip
(handoff QA 3). That was the one thing no assertion could stand in for, so it is closed by his
judgement and not by evidence in this repo.

- handoff: **PR #385 is ready for review and every gate is green.** Nothing is queued for the
  next session on this epic except watching CI: 452 is a Jira **Task**, so `jira-sync` fires
  `In Review` on the ready flip and `Done` on merge by itself — **verify both landed** rather than
  sweeping by hand. Two things outlive the epic: **P1-2**, the 4 dismissed-but-now-evidenced person
  pairs, which are hand work from the probe's §2 and which no code will ever surface, and
  **HOLODEX-457**, which must not run before those pairs are worked. The live QA testbed
  (fixture people 901/902 in this worktree's `data/holodex.db`) can be torn down whenever.

### 2026-09-24 — the testing and security gates. Every gate is green.
- skills: testing-strategy, security-review

**Testing.** `docs/testing-strategy.md` §17 + §17.1–17.3. The write-up's organizing idea is that the
detection SQL exists **twice** — migration 0052 and `sharedExternalIDPairsSQL` — so the two test
files assert the same reading rules against deliberately parallel fixtures, and a rule fixed in one
copy and not the other is the drift they exist to catch.

**The mutation pass earned its keep three times.** 22 mutations, 20 caught. The two survivors are
now documented as survivors instead of being claimed as coverage: **rule 1's outer clause** and
**rule 3's `IN` list** each fail nothing alone, because the shape test and the per-kind exists-guard
already exclude what they exclude. Neither may be deleted as redundant and both copies of the SQL
now say why. More usefully, the pass found **a rule nothing pinned**: rule 1's clause *inside* the
winner subquery guards a **false negative** — a group whose newest memo carries no id but whose
older one names a real one — and every fixture case until now was about false *positives*. Added as
case 16 of the migration fixture and the person-11/12 pair in the sweep fixture; both copies fail
without it.

**Security.** No endpoint, no parameter, no new privilege boundary. The owner gate was checked
rather than assumed (§16's lesson, inverted): `mountDuplicates` is inside the `requireOwner` group
and `duplicates_test.go` already pins a tokenless list at 401, so the new field rides a tested gate.
`detail` carries a provider **namespace** only — both producers cut at the first colon — and the
sweep's job row and log lines carry a bare count.

**One finding, fixed.** `ListReviewPairs` selected `q.detail` raw, and that column means different
things per variation: on a `provider-alias` row it holds a **skipped person name** whose only
correct reader returns it to the *denied* side of the pair alone, because on the holding side the
same name asserts the opposite of the truth. Shipping it raw handed a second consumer a value whose
correct reading lives elsewhere. Now `CASE WHEN q.variation = 'shared-external-id' THEN q.detail
ELSE '' END`, pinned and mutation-checked.

**And a correction worth keeping.** This epic recorded that 0045's `detail` column "had no reader
anywhere". That is **wrong** — `SkippedAliasesForEntity` reads it for the Aliases panel, behind
`skippedAliases`' per-field `authorized` gate. What had no reader was the review-queue *row*, which
is still the fact that forced the plumbing. Fixed in the spec's API section, in the P0-6 session
entry below, and in memory.

**Process note for the next mutation pass:** the mutation script restores with
`git checkout -- <file>`, which silently discarded three uncommitted comment edits made in the same
turn. Stage before mutating — the standing rule exists for exactly this and I skipped it.

- handoff: **every gate is green.** The one thing left before `gh pr ready` is **Kevin's eyeball on
  the chip in the prod skin** (handoff QA 3) — the preview is still running with the fixture pair
  (people 901/902 in `data/holodex.db`). Merge fresh `main` in first, or `gh pr ready` drops the
  jira-sync event. After merge, 452 is a **Task** so CI fires its own transitions; the two things
  that outlive this epic are **P1-2** (the 4 dismissed-but-now-evidenced person pairs, hand work
  from the probe's §2 — no code will ever surface them) and **HOLODEX-457**, which must not run
  before those contested pairs are worked.

### 2026-09-24 — P0-8 the sort rank, P0-6 the chip. Every P0 is in.
- skills: code-review

P0-8 was one `-2` branch in `ListReviewPairs`' `ORDER BY`, and nothing else: a variation outside
`fuzzyVariations` already passes the live-revalidation join untouched, so these rows were listed
correctly all along — only their rank *among* the other non-fuzzy variations was undefined.

**P0-6 surfaced the one real gap in the design.** The approved mockup's chip reads `tmdb says one
person`, but the row had no way to know the provider: `ReviewPair` did not carry `detail`,
`ListReviewPairs` did not select it, so the **queue row** could not see the 0045 column.
(Corrected at the security gate 2026-09-24: this entry originally said the column had no reader
*anywhere*, which is wrong — `SkippedAliasesForEntity` reads it for the Aliases panel. What had no
reader was the review-queue row.) So the approved design needed plumbing
the spec had ruled out ("API: None", "`detail` stays empty"). Put both options to Kevin with the two
rows drawn; **he chose naming the provider.** `detail` now carries the asserting namespace at all
three producers (0052 included — edited in place, since it is unmerged and has never run outside
tests), and `ListReviewPairs` → the API → `DuplicatePair` carry it through. A pair colliding on two
providers cites one deterministically via `min()`.

**One handoff detail corrected in implementation:** its `--bg-accent` wording would mean solid
`bg-accent`, which `app.css` reserves for a page's one primary action. The chip uses the **outlined**
treatment `ExtractionQueueRow`'s staged chips already use — `rounded-full border border-accent
bg-accent/10 text-accent`.

QA ran against the live preview with a seeded contested pair, which also verified the backend
end-to-end: **migration 0052 queued the pair with `detail = 'tmdb'` on the boot, the sweep then
recorded its own job run reporting 0** (nothing new — exactly the designed handoff between them), and
the row renders **above** both `punctuation` rows despite its names sorting last alphabetically.
Three-skin contrast 8.84 / 11.62 / 16.88 with a *different* accent resolved per skin (the real proof
it is token-driven); row height 49.0px identical to a slug row at 700px and 1280px with 39–42-char
names. **Screenshots time out against this preview**, so that is computed-style and geometry
evidence, not a visual check.

- handoff: every P0 criterion is implemented and pushed. Two gates left and neither is code:
  **`/testing-strategy`** (write up the three new test files, including §5's no-component-harness
  rule — which is *why* the chip's logic is a pure function in `queue.ts`) and
  **`/security-review`** (short: no new endpoint; `GET /owner/duplicates` gains `detail`, carrying a
  provider *name*, never the external id, on an already owner-gated payload). Then
  **Kevin's eyeball on the chip in the prod skin** (handoff QA 3) and `gh pr ready`. The preview is
  left running with the fixture pair in `data/holodex.db` (people 901/902) so it can be looked at
  directly; the pre-fixture DB backup is in this session's scratchpad.

### 2026-09-24 (earlier) — P0-1 + P0-4, the every-boot sweep
- skills: code-review

The detector is `sharedExternalIDPairsSQL` in the new `internal/repo/shared_external_id.go` (which
also took the two F71 helpers out of `identity.go`, keeping that file about the spine resolve).
`Repo.SweepSharedExternalIDs` reads the pairs, then writes each through
`queueSharedExternalIDPair` — reusing the writer rather than re-deriving a bulk upsert, so
keep-separate and RD8's upgrade behave identically to the write-time guard by construction. Wired in
`cmd/holodex/main.go` beside `seedIdentityReviewQueue`, **ungated** (RD5), as `shared-id-sweep`.
Nothing else had to be registered — job kinds have no allowlist and no frontend label map.

**One thing the sweep needed that the migration did not:** RD8's upsert counts an UPDATE as an
affected row, so a pair already carrying `shared-external-id` would be re-reported on every boot and
the activity row would never read 0. The `DO UPDATE` now carries its own
`WHERE identity_review_queue.variation <> 'shared-external-id'`. 0052 doesn't need it — on a first
run no row can already hold the value — so that migration is deliberately untouched.

**Mutation testing found something worth recording.** Two rules are load-bearing and proven so:
dropping the spine half of the claimant union loses 3 of the 6 fixture pairs (it degenerates to the
memo self-join ADR-107 D1 rejected), and dropping the upsert's `WHERE` makes a second sweep report 6
instead of 0. But the third — **the video/tag exclusion — is enforced twice, and neither clause alone
is detectable.** Removing the explicit `entity_type IN (…)` still excludes both kinds, because the
per-kind entity-exists guard enumerates exactly person/studio/film and a video row has no branch to
match; removing the guard alone is likewise covered by the `IN` list. Only removing **both** leaks
the video pair and the tag pair. Both stay, and the code comment now says which does what — the `IN`
list states the rule and keeps the correlated subquery off the video memos (the bulk of
`entity_enrichment`), the exists guard makes the exclusion structural. Same redundancy in 0052, by
construction.

- handoff: the backend is one line from done — **P0-8**, the sort tiebreak in `ListReviewPairs`
  (`internal/repo/review_queue.go`): non-fuzzy variations already pass through and sort at `-1`, so
  `shared-external-id` needs `-2` to land ahead of `provider-alias`/`same-title`. After that it is
  **P0-6** (the chip, frontend, three-skin QA per the design handoff) and then the testing and
  security gates — at which point the Draft PR can be marked ready. Note when writing P0-6 that the
  chip keys on `variation`, a sibling of `MATCH_KIND_LABEL` rather than an entry in it, because that
  map is keyed on the derived `match_kind` (which is `''` for every non-fuzzy row).

### 2026-09-24 (earlier) — P0-3, the write-time guard
- skills: code-review

`Repo.AttachExternalID` stops being a silent `INSERT OR IGNORE`. Under the `writeMu` it already
holds it re-reads the owner and, when the id belongs to a different entity of the same kind, queues
the pair and returns **nil** — the enrich must still succeed, because the field values are already
stored and only the identity claim is contested. Shipped with `queueSharedExternalIDPair`, the
shared writer P0-2 describes (RD8 upsert, keep-separate gate, ordered pair), which P0-4's sweep will
reuse unchanged. P0-7's two stale comments rewritten in the same commit.

All three recorded traps held, and the **anti-flood one is mutation-checked**: drop the
`owner == entityID` branch and even a *free* attach queues a self-pair, because `INSERT OR IGNORE`
returns nil for inserted / already-mine / owned-by-another alike. That is the branch, not the
`RowsAffected` question, that keeps a refresh sweep from flooding the queue.

**One correction to a stated rationale.** ADR-107 D4 justifies keeping the guard off the private
`attachExternalID` partly with "a queue insert there would fire on every relink". That half does not
hold: `resolveOrCreateByName` step 1 looks the id up and **returns the owner before ever reaching
the private writer** (`identity.go:166`), so a contested id cannot arrive there — a guard placed
there would be dead code on the scan path, not a flood. The constraint stands on the transactional
reason (a review row written inside a scan transaction that may roll back). Recorded in the code
comment and spec P0-3 so nobody "simplifies" the guard back down. Consequence for testing: **no
test can distinguish a guard placed there by its queue output**, so
`TestScanPathAttachStillSilent` asserts the weaker true thing — the relink path still resolves
id-first and still queues nothing — and says so.

- handoff: the guard is in and pushed, so new collisions stop accumulating. **Next: P0-1 + P0-4**,
  the detector and the ungated boot sweep — port the claimant-set query from migration 0052 steps
  1–3 (it carries the tie-break-toward-agreement rule and the entity-exists guard that the probe
  script's version does not), call `queueSharedExternalIDPair` for each pair, and wire it beside
  `seedIdentityReviewQueue` in `cmd/holodex/main.go` with its own `job_runs` kind. P0-5's two
  exclusions are named tests on the detector; video/tag are already asserted at the guard.
  After that P0-8 is one line and P0-6 is the chip.

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

Kevin then answered the one question the migration surfaced: a shared-id finding **upgrades** an
existing weaker `variation` rather than leaving it. Spec **RD8**, amending P0-2; 0052 writes the
upsert. One caution the mutation check produced: `SELECT DISTINCT` before the upsert is there to
collapse a pair colliding on two providers, **not** because SQLite refuses a double-touch — removing
DISTINCT still passes, so don't document it as load-bearing for that reason.

- handoff: P0-9 is done and pushed; next is **P0-3, the write-time guard**, carrying its three
  recorded traps (`RowsAffected() == 0` is not the signal; guard `Repo.AttachExternalID` never the
  shared `attachExternalID`; check + queue write in one `writeMu` critical section). Both it and
  P0-4's sweep must write the **RD8 upsert**, not `INSERT OR IGNORE` — a producer left on the old
  write reintroduces the weak label for exactly the pairs RD8 was decided about.

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
