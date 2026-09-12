---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-367
status: in-progress
release_note: Providers that ask for it now receive the video's resolved fields and its raw filename alongside the search query, so they can match a release directly and fall back to a performers + studio search when the title misses; the search query no longer repeats the studio and cast when the title is only those words; and the picker can show what a provider actually searched.
---

# HOLODEX-367 · Structured resolve hints (ADR-080 D1 revisit)

ADR-080 D1 kept `/resolve`'s hint one flattened string and said: revisit when a second provider
wants finer control. The partner video provider brought the evidence on 2026-09-11 — 6 of 9
zero-hit resolves are reachable with a performers + studio query the undelimited blob cannot
express, and the raw bracketed basename matches 3/3 through the upstream's filename matcher where
free-text gets 0/3. Design converged over three rounds; the epic is ADR + contract text + two
stories. HOLODEX-370 (see and undo a written-back wrong match) landed first on purpose.

**Decisions that were mine, not the ticket's** (flag on review): a residue-dropped `{title}` is
*rendered-empty*, not *missing* — it must not fail a required token through to the sanitized-title
floor, which would resend exactly the duplication the rule prevents (ADR-095 D5). `hint.fields` is
`/enrich`-shaped (`{key: [values]}`) so the vocabulary is one the provider already speaks (D2).
The operator deny is a per-source boolean sketched as `send_filename: false`; the name is the
story's to settle.

## Gates — definition of done

- [x] spec `write-spec` — contract text (`metadata-provider-contract.md` §2.2 row + example, §2.3
  three `hint.*` rows + video example + `searched[]`, §4.9 residue row, **new §4.10** deep-dive
  with the opt-in table / miss definition / provider obligations, §5 caps row) **and** the F54 spec
  amendment (FR6 residue rule incl. the honest all-or-nothing example, FR7 `query_source`, FR8
  structured hints on both paths, FR9 "Searched: …" caption replacing P1-a; AC-12–18; AC-10
  narrowed; P2-a promoted; test notes per FR)
- [/] architecture `architecture` — ADR-095 drafted (D1–D8), indexed in `README.md`; number was
  reserved via `adr-claims.mjs` before drafting
- [x] design `design-handoff` — `structured-resolve-hints-searched-caption-handoff.md` + SVG mockup +
  QA checklist. Option A (first query inline + `+N more`) chosen over collapsed-only from a
  side-by-side mockup; caption is its own `<p>` under the aria-live status line in every state,
  ink for the query / muted for the label, `<ol>` `max-h-24` scrolling at 10 entries, toggle joins
  the existing tab trap unchanged, batch path = `searched:` prefix on the existing detail line
- [x] backend — HOLODEX-368: `Manifest.ResolveHints`, `Source.SendFilename` (`send_filename: false`,
  default allow), `Hint{Fields, Filename, QuerySource}`, `ResolveResult{Candidates, Searched}`,
  the manifest gate `gateHint` inside `Service.Resolve` (against the manifest the same call fetched
  — no cold-cache window), residue rule in `query.go` (**pattern-aware**, see 2026-09-11 log),
  `query_source` derived in `enrichVideoResolve`, per-provider hints via `enrichQueryHint`'s
  `hintFor(provider)` inside the fan-out, `searched[]` sanitized → interactive response +
  `job_runs.detail` on the batch path (`RecordSearched`). Wire golden + gate + residue + both-path
  tests; `/code-review high --fix` + `/security-review` (clean) run on the diff
- [x] frontend — HOLODEX-369: caption from `searched[]` — `EnrichPicker.svelte` block per the handoff
  (own `<p>` under the aria-live line, first entry in ink + `+N more` → capped scrolling `<ol>`,
  hidden at once on input, reset per response, same stale guard as candidates), state table in
  `web/src/lib/searchedCaption.ts` (+13 vitest cases), `api.ts` type gains `searched?`; enrich stub
  emits `searched[]` + `flood`/`twins` opt into `resolve_hints`. Live-QA'd all three skins at
  1280×800 (colours = `--muted`/`--ink`, contrast ≥ 4.67, stressed `<ol>` 96px scrolling, dialog ≤
  80vh, no overflow, batch row on the Activity log with no path separator). §12 assertion →
  HOLODEX-372
- [x] testing `testing-strategy` — `docs/testing-strategy.md`: §4 row (manifest-gate golden vs the
  ADR-080 golden, deny + explicit default-allow, `fields` ∩ advertised, `Base()` verbatim,
  `query_source` derived/never trusted, residue table incl. the residue-present duplicated render,
  per-provider batch hints, `searched[]` caps + no-path detail), §5 caption row (supersedes the F54
  P1 clause; stressed state = §12 geometry assertion), six Critical invariants, §9 adversarial
  block, §11 gap entry (three traps named). Written ahead of implementation — nothing automated yet
