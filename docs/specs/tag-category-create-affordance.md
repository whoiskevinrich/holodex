# Spec: Tag & Category Create Affordance — closing the /tags creation gap

**Status**: Draft
**Story**: [HOLODEX-243](https://whoiskevinrich.atlassian.net/browse/HOLODEX-243)
**Epic**: [HOLODEX-240](https://whoiskevinrich.atlassian.net/browse/HOLODEX-240) (Tag Categories)
**Owner**: Project owner
**Date**: 2026-07-31

**Depends on**:
- `web/src/routes/tags/+page.svelte` — the unified tag/category pill grid, type filter, and search
  input shipped by the Tag Categories epic (`docs/specs/tag-categories.md`, HOLODEX-240). This
  spec adds one new pill to that existing grid; it does not restructure the page.
- `CategoryPicker.svelte`'s inline "+ Create "query"" row — the existing create-pattern this spec
  promotes from a search-box fallback to a standing, always-visible affordance.
- `POST /tags` (`createOrResolveTag`, `internal/api/categories.go`) and `POST /categories`
  (`createCategory`, same file, owner-gated) — both already implemented and unchanged by this
  spec.

**New ADR required**: No — this is a UI-only addition on an already-decided data model (ADR-078)
and already-shipped identity/creation semantics; no new architectural decision is introduced.

---

## Problem Statement

`/tags`, `/categories/{id}`, and the owner tag-admin page collectively expose rename, alias,
parent, merge, and category-assign controls — every one of them requires an *existing* tag or
category to act on. There is no button, pill, or menu item anywhere in the app that starts a
brand-new one. Today a tag is only created as the side effect of tagging a video (media page's
`+ Add tag` form), or a category only as the side effect of `CategoryPicker`'s inline "+ Create
"query"" row, which only appears when a category search comes up empty inside the assign flow.
This blocks the owner from pre-building a taxonomy (e.g. setting up categories before any tags
exist to put in them) and makes tag creation a happy accident of tagging media rather than a
deliberate action.

## Goals

1. The owner can create a new, empty tag directly from `/tags`, without first attaching it to a
   video.
2. The owner can create a new, empty category directly from `/tags`, without first assigning a
   tag to it.
3. The create affordance is discoverable without scrolling or already knowing about the
   category-assign picker's fallback row.
4. The empty state on `/tags` (today a dead-end message) leads directly into creating the first
   tag or category.

## Non-Goals

- **Any backend change.** `POST /tags` and `POST /categories` already implement resolve-or-create
  and owner-gated create respectively; this spec is UI-only.
- **A new create pattern.** This reuses the existing inline name-input + accent-submit +
  ghost-cancel form shape already shipped for rename/alias/set-parent, and the existing dashed
  "+ Create" pill shape already shipped in `CategoryPicker`. No new component family.
- **Bulk/import creation** (e.g. pasting a list of tag names to create many at once). Single-item
  creation only, matching every other identity action on this page.
- **Removing or changing `CategoryPicker`'s existing inline "+ Create" row.** That row stays as
  the in-context fallback for the assign flow; this spec adds a second, standing entry point, it
  doesn't replace the first.
- **Non-owner access.** Creation stays owner-gated, consistent with every other mutation on this
  page (Manage tags, category rename/delete).

## User Stories

- As the owner, I want to create a new tag directly from `/tags`, so I can build out a taxonomy
  before any video is tagged with it.
- As the owner, I want to create a new category directly from `/tags`, so I don't have to
  discover it accidentally via a category-search fallback while assigning tags.
- As the owner, I want the same near-miss disambiguation I get when adding a tag from the media
  page (a nudge if the name I typed looks like an existing tag/category), so creating from `/tags`
  doesn't produce accidental near-duplicates.
- As a first-time owner with zero tags, I want the empty state to tell me how to create one, so
  the page isn't a dead end before any media has been tagged.

## Requirements

### P0 — Must-Have

**Standing create pill on `/tags`**
- A dashed-border pill, styled like the existing rounded-full tag/category pills but with a
  dashed rather than solid border and a `+` glyph, appears as the **first** item in the pill grid
  (before any tag/category pills), owner-only.
- [ ] The pill is visible regardless of the `All / Tags / Categories` type filter or an active
      search query (it does not get filtered out — it's a control, not a result).
- [ ] Non-owners never see the pill (parity with "Manage tags" and category rename/delete).

**Inline create form**
- Clicking the pill expands it in place into the same inline form shape used by rename/alias/set
  parent: a text input, an accent submit button, a ghost cancel button, and a `text-warn` error
  slot — not a modal or a separate route.
- A small type toggle (Tag / Category) sits above the input, defaulting to Tag. Submitting calls
  `POST /tags` (tag) or `POST /categories` (category) depending on the toggle.
- [ ] Submitting a valid, non-colliding name creates the tag/category and it appears in the grid
      immediately (optimistic or refetch, consistent with how rename already updates the grid).
- [ ] Submitting a name that exact-matches an existing tag or category (case-insensitive fold,
      same as the existing collision check) surfaces the same "already exists" conflict messaging
      used elsewhere, rather than erroring silently.
- [ ] Cancel collapses the pill back to its dashed resting state without creating anything.

**Near-miss disambiguation**
- After a successful create, run the same non-blocking near-miss check the media page's
  `+ Add tag` flow already performs (`api.nearMiss('tag', ...)`), showing the existing "Looks a
  lot like X — use that instead?" card with "Use existing" / "Add as new anyway" if a close match
  exists. Categories, which don't have a near-miss/fuzzy-match system (per HOLODEX-240 Non-Goals),
  skip this step — exact-match collision handling above is sufficient for them.
- [ ] Creating a tag with a name that near-miss-matches an existing tag surfaces the same
      disambiguation card already used on the media page, reusing its copy and actions verbatim.

**Empty-state wiring**
- The existing "No tags indexed yet." / "No categories yet." / "No tags or categories indexed
  yet." empty-state message becomes an invitation that points at the create pill, rather than a
  bare status line.
- [ ] Landing on `/tags` with zero tags and zero categories (owner view) shows copy inviting
      creation, not just an absence statement.
- [ ] Non-owner empty state is unchanged (no invitation to create, since they can't).

### P1 — Nice-to-Have

- Keyboard shortcut or auto-focus: pressing a key (e.g. `/` is already likely reserved for search
  focus — confirm no collision) while the grid is focused opens the create pill directly.

### P2 — Future Considerations

- Bulk/paste-list creation (see Non-Goals) if taxonomy pre-building at scale becomes a real
  workflow.

## Success Metrics

Single-owner, self-hosted app — the practical bar:
- The owner can create a first tag or category from a completely empty `/tags` page without
  visiting the media page or discovering the `CategoryPicker` fallback first.
- Creating from `/tags` and creating implicitly via the media page's `+ Add tag` form produce
  identical resulting state (same collision rules, same near-miss behavior) — no divergent code
  path for "the same tag created two different ways."
- Zero regressions to the existing rename/alias/merge/category-assign flows on `/tags` — the new
  pill is additive to the grid, not a restructuring of it.

## Open Questions

- **You**: should the type toggle default remember the owner's last choice (e.g. if they just
  created three categories in a row, default to Category next time), or always reset to Tag?
  Assumed: always reset to Tag, since it's the more common case and a surprising sticky default
  is worse than one extra click. Flagging as easily revisited.
- **Design**: exact copy for the empty-state invitation — deferred to implementation, low risk.

## Timeline Considerations

No hard deadline. Depends only on the already-merged Tag Categories epic (HOLODEX-240); no other
in-flight work blocks this.

## Implementation note (2026-08-01)

The "Any backend change" Non-Goal above turned out to be wrong in one narrow respect, discovered
during implementation rather than design: `ListTags` (`internal/repo/repo.go`, shared with
`ListStudios` via `namedCountQuery`) inner-joined `video_tags`, so a tag created bare via `POST
/tags` — zero videos, by construction — never appeared in `GET /tags`, silently violating P0 goal
#1 ("appears in the grid immediately"). Fixed with a scoped change: `namedCountQuery` gained an
`includeZero` parameter (left join instead of inner join); `ListTags` passes `true`, `ListStudios`
keeps `false` (studios have no empty-creation path, so its existing behavior is untouched). Covered
by `TestResolveOrCreateTag` (repo) and `TestResolveOrCreateTagEndpoint` (api), both updated from
asserting the old exclusion to asserting the new inclusion. See `docs/testing-strategy.md` §4/§9.
