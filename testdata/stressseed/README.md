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
`poolBase` (9000), so an ID below it is always an addressed video or film. Generation
refuses if a dimension overflows its block rather than quietly overwriting the next one.

Every dimension must also carry an **empty rung** (spec D2), and `validateLadder` refuses
one that does not. The single escape hatch is `noEmptyRung`, a stated reason, and the only
legitimate reason is that the *app* cannot reach the empty state either — see the derived
kinds below.

## Derived kinds are addressed above the pool, not below it

A person, studio and tag each has a detail page where its **name is the `h1`**, so the text
palette has to reach those names and not only the video title. But none of the three can be
created from a name alone: people and studios are reconciled from a video's resolved file
layer, tags are attached to a video, and a studio that loses its last link is deleted
outright. So every rung of `persontext` / `studiotext` / `tagtext` seeds a **carrier video**
to hang its entity off — a pool entity that exists only so the addressed one can, carrying
just the entity under test and deliberately not addressed itself.

That inverts the numbering. A carrier can only be made once the `videos` sequence has been
steered past every addressed video block, and by then the people, studio and tag sequences
have long since passed `poolBase` — the cardinality rungs created their supporting entities
on the way. `AUTOINCREMENT` cannot be rewound, so these blocks start at **`derivedBase`
(20000)**, above their own supporting cast. The alternative was renumbering every existing
block downward to make room at the bottom, which is the one thing D4 promises never to do.

Two consequences worth knowing before adding a dimension: every video dimension must come
first in the table (`validateLadder` enforces it), and the entity is read back after the
carrier is written rather than assumed — for people and studios the row is produced by the
server's own *derivation*, so reading it back is the only proof the derivation ran.

## Not every rung fits every field

A derived entity's name cannot carry the whole palette, and the seeder prints what it
dropped and why on every run rather than leaving a short list to look like an oversight:

| rung | person / studio | tag |
|---|---|---|
| `empty` | the reconcile skips an empty name, so the entity is never created | the repo would insert one, but the HTTP layer refuses it — seeding it would show a state the app cannot reach |
| `lorem` | the resolver splits a multi field on `,;/\n`, so the name would fracture into several entities | 1575 runes is over `model.MaxNameLen` (200), which the repo rejects with `ErrTagNameTooLong` |

The exclusions are computed from those limits rather than written down, so changing the
palette re-derives them and changing a limit follows it. Tag values are lowercased in the
palette because `resolveOrCreateByName` lowercases a tag on the way in (the "fox"/"Fox"
fix) — seeding the mixed-case form would store something other than what the manifest says.

**`role` is deliberately not tortured.** `video_people.role` and `film_people_roles.role`
are free text with no validation, so a rung there would seed cleanly — and find nothing:
no Svelte component renders a role string, `credited_roles` has no frontend consumer at
all, and a role outside `actor`/`director` makes the media page's remove control fail with
"has no role set on this video". A rung that cannot be seen but can break a button is worth
less than no rung. Tracked as HOLODEX-352, to add if the UI ever renders roles.

Blocks are honoured by steering each table's `AUTOINCREMENT` counter before the rows are
written, so entities still go in through the ordinary repo API. Raw `INSERT`s with chosen
IDs would have been simpler and would have stopped the fixture exercising the write path
the app itself uses — tag folding, association rules, FTS triggers.

Every run clears the fixture's own rows first, so an address is a function of the table
alone. Two runs produce the same ids, names, URLs and links. The stamped timestamps do
differ — `indexed_at`, because `UpsertVideo` sets it itself, and `fetched_at` on the
enrichment rows, because `UpsertEnrichment` does; neither takes a value from the caller.

## The enrichment dimension

The `enrich` rungs are the one axis that is neither a link nor a column: they seed the
ADR-090 **precedence** layer, several provider namespaces holding *different* values for
one field, so the ADR-051 `SourceBadge` chip row has something to choose between. After a
bare `go run ./testdata/stressseed`:

| id | rung | what the media page shows |
|---|---|---|
| 900 | `00` | no provider chips at all — the un-enriched state, which is its own layout branch |
| 901 | `01` | one provider. `SourceBadge` shows **no** affordance below two selectable chips, so this is the boundary, not a smaller five |
| 902 | `05` | five competing chips on `Tagline` and `Language` |

