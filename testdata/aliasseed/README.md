# Alias state seed (F58 / ADR-088)

Stages the alias states that are tedious to reach by hand, so the provider badge, the
collision review line, and suppression durability can be looked at in a browser without
re-deriving the setup each time.

```bash
go run ./testdata/aliasseed                # seed into ./data/holodex.db
go run ./testdata/aliasseed -video 91      # ...and link the people to a video
go run ./testdata/aliasseed -clean         # remove everything it created
```

**Stop the backend first.** The server holds its own connection to the same SQLite file,
and two writers will contend past the busy timeout.

## What it stages

| State | Where it lands | Renders as |
|---|---|---|
| Owner-typed alias | `Ishiro Honda` → `Honda-san` | chip with **no** badge |
| Provider-sourced alias | `Inoshiro Honda` → `本多猪四郎` | chip badged `TMDB` |
| RD6 near-duplicate | `Inoshiro-Honda` offered, **dropped** | nothing — that is the point |
| Collision | `Honda-san` offered to `Inoshiro Honda`, already held | review line + `identity_review_queue` |
| Suppression | `Honda Inoshiro` added, deleted, re-offered | still absent after re-apply |
| Entity-generic (RD8) | `Toho Fixture Co` → `TOHO Company Limited` | same panel on studio |

It prints the resulting state, read back through the same repo calls the API uses, so a
silent no-op is visible rather than assumed.

## Why it goes through the repo, not SQL

Every alias mutation calls `ApplyProviderAliases` / `AddEntityAlias` / `DeleteEntityAlias`.
A seed built from `INSERT` statements would encode what someone *believed* those states
look like and drift silently the moment a guard changes. Going through the writer means
the fixture is by construction exactly what production produces — the RD6 drop and the
collision skip are decisions the writer makes, not rows anyone can type.

Entity creation itself is direct SQL: *which* rows exist is uninteresting scaffolding,
unlike the alias state layered on top.

## Notes

- **Idempotent.** Re-running is safe; entities are `INSERT OR IGNORE`, and the alias
  writes are already idempotent by design.
- **Survives the orphan sweep** even with no video link: seeded people carry aliases,
  which counts as authored identity (ADR-072 §4), so the sweep skips them.
- **`-clean` leans on the triggers.** Deleting the entity row takes its aliases and
  suppressions (migration 0022's and 0044's `AFTER DELETE` triggers) and its
  `video_people` rows (`ON DELETE CASCADE`). `identity_review_queue` has no such trigger —
  its rows only self-heal in the read, which `INNER JOIN`s — so those are deleted by hand.
- **Invisible to CI.** Go ignores directories named `testdata` when matching `./...`, so
  this package is never built, vetted, or tested by the normal commands. It compiles only
  when named explicitly (`go build ./testdata/aliasseed`) — worth running after any change
  to the repo alias API, because nothing else will catch a break.
