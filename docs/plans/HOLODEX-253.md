---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-253                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The video detail page now shows a sharper, higher-resolution poster instead of an upscaled list thumbnail.
---

# HOLODEX-253 · Two-tier video poster resolution (F53)

Extraction (`internal/thumbnail`) now produces a second, larger poster derivative alongside the
existing small list thumbnail in the same pass; the `media/{id}` detail page renders that sharper
tier instead of an upscaled list card. Done means the dual-output Tier 1/Tier 2 extraction, the
`POSTER_WIDTH` config, the fallback-serving `/poster` route, and the frontend binding swap are all
merged, with every applicable lockstep gate landed (architecture/design are explicitly not needed
per the spec's routing call).

**Design package:** [spec](../specs/video-poster-multi-size.md) · architecture: not needed (incremental ADR-009 extension, see spec Timeline/routing) · design: not needed (single existing binding swap) · [testing-strategy §10](../testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → [docs/specs/video-poster-multi-size.md](../specs/video-poster-multi-size.md)
- [~] architecture `architecture` — not needed: incremental ADR-009 extension, no new table/cross-cutting decision (spec Timeline/routing §1)
- [~] design `design-handoff` — not needed: single existing `<video poster>` binding swap, no new component/state (spec Timeline/routing §2)
- [x] backend → `internal/thumbnail` (PosterWidth, PosterPath, dual-output Tier1/Tier2), `internal/config`, `internal/api` (PosterURL, `/media/{id}/poster` route + fallback)
- [x] frontend → `media/[id]/+page.svelte` poster binding swap to `poster_url`, `types.ts`
- [x] testing `testing-strategy` → [docs/testing-strategy.md §10](../testing-strategy.md); unit + integration tests added this session
- [x] security `security-review` → clean sign-off, no findings (new public read route mirrors `/thumbnail`'s posture exactly via the shared `serveImageFile` helper)

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [—] open PR and mark ready for review (all applicable gates green)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-05 · Backend + frontend implementation of F53, all applicable gates green
- skills: graphify, simplify, security-review
- handoff: Backend (dual-tier extraction, POSTER_WIDTH config, /poster route+fallback,
  Video.PosterURL) and frontend (poster binding swap) are implemented, tested (go test ./...,
  npm run check, npm run test all pass), simplified (/simplify caught and fixed a real
  dual-tier consistency bug in upload/delete plus a shared serveImageFile helper extraction),
  and security-reviewed (clean, no findings). Every applicable gate (spec/backend/frontend/
  testing/security; architecture+design correctly N/A) is green. Next session should open the
  PR (not draft — nothing gates it), sync Jira, and push.
