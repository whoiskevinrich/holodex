---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-380
status: in-progress
profile: feature
release_note: When a provider returns several records for the same release, the Enrich picker can now show each record's summary — studio chain, how fully it's catalogued — behind an info toggle that opens automatically when two candidates share a name; an auto-applied match's summary is kept in the activity log so it can be checked later.
---

# HOLODEX-380 · F61 — `candidates[].detail`: revealable per-candidate record summary

The partner video provider brought a proposal on 2026-09-13: its upstream catalogues one release
once per distribution outlet — identical label, date, cast; different outlet chain, tag count,
image set — and the picker's `label · disambiguation · confidence · view source ↗` gives the owner
nothing to choose *which record to bind to*, a choice that sets `_studio_external_ids` and `genres`.
Reviewed as maintainer, accepted as proposed with four open questions answered from a three-state
mockup. No ADR: additive optional response key under §2.3's unknown-key promise, same posture as
`profile_url` and `searched[]`.

**Decisions that were mine, not the proposal's** (flag on review): the resolve activity entry
now fires when *either* `searched[]` or an auto-applied candidate's `detail` is present — today
`RecordSearched` is a no-op on empty `searched[]`, which would silently drop the audit line for a
provider that emits `detail` but not `searched[]` (FR5). `detail` is **entity-agnostic**, unlike
`searched[]`'s `video`-only posture, because that posture came from `hint.*` being video-shaped and
nothing here is. Label-collision normalization is case-fold + whitespace-collapse + trim (FR4).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/candidates-detail.md` (F61: FR1–FR6, P1-a, P2-a–c, AC 1–11,
  test notes per surface, Resolved Decisions table) **and** the contract amendment
  (`metadata-provider-contract.md` §2.3 example + `candidates[].detail` row, §5 caps row)
- [x] design `design-handoff` — `candidates-detail-handoff.md` + `candidates-detail-mockup.svg`
  (four panels: collapsed / one toggled / collision auto-expanded / activity row) + numbered,
  verifier-tagged `candidates-detail-qa-checklist.md`. **Text toggle** (`details` / `hide details`,
  the Searched caption's `.btn-quiet` dotted-underline idiom) chosen over an info glyph from a
  side-by-side mockup — zero new components or icons; lines `text-xs text-muted` behind
  `border-l border-rule`, in-flow inside the `<li>`; expansion state keyed by `external_id`
- [x] backend — `Candidate.Detail []string` (`enrich.go`), `sanitizeDetail` beside
  `sanitizeSearched` (8 entries / 256 bytes on a rune boundary, `[]`→nil), wired into
  `sanitizeCandidates`; `RecordSearched(…, applied *Candidate)` writes on searched[] **or** applied
  detail, `· applied: <label> — <lines · joined>`; refresh-all passes `applied` **only after
  `Enrich` succeeds** (code-review finding: a failed apply must not log "applied:"); `Fake` gains
  `FakePerson.Detail` + `EnrichErr`. Tests: `candidate_detail_test.go` (sanitizer table, wire
  ingest, RecordSearched matrix), `TestEnrichRefreshAll_AppliedDetailLogged`,
  `TestEnrichRefreshAll_FailedApplyNotLoggedAsApplied`. `go test ./...` green (26 pkgs)
- [x] frontend — `EnrichCandidate.detail?`; pure `candidateDetail.ts` (`collisionOpen`,
  `hasDetail`, `detailLabel`, `normalizeLabel`) + 9 unit tests; `EnrichPicker.svelte`: `open`
  map keyed by `external_id`, reseeded per response / dropped on edit, `details`/`hide details`
  `.btn-quiet` toggle on a **baseline flex actions line** (an inline-block button in the bare
  `<li>` added a descent gap — 74.5px vs the 66.6px one text line should cost; the flex row fixed
  parity), `<ul>` in flow, `sourceLink` snippet so no-detail rows keep their exact DOM; stub
  `flood`/`twins` carry `detail` (caps + collision + mixed). **Live QA against the stub, all
  three skins:** 7/8 twins rows open on render, row 8 no toggle; toggle collapses without
  confirming; state survives ↑/↓; toggle is a trap stop; flood 25 rows all collapsed, 8-line
  and 256-char rows inside the scroller, dialog overflow 0; line contrast on the active row
  4.71 / 5.47 / 5.88 (Broadcast / Brutalist / Cinémathèque), fonts inherit each skin
- [x] testing `testing-strategy` — `docs/testing-strategy.md`: header entry, §4 backend row, §5
  picker row (live measurements recorded as the record), three Critical-invariants bullets
  (`applied:` ⇒ success; `detail` is presentation/verbatim/never `[]`; collapsed row = one line),
  §10 adversarial block, §11 gaps, §12.4 row, §12.5 gap. **New §12 assertion**
  `collapsed-detail-row-costs-one-line` (`#enrich-opt-2` height ∈ [60, 68] under the existing
  `flood` preparation) — 9/9 pass, **mutation-tested** (bare `<div>` actions line ⇒ 76px in every
  cell), the five existing picker assertions re-run green. QA checklist §2 annotated with the
  harness reconciliation. Found on the way: the **full** matrix crashes the Vite dev server
  (`0xC0000409`) — filed HOLODEX-381; `--only` runs are reliable

