---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-431
status: in-progress
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
- [ ] architecture `architecture` — only if RD10 falls (positioning needs `@floating-ui/dom`)
- [ ] backend — `GET /people/{id}/card` (R5) + API test
- [ ] frontend — `PersonLinkChip` + `PersonHoverCard` (R1–R4, R6, R7), three consumers wired
- [ ] testing `testing-strategy` — section for R8 (timers/focus/single-open, corner flip, API
  shape, geometry rung)
- [ ] security `security-review` — required: `/card` gates `completeness` on `authorized` (RD12)
- [ ] dependency — **HOLODEX-435** (F65.8 ring button) merged; rebase #369 on it before frontend

## Up next — ordered (position = priority)

1. [x] [HOLODEX-431] design handoff with SVG mockup (three skins) — OQ2 decided
2. [ ] [HOLODEX-435] ship the ring button first (own branch/PR, F65.6) — F68 consumes it
3. [ ] [HOLODEX-431] prototype the positioning flip inside `RelatedShelf` first (OQ1) before
   writing the chip — this decides whether an ADR is needed
4. [ ] [HOLODEX-431] backend card endpoint + test
5. [ ] [HOLODEX-431] frontend chip + card + consumers; QA all three skins; geometry rung
6. [ ] [HOLODEX-431] testing-strategy section, /code-review, mark PR ready (→ In Review by CI)
7. [ ] file follow-ups from the spec's Deferred list as HOLODEX issues: in-tile reveal for
   `PeopleGrid`, age-at-release, curation chips + header dropdown, Studio/Film cards

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · design rev 2 after main moved
- skills: design-handoff
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
