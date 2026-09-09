---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-342                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # none — dev-time test fixture only, no user-facing behavior (chore is hidden from the changelog by cliff.toml)
---

# HOLODEX-342 · Dev-time stress fixture for UX and layout QA

Raised as "build a test server with 100 of each entity, maxed-out fields, and a fake enrichment
provider." **Discovery reframed the size of the job downward and the shape of it sideways.**

Three fixtures already exist and cover most of the mechanism: `testdata/demo/generate.mjs` renders
posters with sharp and muxes tiny MP4s, `testdata/enrich-stub/stub.js` is already a four-route
provider sidecar wired into `launch.json` with a `DELAY_MS` knob, and `testdata/aliasseed/main.go`
is the precedent for a Go seeder writing through `internal/repo`. So this is a **stress profile on
three existing things**, not a new subsystem — which is also the version that will not rot.

**The sideways move was the regression mechanism.** The brainstorm offered screenshot/visual
regression; what the owner actually described was *"I need to say 'media 123 has 12 people and the
headshots are unusably small' and have that not regress."* That is not a screenshot diff — it is a
**measurable invariant at a stable address**. Cheaper (no image byte-stability), more durable
(survives restyling), self-documenting, and it generalises by dimension instead of pinning one
page. It is also the only technique available: browser screenshots time out on Holodex, so
`getBoundingClientRect` + computed style is the established QA method here. That decision then
forced D4 — reserved ID blocks, so extending the fixture never renumbers an address an assertion
was written against.

**Three premises in the original brief were wrong, and each would have cost an afternoon:**

- **"Scenes" are not an entity** — `film_videos.scene_number` is a role a video plays inside a
  film. So "10 movies × 12 scenes" and "100 media" are the *same rows*. Turned into an advantage:
  a film's scenes are drawn from video rungs, so the film page inherits the torture free.
- **`FILMS_ENABLED` defaults false** and the routes 404 when off — half the fixture would have
  rendered empty with no error.
- **Lorem ipsum is the friendliest long text there is.** Short Latin words wrap beautifully; it
  only stresses vertical space. Retained as one rung, demoted from the premise.

Likewise "bright backgrounds" was the right instinct one notch too narrow: `app.css` hardcodes 2/3
poster, 1/1 headshot, 8/3 banner and `cropGeometry.ts` is keyed to those frames, so wrong aspect
ratios matter as much as brightness.