The namespaces come from [`../enrich-stub/personas.json`](../enrich-stub/personas.json) —
the same table the stub serves, so a seeded value and a live re-enrich agree. Two copies
would drift, and a drifted copy means the fixture states a value the page stops rendering
the moment anyone hits Refresh.

**The two surfaces are wired differently, and only one needs the mapping.** A person's
field set is a hardcoded Go list (`personScalarFields`) and `personProviders` unions every
provider that has a stored row, so a person's chip row grows from the rows alone. A
*video's* comes from the mapping — a provider is a candidate only if `<name>:<field>`
appears in that field's `sources:` list — which is why `mappings.yaml` carries provider
namespaces on `tagline` and `original_language`, and why the seeder refuses a mapping that
names none. It also refuses a provider-sourced field marked `multi: true`: the resolver
only builds a candidate list for a replace field, so a merge field renders no chip row at
all and the namespaces would be stored and invisible.

`tagline` and `original_language` were chosen because no other dimension owns them. The
ladder already tortures `title` and `overview` for text and `studio`/`actors`/`genres` for
cardinality, so conflicting on any of those would leave a failure un-attributable.

**Only the precedence half is seeded.** A candidate list exists only during a resolve, so
the *adoption* half — the 30-candidate flood, the identical-label twins, the slow/5xx/
malformed providers — is reachable only against the running stub. See its
[README](../enrich-stub/README.md).

`entity_enrichment` is therefore one of the fixture's own tables: every run clears it, and
because `reset()` deletes it, `inspect()` counts it — a database holding nothing but
enrichment rows is somebody's shadow store and is refused, which
`TestClaimCoversEverySeededTable` enforces.

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

`sources.yaml` is the same argument one layer out: the fixture owns its **provider
registry** too, because "five competing chips" is only an address if five namespaces are
configured — and it would not be, if `backend-stress` loaded whichever gitignored
`metadata-sources.yaml` the operator happened to have. The seeder never reads that file;
the server does, via `METADATA_SOURCES_PATH` in the `backend-stress` profile.

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

## The breadth pool (`-count` / `-big`)

The ladder is the *depth* axis: it tortures one entity until a layout breaks. `-count` is
the *breadth* axis, and it finds a different bug class — pagination, virtualization and
scroll perf. A 50-person cast finds a wrapping bug on one page; two thousand people find
that `/people` has no pagination at all.

```
go run ./testdata/stressseed            # 100 of every kind (default)
go run ./testdata/stressseed -big       # 2000 of every kind — ~150s, ~160MB
go run ./testdata/stressseed -count 0   # the ladder alone, for a fast reseed
```

Every kind the app has a list page for is populated, not just media: videos, people,
studios, tags, films **and categories** — the last being the one entity no dimension
addresses, so this is the fixture's only coverage of it.

Bulk entities are **pool** entities, not addressed ones. They are named
`stress bulk <kind> NNNN`, they live in `[9000, 20000)` outside every reserved block, and
the manifest records them as a range and a count rather than one by one — because no
individual bulk entity matters, only how many there are. Adding a breadth pool therefore
never moves an addressed id, which `TestBreadthDoesNotMoveAddressedEntities` asserts
against a `-count 0` run.

They are also deliberately *boring*: one person, one studio and one tag per video, a
well-formed mid-tone image, a short plain name. That is D3 applied to the breadth axis —
if `/people` is slow at 2000 rows, the only variable that could have caused it is 2000.

`-count` is capped at `breadthCeiling()` (10950 today: the pool gap less the ladder's own
draw) and refused in `run()` before the database is opened, so a typo cannot cost you the
fixture it was about to decline to replace.

## Status

Every dimension is in. The skeleton (HOLODEX-343), the ladder machinery (HOLODEX-344), the
relationship cardinality ladder (HOLODEX-347), the text palette (HOLODEX-346) and the
adversarial image set (HOLODEX-345) give thirteen dimensions: `people`, `text`, `tags`,
`studios`, `scenes`, `filmcast`, `videoimage`, `filmimage`, `persontext`, `studiotext`,
`tagtext`, `personimage` and `studioimage`. The palette reaches every free-text field the
app renders; the image rungs reach every picture-rendering kind. Collection breadth
(HOLODEX-350) adds the population axis above.

Still to come: the enrichment profile (HOLODEX-348) and the geometry-assertion harness
(HOLODEX-349).

`-seed` is accepted and recorded but draws nothing yet: the ladder and the breadth pool are
both fully determined by the table and `-count`, so there is nothing random to seed.

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
