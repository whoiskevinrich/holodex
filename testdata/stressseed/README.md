# Stress fixture seeder (HOLODEX-342)

Builds a deliberately adversarial library — extreme text, adversarial images, relationship
counts from zero to absurd — so layout and UX bugs surface locally instead of reaching the
owner. See [`docs/specs/stress-fixture.md`](../../docs/specs/stress-fixture.md).

```bash
go run ./testdata/stressseed                 # from the repository root
go run ./testdata/stressseed -big            # shorthand for -count 2000
go run ./testdata/stressseed -data ./data/x  # somewhere else
rm -rf ./data/stress                         # teardown — the whole fixture, at once
```

`-mappings` defaults to the fixture's own [`mappings.yaml`](mappings.yaml), which
`backend-stress` also passes the server as `METADATA_MAPPINGS_PATH` — so the two agree by
construction. It deliberately does **not** honour the `METADATA_MAPPINGS_PATH` environment
variable: a shell that had exported it for the `backend` profile would otherwise redirect
the fixture onto the operator's own mapping. See "Derived links go through the file layer"
below; the seeder refuses to run, before touching disk, if the mapping cannot express the
ladder.

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

## Derived links go through the file layer

`video_people` and `video_studios` are **derived** tables (ADR-072, ADR-053). Their source
is the video's resolved person- and studio-typed fields, and `cmd/holodex` re-derives every
video's links from that source at startup (`backfillPersonLinks`, `backfillStudioLinks`).

So the fixture seeds the **file tags** the mapping resolves `actors` and `studio` from, not
those tables directly. The first version of this seeder wrote the person links directly;
they survived exactly until the server booted, at which point the startup relink resolved
each video to zero actors, wiped all 50 links and orphan-stamped every person. Studios had
the same latent bug and survived only by luck — `backfillStudioLinks` skips outright once
any link exists, so the seeder's own rows suppressed the pass that would have deleted them.

Verified by deleting all 145 person links, all 36 studio links and all 5 studios from a
seeded database and rebooting: the server rebuilt every one of them, identically, from the
file layer alone.

**Tags are the exception.** `video_tags` is authored, not derived (ADR-075 D3) — only a
file-sourced rescan clears it, and the fixture has no files — so tags go in through
`AttachTagToVideo` and nothing re-derives them. Same for the film half: `film_videos` and
`film_people_roles` are asserted owner links with no reconciler at all (ADR-085 §2).

That is what `mappings.yaml` is for: the mapping decides which tag carries each link, so a
fixture built against a different mapping than the one serving it gets erased.

## Films, scenes and the scene pool

A "scene" is not an entity — `film_videos.scene_number` is a role a video plays inside a
film (ADR-085) — so the `scenes` dimension varies how many videos a film has attached.

Those videos come from a **scene pool** above `poolBase`, not from the addressed video
rungs, which is the obvious reading of the spec's "scenes are drawn from existing video
rungs". Attaching the `people=50` video to a film would put a film section on that page, so
a layout failure there could be the cast or the attachment — exactly the attribution loss
OFAT exists to prevent. The pool carries the **text palette** instead, which is what that
line was actually buying: a film's scene list inherits the full text torture (empty title,
unbroken token, CJK, RTL, emoji, diacritics, lorem) without any addressed entity gaining a
second varied axis.

Scene videos are otherwise plain baseline videos — 2 people, 1 studio, 3 tags. A film's
`cast`, `studios` and `tags` are all derived live from its attached videos, so a bare scene
pool would leave three whole sections of the film page empty at every rung. Baseline rather
than varied, because a per-scene cast would make those lists grow with the scenes rung and
a film-page failure could then be either cause.

`filmcast` is the other half and a different table: `film_people_roles` is authored per
film, with `billing_order` seeded so the cast list's order is deterministic rather than
incidental.

## Reverse cardinality is emergent, not addressed

`person → videos` and `studio → videos` are not dimensions. They fall out of the forward
ladder, because `stress person 001` is in every rung with a cast while `stress person 050`
is in only one:

| direction | spread across the fixture |
|---|---|
| person → videos | 1 (×25 people), 2 (×15), 3 (×5), 4 (×3), 31, 32 |
| tag → videos | 1 … 32 across 30 tags |
| studio → videos | 32, then 1 (×4) — **no "few" bucket** |

Two caveats worth knowing before writing an assertion against these. A **zero rung is
structurally impossible** in this direction: an entity with no links is orphan-stamped or
pruned by the very reconcile that maintains it. And the studio spread has no middle,
because studios 002–005 exist only to be counted by the single `studios=05` rung —
HOLODEX-351 covers addressed filmography dimensions if that middle turns out to matter.

## Status

The skeleton (HOLODEX-343), the ladder machinery (HOLODEX-344) and the relationship
cardinality ladder (HOLODEX-347) are in — six dimensions: `people`, `text`, `tags`,
`studios`, `scenes` and `filmcast`. The remaining dimensions are rows to be added: the full
text palette (HOLODEX-346), adversarial images (HOLODEX-345), collection breadth at
`--count` and `--big` (HOLODEX-350), and the enrichment profile (HOLODEX-348).

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
