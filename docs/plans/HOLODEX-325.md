---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-325                 # the tracker key; must match the branch key regex
status: in-progress                  # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Long Overview/bio/description text on the media, person, and film pages now clamps to a few lines with a "show more" chevron instead of running on unclamped and pushing the rest of the page down.
---

# HOLODEX-325 · Extract ExpandableText shared component

Follow-up to [HOLODEX-320](../design/media-detail-metadata-fold-handoff.md): Overview on the
media detail page renders unclamped — a ~1500-char synopsis pushes Studio and everything below it
far down the page. Explored via `/design-critique` with interactive mockups (grounded in the real
component source and skin tokens) before any code, landing on a clamp-in-place chevron toggle
mechanically identical to `CompletenessPanel`'s.

**Design package:** [docs/design/expandable-text-handoff.md](../design/expandable-text-handoff.md)
\+ [mockup SVG](../design/expandable-text-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` — **not required**: no new functionality, an existing field's display
      treatment
- [x] architecture `architecture` — **not required**: no new seam; purely a frontend presentation
      component, no change to the resolver/decision model
- [x] design `design-handoff` → `docs/design/expandable-text-handoff.md` with a committed SVG
      mockup (collapsed/expanded states), the props table, and the explicit v1 scope boundary
- [x] frontend → new shared component `web/src/lib/components/shared/ExpandableText.svelte`
      (clamp + `CompletenessPanel`-style chevron, `$props.id()`-linked `aria-controls`), wired
      into three plain-text call sites: Media Overview (visitor-view), Person bio, Film
      description header
- [x] testing `testing-strategy` — **not required**: presentational-only change, same call as
      HOLODEX-320. `npm run check` 0 errors, no new warnings
- [x] security `security-review` — **not required**: no auth/access/infrastructure change

## Up next — ordered (position = priority)

1. [ ] [—] owner review of the PR
2. [ ] [—] decide whether to fold `SourceBadge`'s inline value display (owner-view long text on
      Media Overview + Film Details) into `ExpandableText` — see "Deferred" below

## Deferred

- **`SourceBadge` inline clamp.** `SourceBadge.svelte` renders a replace field's resolved value
  inline for owners (`<span>{value}</span>` + provenance chip) — Media's owner-view Overview and
  Film's Details section both go through it, not the plain-`<p>` branch this change touches. An
  owner still sees the full, unclamped value there. Explicitly scoped out this session (per
  owner's choice between "plain-text sites only" vs. "include SourceBadge now") to keep the diff
  small; a `clampLines` option on `SourceBadge` is the natural follow-up.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-06 · design-critique mockups → scoping → implementation → handoff
- skills: design-critique (`/design:design-critique`), simplify
- handoff: Started from a design critique of the awkward long-Overview layout — three mockup
  passes (initial 3-option comparison, then a revised Option B per owner feedback: grey text,
  full width matching the video element, chevron instead of a text button), then an explicit
  scoping pass (`Explore` subagent surveyed all three candidate call sites — Media, Person, Film
  — plus `SourceBadge`'s competing inline-value path) before writing any code. Two decisions were
  put back to the owner as cards rather than assumed: whether v1 should also cover `SourceBadge`
  (chose plain-text-only), and whether to implement immediately (chose yes).

  Implementation is one new component + three call-site swaps. `/simplify`'s four parallel
  reviewers (reuse/simplification/efficiency/altitude) caught three real issues, all fixed: the
  chevron button was missing `aria-controls` (an incomplete port of `CompletenessPanel`'s
  pattern — added a `$props.id()`-generated id), a no-op `w-full` class, and a `lines?: number`
  prop that silently rendered fully-unclamped text for any value outside four hardcoded
  `class:` branches (narrowed the type to `4 | 5`, the only two values any call site uses, and
  collapsed the four branches into a one-line lookup). Altitude review also flagged the
  `text-muted` color and dropped `max-w-prose` as unrequested deviations — both were correct
  reads of the diff in isolation, but both were explicit owner requests from the mockup-revision
  round, not accidental; recorded as no-change-needed rather than reverted.

  **Verification gotcha:** the local films testbed's dev DB was stuck mid-migration (dirty at
  version 35) from an unrelated prior session — cleared by confirming migration 35's tables
  already existed (safe to un-dirty) rather than reseeding. Even after that, no fixture video/
  person/film had long enough text, and TMDB-sourced Overview candidates require the owner
  adoption step (ADR-090) before they resolve into visitor-visible fields — inserting synthetic
  `entity_enrichment`/`field_source_decisions` rows didn't take effect until a server restart
  (in-memory cache). Given that friction was entirely about test-fixture setup and not the
  component itself, verification switched to a throwaway `/dev-test-expandable` route rendering
  `ExpandableText` directly against the real Tailwind build — confirmed the clamp, the toggle,
  `aria-controls` linkage, the chevron's `rotate` (Tailwind v4 uses the CSS `rotate` property,
  not `transform`), and token correctness across all three skins via computed style. Route
  deleted before commit.
  **Next session:** open for review — all gates green.
