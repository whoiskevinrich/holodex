# Stress fixture seeder (HOLODEX-342)

Builds a deliberately adversarial library — extreme text, adversarial images, relationship
counts from zero to absurd — so layout and UX bugs surface locally instead of reaching the
owner. See [`docs/specs/stress-fixture.md`](../../docs/specs/stress-fixture.md).

```bash
go run ./testdata/stressseed                 # seed ./data/stress
go run ./testdata/stressseed -big            # shorthand for -count 2000
go run ./testdata/stressseed -data ./data/x  # somewhere else
rm -rf ./data/stress                         # teardown — the whole fixture, at once
```

Then serve it with the **`backend-stress`** profile in `.claude/launch.json`, which points
the server at the same directory with `FILMS_ENABLED=true`. Stop that server before
re-seeding: it holds its own connection, and two writers on one SQLite file contend past
the busy timeout.

## Status

This is the skeleton (HOLODEX-343): isolated data directory, database, safety guard. It
seeds **no entities yet** — the ladder that generates them is HOLODEX-344, and the
enrichment profile HOLODEX-348.

## Isolation, twice over (spec D5)

The fixture never shares a `DATA_PATH` with a real library, and two independent mechanisms
keep it that way.

**Paths come from the flag, not the environment.** Config is `config.Defaults()` plus
`-data` — never `config.Load`, so no `DATA_PATH` env var, `.env`, or `holodex.yaml` can
redirect the seeder onto real data. Everything it writes derives from that one flag, which
is also what makes `rm -rf` a complete teardown.

**It refuses to write into a database it did not create.** The seeder marks its own
database with a `stress_fixture` table; an unmarked database is inspected **read-only** and
refused if any content table holds rows:

```
$ go run ./testdata/stressseed -data ./data
stressseed: data\holodex.db holds rows this tool did not create
(videos: 209, people: 72, studios: 8, tags: 31, films: 1).
```

An unmarked but *empty* database is claimed rather than refused — starting the
`backend-stress` server before the first seed creates one, and refusing that would be a
trap with no upside. Re-running with a different `-seed` is refused too: half a fixture
from each seed is reproducible from neither.

## Why `MEDIA_PATH` points at an empty directory

`backend-stress` sets `MEDIA_PATH=./data/stress/media`. Seeded rows have no files behind
them by design (spec D1 — the seeder bypasses the scanner), so a real media root would let
the scanner mix real media into the fixture. An empty one is safe in both directions: the
scanner walks it, sees zero files, and skips its end-of-scan deactivation sweep, so it can
never deactivate the seeded rows either.

## Notes

- **Invisible to CI.** Go ignores directories named `testdata` when matching `./...`, so
  this package is never built, vetted, or tested by `make test`. Run the guard's tests
  explicitly after touching it — nothing else will catch a break:

  ```bash
  go test ./testdata/stressseed
  ```

- **The marker is not a migration.** `stress_fixture` is created by this tool, and nothing
  in the server knows about it. It exists only to answer "did I make this?".
