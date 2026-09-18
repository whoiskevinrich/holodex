---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-406
status: in-progress
release_note: The Enrich picker now shows each candidate's picture — a headshot, poster, or studio logo — beside its name, so same-named people and same-title films can be told apart at a glance. Providers that don't send one get a monogram in the same spot.
---

# HOLODEX-406 · F63 — `candidates[].image_url`: candidate thumbnail in the resolve picker

Brainstormed 2026-09-16 from "make the People, Media, and Film pickers support an image". The
facts reshaped it: there is **one** shared `EnrichPicker.svelte` (person, media, film **and**
studio pages mount it), no image field exists on `Candidate` anywhere (core, TS, sidecar), and
the perimeter question was already answered by ADR-056 — `render: image_url` field values are
hot-linked from an `asset_hosts`-allowlisted host through `Service.ImageURLAllowed`. So: one
optional contract key, one more caller of an existing gate, one fixed 40 × 60 column on the row.
No ADR. Supersedes F61's P2-c non-goal, whose stated cost ("the first candidate-level field
Holodex has to *fetch*") does not apply to a rendered, never-fetched thumb.

**Decisions locked from a three-option mockup** (2:3 list · 32 px circle · poster grid): the 2:3
list, because a circle crops posters and a grid drops match strength, `detail`, and the
roving-tabindex list; hot-link over proxy; studios ride along in the same box via `object-contain`
on the logo plate — zero entity-kind branching; the sidecar picks the rendition (TMDB `w185`);
the thumb is layer-1 identity evidence (ADR-090), never an image adoption.

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/candidates-image.md` (F63: FR1–FR4, P2-a/b, AC 1–11, test
  notes per surface, Resolved Decisions table) **and** the contract amendment
  (`metadata-provider-contract.md` §2.3 example + `candidates[].image_url` row, §5 caps row, §6
  S7 note, §8 example)
- [~] architecture `architecture` — n/a: additive optional response key + an existing gate
  (ADR-056 `ImageURLAllowed`) gaining one caller; no seam, no migration, no new config key
- [x] design `design-handoff` — `candidates-image-handoff.md` + `candidates-image-mockup.svg`
  (four panels: person rows with/without image · F61-expanded film row, thumb pinned top · studio
  logo letterboxed + 404→monogram · slot anatomy) + numbered, verifier-tagged
  `candidates-image-qa-checklist.md`. **Idiom settled: `FilmsRow`'s 2:3 plate** (`w-10
  aspect-[2/3] rounded-theme bg-logo-plate` + `font-display text-sm font-semibold
  text-logo-plate-ink` monogram) with `object-contain` per `entity/CLAUDE.md`'s
  frame-follows-source-aspect rule; `ProviderIcon`'s square inline plate rejected. Rule recorded in
  `enrichment/CLAUDE.md`. Flagged for the testing gate: the slot sets a 76 px row floor that masks
  F61's `py-1` mutation in `collapsed-detail-row-costs-one-line` — re-base to equality or assert
  the text block; add an x-offset parity assertion (AC4)
- [x] backend — `Candidate.ImageURL` (`enrich.go`), `sanitizeImageURL` beside
  `sanitizeProfileURL` calling `assetHostAllowed` (the core `Service.ImageURLAllowed` wraps —
  one gate, one more caller); `sanitizeCandidates` now takes the `Source`; over-cap **cleared,
  not truncated**; `Fake` gains `FakePerson.ImageURL`. Tests: `candidate_image_test.go` — the
  16-row riskiest-assumption table (base/asset host kept; http-on-CDN, foreign, suffix- and
  prefix-spoof, ftp/javascript/data, protocol-relative, relative, malformed, empty, over-cap all
  cleared; control chars stripped) + `TestServiceResolveGatesImageURL` (**mutation-tested**: gate
  removed ⇒ foreign host survives), + `TestEnrichVideoResolve_ImageURLGatedOnTheWire` in
  `internal/api` (own host round-trips; foreign host's key is **absent** from the JSON). Person
  and film handlers pass `res.Candidates` through unchanged, so the video handler test is the
  wire proof for all four. `go test ./...` green (26 pkgs); `/code-review high` clean
- [x] sidecar — `tmdbThumbURL` (w185, empty→empty) beside `tmdbImageURL`; `candidate.ImageURL`
  `omitempty`; `tmdbPerson.ProfilePath` + `movieSearchEntry.PosterPath` added (search results
  never decoded them); wired into all 8 builders (person search/find/by-id, movie
  search/find/by-id, company search/by-id). `TestTMDBResolveImageURL` covers person (search +
  by-id), movie (with and without poster → key absent, asserted on the JSON), studio.
  `metadata-sources.yaml.example` now says `asset_hosts` also gates browser-rendered images
- [x] frontend — `EnrichCandidate.image_url?` (`types.ts`); pure `candidateImage.ts`
  (`showThumb`: http(s) `image_url` and not failed — `isHttpUrl` belt-and-suspenders like
  `profile_url`) + 4 vitest cases; `EnrichPicker.svelte`: `<li>` → `flex items-start gap-3`,
  `aria-hidden` slot `w-10 aspect-[2/3] bg-logo-plate overflow-hidden` first child with
  `<img alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer" object-contain>` or
  the `FilmsRow` monogram span, per-row `failed` map (the `open`-map lifecycle, reset on edit /
  response / error), F61 internals wrapped byte-for-byte in `div.min-w-0.flex-1`. Stub: rectangular
  `solidPng`, `/p/<slug>/thumb/{portrait-N,wide}.png` route (anything else 404), `twins` rows walk
  every slot state (4 portraits · wide · 404 · foreign host · no key), `flood` all 25 pictured.
  **Live QA on the studio page (AMV testbed has no people; same shared picker), all three skins:**
  8/8 rows one slot 40 × 60, portraits `naturalWidth/Height` 40/60, wide logo 64/16 in the same
  box with `object-fit: contain`; rows 5 (404), 6 (foreign) and 7 (no key) on the monogram `H`;
  the wire (`/studios/1/enrich/resolve`) carries `image_url` on rows 0–5 and **no key** on 6–7;
  label x = 98.4 on every row; slot top == label top before and after F61 expand, expanded `<ul>`
  left == label left; no slot tab stop (`trapTab` list unchanged), slot has no handler — clicking
  the image applied the candidate and neither `/thumb/` nor `image_url` appears in the page after;
  `flood` 25 rows all exactly **76 px** (F61 bound [70, 76] still holds, no-detail row 7 also 76),
  last row inside the scroll box, dialog overflow 0, all 25 thumbs requested at render (Chrome's
  lazy margin covers the list — recorded, not asserted); monogram-on-plate contrast 12.17 / 13.02
  / 15.27 (Cinémathèque / Broadcast / Brutalist), radius 2 / 0 / 0, `font-display` per skin;
  375 px: dialog 343, text block 232, label truncates, match text visible, h-scroll 0.
  `npm run check` 0 errors, vitest 298/298, `/code-review high --fix` one stale comment fixed
