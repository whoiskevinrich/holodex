# scripts/

Repo tooling: claim allocators (`adr-claims.mjs`, `feature-claims.mjs`), Jira/CI sync
(`jira-*.mjs`, `release-*.mjs`), commit and worklog gates, `hooks/`, and the `detect_*.sql`
probes that measure the live library.

## Rules

- **A probe must anonymize its output. Assume every result is pasted into a chat.** These queries
  run against the owner's **production** library and their output is routinely pasted into a
  session, an issue or a PR. Anything a probe prints is therefore published, and the owner should
  never have to redact it by hand — if they do, the probe is the thing that's broken, not their
  process.

  | Never print | Print instead | Why |
  |---|---|---|
  | An entity's `name` — person, studio, film, tag | its internal `id` | An id is already the handle: `/people/1679` opens the record. A name is the whole disclosure. |
  | A provider's name | a stable generic alias — `provider-1`, `provider-2` | Upholds the standing "name providers generically" rule, and an alias still answers *"do the misses cluster in one provider?"* |
  | A provider-native id | an **opaque per-run ordinal** — `xid-7`, via `dense_rank()` | Two rows sharing a label share an id, which is all the output needs to say. |
  | Any `value` out of `entity_enrichment`, filenames, paths | a count, a length, or a boolean | Bios, titles and paths are free text — they identify by content. |

  Counts, booleans, aggregates and internal integer ids are fine in the clear. If a query cannot
  answer its question without a name, the question is wrong for a probe: narrow it to ids and look
  the names up in the app.

- **Never publish an external id's own bytes — not even a truncation.** This rule was learned the
  expensive way: the first version of the table above said to print `substr(external_id, -8)`,
  reasoning that the tail of a UUID identifies nobody. It shipped, and the very next host run
  printed `provider-1:…lor_Luna` — because **that provider mints human-readable slugs, not UUIDs**,
  and the tail was a fragment of a performer's name. Nothing constrains a provider's id format, so
  no amount of trimming is safe. Replace the id with an ordinal that carries only the one fact the
  output needs: *these two rows mean the same id*.

- **Anonymize by default, with no flag to forget.** Don't ship a probe whose safe output depends on
  the reader remembering to redact a section, or on passing a `--safe` switch. One output, always
  publishable. The owner reading names is a job for the app, which already has the pages.

- **Verify a probe before handing it over, and say what you verified against.** Build a throwaway
  database by applying every `internal/db/migrations/*.up.sql` in order to an empty file, seed a
  fixture that exercises each branch — including the ones that must produce *no* finding — and
  record that fixture in the script's header comment. The local `data/holodex.db` is a small dev
  database that lags production's migrations, so it proves nothing about a query's correctness.

- **A probe is read-only.** Headers document `sqlite3 -readonly`, and temp views are fine under it
  (they live in the temp database). Never write to the library from `scripts/`.

## The probes

| File | Covers |
|---|---|
| `detect_shared_external_id.sql` | **The maintained duplicate detector** (HOLODEX-452, ADR-107 D1): entities that share a provider external id, found by pairing the whole claimant set — the spine's owner union every memo holder. |
| `detect_person_duplicate_evidence.sql` | What evidence exists to *decide* the person pairs already in `identity_review_queue` (sized the F70 compare panel). Its own external-id cross-check counts colliding memos only and misses the spine side — use the detector above for that. |
| `detect_entity_collisions.sql` | Name collisions across all four spine kinds: case/whitespace (Tier A, migration blockers), punctuation/spacing near-misses (Tier B), and film's same-title-different-year pairs. |
| `review_kept_separate_shared_id_pairs.sql` | **The P1-2 decision sheet** (HOLODEX-452, spec F71 P1-2): the pairs a provider says are one record that the owner dismissed *before* that signal existed. Deliberately out of queue — ADR-061's durable no stays intact and no detector re-proposes them — so this is the one-time reconciliation, worked by hand. Leads with `shared_videos`, because a co-appearance is the one fact that ends the question without opening the app. |

All four conform to the rules above as of 2026-09-25.

**Film keys on `(nameKey, year)`, and the probe carries that through.** `ux_films_namekey`
(migration 0047, ADR-096 D3) makes two films sharing a title across different years legal, so
grouping film by name alone would report every remake as a hard collision. Tier A and Tier B group
on a `ykey` that is the year for film and the constant `''` for every other kind — which is what
keeps the other three kinds' numbers identical. Two things that shape cannot express get their own
section: a **NULL year is distinct from every other NULL** under a SQLite unique index, so two
untitled-year films with the same name coexist legally and are a real duplicate the index cannot
stop; and `queueFilmSameTitle` queues same-title pairs under **any** year as the non-fuzzy
`same-title` variation, which a `(name, year)` grouping is blind to by design.
