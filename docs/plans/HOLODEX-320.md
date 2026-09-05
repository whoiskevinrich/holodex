---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-320                 # the tracker key; must match the branch key regex
status: in-review                  # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The media page's Metadata section now sits with the rest of the fields you curate, folds away when you don't need it, and no longer repeats the genres, cast, poster, and synopsis already shown elsewhere on the page — the synopsis moved up under the title.
---

# HOLODEX-320 · Media detail: move, trim, and fold the Metadata section

Follow-up to [`media-detail-reorder-handoff.md`](../design/media-detail-reorder-handoff.md). That
change settled the page **order**; this one settles what the Metadata section is **for**. Three
asks from the owner: move it into the gap between the Films/People row and the More-with shelves,
drop the rows the page already surfaces somewhere better (Genres, Actors, Poster, Description), and
make it collapsible exactly the way the Completeness panel is.

The trim is the part with a decision in it. Three of the four were true duplicates — genres already
materialize into Tag rows, actors already derive the People grid (ADR-072), and `poster_url` only
ever rendered a second `<img>` beside the player's own poster. **Overview was not.** It lived only
inside the owner-only Metadata section, so deleting the row would have left a video's synopsis with
no home at all — still enriching, still writing back, invisible and uncurable. It moves to the
header under the meta line instead, as a verbatim port of the `long_text` branch, which also makes
it visible to visitors for the first time (accepted: a plot summary is page content, not owner
tooling).

**Design package:** [docs/design/media-detail-metadata-fold-handoff.md](../design/media-detail-metadata-fold-handoff.md)
\+ [mockup SVG](../design/media-detail-metadata-fold-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` — **not required**: no new functionality. Every affordance already existed
      on the page; this moves, de-duplicates and folds existing elements
- [x] architecture `architecture` — **not required**: no new seam. Rides ADR-051 (per-field
      precedence) and ADR-090 (adoption vs precedence) unchanged; the resolver, shadow store and
      decision model are untouched
- [x] design `design-handoff` → `docs/design/media-detail-metadata-fold-handoff.md` with a
      committed SVG mockup (before/after order + collapsed and expanded states), the removal table
      with Overview called out as the one non-duplicate, and the anchor-migration table
- [x] frontend → one file, `web/src/routes/media/[id]/+page.svelte`: section moved above the
      More-with shelves; a single `METADATA_ELSEWHERE` list replaces the per-render-site exclusions
      so "shown exactly once" is auditable in one place; Overview rendered in the header; fold
      mechanically identical to `CompletenessPanel` (chevron, `rotate-180`, `max-height` +
      `motion-reduce`, `inert`, `aria-expanded`/`aria-controls`), with a field count as the
      always-visible summary
- [x] testing `testing-strategy` — **not required**: visual reorder and de-duplication with no new
      behavior surface, same call as the earlier media-detail reorder. Suites green: `npm run check`
      0 errors (14 pre-existing warnings, none in the touched file), 189 frontend tests
- [x] security `security-review` — **not required**: no auth, access or infrastructure change.
      Every gate on the touched elements is unchanged; the one visibility change (Overview now
      readable by visitors) is deliberate and recorded in the handoff

## Up next — ordered (position = priority)

1. [ ] [—] owner review of the PR — in particular the two judgment calls: Overview becoming
      visitor-visible, and the fold covering the field list only (actions and the extraction review
      panel stay outside it)
2. [ ] [—] decide whether the missing provider-poster precedence chooser deserves its own ticket
      (see "Deferred" below)

## Deferred

- **Provider-poster precedence chooser.** `poster_url` has `Display: "image_url"`, so its Metadata
  row rendered an `<img>` plus a read-only `ProvenanceBadge` — it never rendered `SourceBadge`.
  There is therefore no way anywhere in the UI to choose *which provider's* poster wins; owners can
  upload their own or take whatever wins by default. That was true before this change and is still
  true after it. Building the control was explicitly declined here rather than smuggled into a
  reorder ticket.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-05 · mockup review → implementation → handoff
- skills: simplify
- handoff: Mockup-first, per the owner's standing preference. Four questions went back as cards
  before any code: which section to move (Metadata), what to do about "Description", what to do
  about Poster, and how the fold should behave. **One of those questions was wrong and had to be
  corrected mid-review** — the Poster card claimed the row carried a `SourceBadge` precedence
  chooser that would be lost. It does not; `Display: "image_url"` takes an earlier branch. Re-asked
  with the correction, and the owner chose plain removal over building a chooser that never existed.
  Overview, by contrast, genuinely had nowhere else to go, so it moved to the header rather than
  being dropped.

  Implementation is one file. The move is a block relocation; the trim is a single
  `METADATA_ELSEWHERE` list (which absorbed the pre-existing `studio`/`title` exclusions, so the
  rule now lives in one place instead of three); the fold is a copy of `CompletenessPanel`'s
  mechanism down to the `inert` attribute and the `motion-reduce` opt-out.

  **The non-obvious cleanup:** `CompletenessQueueRow` deep-links to `#field-<canonical>`, so
  removing the Genres and Actors rows orphaned two live targets. Those ids moved onto the Tags
  section and the People grid wrapper, and the hidden-anchor fallback now skips them so no id is
  duplicated. `/simplify` then folded the old studio/title exclusion comment into the new one — the
  change had made them redundant — and compacted the list to a single line.

  Verified live against the films testbed at `/media/208`: order reads
  `Tags > People > Metadata > More with adventure > Manage > File > Completeness`; the field list is
  down to eight rows; genres show in Tags, confirming the duplication the removal resolves; Overview
  renders in the header ahead of Tags; the fold starts collapsed and the toggle flips
  `aria-expanded`, `inert`, `max-height` and `rotate-180` together. Screenshots time out on this app
  and the pane stopped painting, so the fold was verified through DOM state rather than pixels — the
  shipped Completeness panel measures identically "broken" through `getComputedStyle`, which is how
  the harness was ruled the cause. Three-skin QA was not run as a measurement pass because the
  change introduces no new colors or tokens: every class added is a verbatim reuse of one already on
  this page or in `CompletenessPanel`.
  **Next session:** open for review — all gates green.