- [x] testing `testing-strategy` — `docs/testing-strategy.md`: header entry, §4 backend row
  (16-row gate table, mutation-tested service round-trip, wire shape, sidecar), §5 picker row (the
  live twins/flood/three-skin/375 px numbers as the record), three Critical-invariants bullets
  (rendered-never-fetched-never-stored; slot always 40 × 60; broken image → monogram), §10
  adversarial block, §11 gaps, §12.4 rows, §12.5 gap. **Geometry harness**:
  `collapsed-detail-row-costs-one-line` re-based to the equality **[76, 76]** (the slot makes 76 a
  floor); new `collapsed-detail-text-block-costs-one-line` (**60**, exact — restores the `py-1`
  sensitivity the floor swallowed); new `candidate-slot-is-40-wide-on-every-row` (`applies: each`,
  `atLeast: 25` — x-offset parity by construction, the harness has no cross-element metric). All
  three 9/9, every other picker assertion re-run green (63 loads). **Mutation runs**: `py-1`
  dropped ⇒ text-block fails, row passes (as predicted); bare `<div>` actions line ⇒ row 77 fails;
  `w-10` dropped ⇒ slot assertion fails — **but only after the stub's portraits went from 40 × 60
  to 80 × 120**: a thumb the exact size of its slot let the box borrow the image's width and the
  mutation passed. `aspect-[2/3]` dropped on a pictured row is NOT caught (the image supplies the
  2:3) — recorded in the `finds` text and §12.5, not claimed. F61's stale §10 figures (60–68 /
  74–76) corrected to the re-based numbers
- [x] security `security-review` — the allowlist now gates a second browser-rendered surface;
  confirm `sanitizeCandidates` is upstream of every resolve handler (person, video, film, studio)
  and that nothing is fetched server-side

## Up next — ordered (position = priority)

1. [x] [S] Draft PR #346 opened with the spec gate; `needs-spec` cleared.
2. [x] [M] `/design-handoff` landed (handoff + SVG + QA checklist); `needs-design` cleared.
2a. [x] [—] QA §4.6 is the one `[human]` taste call: 40 × 60 slot (76 px rows) vs 32 × 48 — Kevin kept 40 × 60 (2026-09-17).
3. [x] [M] Backend + sidecar gates (FR1/FR2/FR4) with the sanitizer table.
4. [x] [M] Frontend FR3 + stub personas + three-skin QA.
5. [x] [S] `/testing-strategy` landed (three §12 assertions, all mutation-tested).
5a. [x] [S] `/security-review`, clear `needs-security-review`.
6. [ ] [S] Mark ready → CI fires In Review. Post-merge: contract-sync note lands downstream in the
   sidecar repo via its contract-watch skill (never from this branch).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-17 · security review, §4.6 decided, mark ready