**Overlaps:** the enrichment profile stresses ADR-090's two layers and ADR-051's `SourceBadge` row;
films and scene numbering are ADR-085's; the three-skin obligation is ADR-021's.

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/stress-fixture.md` (D1–D9, ladder table, acceptance criteria,
  two open questions). Jira `needs-spec` label to clear once reviewed
- [~] architecture `architecture` — not applicable; dev-time seam, no production code path. The
  seeder writes through existing repo APIs. Revisit only if it needs a hook the server binary ships
- [~] design `design-handoff` — not applicable; no user-facing surface
- [ ] testing `testing-strategy` — `docs/testing-strategy.md` to describe the geometry-assertion
  harness and when to add an assertion (HOLODEX-349)
- [~] security `security-review` — not applicable; no auth, access, or production infrastructure
  change. The seeder's blast radius is bounded by D5 (isolated `DATA_PATH` + refuse-if-not-mine)
- [~] three-skin QA — not applicable to the fixture itself; it is the *instrument* for three-skin QA

## Up next — ordered (position = priority)

1. [x] [discovery] Map the injection seams before designing — entity inventory, scanner path,
   existing fakes, provider contract, asset paths, config gating
2. [x] [brainstorm] Settle injection layer, ladder range, collection size, enrichment scope, and
   the regression mechanism — all five decided with the owner
3. [x] [spec] `docs/specs/stress-fixture.md`
4. [x] [tracking] Epic HOLODEX-342 + children HOLODEX-343…350, branch renamed onto the key,
   Jira moved to In Progress
5. [x] [dev-tooling] **HOLODEX-343** — seeder skeleton, isolated `DATA_PATH`, refuse-if-not-mine
   guard, committed `backend-stress` launch entry with `FILMS_ENABLED=true`
6. [x] [dev-tooling] **HOLODEX-344** — declarative ladder table, OFAT generation, reserved ID
   blocks, `manifest.json`. Two dimensions ship (`people`, `text`) — enough to prove blocks,
   OFAT, name encoding and the manifest end to end; the rest are rows for the tickets below
7. [x] [dev-tooling] **HOLODEX-347** — relationship-cardinality ladder. Six dimensions now:
   `people`, `text`, `tags`, `studios`, `scenes`, `filmcast`. The studios rungs forced a new
   decision (D10, below): the fixture owns a committed `testdata/stressseed/mappings.yaml`
8. [x] [dev-tooling] **HOLODEX-346** — text torture palette. Nine dimensions now: the video
   half gained `overview` and two rungs the palette was missing (`bidi`, multi-codepoint
   `emoji`), and three derived dimensions address the person, studio and tag *names*
   (`persontext`, `studiotext`, `tagtext`). Two deliberate deviations, both below
9. [x] [dev-tooling] **HOLODEX-345** — adversarial image set. Thirteen dimensions now: one
   image dimension per picture-rendering kind (`videoimage`, `filmimage`, `personimage`,
   `studioimage`), seven rungs each. Two deviations from the ticket's AC, both below
10. [ ] [dev-tooling] **HOLODEX-350** — collection breadth at `--count` and `--big`
11. [ ] [enrichment] **HOLODEX-348** — enrichment stress profile, both ADR-090 layers
12. [ ] [testing] **HOLODEX-349** — geometry assertion harness + `docs/testing-strategy.md`
13. [ ] [review] First three-skin run against the fixture. If it finds zero unknown bugs, the
    fixture is not adversarial enough — treat that as a failure of the fixture, not a pass
14. [ ] [dev-tooling] **HOLODEX-352** — torture the person `role` string, *after* the UI
    renders one. Split out of 346: role is free text with no validation, so a rung would
    seed cleanly and find nothing — no component interpolates a role, `credited_roles` has
    no frontend consumer at all, and a role outside `actor`/`director` breaks the media
    page's remove control. Not a gate on this epic
15. [ ] [dev-tooling] **HOLODEX-351** — addressed `person → videos` / `studio → videos`
    filmography dimensions. Split out of 347: the reverse direction is currently *emergent*
    and has no "few" bucket for studios. Lowest priority, and not a gate on this epic — read
    the ticket's "why it was not just patched in" first, the cheap fixes all corrupt an axis
16. [ ] [—] Mark the PR ready once the spec is reviewed and the testing gate lands — **and in the
    same step sweep every completed child to `In Review` by hand.** CI transitions exactly one
    issue: `scripts/jira-transition.mjs:48` takes `extractKeys(BRANCH_REF)[0]` and calls
    `syncKeys({ keys: [key] })`, so only HOLODEX-342 (the branch key) ever moves. Nothing walks to
    children, and they would otherwise sit at `In Progress` forever
17. [ ] [—] On merge, sweep the completed children to `Done` the same way. Owner's decision
    (2026-09-08): children track the epic's PR lifecycle manually rather than going `Done` when
    their work lands, so `Done` keeps meaning "merged to main" even if a branch is abandoned

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-08 (last) · HOLODEX-345 — adversarial image set across four kinds
- skills: code-review
- **"Fully transparent PNG" is not a state this app can store, so the rung became the
  *flattening* instead.** Every ingest path runs `personimage.Normalize`, which
  re-encodes to JPEG; JPEG has no alpha and Go reads pixels premultiplied, so a
  transparent pixel is written as pure black *whatever colour sits under it* — proven
  by feeding it explicitly-white transparent pixels and getting rgb(0,0,0) back. A
  literal "transparent" rung would therefore have been a byte-identical duplicate of
  `black`. The rung now seeds a transparent-background PNG **source** and lets the
  normalizer do to it what it does to a real uploaded logo. That is the third instance
  of this epic's pattern — 347's "only real if the resolver can express it", 346's
  "only real if the UI renders it", and now **only real if the storage layer can hold it**.
- **Where that rung earns its place is narrower than it looks, and the narrowing is the
  useful part.** A black plate under `object-cover` fills the frame and is honestly
  indistinguishable from `black`. Under `object-contain` — the studio logo and icon, the
  film banner — it does not fill its well, so it reads as a logo that half-disappeared.
  Kept on the cover frames anyway so one rung key means one thing on every kind. Also
  corrected a claim I had written twice: **there is no light skin**; all three compute a
  near-black body background, which makes `black` the *quiet* failure rather than the
  obvious one.
- **The bytes go through the app's normalizer, and that decision did real work.** Passing
  each kind's own configured `*_MAX_DIMENSION` means the studio's 1000px cap reshapes the
  ratio rung's 1400×600 to 1000×428 — the size the running app would actually store, and
  the size the manifest now reports. Verified live: a 21/9 headshot and a 1/1 poster, the
  ticket's two named cases, at the addresses the manifest gives.
- **A monogram that states a size must state the stored one.** The first cut computed the
  label before the downscale, so the studio ratio rung would have carried "1400x600"
  burned into a 1000×428 file — a wrong answer in the one place a reader cannot check it.
  Fixed by giving the renderer and the label a single `geometry()` and pre-sizing inside
  the cap so `Normalize` has nothing left to resize.
- **Six mutations, one of which exposed a hole in my own test.** Deleting `clearImages`
  left the reseed test green, because it planted its stale file against a *person* image —
  whose id comes from an unsteered AUTOINCREMENT, so run two writes new rows at new ids
  and never looks there. The case that actually matters is the **video**: a thumbnail is
  addressed by video id alone and video ids are stable by D4, so a leftover file sits
  exactly where the next run's `missing` rung must not have one. Retargeted; all six
  mutations now fail with their intended diagnostic.
- Two findings the ladder's own guards caught: `nameAxes.Value` promised "the name as
  stored" but reported the text variant, which is false for a dimension that names its
  entity from the coordinate (HOLODEX-344's file-layer test failed on all 14 rungs); and
  the palette-exclusion report printed each derived kind twice once a kind had two
  dimensions.
- Handoff: `go run ./testdata/stressseed` seeds 81 entities across 13 dimensions; all 28
  image rungs and 62 image references verified through the live API after a boot —
  `none` references nothing, `missing` 404s on every slot, everything else 200s as JPEG.
  Next is HOLODEX-350 (breadth at `--count`/`--big`), then 348 (enrichment), then 349
  (the assertion harness — the last open gate).

### 2026-09-08 · HOLODEX-346 — text palette across every free-text field
- skills: code-review
- **A rung is only real if the UI renders it** — 347's "only real if the resolver can
  express it", one layer further out. The ticket named `role` as a free-text field to
  torture. It is free text with no validation, so it would have seeded cleanly and found
  nothing: no Svelte component interpolates a role string, `credited_roles` is serialized
  by the API with no frontend consumer at all, and a role outside `actor`/`director` makes
  the media page's remove control fail. A rung that cannot be seen but can break a button
  is worth less than no rung. Declined and filed as **HOLODEX-352**, to add when the UI
  renders roles.
- **Three of the six fields belong to entities the ladder could not address, and the fix
  inverted the numbering rather than renumbering.** A person, studio and tag each has a
  detail page where the name is the `h1`, but none can be created from a name alone — so
  every rung seeds a carrier video, and a carrier can only exist once the `videos` sequence
  has left the addressed range. By then the people/studio/tag sequences are past
  `poolBase` and `AUTOINCREMENT` cannot be rewound, so those blocks start at
  `derivedBase` (20000), *above* their own supporting cast. The obvious alternative —
  renumber the existing blocks downward to make room at the bottom — is the one thing D4
  promises never to do.
- **The palette does not fit every field, and the seeder now says so out loud.** An empty
  person or studio name is skipped by the reconcile; an empty tag is creatable through the
  repo but refused by the HTTP layer, so seeding one would show a state the app cannot
  reach; lorem's commas fracture a multi-field name, and its 1575 runes are over
  `model.MaxNameLen` for a tag. The exclusions are *computed* from those limits and printed
  with their cause on every run — a short list with no stated reason is indistinguishable
  from a bug. That also forced D2 to grow an escape hatch: `noEmptyRung`, a stated reason,
  which `validateLadder` demands and only "the app cannot reach it either" justifies.
- **Two rungs the palette claimed but never had.** `finds` promised "bidi bleed" while the
  only direction rung was pure Arabic — mixed LTR/RTL is a different failure (neutrals
  taking direction from the adjacent run) and is now its own rung. The emoji rung was 20
  single code points, not the multi-codepoint sequences the ticket asked for. And lorem sat
  at 1393 characters under a comment claiming 1500. All three are now asserted, because all
  three decay invisibly — the joiners and combining marks cannot be seen in the source line.
- **Two review findings were real and pre-existing.** `steer` wrote `sqlite_sequence`
  unconditionally, so a backward steer over existing rows was silent — the assumption every
  block rests on, never checked where it is used; it now refuses. And `reset` did not clear
  `identity_review_queue`, which every `resolveOrCreateByName` feeds through `FlagNearMiss`
  and which stores bare integer ids with no foreign key, so a stale suggested-merge row
  would outlive the entities that produced it and reappear against whatever later holds
  those addresses.
- Handoff: `go run ./testdata/stressseed` seeds 53 addressed entities across nine
  dimensions; verified through the API after a server boot — all 30 text-related addresses
  match the manifest, including the tag lowercase round-trip and the empty rung resolving
  to an *absent* overview rather than a blank one. Next is HOLODEX-345 (images), then 350
  (breadth), 348 (enrichment), 349 (the assertion harness — the last open gate).

### 2026-09-08 · HOLODEX-347 — relationship cardinality, films, scenes, film cast
- skills: code-review
- **A ladder rung is only real if the resolver can express it.** `studio` is a REPLACE
  field in both of the owner's mapping profiles, so the resolver returns exactly one
  value through `firstNonEmpty` — a "5 studios" rung would have resolved back down to 1
  and the manifest would have stated a cardinality the page never renders. That is the
  same class of lie as HOLODEX-344's wiped links, one layer up: not wrong data at a
  stable address, but a *number* at a stable address that nothing on the page agrees
  with. Found by reading the resolver rather than by seeding and looking.
- **So the fixture now owns its mapping** (new D10). `testdata/stressseed/mappings.yaml`
  is committed, `backend-stress` points the server at it, and `-mappings` deliberately
  stopped honouring `METADATA_MAPPINGS_PATH` — a shell that had exported it for the
  `backend` profile would otherwise silently redirect the fixture onto a mapping that
  collapses the ladder. The mapping is not a setting here, it is part of the contract.
- **The seeder refuses what it cannot express, and the maxima are derived from the
  table.** `loadFields` fails before touching disk on a missing file source *or* a
  replace field under a rung above 1, and `ladderDemands` is computed by applying every
  rung to the baseline — so raising a rung cannot leave the check behind.
- **HOLODEX-344's studio links were a latent version of the same bug and survived only
  by luck.** `backfillStudioLinks` skips outright when `StudioLinkCount > 0`, so the
  seeder's own rows suppressed the pass that would have deleted them; the resolved
  `studio` field on those pages was empty the whole time. Proved by deleting all 145
  person links, all 36 studio links and all 5 studios from a seeded database and
  rebooting: the server rebuilt every one, identically, from the file layer alone. That
  is the assertion worth keeping — a green suite only proves the seeder wrote what it
  meant to.
- **Declined the spec's own wording on scenes.** "Scenes are drawn from existing video
  rungs" reads as attaching the addressed rungs to films, which would put a film section
  on the `people=50` page and make a failure there unattributable — the exact loss D3
  exists to prevent. Scene videos are a pool above `poolBase` carrying the text palette
  instead, which is what that line was actually buying. Spec updated rather than quietly
  deviated from.
- Scene videos are *baseline* videos, not bare ones: a film's cast, studios and tags are
  each derived live from its attached videos, so a bare pool would have left three whole
  sections of the film page empty at every rung. Constant rather than varied, or those
  lists would grow with the scenes rung and reintroduce the same attribution loss.
- The scene pool consumes the `videos` sequence, and SQLite's AUTOINCREMENT counter only
  moves forward — so every video dimension must precede every film one. `validateLadder`
  enforces it, and the review caught that the first version tracked "have I seen a film
  dimension" in a string keyed on `dim.key`: a film row with an empty key would have made
  the sentinel indistinguishable from "none seen yet" and silently disabled the guard.
- Deliberate scope cut, filed as **HOLODEX-351**: `person → videos` and `studio → videos`
  stay *emergent*. They fall out of the forward ladder with a genuine 1/few/many spread
  for people and tags, but no "few" bucket for studios — and a zero rung is structurally
  impossible in that direction, because an entity with no links is orphan-stamped by the
  reconcile that maintains it. Every cheap fix corrupts an existing axis.
- Handoff: `go run ./testdata/stressseed` (from the repo root, no flags) seeds 31 addressed
  entities across six dimensions plus a 12-video scene pool, and survives `backend-stress`
  — verified through the API at every rung. Next is HOLODEX-346 (text palette), then 345
  (images), 350 (breadth), 348 (enrichment).

### 2026-09-08 · HOLODEX-344 — ladder table, OFAT, reserved blocks, manifest
- skills: code-review
- **The fixture destroyed itself the first time it was served, and only a live check
  caught it.** `video_people` is a *derived* table (ADR-072): `cmd/holodex` re-derives
  every video's links from the resolved file layer at startup. The first cut seeded the
  link table directly, so all 19 tests passed against a database that the startup relink
  then emptied — 50 links wiped, every person orphan-stamped. The fix is to seed the file
  tag the mapping maps to `actors` and let the derivation produce the links; the backfill
  now logs `pre_links=107 post_links=107` and changes nothing. **Generalisable:** for any
  table the server derives, a green test suite proves only that the seeder wrote what it
  meant to — booting the thing is the only test that matters.
- **That forced a real dependency: the seeder must read the same
  `metadata-mappings.yaml` the server gets.** The mapping decides which tag carries the
  cast (it picked `Artist` on the films profile, not `Cast`), so a fixture built against a
  different mapping is erased by the one serving it. Hence `-mappings`, defaulting to
  `METADATA_MAPPINGS_PATH`, and a refusal — before touching disk — if no person-typed
  field maps to a file tag.
- **Reserved ID blocks without a caller-chosen-ID escape hatch:** no repo method accepts
  one, and raw `INSERT`s would have stopped the fixture exercising the write path the app
  uses (tag folding, association rules, FTS triggers). Steering each table's
  `sqlite_sequence` before the block's rows land keeps the real API *and* the address.
  `sqlite_sequence` has no unique index, so it is delete-then-insert, not an upsert.
- **A mutation test deleted code rather than confirming it.** Removing the sequence rewind
  from `reset()` changed nothing — steering already set every sequence — so the rewind was
  redundant and the comment claiming it "makes the addresses stable" was false. Two further
  mutations (no block steering, per-person reconcile) failed loudly with the intended
  diagnostics, which is what makes the passing suite worth believing.
- Deliberate scope cut: two dimensions, not six. Blocks, OFAT, name encoding and the
  manifest are all proven by two; the rest are table rows belonging to 345/346/347/350.
- Handoff: `go run ./testdata/stressseed -mappings <profile's mappings>` seeds 14 entities
  in blocks 100-105 (people 0…50) and 200-207 (text: empty, unbroken, CJK, RTL, emoji,
  diacritics, lorem), writes `manifest.json`, and survives `backend-stress` — verified
  through the API. Next is HOLODEX-347; `video_studios` is derived exactly like
  `video_people`, so read `filelayer.go` before adding those rungs.

