---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-435
status: in-progress
release_note: The completeness ring on cards and rows is now a button — click it to refresh that one video, person or studio from every provider, the same way the bulk sweep does, and watch the ring redraw.
---

# HOLODEX-435 · F65.8 Completeness ring fires a single-entity enrichment refresh

`CompletenessRing` becomes a `<button>`: a press runs the F66 sweep's per-entity step
(`POST …/enrich/refresh-all`, every supporting provider, Force) with sweep semantics — no picker,
no toast — then re-reads its own bands from a new owner-gated `GET …/{id}/completeness` and
redraws. Every mount hoists the ring out of its link (video card via a frame-mirroring overlay
sibling; poster card and the people/studio rows via a stretched link). Child of the F65 epic
HOLODEX-412; requested 2026-09-20 while designing the F68 hover card (HOLODEX-431), which
consumes it. Spec: [`entity-completeness-score.md`](../specs/entity-completeness-score.md) F65.8 +
RD9 · Design: [handoff § F65.8](../design/completeness-ring-badge-handoff.md) +
[mockup](../design/completeness-ring-refresh-mockup.svg).

## Gates — definition of done

- [x] spec `write-spec` — F65.8 row + RD9 in `entity-completeness-score.md` (amendment banner)
- [x] design `design-handoff` — § F65.8 appended to `completeness-ring-badge-handoff.md`
  (states, hoisting per mount, a11y), superseded rows struck; `completeness-ring-refresh-mockup.svg`
- [x] architecture — none needed: no new subsystem; rides ADR-099 (store) + ADR-103 D7 (sweep step)
- [x] backend — `GET /{kind}/{id}/completeness` in the enrich route loop (owner group), drains
  first; `TestEntityCompletenessSummary`
- [x] frontend — `CompletenessRing` button (busy/idle/done, `entity` prop, self re-read),
  `api.entityCompleteness`, `ring.ts` helpers + tests, `app.css` spin + overlay + `:has()` focus-lift,
  four mounts hoisted (`VideoCard`, `PersonPosterCard`, `/people` both modes, `/studios`);
  `completeness/CLAUDE.md` + `video/CLAUDE.md` rules
- [x] testing `testing-strategy` — §4 row + date entry (HTTP test, Vitest helpers, live hoisting checks)
- [x] security — n/a: the new read sits in the existing owner group; the write is the existing
  owner-gated refresh-all
- [x] code-review `code-review high --fix` — 2 fixed (shared per-entity refresh state so duplicate mounts
  spin/redraw together; studios row indentation), 1 skipped (override keyed to prior bands can mask an
  exact revert — needs a fetch generation the lists don't expose)
- [ ] Kevin's look on the testbed (ring press on a card; select-mode row) → `gh pr ready`

## Up next — ordered (position = priority)

1. [x] [HOLODEX-435] `/code-review high --fix`, commit, push, Draft PR
2. [ ] [HOLODEX-435] Kevin's look → mark ready (CI moves 435 to In Review; it is not epic-keyed)
3. [ ] [HOLODEX-431] after merge: rebase PR #369 on main, then the `RelatedShelf` flip prototype
4. [ ] [HOLODEX-412] on merge of 435, the epic's remaining sweep is Kevin's — 435 is a child, so
   CI moves only 435; the epic stays where it is

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · story → spec → design → build → live QA
- skills: product-brainstorming (F68 session), write-spec, design-handoff
- handoff: Kevin reversed the F68 ring call ("the ring should fire enrichment like the sweep, for
  one entity"); packaged as its own story + PR under F65 (his pick), F65.8 because the spec's
  requirement numbers already ran to .7. Found: `refresh-all` **is** the sweep's per-entity step
  (same `RefreshPair`, Force) so no new write path; every existing ring mount was inside an `<a>`,
  so the real work was hoisting (overlay for the video card, stretched links elsewhere) and one
  new read so the ring can redraw itself without list re-fetch plumbing. Built + live-verified on
  the films testbed (worktree servers started by hand — `preview_start` refuses a cwd outside the
  session worktree; Vite on :5174, backend :7800, both from this worktree, served content
  curl-checked). A real press moved a video `75/17 → 100/67` with exactly two requests. Prettier
  churned the two TS files once (no repo config) — reverted and re-applied by hand. Code review
  found the landing page mounts each video twice → busy/bands moved to a per-entity store
  (`ringRefresh.svelte.ts`), re-verified live (twins spin and redraw together). Draft PR open;
  next: Kevin's look → ready.
