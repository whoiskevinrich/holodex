# Spec: Dev-time stress fixture for UX and layout QA

**Status**: Draft
**Phase**: Jira [HOLODEX-342](https://whoiskevinrich.atlassian.net/browse/HOLODEX-342) (Epic)
**Owner**: Project owner
**Date**: 2026-09-08

A deterministic, deliberately adversarial dev-time fixture that makes UX and layout bugs surface
locally — and gives the owner and the agent a shared, stable vocabulary for naming them.

**ADR**: none. This is a dev-time seam, not production architecture; the seeder writes through
existing repo APIs and adds no production code path. Revisit if it ever needs a hook the server
ships with.
**Design handoff**: not applicable — no user-facing surface.

---

## Objective

Today the only way to stress the UI is to point a dev server at a real media library
(`backend-amv` / `backend-films`) and hope it happens to contain a gnarly enough case. Real
libraries are **friendly**: reasonable tag counts, reasonable name lengths, well-behaved posters,
every field populated. The layout bugs that reach the owner are precisely the ones real data never
produced.

This fixture is deliberately **worse** than real data, and it is addressable: every extreme case
has a stable URL and a machine-readable description, so "media 123 looks wrong" is a complete bug
report rather than the start of a hunt.

---

## Scope

### In scope

- A Go seeder (`testdata/stressseed/`) that writes entity rows through `internal/repo` and drops
  pre-rendered images at the derived asset paths.
- A declarative ladder of extreme values across text, images, and relationship cardinality —
  covering the **empty** end as well as the maximum end.
- An isolated `DATA_PATH` and a committed `backend-stress` launch profile.
- A stress profile for the existing `testdata/enrich-stub/` provider, covering both ADR-090 layers.
- A geometry-assertion harness that turns an observed layout bug into a runnable invariant.

### Out of scope

- **Exercising the scanner, extraction, or thumbnail generation.** The seeder bypasses them by
  design (see D1). Those paths keep their existing coverage.
- **Screenshot / visual-regression diffing.** Superseded by D6.
- **A component gallery** (`/dev/kitchen-sink`). Deferred deliberately — a complement for pure
  layout work, not a substitute: no resolver, no real API shapes, no scroll perf.
- **Any production code path.** Nothing in this epic ships in the server binary.

---

## Decisions

### D1 — Injection: seed rows, drop images. Do not go through the scanner.

`repo.UpsertVideo` performs no filesystem check, and asset paths are deterministic
(`DATA_PATH/thumbnails/{id}.jpg`, `person-images/{personID}/{id}.jpg`, …). So the seeder writes
rows and writes image files directly, skipping `ffmpeg`, `ffprobe` and `exiftool` entirely.

**Rationale**: regeneration speed is the single variable that decides whether a fixture is used
daily or rots. Driving 100+ files through the real scanner is minutes; this is seconds.

**Accepted cost**: the scanner is not under test here. That is correct — this fixture targets the
*rendering* of data, not its *ingestion*.

**Rejected**: generating real 1-frame MP4s and letting the scanner ingest them (the
`testdata/demo/generate.mjs` approach). More faithful, far too slow to run casually, and it tests a
layer this epic is not about.

### D2 — The ladder runs from zero, not from the maximum.

Every dimension gets rungs at 0 / 1 / few / many / absurd.

**Rationale**: empty states are half the layout bug class — HOLODEX-328 existed because empty
Films/People sections needed handling. A fixture where everything is maxed finds maximum bugs and
ships empty-state bugs.

### D3 — One factor at a time. Never cross-product.

Vary one dimension at a time against a boring baseline; hold everything else neutral.

**Rationale**: the cross-product of 5 tag counts × 5 people counts × 6 text variants × 6 image
variants is 900 entities, and — worse — a failure cannot be attributed to a dimension. OFAT covers
every dimension in roughly 30 entities and tells you exactly which knob broke it. Cross-product
feels more thorough and is strictly less diagnostic.

### D4 — Addressing: reserved ID blocks, encoded names, machine-readable manifest.

Three layers, serving three readers:

| Layer | Reader | Form |
|---|---|---|
| Reserved ID blocks | the URL | `1xx` people-count, `2xx` text, `3xx` images, … |
| Encoded entity name | the owner's eyes, and search | `STRESS tags=30 people=05 text=cjk img=bright` |
| `manifest.json` | the agent | `id -> {dimension, value, variant, url}` |

**Rationale**: sequential IDs would renumber on every fixture extension, invalidating every
assertion previously written against them. Blocks make a rung insertion local. The manifest is what
lets the agent resolve "media 123" without reverse-engineering the seeder, and — more importantly —
enumerate *every* page sharing a dimension, so a fix can be checked for generality rather than
patched at the one example the owner happened to notice.

### D5 — Isolation is simultaneously the teardown and the safety guard.

The fixture gets its own `DATA_PATH` and its own committed `backend-stress` entry in
`.claude/launch.json.example`, never shared with `backend-amv` / `backend-films`.

**Rationale**: teardown becomes `rm -rf`, and a seeder that only ever writes inside its own tree
cannot damage a real library through an env-var mistake. Belt and braces: the seeder refuses to run
if the target DB contains rows it did not create.

**Related trap**: a worktree with no local `.claude/launch.json` silently previews the *main*
worktree's code with no error. The `backend-stress` entry must be committed to the example file, or
QA runs against the wrong build.

### D6 — Regression mechanism is measurable invariants, not screenshot diffs.

The loop this is designed around:

1. The owner looks at the fixture and describes a problem in plain language — *"media 123 has 12
   people and the headshots are unusably small."*
2. The agent resolves `123` through the manifest, measures the actual geometry with
   `getBoundingClientRect` and computed style, across all three skins.
3. The agent writes the finding back as an assertion — *on every page where `people >= 10`, each
   headshot's rendered width must be ≥ 40px* — which fails until fixed, and keeps failing if it
   regresses.

**Rationale**: this is what the owner actually asked for, and it is cheaper than the thing it
resembles. It needs no image byte-stability, survives restyling, reads as documentation, and
generalises by dimension rather than pinning one page. It is also the only technique available:
browser screenshots time out on Holodex, so computed-style and geometry measurement is the
established QA method here.

### D7 — Text: lorem ipsum is the friendliest long text that exists.

The 1500-character lorem case is retained for vertical overflow, but it is the *weakest* rung.
Short Latin words with spaces everywhere wrap beautifully. The rungs that actually break flex
containers are a 60+ character unbroken token, CJK, RTL, emoji, and combining diacritics.

Two of those are a pair rather than one rung each (HOLODEX-346). **Pure RTL** finds direction
and alignment bugs; **mixed LTR/RTL** finds bidi bleed, where neutral characters — digits,
brackets, an em dash — take their direction from whichever run they sit beside and land at the
wrong end of the line. Only the second produces that. Likewise **emoji** means the
*multi-codepoint* sequence — a ZWJ family, a two-codepoint flag, a skin-tone modifier — because
anything counting runes or sizing a line box per code point breaks there and not on a single
emoji. Each rung's defining property is asserted in `ladder_test.go`: the joiners and combining
marks are invisible in the source line, so nothing else would notice them being stripped.

### D11 — The palette reaches every free-text field the app renders, and no others (HOLODEX-346).

Not just the video title. A video also carries an **overview** — resolvable only, there is no
`videos.overview` column — and a person, studio and tag each has a detail page where its **name
is the `h1`**. All five get the palette.

Two consequences fall out of *how* the last three exist. None can be created from a name alone
(people and studios are reconciled from a video's file layer, tags are attached to a video), so
each rung seeds an unaddressed **carrier video**; and because a carrier can only be made after
the `videos` sequence has left the addressed range, their blocks sit **above** the supporting
pool at `derivedBase`, inverting the usual numbering. Renumbering the existing blocks downward
to avoid that was rejected: D4 promises an address never moves.

**The palette is filtered per field, by the platform's own limits, with the reason printed.** An
empty person or studio name is skipped by the reconcile so the entity would never exist; an
empty tag is creatable through the repo but refused by the HTTP layer, so seeding one would show
a state the app cannot reach; lorem's commas fracture a multi-field name into several entities,
and its length is over `model.MaxNameLen` for a tag. Those exclusions are computed rather than
listed, so changing the palette re-derives them. This also means the derived dimensions carry no
empty rung, which D2 otherwise forbids — the escape hatch is a stated reason on the dimension,
and "the app cannot reach the empty state either" is the only one that qualifies.

**`role` is excluded on different grounds: it is invisible.** It is free text with no
validation, so it would seed cleanly — but nothing in the frontend renders a role string, and a
role outside `actor`/`director` breaks the media page's remove control. A rung that cannot be
seen but can break a button is worth less than no rung. Parked as HOLODEX-352, to add when the
UI renders roles. **Generalising: a rung is only real if the UI renders it** — the same shape as
HOLODEX-347's "a rung is only real if the resolver can express it", one layer further out.

### D8 — Brightness is one image rung of seven, and the images go in through the app's normalizer.

Bright backgrounds catch text-over-image contrast — real, and retained. But `app.css` hardcodes
`2/3` poster, `1/1` headshot and `8/3` banner frames, and `cropGeometry.ts` is keyed to those
rules, so **wrong aspect ratios** matter as much as brightness. The set is `none`, `bright`,
`black`, `ratio`, `tiny`, `alpha`, `missing` — one dimension each for the four kinds that render a
picture (video, film, person, studio); a tag has none, because a tag is a chip and a heading.

Three things HOLODEX-345 settled that the original wording got wrong:

- **`none` and `missing` are two rungs, not one.** "No image" draws the UI's own empty state and
  issues no request. "Referenced but absent" is a row pointing at a file that is not there, and it
  fails *three different ways*: `EntityImageSlot` (studio, film) has no error handler at all and
  draws the browser's broken-image glyph, `PersonImageFrame` hides the `img` and leaves an empty
  well, and `VideoCard` retries five times with backoff before falling back to a play glyph. Only
  the second of those was in the original list.
- **"Transparent PNG" is not a state the app can store**, so the rung is the *flattening* instead.
  Every ingest path runs `personimage.Normalize`, which re-encodes to JPEG; JPEG has no alpha and
  Go premultiplies, so a transparent pixel is written as pure black whatever colour sits under it
  (asserted by `TestAlphaFlattensToBlack`). The `alpha` rung therefore seeds a transparent-background
  PNG *source* and lets the normalizer do what it does to a real uploaded logo. It differs from
  `black` only where the frame is `object-contain` — the studio logo and icon, the film banner —
  and it is kept on the cover frames anyway so one rung key means one thing on every kind.
- **Every skin is a dark ground**, so `black` is the quieter failure rather than the obvious one:
  a black plate reads as an absent image, not a broken one. (The spec's earlier draft of this
  section assumed a light skin to check against; there isn't one.)

**Consequence**: the seeder writes its bytes through `Normalize` with each kind's own configured
`*_MAX_DIMENSION`, so the stored size is the size the running app would store — a studio's 1000px
cap reshapes the `ratio` rung's 1400×600 to 1000×428, and the manifest reports what landed rather
than what was asked for. And because nothing cascades from the database to the asset directories,
a run clears them first: a video thumbnail is addressed by video id alone, video ids are stable by
D4, so a file left by an earlier run would sit exactly where the next run's `missing` rung must
not have one.

### D9 — Enrichment stresses both ADR-090 layers, config-first.

One `stub.js` process; several `metadata-sources.yaml` entries pointing at it on different paths.
Five "providers" therefore cost no extra processes.

- **Precedence (layer 2)**: 5+ namespaces returning *different* values for the same field, so the
  ADR-051 `SourceBadge` chip row has something to render. Five sources that agree teach nothing.
- **Adoption (layer 1)**: a 30-candidate `/resolve` response, candidates distinguished only by
  `disambiguation`, plus slow / 5xx / malformed-JSON providers so loading and error states are
  reachable on demand. This is groundwork for planned `EnrichPicker` UX work.


### D10 — The fixture owns its metadata mapping (HOLODEX-347).

`backend-stress` and the seeder both read a committed `testdata/stressseed/mappings.yaml`, rather
than the operator's gitignored `metadata-mappings.local.*.yaml`. `-mappings` still overrides both
halves together, and deliberately does **not** honour `METADATA_MAPPINGS_PATH` — a shell that had
exported it for the `backend` profile would otherwise redirect the fixture onto a different mapping
than the one serving it.

**Rationale**: the mapping is not a setting here, it is part of the fixture's contract. Two facts
force it. The mapping decides which file tag carries each derived link, and the server re-derives
`video_people` / `video_studios` from the file layer on every boot (ADR-072, ADR-053) — so a fixture
seeded against a different mapping than the one serving it has its links wiped rather than read.
And `studio` is a REPLACE field in both of the operator's own profiles, so the resolver returns
exactly one value through `firstNonEmpty`: the 5-studio rung below would silently collapse to 1, and
the manifest would state a cardinality the page never renders. `multi: true` on `studio` is a
legitimate configuration — `video_studios` has been many-to-many since migration 0017 and the media
detail page renders the list — it is simply not the shape either operator profile happens to use.

**Consequence**: the seeder refuses, before touching disk, if the mapping cannot express the ladder —
a missing file source, or a replace field under a rung above 1. The demanded maxima are derived from
the ladder table, so raising a rung cannot leave the check behind.

---

## Ladder dimensions

Rungs are illustrative; the authoritative list is the declarative table in the seeder.

| Dimension | Rungs | Finds |
|---|---|---|
| Video → people | 0, 1, 5, 10, 25, 50 | headshot shrink, row wrap, overflow |
| Video → tags | 0, 1, 5, 30 | chip wrapping, row height blowout |
| Video → studios | 0, 1, 5 | empty section, single-item layout |
| Film → cast | 0, 1, 5, 10, 25, 50 | shared tile sizing with the media page |
| Film → scenes | 0, 1, 6, 12 | scene badge, ordering, empty film |
| Video title + overview | empty, 1 char, 1500 lorem, 60-char unbroken, CJK, RTL, mixed bidi, emoji, diacritics | wrap, truncation, container overflow, bidi bleed |
| Person / studio / tag name | the same palette less `empty` and `lorem` (see D11) | heading wrap, the docked rename pencil, cast-tile and chip labels |
| Images (×4 kinds) | none, bright, black, ratio, tiny, alpha, missing | contrast, crop geometry, the three broken-image paths |
| Collection size | `--count` 100 default, `--big` ~2000 | pagination, virtualization, scroll perf |

**Note on scenes**: "scene" is not an entity — `film_videos.scene_number` is a role a video plays
inside a film.

A film's scenes are drawn from a **scene pool** above `poolBase`, *not* from the addressed video
rungs (HOLODEX-347, correcting this spec's first reading). Attaching the `people=50` video to a film
would put a film section on that page, so a layout failure there could be the cast or the
attachment — exactly the attribution loss D3 exists to prevent. The pool carries the text palette
instead, which is what "inherits the torture at no extra cost" was actually buying: the film's scene
list renders the empty title, the unbroken token, CJK, RTL, emoji, diacritics and lorem without any
addressed entity gaining a second varied axis.

Scene videos are otherwise plain baseline videos. A film's `cast`, `studios` and `tags` are each
derived live from its attached videos, so a bare pool would leave three whole sections of the film
page empty at every rung; baseline rather than varied, because a per-scene cast would make those
lists grow with the scenes rung and reintroduce the same attribution loss.

**Note on films**: `FILMS_ENABLED` defaults to false and the routes 404 when off. The
`backend-stress` profile must set it or half the fixture silently renders an empty page.

**Note on reverse cardinality**: `person → videos` and `studio → videos` are *emergent*, not
addressed — they fall out of the forward ladder, because `stress person 001` is in every rung with a
cast while `stress person 050` is in only one. That yields a genuine 1/few/many spread for people
(1 ×25, 2 ×15, 3 ×5, 4 ×3, 31, 32) and tags (1…32), but no "few" bucket for studios. Note also that
a **zero rung is structurally impossible** in this direction: an entity with no links is
orphan-stamped or pruned by the reconcile that maintains it, so D2 cannot apply. Addressed
filmography dimensions are HOLODEX-351.

---

## Acceptance criteria

1. `go run ./testdata/stressseed --seed 1` twice produces identical database rows.
2. The seeder refuses to run against a `DATA_PATH` containing rows it did not create.
3. `manifest.json` resolves any fixture entity ID to its dimension, value and URL, and supports
   enumerating all entities sharing a dimension.
4. Every ladder dimension above has a rung at 0 and a rung at its maximum. (Reverse
   cardinality is exempt and is not a dimension — see the note above.)
5. Detail pages for media, person, studio, film, tag and category all render at every rung in all
   three skins without console errors.
6. The enrichment stub yields at least five conflicting namespaces on a shared field, and at least
   one 30-candidate `/resolve` response.
7. The geometry harness runs against the manifest, evaluates at least one real invariant across all
   three skins, and reports the page, variant, selector and measured-vs-expected value on failure.
8. `docs/testing-strategy.md` describes the harness and when to add an assertion.

**Overall success criterion**: if the first three-skin run surfaces zero layout bugs that were not
already known, the fixture is not adversarial enough and needs another turn of the screw. A fixture
that passes on day one is a demo, not a test.

---

## Open questions

- **Where do assertions live?** A Go test, a Node script, or a standalone CLI. The constraint is
  that it must be runnable by the agent inside a session and by the owner from a terminal. Resolve
  in HOLODEX-349.
- **Does the fixture belong in CI?** Out of scope for now — it is a local tool. If the geometry
  harness proves stable it becomes a candidate, but the seeder's runtime and browser dependency
  need measuring first.

---

## Related

- [HOLODEX-342](https://whoiskevinrich.atlassian.net/browse/HOLODEX-342) — epic; children
  HOLODEX-343 … HOLODEX-350.
- ADR-090 — the adoption / precedence split that D9 stresses.
- ADR-051 — per-field source of truth; the `SourceBadge` chip row.
- ADR-085 — films entity, `film_videos.scene_number`, and `film_people_roles` as an
  asserted owner link with no reconciler.
- ADR-053 / ADR-072 — `video_studios` and `video_people` as derived tables; why the
  fixture seeds the file layer rather than the link tables.
- ADR-075 D3 — `video_tags` as an authored table, the one relationship that is not derived.
- `docs/design/theming.md` and `.claude/rules/frontend-theming.md` — the three-skin QA obligation
  this fixture is built to serve.