### 2026-09-08 · HOLODEX-343 — seeder skeleton, isolated DATA_PATH, safety guard
- skills: code-review
- **The guard has to allow the empty database, or it is a trap rather than a guard.** The
  obvious rule — refuse any database this tool did not mark — breaks the order people
  actually work in: starting `backend-stress` before the first seed creates an empty,
  migrated database, and refusing that teaches the owner to reach for `-force`. So an
  unmarked database is claimable only while every content table is empty. Tempting
  additions (`job_runs`, so "a server has run here" counts as foreign) would have
  reintroduced exactly that trap: the initial scan records a job run.
- **Verified the refusal against the real dev library, not a fixture of one.** Pointing the
  seeder at `./data` refused with `videos: 209, people: 72, studios: 8, tags: 31, films: 1`
  and left the file byte-identical (same mtime and size, WAL untouched) — which is the
  claim that actually matters, and the read-only inspection is what makes it true.
- **The empty `MEDIA_PATH` is load-bearing, not cosmetic.** Seeded rows have no files behind
  them, so a real media root would let the scanner mix real media in; an empty one makes
  the scanner see zero files and skip its deactivation sweep, so it cannot delete the
  fixture either. Confirmed in the running server's log.
- **Deviation from the ticket, deliberate:** the seed is plumbed, recorded in the marker and
  guarded against a mid-fixture change, but no `*rand.Rand` is constructed yet — there is
  nothing to draw from until the ladder lands. It gets built where it is consumed
  (HOLODEX-344).
