---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-289                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fixed a bug where the video detail page hid the Studio section entirely (with no way to add one) when no studio was linked yet.
---

# HOLODEX-289 · Studio add-affordance on the media detail page

Owners can always add a studio to a video from the Media detail page, even when the video currently has zero studio candidates — the Studio pencil/add affordance renders regardless of whether the resolver produced a `studio` entry in `resolved[]`.

**Design package:** N/A — one-line bug fix restoring an existing affordance's availability; no requirement, architecture, or UX change (`StudioPicker.svelte`'s picker/search/create UI is unchanged, only when it's offered).

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — not applicable: bug fix, no requirement change
- [~] architecture `architecture` → `docs/architecture/ADR-*` — not applicable: no architectural decision changed
- [~] design `design-handoff` → `docs/design/**` — not applicable: no new UI/UX, restores existing StudioPicker affordance
- [x] frontend — `web/src/lib/components/entity/StudioPicker.svelte`, `web/src/routes/media/[id]/+page.svelte`
- [x] testing — verified live against local dev servers (repro, fix, search/create, persistence, visitor-view no-regression, all 3 skins); `npm run check` clean (0 errors, only pre-existing unrelated warnings)
- [~] security `security-review` — not applicable: no auth/access/infrastructure change

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [—] Push branch, open PR (ready — all applicable gates green) — [PR #260](https://github.com/whoiskevinrich/holodex/pull/260)
2. [ ] [—] Sync Jira HOLODEX-289 status once PR is open (CI fires In Review on the PR-ready webhook, ADR-058)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-26 · session
- skills: simplify
- handoff: Root-caused to `internal/resolver/resolver.go` dropping a valueless replace field with no standing decision from `resolved[]`; fixed by widening `StudioPicker.svelte`'s `field` prop to `ResolvedField | undefined` with an internal placeholder fallback (relocated here from the page component per `/simplify`'s altitude review, since the component-level fix generalizes to any future caller). Verified end-to-end in-browser. Pushed `HOLODEX-289-studio-add-affordance`, opened [PR #260](https://github.com/whoiskevinrich/holodex/pull/260) ready-for-review (all applicable gates green), Jira HOLODEX-289 transitioned to In Progress at start of work. Next: nothing — CI will fire In Review/Done/Released on PR/merge/deploy per ADR-058; only remaining step is Kevin's own review/merge.
