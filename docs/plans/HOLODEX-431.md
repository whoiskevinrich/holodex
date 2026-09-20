---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-431
status: in-review
release_note: Hovering a person's name on a film page, the search page or a "More with" shelf now shows a small card — headshot, age, how many titles and films you have with them, aliases — with links to their profile and provider pages.
---

# HOLODEX-431 · F68 Person hover card

Text-only person links (film billed chips, `/search` rows, the "More with …" shelf title) get a
floating hover card: headshot, name + flags, current age, title/film counts, aliases and a link
row (Profile · Titles · Films · external ids · owner-only Enrich/Edit). Person gets its first
shared link component, `PersonLinkChip`, which is the only mount point. Data comes from a new
public `GET /people/{id}/card` (3-field resolve subset + cheap counts). Brainstormed 2026-09-16
(model B over read-only A / enriched-chip C; text-only surfaces only). Spec:
[`docs/specs/person-hover-card.md`](../specs/person-hover-card.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/person-hover-card.md` (F68), RD1–RD11 locked
- [x] design `design-handoff` — [person-hover-card-handoff.md](../design/person-hover-card-handoff.md)
  + [mockup SVG](../design/person-hover-card-mockup.svg) rev 2: default skin only (ADR-102),
  owner vs visitor, loading/sparse/corner; OQ2 resolved = chip keeps the consumer's classes;
  owner row replaced by the F65 `CompletenessRing` + header-block-as-profile-link (RD12)
- [x] architecture `architecture` — not needed: the OQ1 probe (2026-09-20) showed one measure +
  a horizontal clamp does it inside `RelatedShelf`; RD10 stands, no floating-ui, no ADR
- [x] backend — `GET /people/{id}/card` (R5) reusing `personResolve`; `CountFilmsForPerson`,
  `PersonImageVersions`; `TestPersonCard` + `TestPersonCard_SparseOmitsKeys`
- [x] frontend — `PersonLinkChip` (transparent wrapper) + `PersonHoverCard` + `personCard.svelte.ts`
  (cache, open latch, `placeCard`), four consumers wired (billed chips, `/search` rows, More-with
  title, `PosterTile` via `PeopleGrid`), `#videos`/`#films` anchors, `hover-card-in` motion;
  live-verified on the films testbed incl. keyboard, single-open, clamp, ring press, visitor,
  coarse pointer, three skins
- [x] testing `testing-strategy` — §4 row: HTTP tests, `placeCard` under Vitest (6), the contract
  as live `[agent]` checks; geometry rung deferred (harness has no pointer step) — follow-up
- [x] security `security-review` — clean 2026-09-20: `completeness` under `authorized`, `Cache-Control:
  private`, no new visitor exposure (every other field is on the profile), parameterised SQL, no `{@html}`,
  external links through the validated template path
- [x] dependency — **HOLODEX-435** (F65.8 ring button) merged as `7f8c067` (#373); #369 rebased 2026-09-20

## Up next — ordered (position = priority)

1. [x] [HOLODEX-431] design handoff with SVG mockup (three skins) — OQ2 decided
2. [x] [HOLODEX-435] ring button shipped (PR #373 → `7f8c067`); #369 rebased on it
3. [x] [HOLODEX-431] OQ1 probe done — flip vertically, clamp horizontally, no ADR (RD10/R3 amended)
4. [x] [HOLODEX-431] backend card endpoint + test
5. [x] [HOLODEX-431] frontend chip + card + four consumers; three skins QA'd live
6. [x] [HOLODEX-431] `/code-review high --fix` (4 fixed: ring press unmounted the ring → focus-loss close; per-chip abort killed a shared flight; no unmount cleanup; nested `group` hover leak), `/security-review` clean; PR #369 marked ready 2026-09-20 on Kevin's word — billed chips + 2.1–2.3 still owed a human look on the real library
6b. [ ] file follow-ups: geometry-harness hover preparation for the card; alias "+N more"
7. [ ] file follow-ups from the spec's Deferred list as HOLODEX issues: in-tile reveal for
   `PeopleGrid`, age-at-release, curation chips + header dropdown, Studio/Film cards

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · design rev 2 after main moved
- skills: design-handoff, code-review, security-review
- handoff: rebased onto main (#365 instance skin, #350 completeness ring). Kevin: focus the
  mockup on the default skin (skin is instance identity now; Cinémathèque tokens unchanged) and
  drop the owner Enrich/Edit links — the F65 ring is the owner's indicator, clicking the card
  (header block → profile) is the edit path. Ring stays non-interactive per its contract; `/card`
  returns `completeness` owner-only, which makes the security gate real. Spec RD3/RD4/RD12,
  R4/R5 amended; mockup regenerated. **Then Kevin reversed the ring call:** the ring should
  fire enrichment (sweep semantics, single entity) — filed as its own story **HOLODEX-435**
  (F65.8, child of 412) with its own branch/PR to merge first; RD12 now says the ring is the
  card's one action, mounted as a sibling of the header link. Next: build 435, then the flip
  prototype here.

### 2026-09-19 · brainstorm → story → spec → design
- skills: product-brainstorming, write-spec, design-handoff
- handoff: brainstormed four options (A read-only card / B link row / C enriched chip / D in-tile
  reveal); Kevin chose **B** on text-only surfaces with all four link groups, current age; spec
  session settled `/search` page only (not the dropdown) and curation chips deferred. Filed
  HOLODEX-431 (F68, reserved via `feature-claims --reserve`), renamed branch
  `HOLODEX-431-person-hover-card`, In Progress fired; rebased onto main (36 commits). Spec
  committed; Draft PR #369 open. **Design gate closed** the same session: handoff + SVG (six
  panels) committed; found that the film billed chips are owner-only and dashed-by-meaning, so
  `PersonLinkChip` is a transparent wrapper (OQ2); `#videos`/`#films` anchors don't exist yet
  (build adds them); R7 amended to the app's `shadow-lg` panel convention. Next: prototype the
  flip inside `RelatedShelf` (OQ1) before writing the chip — it decides whether an ADR is needed.

### 2026-09-20 · #373 merged → rebase → OQ1 prototype
- skills: —
- handoff: merged the ring story (#373, squash, `7f8c067`), removed its worktree, rebased this
  branch onto main (force-with-lease; the tree now has the ring button). OQ1 prototyped as a live
  DOM probe on `/media/190`'s real `RelatedShelf` heading (Vite for this worktree on :5177 —
  :5173/:5174/:7800 are other sessions' — against their `backend-amv`): nothing clips, both
  vertical placements hit-test cleanly over the shelf cards, and the horizontal rule turned out to
  be a clamp, not an end-align (a 400 px window overflowed by 8 px start-aligned; end-aligned would
  underflow). RD10 + R3 + handoff amended; no ADR. Next: backend `GET /people/{id}/card` + test,
  then the chip + card (remember: `bottom-full`/`mb-1.5` only exist once a source file uses them).

### 2026-09-20 · scope: grid tiles join v1
- skills: —
- handoff: Kevin clicked a Cast tile expecting the card ("I'm not seeing the hover card") — nothing
  is built yet, and that tile was a Non-Goal. He chose to add the Cast / People grid tiles as a
  fourth v1 surface (RD2 rev 2): `PosterTile` gets an opt-in prop, `PeopleGrid` sets it, film tiles
  don't; remove badge stays outside the chip. Spec, handoff, Jira updated. Next: build — backend
  `GET /people/{id}/card` first.

### 2026-09-20 · build
- skills: —
- handoff: built end to end on this branch. Backend: `GET /people/{id}/card` reuses `personResolve`
  (the spec's 3-field subset was dropped — same cost class, one truth), `completeness` owner-only,
  `Cache-Control: private`; `entityCompletenessSummary` now shares `storedCompleteness`. Frontend:
  the chip became a **wrapper around the consumer's own `<a>`** (the search row's option role /
  roving tabindex made a chip-rendered link the wrong shape); card + cache + open latch +
  `placeCard`; mounts on billed chips, `/search` page rows, RelatedShelf person title, PosterTile
  (PeopleGrid). Live QA on the films testbed (own backend on :7802, Vite :5177 — :5173/:5174/:7800
  are other sessions') found and fixed: the card inherited the shelf heading's uppercase; the
  loading state double-fetched the headshot. Keyboard checks need `document.hasFocus()` — click
  the pane first. Next: code review, security review, Kevin's look → ready.

### 2026-09-20 · review → ready
- skills: code-review, security-review
- handoff: code review caught that a real ring press (button focused) unmounted the ring and
  closed the card — fixed by re-fetching over the stale card; the cache now leases a shared
  flight (a passer-by only releases, never aborts another chip's request); unmount cleanup;
  named `group/card`. Security review clean. Kevin: "commit and push, then mark #369 ready" —
  done; CI moves 431 to In Review. Human pass still owed on the real library (billed chips).
