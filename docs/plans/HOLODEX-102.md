---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-102                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: A video's TMDB credits now create real Person records — with linked roles and downloaded headshots — for its cast and crew, not just flat actor/director text.
---

# HOLODEX-102 · Video Credits → People + Headshots (F32)

A video's TMDB enrich response carries structured `people[]` cast/crew credits (name, role,
external_id, order, headshot) alongside the existing flat `actors`/`director` text. Core enrich
resolve-or-creates each Person by namespaced external_id (id-first dedup, ADR-055) and downloads
a real headshot through the existing SSRF-guarded asset pipeline; `RelinkVideoPeople` (F40/
ADR-072) remains the sole writer of `video_people`, deriving links from the resolved
`actors`/`director` text — `people[]`'s job is only to make sure the right Person row (id +
headshot) exists first.

**Design package:** [docs/specs/video-credits-people.md](../specs/video-credits-people.md) ·
ADR folded into [PR #219](https://github.com/whoiskevinrich/holodex/pull/219)'s description (the
one architecture-worthy bit — generalizing `downloadAssets` to a per-credit `personID` — was
small enough not to need a standalone ADR) · [docs/testing-strategy.md](../testing-strategy.md)
§9/§11

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/video-credits-people.md`
- [x] architecture `architecture` → folded into PR #219 description (no standalone ADR needed)
- [x] design `design-handoff` → no new UI surface; live QA (slice 4) confirmed F30's People
      poster grid + Actors/Director chips already link correctly with zero frontend code changes
- [x] backend → 4 slices: `person_external_ids` migration + repo dedup (da7ed62), TMDB provider
      `people[]` emission (5574504), core enrich consumption + headshot download (8d25fe1),
      8 code-review findings fixed (899436f)
- [x] frontend → verified via live QA against real TMDB data, no code change required (see design)
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §9/§11 (1bf8bd5)
- [x] security `security-review` → spoofing vuln in `sanitizePeople` (missing whitespace
      rejection in provider `external_id`) found + fixed; sibling gap in the pre-existing
      `_studio_external_ids` mechanism filed as [HOLODEX-258](https://whoiskevinrich.atlassian.net/browse/HOLODEX-258),
      deliberately out of this PR's scope

## Up next — ordered (position = priority)

1. [ ] [—] Get [PR #219](https://github.com/whoiskevinrich/holodex/pull/219) reviewed and merged
      to main (all 7 gates green; Jira issue in **In Review**)
2. [ ] [—] HOLODEX-258 → fix the identical `sanitizePeople`-whitespace gap in
      `_studio_external_ids` (F38), deferred out of this PR on purpose

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-06-30 – 2026-08-06 · F32 implementation (4 slices)
- skills: write-spec, architecture (folded), testing-strategy, security-review
- handoff: `person_external_ids` migration + dedup, TMDB `people[]` emission, core enrich
  consumption + headshot download, and live 3-skin QA all landed; PR #219 opened.

### 2026-08-06 · code-review + simplify + doc/Jira sync
- skills: code-review, simplify
- handoff: All 8 `/code-review` findings fixed (899436f) — cap alignment, sidecar collision/
  casing/sanitize-bypass hardening, headshot dedup + bounded concurrency, URL/constant reuse —
  then `/simplify`-cleaned (relocated the casing-fold fallback out of `identity.go`'s SQL-only
  spine into `internal/repo/extid_fold.go`, precomputed per Reconcile call). Synced
  `docs/specs/tmdb-provider.md`'s stale "top 10" actors cap to 20 (63ad4bb). PR #219 confirmed
  ready for review; Jira HOLODEX-102 transitioned to **In Review** with its gate checklist
  synced to the PR. **Next session:** watch for PR review feedback; if merged, transition Jira
  to Done and pick up HOLODEX-258 (sibling `_studio_external_ids` sanitize gap) if not already
  claimed elsewhere.