## Up next — ordered (position = priority)

1. [x] [S] Draft PR #333 opened with the spec gate; gate-status checkboxes mirror Jira.
2. [x] [M] `/design-handoff` landed (handoff + SVG + QA checklist); `needs-design` cleared in Jira.
3. [x] [M] Backend FR1/FR2/FR5 + tests landed.
3a. [x] [M] Frontend FR3/FR4 landed with stub fixtures and live three-skin QA.
3b. [x] [S] `/testing-strategy` reconciled the checklist and added the parity assertion.
3c. [ ] [—] QA §4.6 (touch hit target) is the one `[human]` item still open: the toggle has no
   vertical padding on purpose (parity); say if it needs `py-1`.
4. [x] [S] PR #333 marked ready 2026-09-13 → CI fired In Review.
4a. [x] [S] §4.6 ruled: `py-1` on the toggle (24 px target). Row = 76; §12 bound re-based to
   [70, 76] and mutation-tested both ways (drop `py-1` ⇒ 68, bare actions line ⇒ 77).
5. [ ] [—] Tell the provider side the merged contract text matches the proposal so they can emit
   `detail` on every candidate (the audit path needs it on lone candidates too).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-13 · proposal reviewed, decisions locked, spec + contract amendment written
- skills: write-spec, design-handoff, code-review (high --fix ×2), testing-strategy; maintainer review of the provider proposal with a, code-review, testing-strategy
  three-state mockup (show_widget) to settle Q2/Q4, then a toggle-variant mockup for the handoff
- Reviewed the proposal against the decoder, sanitizer, picker row, and refresh-all path; answered
  the four open questions (verbatim / toggle + inline / audit-log in v1 / auto-expand on
  collision); created HOLODEX-380, renamed the branch, fired In Progress; wrote the F61 spec and
  the §2.3 + §5 contract rows.
- Committed the spec gate, opened Draft PR #333, then ran `/design-handoff`: text toggle chosen
  over glyph; handoff doc, four-panel SVG, and QA checklist committed; spec FR3/P1-a/AC synced.
- Backend gate built and reviewed in the same session: `/code-review high --fix` found the
  applied-before-Enrich ordering bug; fixed with a failure-path test. Graph updated.
- Frontend built, `/code-review high --fix` clean, verified live on `backend-stub` + `web` +
  `enrich-stub` (launch entries added to the gitignored launch.json) across all three skins.
- Testing gate: strategy doc updated, one mutation-tested geometry assertion added, HOLODEX-381
  filed for the full-matrix Vite crash.
- PR marked ready (In Review). Owner ruled §4.6: `py-1` added; every doc that pinned the 66.6/68
  numbers re-based to 76 (handoff, spec AC, checklist, strategy §5/§12/invariants, assertion).
- Handoff: nothing open on the branch. Merge moves 380 to Done; HOLODEX-381 (full geometry
  matrix crashes Vite) is the one follow-up filed out of this work.