- skills: security-review (identification subagent; zero findings ≥ Medium, so no filter pass)
- `/security-review` clean: `sanitizeImageURL` → `assetHostAllowed` is exact-host + scheme-checked
  (userinfo / port / suffix / `javascript:` / control-char cases all fail closed), sits at the one
  `Service.Resolve` chokepoint above all five resolve handlers, nothing is fetched server-side,
  `image_url` never reaches the activity log, and the only sink is a plain `<img src>` binding
  with `referrerpolicy="no-referrer"`. `needs-security-review` cleared.
- QA §4.6 settled from a live-measured show_widget mockup (three skins, both sizes): 32 × 48 lets
  the 52 px text stack set the row again (68 px full / 36 px sparse) and thins a wide logo's
  letterbox to ~13 px; Kevin kept 40 × 60, so the `[76, 76]` equality stands.
- Handoff: every gate green; PR #346 marked ready → CI fires In Review. Nothing is open on the
  branch. Post-merge, the contract-sync note lands downstream in the sidecar repo via its
  contract-watch skill — never from here.

### 2026-09-17 · design handoff
- skills: design-handoff (Explore subagent for the two plate idioms + F61 handoff conventions), code-review (high --fix, clean), code-review, testing-strategy
- Settled the open design question from the source, not a card: `FilmsRow`'s 2:3 tile is the
  slot shape; `ProviderIcon`'s plate is a square inline icon. One deviation — `object-contain`
  — because `entity/CLAUDE.md` says frames follow source aspect unless ingest gates it.
- Committed `candidates-image-mockup.svg` (960 × 1040, Cinémathèque, gold dashed = new),
  `candidates-image-handoff.md` (placement markup, content/state tables, a11y, responsive at
  512 / 343 px, edge cases, implementation notes), and the QA checklist (§1 personas `faces` /
  `broken` / `logos` / `flood` / `hostile`; 6 smoke, 14 agent, 6 human). Spec's open question
  struck through; `enrichment/CLAUDE.md` gains the ungated-aspect rule.
- Found on the way: the 76 px row floor neutralises the F61 assertion's `py-1` mutation —
  noted for `/testing-strategy`.
- Backend + sidecar built in the same session: `sanitizeImageURL` + 16-row table +
  service/API round-trips (gate mutation-tested); TMDB emits `image_url` at w185 from all 8
  builders. `go test ./...` green, `/code-review high --fix` no findings.
- Frontend built and verified live (studio page, three skins, mobile) — the four slot states
  live on the existing `twins` rows instead of three new personas (no registry churn). QA
  checklist §1.2 / §3.6 / §3.11 reconciled with what was measured; screenshots worked this time.
- Testing gate: strategy doc updated across §4/§5/invariants/§10/§11/§12; harness re-based +
  two assertions added, four mutations run. Found on the way: the stub's 40 × 60 portraits were
  the slot's own size and masked the `w-10` mutation — now 80 × 120 with the reason in `stub.js`.
  Seeded `data/stress` and added `backend-stress` to the local launch.json for the harness.
- Handoff: six gates green on Draft PR #346; only `/security-review` (clear the label) and mark
  ready remain. Local `.claude/launch.json` gained `backend-stub`, `enrich-stub`, `backend-stress`
  — gitignored, per-worktree.

### 2026-09-16 → 17 · brainstorm, story filed, spec + contract amendment written
- skills: product-brainstorming (Explore subagent for the picker/contract/perimeter facts, three-option
  show_widget mockup with a stressed-state strip, decisions via cards), write-spec
- Brainstorm reframed "three pickers" as one shared component and "fetch through the perimeter"
  (F61's reason to defer) as "render through ADR-056's gate". Kevin chose: 2:3 thumb list,
  hot-link via allowlist, studios ride along. Set aside: hover/zoom, current-image-in-header,
  proxy/cache, size negotiation.
- Filed HOLODEX-406 with the gate-status checklist and `needs-spec` / `needs-design` /
  `needs-security-review`; renamed the branch to `HOLODEX-406-picker-candidate-thumb`; fired
  In Progress.
- Wrote `docs/specs/candidates-image.md` (F63) and the four contract edits (§2.3 row + example,
  §5 caps row, §6 S7 note, §8 example).
- Handoff: spec gate green, nothing else built. Next session opens the Draft PR (if this one
  didn't) and runs `/design-handoff` — the monogram-plate idiom is the one design question open.
