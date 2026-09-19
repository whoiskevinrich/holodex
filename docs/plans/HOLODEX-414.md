---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-414
status: in-progress
release_note: The Enrich picker's candidate pictures now match what you're matching — a widescreen still for a media file, the poster for a film, a headshot for a person, and the logo for a studio — instead of one portrait box for everything.
---

# HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo)

Follow-up to HOLODEX-406 / F64 (PR #346), which gave every picker row one fixed 40 × 60 slot
with deliberately no entity-kind branching. Kevin's ask (2026-09-18): people keep profile
posters, films keep film posters, **media uses a backdrop-compatible (16:9) thumbnail, studios
use the studio logo**. Same slot, same plate, same monogram, same error handling — only the box
shape follows the kind, and the sidecar sends `backdrop_path` (w300) on the video path.

**Decisions locked from a two-option mockup** (A: slot height locked at 60, width follows the
kind · B: wide slots locked at 80 wide, height follows): **A** — 40 × 60 / 108 × 60 / 120 × 60,
every row stays 76 px, because 406 §4.6 already ruled against handing row height back to the
text stack and thinning a logo's letterbox. **Poster fallback** for a media candidate with no
backdrop (letterboxed in the 16:9 box), not monogram-only. Explicit `w-* h-15` pairs replace
`aspect-[2/3]` so the img can never widen the box (the 406 fixture-size trap). New required
`entityType` prop on `EnrichPicker`; five mounts (four detail pages + `EnrichQueueRow` via
`row.entity_type`). No `asset_hosts` change — backdrops are on the same TMDB host — so no
security-review gate.

## Gates — definition of done

- [x] design `design-handoff` — `candidates-image-per-kind-handoff.md` +
  `candidates-image-per-kind-mockup.svg` (person/film unchanged · media backdrop / poster
  fallback / monogram · studio wordmark / symbol · slot anatomy with px) + verifier-tagged
  `candidates-image-per-kind-qa-checklist.md`; supersession pointer added atop the 406 handoff.
  Mockup reviewed inline by Kevin before any of it was written (A + poster fallback chosen).
- [x] spec `write-spec` — amended `docs/specs/candidates-image.md`: header amendment note, FR3
  (kind-shaped box table, required `entityType` prop, explicit `w-* h-15`), FR4 (rendition per
  `entity_type` table + video/film Given/When/Then), AC4/7/9, sidecar + geometry test notes,
  Resolved Decisions 7–9 (kind-shaped · height locked · poster fallback); contract §2.3
  `image_url` row now carries shape guidance per `entity_type` and target widths (185 / 300)
- [~] architecture `architecture` — n/a: presentation rule + one sidecar field; no seam
- [x] sidecar — `resolveMovie` / `searchMovie` / `findMovieByIMDB` take `entityType`;
  `movieSearchEntry.BackdropPath` (the find result reuses that struct; `movieDetails` already had
  it); `movieThumbURL(entityType, backdrop, poster)` — video → w300 backdrop else w185 poster,
  film → poster. `TestTMDBResolveImageURL`: search (backdrop · neither · poster-only fallback),
  film search, by-tmdb-id and by-imdb-id for both kinds; fixtures carry `backdrop_path` on all
  three movie responses. **Mutation-tested**: dropping the `entityType == "video"` guard fails
  all three film paths. `go test ./providers/tmdb` green; `/code-review high` clean
- [x] frontend — `slotShape` / `SLOT_CLASS` in `candidateImage.ts` (+4 vitest); `EnrichPicker`
  required `entityType` prop, slot `{SLOT_CLASS[slotShape(entityType)]}` replacing
  `aspect-[2/3] w-10`; five mounts (four pages as literals, `EnrichQueueRow` from
  `row.entity_type`) — dropping the prop from one mount is a `npm run check` error (verified);
  stub `landscape-N` 300 × 169 rendition, `twins` reads `entity_type` (video → backdrops ×4 +
  poster fallback, no new persona); `enrichment/CLAUDE.md` rule. **Live QA against the stub
  (backend-stub :7802 / web-stub :5174 — 7800/5173 belong to another chat):** media picker
  108 × 60 on all 8 rows, label x parity, backdrops fill, poster letterboxes, 404/foreign/none →
  monogram, 76 floor on the no-detail row; studio picker 120 × 60, 64 × 16 wordmark → 120 × 30
  letterboxed; three skins contrast 12.17 / 13.02 / 15.27, radius 2 / 0 / 0 (F64's numbers);
  375 px: no overflow, slot intact, text block **151** (studio) — the name gets 61 px before its
  ellipsis → §4.4 human call. Hidden-tab gotcha: `getBoundingClientRect` reads the frozen 0.98
  entrance transform and lazy images never load — use `offsetWidth` and flip `loading` to eager
- [x] testing `testing-strategy` — `candidate-slot-is-40-wide-on-every-row` →
  `candidate-slot-is-108-wide-on-every-row` (the stressed picker is a VIDEO page, not the
  studio page the handoff first assumed); 27/27 across 9 cells; **mutation-tested**: no `w-27`
  → width fails ×9, no `h-15` → row floor fails ×9 (the pair covers both axes);
  `docs/testing-strategy.md` §4 invariant, §12 rows, and a new HOLODEX-414 Given/When/Then block
- [~] security `security-review` — n/a unless `asset_hosts` widens (it does not)
- [x] `/code-review high --fix` before each commit (clean ×3); three-skin QA per the checklist §3
- [ ] PR: Draft now (design gate landed), ready when the gates above are green; Jira
  `needs-design` cleared on this push, `needs-spec` cleared when the spec edit lands.
  **All gates green as of the frontend push — merge main, then mark ready** (see
  `feedback-merge-main-before-marking-ready`)

## Up next

1. Merge `origin/main` into the branch, re-run `go test ./providers/tmdb`, `npm run check`,
   vitest; then `gh pr ready 352` (fires In Review).
2. Human QA §4.1–4.3 on the real testbed (backend-films + provider-tmdb): does the still help;
   does the poster-fallback read as "poster only". §4.4 is **answered** (see session 4).
3. Post-merge, downstream: the sidecar repo's contract-sync note for §2.3's per-kind shape
   guidance (never from this branch).

## Session log

- **2026-09-18** — `/design-handoff`. Filed HOLODEX-414, renamed the worktree branch to
  `HOLODEX-414-picker-thumb-per-kind`, fired In Progress. Rendered the A/B mockup inline, Kevin
  picked A + poster fallback via cards. Wrote the handoff, SVG, QA checklist, 406 supersession
  note. Handoff: design gate is green; next session starts at the spec/contract edit, then
  the sidecar `resolveMovie` signature change.

- **2026-09-18 (2)** — spec + sidecar. Skills: `/write-spec` (edit), `/code-review high --fix`
  (clean). Handoff: spec and sidecar gates green, `needs-spec` cleared; next session starts at
  the frontend — `slotShape`/`SLOT_CLASS` in `candidateImage.ts`, the `entityType` prop, five
  mounts, stub `wide-N` 300 × 169 rendition + video `twins`, then the geometry assertion.
- **2026-09-18 (3)** — frontend + stub + harness + testing strategy. Skills: `/code-review high
  --fix` (clean). Live QA on media + studio pickers, three skins, 375 px. Handoff: **every gate
  is green**; next session merges main and marks PR #352 ready, then hands §4 to Kevin.
- **2026-09-18 (4)** — main merged, PR #352 marked ready (CI fired In Review). Kevin's §4.4
  answer from a 315 px dialog: "Six P…" — too tight. Four-panel mockup at that width → **Option
  3**: below `sm` the wide boxes drop to 80 (landscape 80 × 45 `h-11.25`, logo 80 × 40) AND the
  match strength stacks under the name (`flex-col … sm:flex-row`, label `max-w-full`). Measured
  at 363: slot 80 × 40, text block 180, "Six Point Harness" 110 px unclipped; probes at 1280
  unchanged (108/120 × 60, row layout). Harness 27/27 (768 is above `sm`). Spec RD 10, handoff,
  QA §3.10/§4.4, testing strategy, component rule updated. **Side effect to disclose:** three
  Re-match attempts on media 204 (Aladdin) auto-applied its already-linked tmdb match via RD1 —
  same record, three extra enrich-runs in its activity. Handoff: pushed to #352 (still ready);
  §4.1–4.3 remain Kevin's.

### 2026-09-18 · session
- skills: code-review