- Handoff: `go run ./testdata/stressseed` seeds `./data/stress`; `backend-stress` serves it
  with films enabled (verified: `/api/v1/films` 200 rather than 404). Next is HOLODEX-344 —
  the ladder table, OFAT generation, reserved ID blocks and `manifest.json`.

### 2026-09-08 · brainstormed, reframed, specced
- skills: product-brainstorming, write-spec
- **The most valuable output of discovery was finding what already existed.** Three fixtures cover
  most of the mechanism; the brief described building them again. Checking before designing turned
  a new subsystem into a profile on existing code — and the existing code is the part that will
  stay maintained, which is the actual defence against fixture rot.
- **The owner's answer reframed a question I had framed wrong.** I offered "screenshot regression
  or not"; the real requirement was a stable *address* plus a *measurable invariant*, which is
  cheaper than either option I put up. Worth generalising: when an answer does not fit the options
  offered, the options were the wrong axis — re-derive rather than pick the nearest.
- **Every specific number in the brief was a maximum, and half the bug class is at zero.** The
  owner's own `1/5/…/50` ladder already had the instinct; extending it to 0 across every dimension
  was the single largest scope change, and HOLODEX-328 is the precedent that justifies it.
- Corrected one of my own claims mid-session: I reported `.claude/launch.json` as carrying a
  committed TMDB token in a public repo. It is gitignored at `.gitignore:58` and was never
  committed — and main had already resolved the broader concern in ADR-094 / HOLODEX-341. Flagging
  a suspected leak is right; asserting it before checking `git ls-files` is not.
- Handoff: branch `HOLODEX-342-stress-fixture`, Jira In Progress, epic + 8 children filed. Spec
  landed; no code yet. Next actionable is HOLODEX-343 (seeder skeleton) — it unblocks every other
  child, and its safety guard (D5) should exist before any seeding code runs.
