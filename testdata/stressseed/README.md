# Stress fixture seeder (HOLODEX-342)

Builds a deliberately adversarial library — extreme text, adversarial images, relationship
counts from zero to absurd — so layout and UX bugs surface locally instead of reaching the
owner. See [`docs/specs/stress-fixture.md`](../../docs/specs/stress-fixture.md).

```bash
go run ./testdata/stressseed -mappings <the profile's metadata-mappings.yaml>
go run ./testdata/stressseed -big            # shorthand for -count 2000
go run ./testdata/stressseed -data ./data/x  # somewhere else
rm -rf ./data/stress                         # teardown — the whole fixture, at once
```

`-mappings` must point at the **same** mapping file the server will use — the one
`backend-stress` sets `METADATA_MAPPINGS_PATH` to. It defaults to that environment
variable, then to `./metadata-mappings.yaml`. See "The cast goes through the file layer"
below for why the fixture and the server have to agree on it; the seeder refuses to run,
before touching disk, if the mapping cannot carry a cast.

Then serve it with the **`backend-stress`** profile in `.claude/launch.json`, which points
the server at the same directory with `FILMS_ENABLED=true`. Stop that server before
re-seeding: it holds its own connection, and two writers on one SQLite file contend past
the busy timeout.

## The ladder

What varies is a **table** (`ladder.go`), not a program. Adding a dimension is a row;
adding a rung is an entry in that row. Each rung starts from a neutral baseline and
mutates exactly one axis — one factor at a time, never a cross-product (spec D3), so a
failure names the knob that caused it.

Each dimension reserves an **ID block** (spec D4), so inserting a rung renumbers only
within one block and never invalidates an assertion written against another. Supporting
entities — the people, tags and studios that exist only to be counted — are steered above
`poolBase` (9000), so an ID below it is always an addressed entity. Generation refuses if
a dimension overflows its block rather than quietly overwriting the next one.

Blocks are honoured by steering each table's `AUTOINCREMENT` counter before the rows are
written, so entities still go in through the ordinary repo API. Raw `INSERT`s with chosen
IDs would have been simpler and would have stopped the fixture exercising the write path
the app itself uses — tag folding, association rules, FTS triggers.

Every run clears the fixture's own rows first, so an address is a function of the table
alone. Two runs produce the same ids, names, URLs and links; `indexed_at` differs, because
`UpsertVideo` stamps it itself.

## `manifest.json`

Written to the fixture's data directory. `entities` resolves an id — "media 103" — to what
makes it special, and `by_dimension` enumerates every page sharing a dimension, which is
what turns "fix the page the owner noticed" into "check the fix everywhere it could be
wrong". Each entry carries the entity's full coordinate (`axes`), so an assertion can be
written against a threshold rather than a list of ids that goes stale.

## The cast goes through the file layer

`video_people` is a **derived** table (ADR-072). Its source is the video's resolved
person-typed fields, and `cmd/holodex` re-derives every video's links from that source at
startup (`backfillPersonLinks`).

So the fixture seeds a **file tag** the mapping maps to `actors`, not `video_people`
directly. The first version of this seeder wrote the links directly; they survived exactly
until the server booted, at which point the startup relink resolved each video to zero
actors, wiped all 50 links and orphan-stamped every person. Verified fixed — the backfill
now logs `pre_links=107 post_links=107` and changes nothing.

That is why `-mappings` matters: the mapping decides which tag carries the cast, so a
fixture built against a different mapping than the one serving it gets erased.

## Status

The skeleton (HOLODEX-343) and the ladder machinery (HOLODEX-344) are in. Two dimensions
ship — `people` cardinality and `text` torture — which is what proves blocks, OFAT, the
name encoding and the manifest end to end. The remaining dimensions are rows to be added:
relationship cardinality for tags/studios/films/scenes (HOLODEX-347), the full text
palette (HOLODEX-346), adversarial images (HOLODEX-345), collection breadth at `--count`
and `--big` (HOLODEX-350), and the enrichment profile (HOLODEX-348).

`-count` / `-big` are accepted and recorded, but nothing consumes them yet; the collection
filler is HOLODEX-350. `-seed` likewise: the ladder is fully determined by the table, so
there is nothing random to draw yet.

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
