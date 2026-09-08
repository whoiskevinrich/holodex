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
5. [ ] [dev-tooling] **HOLODEX-343** — seeder skeleton, isolated `DATA_PATH`, refuse-if-not-mine
   guard, committed `backend-stress` launch entry with `FILMS_ENABLED=true`
6. [ ] [dev-tooling] **HOLODEX-344** — declarative ladder table, OFAT generation, reserved ID
   blocks, `manifest.json`
7. [ ] [dev-tooling] **HOLODEX-347** — relationship-cardinality ladder (people, tags, studios,
   films, scenes)
8. [ ] [dev-tooling] **HOLODEX-346** — text torture palette
9. [ ] [dev-tooling] **HOLODEX-345** — adversarial image set
10. [ ] [dev-tooling] **HOLODEX-350** — collection breadth at `--count` and `--big`
11. [ ] [enrichment] **HOLODEX-348** — enrichment stress profile, both ADR-090 layers
12. [ ] [testing] **HOLODEX-349** — geometry assertion harness + `docs/testing-strategy.md`
13. [ ] [review] First three-skin run against the fixture. If it finds zero unknown bugs, the
    fixture is not adversarial enough — treat that as a failure of the fixture, not a pass
14. [ ] [—] Mark the PR ready once the spec is reviewed and the testing gate lands

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-08 (last) · brainstormed, reframed, specced
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