- [x] security `security-review` — on the ADR/contract posture (docs-only PR). **One finding,
  fixed in-PR:** D2's "canonical keys ∩ advertised" would have sent `overview`/`tagline` (owner
  free text) and `homepage`/`external_provider_id`/`poster_url` (cross-provider ids) to an
  opted-in provider; `hint.fields` is now bounded to the five §4.9 search fields — ADR, contract
  §2.3/§4.10, F54 FR8/AC-15, testing-strategy row + invariant + §9 case, README row all amended.
  Confirmed clean: opt-in + deny layering, `Base()` only, `/admin/activity/*` under `requireOwner`
  + `redactFileMetadataForVisitor` keep basenames owner-only, `searched[]` on the existing
  `SanitizeValue` perimeter, SSRF allowlist unchanged, `query_source` server-derived. **Re-reviewed
  on HOLODEX-368's code diff (2026-09-11): clean** — one Low candidate (`fields.title` = stem for an
  untagged file under `send_filename: false`) filtered as by-design (D1 scopes the deny to
  `filename`; the same stem words already ride `hint.query`; example config says so).

## Up next — ordered (position = priority)

1. [x] [M] Contract text — §2.2 / §2.3 / §4.9 / new §4.10 / §5, in PR #327.
2. [x] [S] F54 spec edit — FR6–FR9, AC-12–18, in PR #327.
3. [x] [M] `/design-handoff` for HOLODEX-369 — in PR #327.
4. [x] [M] `/testing-strategy` — in PR #327.
5. [x] [M] `/security-review` — design posture, in PR #327 (finding fixed in-PR).
6. [x] [M] Build HOLODEX-368 (request side) on this branch — in PR #327; `/security-review` re-run
   on the code diff (clean).
6a. [x] [M] Build HOLODEX-369 (caption) on this branch — in PR #327.
6b. [ ] [S] Mark PR #327 **ready for review** (all gates green; the act that moves 367 to In Review).
7. [ ] [—] When the PR is marked ready: sweep HOLODEX-368/369 with the epic (CI moves only the
   branch's key). HOLODEX-372 (geometry preparation) stays To Do — not a gate.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-11 · epic refreshed from the filename probe, ADR-095 drafted
- skills: architecture, design-handoff, testing-strategy, security-review
- Folded the provider's filename-matcher probe into the epic (verbatim basename, default-allow
  load-bearing, batch path too), marked HOLODEX-370 landed, fired In Progress, branched off main
  as `HOLODEX-367-structured-resolve-hints`. Drafted ADR-095 (D1–D8) and the README row.
- Handoff: ADR-095 is on the branch in Draft PR #327; contract text (§2.2/2.3/4.9/5) is the next
  thing to write, in the same PR.

### 2026-09-11 · contract text written
- skills: —
- Wrote the provider-contract text for ADR-095: rows in §2.2/§2.3/§4.9/§5 and a new §4.10
  deep-dive in the §4.7–4.9 house style (a §4.10 was my call — the miss definition and the
  provider's obligations wanted a home, and §5 is a caps table). Ticked ADR-095 AI 1–2.
- Handoff: PR #327 now carries ADR + contract text; next is the F54 spec edit (caption slot →
  "Searched: …", ACs for `query_source` + residue), then `/design-handoff` for HOLODEX-369.

### 2026-09-11 · F54 spec amended
- skills: write-spec (by hand — the spec exists; amended in place rather than a new doc)
- Added FR6–FR9 + AC-12–18 to F54, narrowed AC-10, struck the superseded Non-Goal / P1-a / P2-a
  with pointers. Caught my own wrong example while writing: the residue rule is all-or-nothing, so
  a title *with* residue keeps its duplication — the spec now shows that render honestly and says
  why (decomposition is FR8's job, not a cleverer blob). Spec gate closed.
- Handoff: PR #327 = ADR-095 + contract §2.2/2.3/4.9/4.10/5 + F54 FR6–FR9. Next:
  `/design-handoff` for HOLODEX-369 (caption), then testing strategy + security review, then
  mark ready. Backend story HOLODEX-368 can start any time — its spec is complete.

### 2026-09-11 · design handoff for the caption
- skills: design-handoff
- Mocked two mechanisms side by side (first-query-inline + "+N more" vs collapsed-only) grounded in
  `EnrichPicker.svelte`'s status-line idiom; owner picked A. Wrote the handoff, the SVG (three
  states + tab order + tokens + batch row, geometry-checked in the browser), and a numbered
  `[smoke]/[agent]/[human]` QA checklist. Design gate closed.
