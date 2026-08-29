---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-290                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Studio links on the Media and Film pages now show the studio's icon and video count, not just a plain text link.
---

# HOLODEX-290 · StudioLinkCard (reusable Studio display)

Reworks the ad hoc Studio-links markup on the Media and Film detail pages (plain
comma-joined text, no icon, no count, an uppercase "Studios" label on Film only) into one
shared `StudioLinkCard.svelte`: icon + name (linked) + video count, one card per linked
studio. Presentational only — the existing owner edit affordances (`StudioPicker`,
`FilmStudioCascadeDialog`) are untouched.

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — not applicable: no new capability, purely a display-component change (see handoff §"Why no spec/ADR")
- [~] architecture `architecture` → `docs/architecture/ADR-*` — not applicable: extends an existing query's `SELECT` list into fields `model.Studio` already declares, no schema/architecture change
- [x] design — [`docs/design/studio-link-card-handoff.md`](../design/studio-link-card-handoff.md) + [mockup](../design/studio-link-card-mockup.svg)
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (backend query-extension row §4, `StudioLinkCard` target-coverage row §5, shared-query + icon-fallback adversarial block §10)
- [x] backend — `internal/repo/studios.go` (`StudiosForVideos`), `internal/repo/films.go` (`FilmStudios`): `VideoCount` + `ImageVersions` added (not `icon_url` directly — see handoff correction below), `setStudioImageURLs` wired at both API call sites (`internal/api/handlers.go`, `internal/api/films.go`)
- [x] frontend — new `web/src/lib/components/entity/StudioLinkCard.svelte`; call-site swaps live in `web/src/routes/media/[id]/+page.svelte` and `web/src/routes/films/[id]/+page.svelte`
- [x] security `security-review` — confirmed not applicable: display-only change extending an existing read query's populated fields, no new auth/access/infrastructure surface

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] Extend `StudiosForVideos`/`FilmStudios` to carry `VideoCount` + `ImageVersions` — `internal/repo/studios.go`, `internal/repo/films.go`
2. [x] [frontend] Build `StudioLinkCard.svelte` and swap both call sites per the handoff §2/§3
3. [x] [—] Push branch, open Draft PR — [PR #269](https://github.com/whoiskevinrich/holodex/pull/269)
4. [ ] [—] Push this implementation commit and mark the PR ready for review (all gates now green)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-29 · session (3)
- skills: simplify
- handoff: Implemented both remaining gates. Backend: discovered the handoff's §4 plan was slightly off — `IconURL` is a computed serving URL (via `ImageVersions` + `setStudioImageURLs()`), not a stored `icon_url` column — so `StudiosForVideos`/`FilmStudios` were extended to populate `VideoCount` (correlated COUNT subquery, now factored into a shared `studioActiveVideoCountSubquery` const alongside `GetStudio`) and `ImageVersions` (via the existing `studioImageVersions`/`attachStudioImages` batch helpers), with `setStudioImageURLs` wired at both API call sites; two new pinning tests added (`TestStudiosForVideos_IncludesIconAndCount`, `TestFilmStudios_IncludesIconAndCount`), full backend suite green. Frontend: built `StudioLinkCard.svelte` per the handoff's exact markup, swapped both call sites, verified live in the browser (icon-set + monogram-fallback states, both Media and Film detail pages) across all three skins via computed-style checks. Ran `/simplify` (4 parallel review agents): applied 3 real fixes (shared count-subquery constant, `FilmStudios` now reuses `attachStudioImages` instead of reimplementing it, a redundant-looking media-page condition reverted after it broke TS narrowing) and deferred one out-of-scope efficiency finding to [HOLODEX-291](https://whoiskevinrich.atlassian.net/browse/HOLODEX-291) (merge-writeback caller pays for unread count/image fields). Security-review confirmed not applicable. All gates green — next: push and mark PR #269 ready for review.

### 2026-08-29 · session (2)
- skills: testing-strategy, simplify
- handoff: Added the testing-strategy coverage for this change: a backend row (§4) for the `StudiosForVideos`/`FilmStudios` icon+count extension — including the non-obvious shared-query regression guard (`mergeEntity`'s writeback propagation in `entity_identity.go:185` also calls `StudiosForVideos` and must be unaffected by the widened `SELECT`) — a frontend target-coverage row (§5) for `StudioLinkCard.svelte`, and a Given/When/Then adversarial block (§10) covering the NULL-icon scan, the shared-query regression, and per-card icon/no-icon independence. Testing gate is now green. Next: extend the two backend queries (`internal/repo/studios.go`, `internal/repo/films.go`), then build `StudioLinkCard.svelte` and swap both call sites.

### 2026-08-29 · session (1)
- skills: design-handoff
- handoff: Explored the current Media/Film studio markup and every existing Studio-related component (StudioPicker, EntityImageSlot, EntityVideoMeta) to ground the new component in real conventions, then found the backend currently only selects `id, name` for a video's/film's linked studios (`StudiosForVideos`, `FilmStudios`) — no icon or count reaches the frontend today, so the card can't be built frontend-only. Resolved the mockup's open ambiguities (video-count text format, icon vs. logo role, multi-studio layout) against existing app conventions rather than asking, since they were mechanical fits not open design tradeoffs. Wrote the handoff (`docs/design/studio-link-card-handoff.md`) + SVG mockup, created HOLODEX-290, and opened the branch. Next: extend the two backend queries, then build `StudioLinkCard.svelte` and swap both call sites.
