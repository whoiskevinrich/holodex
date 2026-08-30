---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-296                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: [HOLODEX-297]    # [KEY-…] cross-epic deps that must land first
release_note:                # internal refactor, no user-facing behavior change — no release note
---

# HOLODEX-296 · Extract shared poster-tile component for Films/People chips

Extract the near-identical poster-tile `<li>` markup duplicated between the Media detail page's
Films section and `PeopleGrid.svelte`'s People section (flagged by the `/simplify` pass on
HOLODEX-272's Films/People row reorder) into one shared `entity/PosterTile.svelte`, with zero
visible/behavioral change.

**Design package:** none — pure refactor, no new functionality/architecture/UX surface; see
Gates below for why each pre-implementation gate is marked n/a.

## Gates — definition of done

- [~] spec `write-spec` → n/a — no behavior/requirement change, pure internal extraction
- [~] architecture `architecture` → n/a — no data model/stack/deployment change
- [~] design `design-handoff` → n/a — visually and interactively identical to before (verified live, all 3 skins)
- [ ] backend — n/a, frontend-only change
- [x] frontend — `web/src/lib/components/entity/PosterTile.svelte` created; `+page.svelte` (Films) and `PeopleGrid.svelte` (People) both consume it
- [x] testing `testing-strategy` → n/a formal doc, but verified: `npm run check` clean, live DOM/interaction parity check (Films + People tiles, remove button, visitor-mode gating), 3-skin QA
- [~] security `security-review` → n/a — no auth/access/infra surface touched

## Up next — ordered (position = priority)

1. [x] [frontend] Create `PosterTile.svelte` sharing the `<li>`/`<a>`/label/remove-button shell, `image`/`badge` as snippets — `web/src/lib/components/entity/PosterTile.svelte`
2. [x] [frontend] Wire Films section on Media detail page through it — `web/src/routes/media/[id]/+page.svelte`
3. [x] [frontend] Wire `PeopleGrid.svelte`'s People tile through it — `web/src/lib/components/entity/PeopleGrid.svelte`
4. [x] [frontend] Register new component in folder's `CLAUDE.md` — `web/src/lib/components/entity/CLAUDE.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-30 · Extraction implemented and verified
- skills: (none — pure refactor scoped out of spec/ADR/design/security per Gates above)
- handoff: PosterTile.svelte extraction is code-complete and verified (check clean, live 3-skin QA, interaction parity); next session should finish commit/push/PR for HOLODEX-296, based on `claude/media-detail-reorder-c2d1ab` since it depends on the not-yet-merged Films/People row reorder (PR #275).

### 2026-08-30 · Two `/code-review high --fix` passes
- skills: `/code-review high --fix` (×2)
- handoff: PR #276 opened (draft, base `claude/media-detail-reorder-c2d1ab`); first pass hoisted a duplicate `personKey(p)` call in `PeopleGrid.svelte`, second pass fixed a second lingering duplicate call (each-key expression vs. `{@const}` — hoisted into a `keyedPeople` derived array instead) and corrected this file's `status` (was `in-review` while the PR is still Draft — now `in-progress`). `depends-on` stays `[]`: PR #275 (the reorder work) has no HOLODEX ticket of its own — spawned a follow-up task to create one and backfill the key. Remaining review findings (pre-existing busy-key race on rapid multi-item remove, monogram-plate/remove-button markup duplicated elsewhere in the codebase, no component test harness) are real but outside this ticket's scope — left as-is.

### 2026-08-30 · Backfilled HOLODEX-297 for the reorder dependency
- skills: (none — Jira/GitHub housekeeping)
- handoff: created [HOLODEX-297](https://whoiskevinrich.atlassian.net/browse/HOLODEX-297) to cover the Films/People reorder work this ticket depends on, and set `depends-on: [HOLODEX-297]` above. Renaming that PR's branch to include the key via GitHub's branch-rename API unexpectedly deleted the old ref and auto-closed PR #275 — recreated as **PR #277** with identical content on the renamed branch `HOLODEX-297-media-detail-reorder`. This PR (#276) is unaffected: GitHub auto-retargeted its base ref from `claude/media-detail-reorder-c2d1ab` to `HOLODEX-297-media-detail-reorder`, and it's still OPEN against the same commit. No action needed here beyond noting the new branch/PR name for future reference.