- Handoff: two gates left on PR #327 — `/testing-strategy` and `/security-review` — then mark
  ready. HOLODEX-368 (request side) and HOLODEX-369 (caption) are both spec+design complete.

### 2026-09-11 · testing strategy
- skills: testing-strategy
- Extended `docs/testing-strategy.md` in the five places the F54 plan lives (date line, §4, §5,
  Critical invariants, §9, §11). The §11 entry names the traps: the ADR-080 golden must not be
  edited to make the opted-in golden pass; both rejected residue variants have plausible tests, so
  the table asserts the duplicated render; the caption's stressed state needs a §12 harness
  preparation that doesn't exist yet.
- Handoff: one gate left on PR #327 — `/security-review` (raw basename to opted-in providers) —
  then mark ready. Nothing in the PR is code; the review is of the ADR/contract posture.

### 2026-09-11 · HOLODEX-369 built (picker caption)
- skills: code-review
- Built the caption exactly to the handoff. Two calls that were mine: (1) the handoff assumed an
  `EnrichPicker` test file that does not exist (no component harness here), so the state table is a
  pure `searchedCaption.ts` helper with vitest, and the DOM/a11y/skin/stressed items were measured
  live via `javascript_tool` (the stub gained a `searchedFor` cascade + a `stress` ten-entry mode
  for it, and `flood`/`twins` opt into `resolve_hints` so every wire shape is reachable); (2) added
  `shrink-0` to the expanded `<ol>` — under 80vh flex pressure it was the first thing squeezed
  (49px of its 96px cap), and the cap is the design. `/code-review high --fix`: one finding (stub's
  stressed entry short for short queries), fixed. Deferred the §12 `enrich-picker-open` preparation
  to HOLODEX-372 rather than leave it as a comment. Live batch check: refresh-all across four stub
  providers wrote four `searched:` rows, each with its own render (flood got `{studio?} {title}` +
  the basename; the others the sanitized floor), no path separator on the Activity log.
- Handoff: every gate on PR #327 is green — **mark it ready for review** (that fires In Review on
  367), then sweep 368/369 to In Review by hand. HOLODEX-372 is the only loose end and is not a gate.

### 2026-09-11 · HOLODEX-368 built (request side)
- skills: code-review, security-review
- Implemented ADR-095 D1–D5 + D8 and the `searched[]` decode (D6 request-side half). Design calls
  that were mine: the opt-in gate lives in `Service.Resolve` against the just-fetched manifest
  (not a lazily-warmed cache like `preferredPatterns`) so an opted-in provider's first resolve
  after boot is already structured; the batch path records a `(N candidates) · searched: …`
  enrich-kind activity row **only** when the provider emitted `searched[]` (no new rows for
  providers that don't); `SanitizeTitle` export removed (its only caller was the old batch hint).
  `/code-review high --fix` found one real gap — the content-based residue rule turned
  `{title} {year?}` into a bare year for a stem-titled file — owner chose **pattern-aware**
  (strip studio words only with `{studio}` in the tier, performers only with `{performers}`,
  dates only with `{year}`); FR6, ADR-095 D5, contract §4.9 and the testing-strategy row amended
  in the same push. `/security-review` on the code diff: clean.
- Handoff: PR #327 (still Draft) now carries the full request side; the provider can opt in
  today. Next: HOLODEX-369 — the picker caption — then mark the PR ready and sweep 368/369 with
  the epic.

### 2026-09-11 · security review of the design
- skills: security-review
- Reviewed the posture, not code (docs-only PR). Found that D2's `hint.fields` = "canonical keys ∩
  advertised" widened outbound data past the §4.9 blob the ADR claimed to match — a provider
  advertising `overview` would receive the owner's own free text (a file's `Comment` tag), and
  `homepage`/`external_provider_id`/`poster_url` would leak which other providers the owner uses.
  Bounded `hint.fields` to the five §4.9 source fields across ADR, contract, F54, testing strategy,
  README. Verified the basename claims against code: `/admin/activity/*` is inside `requireOwner`,
  `redactFileMetadataForVisitor` keeps file identity owner-only.
- Handoff: all seven design gates green on PR #327, which stays **Draft** — it is the epic's one
  PR and still owes the implementation (HOLODEX-368 then 369). Next session: build 368 on this
  branch, `/code-review high --fix`, re-run `/security-review` on the real diff.
